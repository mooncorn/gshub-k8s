package reconciler

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/mooncorn/gshub/api/internal/database"
	"github.com/mooncorn/gshub/api/internal/models"
	"github.com/mooncorn/gshub/api/internal/services/k8s"
	"github.com/mooncorn/gshub/api/internal/services/portalloc"
	"go.uber.org/zap"
)

// ServerReconciler reconciles pending servers by creating K8s resources
type ServerReconciler struct {
	db                 *database.DB
	k8sClient          *k8s.Client
	portAllocService   *portalloc.Service
	logger             *zap.Logger
	done               chan struct{}
	ticker             *time.Ticker
	reconcileTicket    time.Duration
	k8sNamespace       string
	k8sGameCatalogName string
}

// NewServerReconciler creates a new reconciler
func NewServerReconciler(db *database.DB, k8sClient *k8s.Client, portAllocService *portalloc.Service, logger *zap.Logger, k8sNamespace, k8sGameCatalogName string) *ServerReconciler {
	return &ServerReconciler{
		db:                 db,
		k8sClient:          k8sClient,
		portAllocService:   portAllocService,
		logger:             logger,
		done:               make(chan struct{}),
		reconcileTicket:    15 * time.Second, // Run every 15 seconds
		k8sNamespace:       k8sNamespace,
		k8sGameCatalogName: k8sGameCatalogName,
	}
}

// Start begins the background reconciliation loop
func (r *ServerReconciler) Start(ctx context.Context) {
	r.ticker = time.NewTicker(r.reconcileTicket)
	go r.loop(ctx)
	r.logger.Info("Server reconciler started", zap.Duration("interval", r.reconcileTicket))
}

// Stop gracefully stops the reconciliation loop
func (r *ServerReconciler) Stop() {
	if r.ticker != nil {
		r.ticker.Stop()
	}
	close(r.done)
	r.logger.Info("Server reconciler stopped")
}

// loop runs the reconciliation loop
func (r *ServerReconciler) loop(ctx context.Context) {
	for {
		select {
		case <-r.done:
			return
		case <-r.ticker.C:
			r.reconcile(ctx)
		}
	}
}

// reconcile processes servers in transitional states
func (r *ServerReconciler) reconcile(ctx context.Context) {
	startTime := time.Now()

	// Note: State detection (starting->running, stopping->stopped) is now handled by the
	// supervisor reporting status via the internal API in real-time. The reconciler only handles:
	// 1. Creating K8s resources for pending servers
	// 2. Timeout detection for stuck servers
	// 3. Heartbeat timeout detection for unresponsive servers

	// 1. Handle startup timeouts - mark servers as failed if stuck in "starting"
	r.reconcileStartupTimeouts(ctx)

	// 2. Handle "pending" servers - create K8s resources
	r.reconcilePendingServers(ctx)

	// 3. Handle heartbeat timeouts - mark running servers as failed if unresponsive
	r.reconcileHeartbeatTimeouts(ctx)

	r.logger.Debug("reconciliation cycle complete", zap.Duration("duration", time.Since(startTime)))
}

// reconcileStartupTimeouts handles servers stuck in "starting" state for too long
func (r *ServerReconciler) reconcileStartupTimeouts(ctx context.Context) {
	servers, err := r.db.GetServersByStatus(ctx, string(models.ServerStatusStarting))
	if err != nil {
		r.logger.Error("failed to get starting servers", zap.Error(err))
		return
	}

	for _, server := range servers {
		serverID := server.ID.String()

		// Check timeout (5 minutes)
		if time.Since(server.UpdatedAt) > 5*time.Minute {
			r.db.TransitionServerStatus(ctx, serverID,
				models.ServerStatusStarting, models.ServerStatusFailed,
				"Timeout waiting for pod to be ready")
			r.logger.Warn("server startup timed out", zap.String("server_id", serverID))
		}
	}
}

// reconcilePendingServers handles servers in "pending" state - creates K8s resources
func (r *ServerReconciler) reconcilePendingServers(ctx context.Context) {
	pendingServers, err := r.db.GetServersByStatus(ctx, string(models.ServerStatusPending))
	if err != nil {
		r.logger.Error("failed to get pending servers", zap.Error(err))
		return
	}

	if len(pendingServers) == 0 {
		return
	}

	r.logger.Debug("reconciling pending servers", zap.Int("count", len(pendingServers)))

	// Load game catalog once
	catalog, err := r.k8sClient.LoadGameCatalog(ctx, r.k8sNamespace, r.k8sGameCatalogName)
	if err != nil {
		r.logger.Error("failed to load game catalog", zap.Error(err))
		return
	}

	// Reconcile each pending server
	successCount := 0
	failureCount := 0

	for _, server := range pendingServers {
		if err := r.reconcileServer(ctx, &server, catalog); err != nil {
			r.logger.Error("failed to reconcile server",
				zap.String("server_id", server.ID.String()),
				zap.Error(err))
			failureCount++
		} else {
			successCount++
		}
	}

	if successCount > 0 || failureCount > 0 {
		r.logger.Info("pending servers reconciled",
			zap.Int("processed", len(pendingServers)),
			zap.Int("succeeded", successCount),
			zap.Int("failed", failureCount))
	}
}

