package reconciler

import (
	"context"
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

	// 1. Handle "stopping" servers - confirm K8s delete completed
	r.reconcileStoppingServers(ctx)

	// 2. Handle "starting" servers - check if pod is ready
	r.reconcileStartingServers(ctx)

	// 3. Handle "pending" servers - create K8s resources
	r.reconcilePendingServers(ctx)

	r.logger.Debug("reconciliation cycle complete", zap.Duration("duration", time.Since(startTime)))
}

// reconcileStoppingServers handles servers in "stopping" state - waits for K8s deletion
func (r *ServerReconciler) reconcileStoppingServers(ctx context.Context) {
	servers, err := r.db.GetServersByStatus(ctx, string(models.ServerStatusStopping))
	if err != nil {
		r.logger.Error("failed to get stopping servers", zap.Error(err))
		return
	}

	for _, server := range servers {
		serverID := server.ID.String()
		gsName := fmt.Sprintf("server-%s", serverID)

		gs, err := r.k8sClient.GetGameServer(ctx, r.k8sNamespace, gsName)
		if err != nil && errors.IsNotFound(err) {
			// GameServer deleted - complete transition to stopped
			if ok, _ := r.db.TransitionServerStatus(ctx, serverID,
				models.ServerStatusStopping, models.ServerStatusStopped, ""); ok {
				r.logger.Info("server stopped", zap.String("server_id", serverID))
				if err := r.db.MarkServerStopped(ctx, serverID); err != nil {
					r.logger.Error("failed to set stopped_at timestamp", zap.String("server_id", serverID), zap.Error(err))
				}
			}
			continue
		}

		// Check if terminating (DeletionTimestamp set)
		if gs != nil && gs.ObjectMeta.DeletionTimestamp != nil {
			r.logger.Debug("GameServer terminating, waiting...", zap.String("server_id", serverID))
			continue
		}

		// GameServer exists but not terminating - re-issue delete
		if gs != nil {
			r.logger.Warn("re-issuing GameServer delete", zap.String("server_id", serverID))
			_ = r.k8sClient.DeleteGameServer(ctx, r.k8sNamespace, gsName)
		}
	}
}

// reconcileStartingServers handles servers in "starting" state - waits for pod to be Ready
func (r *ServerReconciler) reconcileStartingServers(ctx context.Context) {
	servers, err := r.db.GetServersByStatus(ctx, string(models.ServerStatusStarting))
	if err != nil {
		r.logger.Error("failed to get starting servers", zap.Error(err))
		return
	}

	for _, server := range servers {
		serverID := server.ID.String()
		gsName := fmt.Sprintf("server-%s", serverID)

		// Check timeout (5 minutes)
		if time.Since(server.UpdatedAt) > 5*time.Minute {
			r.db.TransitionServerStatus(ctx, serverID,
				models.ServerStatusStarting, models.ServerStatusFailed,
				"Timeout waiting for pod to be ready")
			r.logger.Warn("server startup timed out", zap.String("server_id", serverID))
			continue
		}

		gs, err := r.k8sClient.GetGameServer(ctx, r.k8sNamespace, gsName)
		if err != nil {
			if !errors.IsNotFound(err) {
				r.logger.Error("failed to get GameServer", zap.String("server_id", serverID), zap.Error(err))
			}
			continue
		}

		// Check if terminating (subscription may have been deleted)
		if gs.ObjectMeta.DeletionTimestamp != nil {
			r.logger.Debug("GameServer terminating during startup", zap.String("server_id", serverID))
			continue
		}

		if gs.Status.State == "Ready" {
			// Get node IP from allocations
			allocations, err := r.portAllocService.GetServerPorts(ctx, server.ID)
			nodeIP := ""
			if err == nil && len(allocations) > 0 {
				nodeIP = allocations[0].NodeIP
			}

			if ok, _ := r.db.TransitionServerStatus(ctx, serverID,
				models.ServerStatusStarting, models.ServerStatusRunning, ""); ok {
				if nodeIP != "" {
					r.db.UpdateServerNodeIP(ctx, serverID, nodeIP)
				}
				if gs.Status.Address != "" {
					r.db.UpdateServerPodIP(ctx, serverID, gs.Status.Address)
				}
				r.logger.Info("server running", zap.String("server_id", serverID))
			}
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

		// Build resource requirements from plan config
		// Add sidecar overhead: 100m CPU + 128Mi memory
		cpuMillicores := parseCPUToMillicores(planConfig.CPU) + 100
		memBytes := parseMemoryToBytes(planConfig.Memory) + 128*1024*1024

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
	labels := map[string]string{"server": serverID, "game": string(server.Game)}

	err = r.k8sClient.CreatePVC(ctx, r.k8sNamespace, pvcName, planConfig.Storage, labels)
	if err != nil && !isAlreadyExistsError(err) {
		r.logger.Error("failed to create PVC", zap.String("server_id", serverID), zap.Error(err))
		return r.db.UpdateServerLastReconciled(ctx, serverID)
	}

	// STEP 3: Create GameServer with static ports pinned to the allocated node
	gsName := fmt.Sprintf("server-%s", serverID)
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

	err = r.k8sClient.CreateGameServerWithStaticPorts(ctx, r.k8sNamespace, gsName, gameConfig.Image,
		nodeName, staticPorts, volumes, gameConfig.Env, planConfig.CPU, planConfig.Memory,
		pvcName, labels, gameConfig.HealthCheck)
	if err != nil && !isAlreadyExistsError(err) {
		r.logger.Error("failed to create GameServer", zap.String("server_id", serverID), zap.Error(err))
		return r.db.UpdateServerLastReconciled(ctx, serverID)
	}

	// STEP 4: Transition to "starting" - reconcileStartingServers will handle the Ready check
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