// reconcileHeartbeatTimeouts handles servers that have stopped sending heartbeats
func (r *ServerReconciler) reconcileHeartbeatTimeouts(ctx context.Context) {
	const heartbeatTimeoutMinutes = 2 // 4 missed heartbeats (30s interval)

	// Get running servers without recent heartbeat
	servers, err := r.db.GetServersWithoutRecentHeartbeat(ctx, models.ServerStatusRunning, heartbeatTimeoutMinutes)
	if err != nil {
		r.logger.Error("failed to get servers without heartbeat", zap.Error(err))
		return
	}

	for _, server := range servers {
		serverID := server.ID.String()

		// Skip servers that just started (give time for first heartbeat)
		// Use UpdatedAt as a proxy for when the server became "running"
		if time.Since(server.UpdatedAt) < 3*time.Minute {
			continue
		}

		r.logger.Warn("heartbeat timeout detected",
			zap.String("server_id", serverID),
			zap.Timep("last_heartbeat", server.LastHeartbeat),
			zap.Time("updated_at", server.UpdatedAt))

		// Check if deployment still exists
		deployName := fmt.Sprintf("server-%s", serverID)
		exists, err := r.k8sClient.DeploymentExists(ctx, r.k8sNamespace, deployName)
		if err != nil {
			r.logger.Error("failed to check deployment existence",
				zap.Error(err),
				zap.String("server_id", serverID))
			continue
		}

		if !exists {
			// Deployment gone but DB says running - update status
			r.db.TransitionServerStatus(ctx, serverID,
				models.ServerStatusRunning, models.ServerStatusFailed,
				"Server stopped unexpectedly (deployment not found)")
			r.logger.Warn("server deployment not found, marking failed", zap.String("server_id", serverID))
			continue
		}

		// Deployment exists but supervisor not responding - mark as failed
		transitioned, _ := r.db.TransitionServerStatus(ctx, serverID,
			models.ServerStatusRunning, models.ServerStatusFailed,
			"Server unresponsive (heartbeat timeout). Click Start to restart.")

		if transitioned {
			r.logger.Warn("server marked failed due to heartbeat timeout", zap.String("server_id", serverID))
		}
	}
}

// reconcileServer processes a single pending server
func (r *ServerReconciler) reconcileServer(ctx context.Context, server *models.Server, catalog *k8s.GameCatalog) error {
	serverID := server.ID.String()

	// Get game configuration
	gameConfig, err := catalog.GetGameConfig(string(server.Game))
	if err != nil {
		errMsg := fmt.Sprintf("invalid game config: %v", err)
		r.logger.Warn("marking server as failed", zap.String("server_id", serverID), zap.String("reason", errMsg))
		return r.db.MarkServerFailed(ctx, serverID, errMsg)
	}

	// Get plan configuration
	planConfig, err := gameConfig.GetPlanConfig(string(server.Plan))
	if err != nil {
		errMsg := fmt.Sprintf("invalid plan config: %v", err)
		r.logger.Warn("marking server as failed", zap.String("server_id", serverID), zap.String("reason", errMsg))
		return r.db.MarkServerFailed(ctx, serverID, errMsg)
	}

	// Calculate supervisor overhead
	supervisorCPU := 50   // 50m default
	supervisorMem := int64(64 * 1024 * 1024) // 64Mi default
	if gameConfig.SupervisorOverhead != nil {
		if gameConfig.SupervisorOverhead.CPU != "" {
			supervisorCPU = parseCPUToMillicores(gameConfig.SupervisorOverhead.CPU)
		}
		if gameConfig.SupervisorOverhead.Memory != "" {
			supervisorMem = parseMemoryToBytes(gameConfig.SupervisorOverhead.Memory)
		}
	}

	// STEP 1: Allocate ports (if not already allocated)
	allocations, err := r.portAllocService.GetServerPorts(ctx, server.ID)
	if err != nil {
		r.logger.Error("failed to check port allocations", zap.String("server_id", serverID), zap.Error(err))
		return r.db.UpdateServerLastReconciled(ctx, serverID)
	}

	if len(allocations) == 0 {
		// Need to allocate ports - build requirements from game config
		portReqs := make([]portalloc.PortRequirement, len(gameConfig.Ports))
		for i, p := range gameConfig.Ports {
			portReqs[i] = portalloc.PortRequirement{
				Name:     p.Name,
				Protocol: p.Protocol,
			}
		}

		// Build resource requirements from plan config + supervisor overhead
		cpuMillicores := parseCPUToMillicores(planConfig.CPU) + supervisorCPU
		memBytes := parseMemoryToBytes(planConfig.Memory) + supervisorMem

		resourceReq := &portalloc.ResourceRequirement{
			CPUMillicores: cpuMillicores,
			MemoryBytes:   memBytes,
		}

		allocations, err = r.portAllocService.AllocatePorts(ctx, server.ID, portReqs, resourceReq)
		if err != nil {
			errMsg := fmt.Sprintf("no capacity available: %v", err)
			r.logger.Warn("marking server as failed - no capacity", zap.String("server_id", serverID))
			return r.db.MarkServerFailed(ctx, serverID, errMsg)
		}

		r.logger.Info("allocated ports and resources for server",
			zap.String("server_id", serverID),
			zap.String("node", allocations[0].NodeName),
			zap.Int("port_count", len(allocations)),
			zap.Int("cpu_millicores", cpuMillicores),
			zap.Int64("memory_bytes", memBytes))
	}

	// STEP 2: Create PVC if it doesn't exist
	pvcName := fmt.Sprintf("server-%s", serverID)
	labels := map[string]string{
		"server":           serverID,
		"game":             string(server.Game),
		"app":              "game-server",
	}

	err = r.k8sClient.CreatePVC(ctx, r.k8sNamespace, pvcName, planConfig.Storage, labels)
	if err != nil && !isAlreadyExistsError(err) {
		r.logger.Error("failed to create PVC", zap.String("server_id", serverID), zap.Error(err))
		return r.db.UpdateServerLastReconciled(ctx, serverID)
	}

	// STEP 3: Generate auth token for supervisor
	authToken, err := generateAuthToken()
	if err != nil {
		r.logger.Error("failed to generate auth token", zap.String("server_id", serverID), zap.Error(err))
		return r.db.UpdateServerLastReconciled(ctx, serverID)
	}
	if err := r.db.SetServerAuthToken(ctx, serverID, authToken); err != nil {
		r.logger.Error("failed to save auth token", zap.String("server_id", serverID), zap.Error(err))
		return r.db.UpdateServerLastReconciled(ctx, serverID)
	}

	// STEP 4: Create Deployment with supervisor
	deployName := fmt.Sprintf("server-%s", serverID)
	nodeName := allocations[0].NodeName

	// Build static port configs from allocations
	staticPorts := make([]k8s.StaticPortConfig, len(allocations))
	for i, alloc := range allocations {
		// Find the container port from game config
		var containerPort int32
		for _, p := range gameConfig.Ports {
			if p.Name == alloc.PortName {
				containerPort = p.Port
				break
			}
		}
		staticPorts[i] = k8s.StaticPortConfig{
			Name:          alloc.PortName,
			ContainerPort: containerPort,
			HostPort:      int32(alloc.Port),
			Protocol:      corev1.Protocol(alloc.Protocol),
		}
	}

	// Convert volume configs
	var volumes []k8s.VolumeConfig
	for _, vol := range gameConfig.Volumes {
		volumes = append(volumes, k8s.VolumeConfig{
			Name:      vol.Name,
			MountPath: vol.MountPath,
			SubPath:   vol.SubPath,
		})
	}

	// Compute effective env (merge game defaults, plan defaults, and user overrides)
	effectiveEnv := k8s.MergeEnvVars(gameConfig.Env, planConfig.Env, server.EnvOverrides)

	// Add supervisor environment variables
	effectiveEnv["GSHUB_SERVER_ID"] = serverID
	effectiveEnv["GSHUB_API_ENDPOINT"] = fmt.Sprintf("http://api.%s.svc:8081", r.k8sNamespace)
	effectiveEnv["GSHUB_AUTH_TOKEN"] = authToken

	// Add process configuration for supervisor
	if gameConfig.Process != nil {
		if len(gameConfig.Process.StartCommand) > 0 {
			cmdJSON, _ := json.Marshal(gameConfig.Process.StartCommand)
			effectiveEnv["GSHUB_START_COMMAND"] = string(cmdJSON)
		}
		if gameConfig.Process.WorkDir != "" {
			effectiveEnv["GSHUB_WORK_DIR"] = gameConfig.Process.WorkDir
		}
		if gameConfig.Process.GracePeriod > 0 {
			effectiveEnv["GSHUB_GRACE_PERIOD"] = fmt.Sprintf("%d", gameConfig.Process.GracePeriod)
		}
	}

	// Add health check configuration for supervisor
	if gameConfig.HealthCheck != nil {
		effectiveEnv["GSHUB_HEALTH_TYPE"] = gameConfig.HealthCheck.Type
		effectiveEnv["GSHUB_HEALTH_PORT"] = gameConfig.HealthCheck.Port
		effectiveEnv["GSHUB_HEALTH_PROTOCOL"] = gameConfig.HealthCheck.Protocol
		if gameConfig.HealthCheck.InitialDelay != "" {
			effectiveEnv["GSHUB_HEALTH_INITIAL_DELAY"] = gameConfig.HealthCheck.InitialDelay
		}
		if gameConfig.HealthCheck.Timeout != "" {
			effectiveEnv["GSHUB_HEALTH_TIMEOUT"] = gameConfig.HealthCheck.Timeout
		}
		if gameConfig.HealthCheck.Interval != "" {
			effectiveEnv["GSHUB_HEALTH_INTERVAL"] = gameConfig.HealthCheck.Interval
		}
		if gameConfig.HealthCheck.Pattern != "" {
			effectiveEnv["GSHUB_HEALTH_PATTERN"] = gameConfig.HealthCheck.Pattern
		}
	}

	// Determine image to use (prefer supervisorImage, fallback to legacy image)
	image := gameConfig.SupervisorImage
	if image == "" {
		image = gameConfig.Image
	}

	// Calculate total resources (plan + supervisor overhead)
	totalCPU := fmt.Sprintf("%dm", parseCPUToMillicores(planConfig.CPU)+supervisorCPU)
	totalMemBytes := parseMemoryToBytes(planConfig.Memory) + supervisorMem
	totalMem := fmt.Sprintf("%d", totalMemBytes)

	// Get grace period
	gracePeriod := int32(30)
	if gameConfig.Process != nil && gameConfig.Process.GracePeriod > 0 {
		gracePeriod = int32(gameConfig.Process.GracePeriod)
	}

	err = r.k8sClient.CreateGameDeployment(ctx, k8s.DeploymentParams{
		Namespace:   r.k8sNamespace,
		Name:        deployName,
		Image:       image,
		NodeName:    nodeName,
		Ports:       staticPorts,
		Volumes:     volumes,
		Env:         effectiveEnv,
		CPURequest:  totalCPU,
		MemRequest:  totalMem,
		PVCName:     pvcName,
		Labels:      labels,
		GracePeriod: gracePeriod,
	})
	if err != nil && !isAlreadyExistsError(err) {
		r.logger.Error("failed to create Deployment", zap.String("server_id", serverID), zap.Error(err))
		return r.db.UpdateServerLastReconciled(ctx, serverID)
	}

	// STEP 5: Transition to "starting" - supervisor will report status via internal API
	transitioned, err := r.db.TransitionServerStatus(ctx, serverID,
		models.ServerStatusPending, models.ServerStatusStarting, "Creating game server...")
	if err != nil {
		r.logger.Error("failed to transition to starting", zap.String("server_id", serverID), zap.Error(err))
		return err
	}
	if !transitioned {
		// Status changed (maybe to stopping/expired) - don't continue
		r.logger.Debug("server status changed, skipping", zap.String("server_id", serverID))
		return nil
	}

	r.logger.Info("server transitioning to starting",
		zap.String("server_id", serverID),
		zap.String("node", nodeName),
		zap.Int("port_count", len(allocations)))

	return nil
}

// generateAuthToken creates a secure random token for supervisor authentication
func generateAuthToken() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// isAlreadyExistsError checks if an error is due to a resource already existing
func isAlreadyExistsError(err error) bool {
	return errors.IsAlreadyExists(err)
}

// parseCPUToMillicores converts a CPU string (e.g., "1", "500m", "2") to millicores
func parseCPUToMillicores(cpu string) int {
	q := resource.MustParse(cpu)
	return int(q.MilliValue())
}

// parseMemoryToBytes converts a memory string (e.g., "2Gi", "512Mi") to bytes
func parseMemoryToBytes(memory string) int64 {
	q := resource.MustParse(memory)
	return q.Value()
}

