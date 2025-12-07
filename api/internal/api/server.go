package api

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/mooncorn/gshub/api/config"
	"github.com/mooncorn/gshub/api/internal/api/middleware"
	"github.com/mooncorn/gshub/api/internal/database"
	"github.com/mooncorn/gshub/api/internal/models"
	"github.com/mooncorn/gshub/api/internal/services/broadcast"
	"github.com/mooncorn/gshub/api/internal/services/k8s"
	"github.com/mooncorn/gshub/api/internal/services/portalloc"
	stripeservice "github.com/mooncorn/gshub/api/internal/services/stripe"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

type ServerHandler struct {
	db               *database.DB
	k8sClient        *k8s.Client
	config           *config.Config
	stripeService    *stripeservice.Service
	portAllocService *portalloc.Service
	hub              *broadcast.Hub
}

func NewServerHandler(db *database.DB, k8sClient *k8s.Client, cfg *config.Config, stripeSvc *stripeservice.Service, portAllocSvc *portalloc.Service, hub *broadcast.Hub) *ServerHandler {
	return &ServerHandler{
		db:               db,
		k8sClient:        k8sClient,
		config:           cfg,
		stripeService:    stripeSvc,
		portAllocService: portAllocSvc,
		hub:              hub,
	}
}

// CheckoutResponse is the response for creating a checkout session
type CheckoutResponse struct {
	SessionID        string `json:"session_id"`
	CheckoutURL      string `json:"checkout_url"`
	PendingRequestID string `json:"pending_request_id"`
}

// CheckoutSuccessResponse is the response for confirming checkout
type CheckoutSuccessResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

// CreateCheckoutSession creates a Stripe checkout session for a server
func (h *ServerHandler) CreateCheckoutSession(c *gin.Context) {
	userIDStr := middleware.GetUserID(c)
	if userIDStr == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid user ID"})
		return
	}

	var req models.CreateServerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Check if subdomain already exists
	// TODO: Consider reserving subdomains for pending requests as well
	exists, err := h.db.SubdomainExists(c.Request.Context(), req.Subdomain)
	if err != nil {
		log.Printf("failed to check subdomain: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to check subdomain"})
		return
	}
	if exists {
		log.Printf("subdomain already taken: %s", req.Subdomain)
		c.JSON(http.StatusConflict, gin.H{"error": "subdomain already taken"})
		return
	}

	// Get price ID for game+plan combination
	priceID, err := h.config.GetPriceID(string(req.Game), string(req.Plan))
	if err != nil {
		log.Printf("invalid game or plan: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Create pending server request
	displayName := &req.DisplayName
	if req.DisplayName == "" {
		caser := cases.Title(language.English)
		output := caser.String(strings.ToLower(req.Game))
		defaultName := "My " + output + " Server"

		displayName = &defaultName
	}

	pendingRequestID, err := h.db.CreatePendingServerRequest(
		c.Request.Context(),
		userID,
		displayName,
		req.Subdomain,
		req.Game,
		req.Plan,
	)
	if err != nil {
		log.Printf("failed to create pending request: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create pending request"})
		return
	}

	// Get user email for Stripe
	user, err := h.db.GetUserByID(c.Request.Context(), userID)
	if err != nil {
		log.Printf("failed to get user email: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get user email"})
		return
	}

	// Create Stripe checkout session
	sessionID, checkoutURL, err := h.stripeService.CreateCheckoutSession(
		c.Request.Context(),
		userID,
		*pendingRequestID,
		priceID,
		user.Email,
	)
	if err != nil {
		log.Printf("failed to create checkout session: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create checkout session"})
		return
	}

	// Update pending request with session ID
	err = h.db.UpdatePendingServerRequestWithSession(c.Request.Context(), *pendingRequestID, sessionID)
	if err != nil {
		log.Printf("failed to update pending request: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update pending request"})
		return
	}

	c.JSON(http.StatusOK, CheckoutResponse{
		SessionID:        sessionID,
		CheckoutURL:      checkoutURL,
		PendingRequestID: pendingRequestID.String(),
	})
}

// ListServers returns all servers belonging to the current user
func (h *ServerHandler) ListServers(c *gin.Context) {
	userIDStr := middleware.GetUserID(c)
	if userIDStr == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid user ID"})
		return
	}

	servers, err := h.db.ListServersByUser(c.Request.Context(), userID)
	if err != nil {
		log.Printf("failed to list servers: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list servers"})
		return
	}

	if servers == nil {
		servers = []models.Server{}
	}

	c.JSON(http.StatusOK, models.ServerListResponse{
		Servers: servers,
		Total:   len(servers),
	})
}

// GetServer returns server details including K8s status for a specific server
func (h *ServerHandler) GetServer(c *gin.Context) {
	userIDStr := middleware.GetUserID(c)
	if userIDStr == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid user ID"})
		return
	}

	serverID := c.Param("id")
	if serverID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "server ID required"})
		return
	}

	// Get server with details from database
	server, err := h.db.GetServerByIDWithDetails(c.Request.Context(), serverID)
	if err != nil {
		log.Printf("failed to get server: %v", err)
		c.JSON(http.StatusNotFound, gin.H{"error": "server not found"})
		return
	}

	// Verify server belongs to user
	if server.UserID != userID {
		c.JSON(http.StatusNotFound, gin.H{"error": "server not found"})
		return
	}

	// Try to get K8s GameServer status if server is in an active state
	var k8sState *string
	if server.Status == models.ServerStatusRunning ||
		server.Status == models.ServerStatusPending ||
		server.Status == models.ServerStatusStarting ||
		server.Status == models.ServerStatusStopping {
		gsName := "server-" + serverID
		gs, err := h.k8sClient.GetGameServer(c.Request.Context(), h.config.K8sNamespace, gsName)
		if err == nil && gs != nil {
			state := string(gs.Status.State)
			k8sState = &state
		}
	}

	// Load game catalog to get default env
	var gameConfigInfo *models.GameConfigInfo
	catalog, err := h.k8sClient.LoadGameCatalog(c.Request.Context(), h.config.K8sNamespace, h.config.K8sGameCatalogName)
	if err == nil {
		if gameConfig, err := catalog.GetGameConfig(string(server.Game)); err == nil {
			effectiveEnv := mergeEnvVars(gameConfig.Env, server.EnvOverrides)
			gameConfigInfo = &models.GameConfigInfo{
				DefaultEnv:   gameConfig.Env,
				EffectiveEnv: effectiveEnv,
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"server":      server,
		"k8s_state":   k8sState,
		"game_config": gameConfigInfo,
	})
}

// UpdateServerEnv updates the environment variable overrides for a server
func (h *ServerHandler) UpdateServerEnv(c *gin.Context) {
	userIDStr := middleware.GetUserID(c)
	if userIDStr == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid user ID"})
		return
	}

	serverID := c.Param("id")
	if serverID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "server ID required"})
		return
	}

	var req models.UpdateServerEnvRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get server and verify ownership
	server, err := h.db.GetServerByID(c.Request.Context(), serverID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "server not found"})
		return
	}

	if server.UserID != userID {
		c.JSON(http.StatusNotFound, gin.H{"error": "server not found"})
		return
	}

	// Validate env keys
	for key, value := range req.EnvOverrides {
		if key == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "empty environment variable key"})
			return
		}
		if len(key) > 256 || len(value) > 4096 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "environment variable too long"})
			return
		}
	}

	// Update env overrides in database
	if err := h.db.UpdateServerEnvOverrides(c.Request.Context(), serverID, req.EnvOverrides); err != nil {
		log.Printf("failed to update env overrides: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update environment variables"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "updated",
		"message": "Environment variables updated. Restart server for changes to take effect.",
	})
}

// mergeEnvVars merges catalog defaults with user overrides (full override mode)
func mergeEnvVars(defaults, overrides map[string]string) map[string]string {
	if overrides == nil {
		// NULL overrides = use defaults as-is
		result := make(map[string]string, len(defaults))
		for k, v := range defaults {
			result[k] = v
		}
		return result
	}

	// Full override mode: overrides completely replace defaults
	result := make(map[string]string, len(overrides))
	for k, v := range overrides {
		result[k] = v
	}
	return result
}

// StopServer stops a running game server by deleting it from K8s
func (h *ServerHandler) StopServer(c *gin.Context) {
	userIDStr := middleware.GetUserID(c)
	if userIDStr == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid user ID"})
		return
	}

	serverID := c.Param("id")
	if serverID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "server ID required"})
		return
	}

	// Get server from database
	server, err := h.db.GetServerByID(c.Request.Context(), serverID)
	if err != nil {
		log.Printf("failed to get server: %v", err)
		c.JSON(http.StatusNotFound, gin.H{"error": "server not found"})
		return
	}

	// Verify server belongs to user
	if server.UserID != userID {
		c.JSON(http.StatusNotFound, gin.H{"error": "server not found"})
		return
	}

	// STEP 1: Atomically transition to "stopping"
	// This prevents race conditions with concurrent stops or start-after-stop
	transitioned, err := h.db.TransitionServerStatusFrom(
		c.Request.Context(), serverID,
		[]models.ServerStatus{models.ServerStatusRunning, models.ServerStatusPending, models.ServerStatusStarting},
		models.ServerStatusStopping,
		"Stopping server...",
	)
	if err != nil {
		log.Printf("failed to transition to stopping: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
		return
	}
	if !transitioned {
		// Server not in stoppable state - check if already stopping
		if server.Status == models.ServerStatusStopping {
			c.JSON(http.StatusAccepted, gin.H{"status": "stopping", "message": "stop already in progress"})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": "server cannot be stopped from current state"})
		return
	}

	// STEP 2: Fire-and-forget: trigger K8s deletion immediately
	// Reconciler will confirm completion and transition to stopped
	go h.triggerServerStop(serverID)

	c.JSON(http.StatusAccepted, gin.H{"status": "stopping", "message": "server is stopping"})
}

// StartServer starts a stopped game server by setting status to pending
func (h *ServerHandler) StartServer(c *gin.Context) {
	userIDStr := middleware.GetUserID(c)
	if userIDStr == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid user ID"})
		return
	}

	serverID := c.Param("id")
	if serverID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "server ID required"})
		return
	}

	// Get server from database
	server, err := h.db.GetServerByID(c.Request.Context(), serverID)
	if err != nil {
		log.Printf("failed to get server: %v", err)
		c.JSON(http.StatusNotFound, gin.H{"error": "server not found"})
		return
	}

	// Verify server belongs to user
	if server.UserID != userID {
		c.JSON(http.StatusNotFound, gin.H{"error": "server not found"})
		return
	}

	// Atomically transition to pending (only from stopped/failed)
	transitioned, err := h.db.TransitionServerStatusFrom(
		c.Request.Context(), serverID,
		[]models.ServerStatus{models.ServerStatusStopped, models.ServerStatusFailed},
		models.ServerStatusPending,
		"Starting server...",
	)
	if err != nil {
		log.Printf("failed to transition to pending: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
		return
	}
	if !transitioned {
		c.JSON(http.StatusBadRequest, gin.H{"error": "server cannot be started from current state"})
		return
	}

	// Fire-and-forget: trigger K8s resource creation immediately
	// Reconciler will handle status transitions and retries if this fails
	go h.triggerServerStart(server)

	c.JSON(http.StatusAccepted, gin.H{"status": "starting", "message": "server is starting"})
}

// triggerServerStart attempts to create K8s resources for a server immediately.
// If it fails, the server remains in "pending" status and the reconciler will retry.
func (h *ServerHandler) triggerServerStart(server *models.Server) {
	ctx := context.Background()
	serverID := server.ID.String()

	// Load game catalog
	catalog, err := h.k8sClient.LoadGameCatalog(ctx, h.config.K8sNamespace, h.config.K8sGameCatalogName)
	if err != nil {
		log.Printf("triggerServerStart: failed to load game catalog for server %s: %v", serverID, err)
		return // Reconciler will retry
	}

	// Get game configuration
	gameConfig, err := catalog.GetGameConfig(string(server.Game))
	if err != nil {
		log.Printf("triggerServerStart: invalid game config for server %s: %v", serverID, err)
		return // Reconciler will handle and mark as failed
	}

	// Get plan configuration
	planConfig, err := gameConfig.GetPlanConfig(string(server.Plan))
	if err != nil {
		log.Printf("triggerServerStart: invalid plan config for server %s: %v", serverID, err)
		return // Reconciler will handle and mark as failed
	}

	// Check if ports already allocated
	allocations, err := h.portAllocService.GetServerPorts(ctx, server.ID)
	if err != nil {
		log.Printf("triggerServerStart: failed to check port allocations for server %s: %v", serverID, err)
		return // Reconciler will retry
	}

	// Allocate ports if needed
	if len(allocations) == 0 {
		portReqs := make([]portalloc.PortRequirement, len(gameConfig.Ports))
		for i, p := range gameConfig.Ports {
			portReqs[i] = portalloc.PortRequirement{
				Name:     p.Name,
				Protocol: p.Protocol,
			}
		}

		// Add sidecar overhead: 100m CPU + 128Mi memory
		cpuQty := resource.MustParse(planConfig.CPU)
		memQty := resource.MustParse(planConfig.Memory)
		cpuMillicores := int(cpuQty.MilliValue()) + 100
		memBytes := memQty.Value() + 128*1024*1024

		resourceReq := &portalloc.ResourceRequirement{
			CPUMillicores: cpuMillicores,
			MemoryBytes:   memBytes,
		}

		allocations, err = h.portAllocService.AllocatePorts(ctx, server.ID, portReqs, resourceReq)
		if err != nil {
			log.Printf("triggerServerStart: failed to allocate ports for server %s: %v", serverID, err)
			return // Reconciler will handle and mark as failed
		}
		log.Printf("triggerServerStart: allocated ports for server %s on node %s", serverID, allocations[0].NodeName)
	}

	// Create PVC if it doesn't exist
	pvcName := fmt.Sprintf("server-%s", serverID)
	labels := map[string]string{"server": serverID, "game": string(server.Game)}

	if err := h.k8sClient.CreatePVC(ctx, h.config.K8sNamespace, pvcName, planConfig.Storage, labels); err != nil {
		log.Printf("triggerServerStart: failed to create PVC for server %s: %v", serverID, err)
		// Continue - may already exist, reconciler will handle
	}

	// Build static port configs from allocations
	gsName := fmt.Sprintf("server-%s", serverID)
	nodeName := allocations[0].NodeName

	staticPorts := make([]k8s.StaticPortConfig, len(allocations))
	for i, alloc := range allocations {
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

	// Compute effective env (merge catalog defaults with user overrides)
	effectiveEnv := mergeEnvVars(gameConfig.Env, server.EnvOverrides)

	// Create GameServer
	if err := h.k8sClient.CreateGameServerWithStaticPorts(ctx, h.config.K8sNamespace, gsName, gameConfig.Image,
		nodeName, staticPorts, volumes, effectiveEnv, planConfig.CPU, planConfig.Memory,
		pvcName, labels, gameConfig.HealthCheck); err != nil {
		log.Printf("triggerServerStart: failed to create GameServer for server %s: %v", serverID, err)
		return // Reconciler will retry
	}

	// Transition to starting - reconciler will handle the Ready check
	transitioned, err := h.db.TransitionServerStatus(ctx, serverID,
		models.ServerStatusPending, models.ServerStatusStarting, "Creating game server...")
	if err != nil {
		log.Printf("triggerServerStart: failed to transition to starting for server %s: %v", serverID, err)
		return
	}
	if transitioned {
		log.Printf("triggerServerStart: server %s transitioned to starting", serverID)
	}
}

// triggerServerStop attempts to delete K8s resources for a server immediately.
// If it fails, the reconciler will retry the deletion.
func (h *ServerHandler) triggerServerStop(serverID string) {
	ctx := context.Background()
	gsName := "server-" + serverID

	if err := h.k8sClient.DeleteGameServer(ctx, h.config.K8sNamespace, gsName); err != nil {
		log.Printf("triggerServerStop: failed to delete GameServer for server %s: %v", serverID, err)
		// Reconciler will retry
		return
	}

	log.Printf("triggerServerStop: deleted GameServer for server %s", serverID)
}

// HandleStripeWebhook handles Stripe webhook events with proper error handling and deduplication
func (h *ServerHandler) HandleStripeWebhook(c *gin.Context) {
	// Read raw request body
	body, err := c.GetRawData()
	if err != nil {
		log.Printf("webhook_error=read_body error=%v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read request body"})
		return
	}

	// Verify webhook signature
	signature := c.GetHeader("Stripe-Signature")
	if signature == "" {
		log.Printf("webhook_error=missing_signature")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing signature header"})
		return
	}

	event, err := h.stripeService.VerifyWebhookSignature(body, signature)
	if err != nil {
		log.Printf("webhook_error=invalid_signature error=%v", err)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid signature"})
		return
	}

	log.Printf("webhook_received event_id=%s event_type=%s", event.ID, event.Type)

	// Check if this event has already been processed (deduplication)
	existingEvent, err := h.db.GetStripeWebhookEvent(c.Request.Context(), event.ID)
	if err == nil && existingEvent != nil {
		// Event was already processed
		if existingEvent.Status == models.WebhookStatusCompleted {
			log.Printf("webhook_duplicate event_id=%s (already processed successfully)", event.ID)
			c.JSON(http.StatusOK, gin.H{"status": "received"})
			return
		}
		// Event was marked as failed, allow retry
		log.Printf("webhook_retry event_id=%s (retrying after previous failure)", event.ID)
	}

	// Process the webhook event
	err = h.stripeService.HandleStripeEvent(c.Request.Context(), event)
	if err != nil {
		// Record failure
		errMsg := err.Error()
		_, dbErr := h.db.CreateStripeWebhookEvent(
			c.Request.Context(),
			event.ID,
			string(event.Type),
			models.WebhookStatusFailed,
			&errMsg,
		)
		if dbErr != nil {
			log.Printf("webhook_error=record_failure event_id=%s error=%v", event.ID, dbErr)
		}

		log.Printf("webhook_error=processing_failed event_id=%s event_type=%s error=%v", event.ID, event.Type, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to process webhook"})
		return
	}

	// Record successful processing
	_, err = h.db.CreateStripeWebhookEvent(
		c.Request.Context(),
		event.ID,
		string(event.Type),
		models.WebhookStatusCompleted,
		nil,
	)
	if err != nil {
		log.Printf("webhook_error=record_success event_id=%s error=%v", event.ID, err)
		// Don't fail the response even if we can't record it
	}

	log.Printf("webhook_processed event_id=%s event_type=%s status=success", event.ID, event.Type)
	c.JSON(http.StatusOK, gin.H{"status": "received"})
}

// StreamLogs streams real-time logs from a game server via SSE
func (h *ServerHandler) StreamLogs(c *gin.Context) {
	userIDStr := middleware.GetUserID(c)
	if userIDStr == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid user ID"})
		return
	}

	serverID := c.Param("id")
	if serverID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "server ID required"})
		return
	}

	// Verify server ownership
	server, err := h.db.GetServerByID(c.Request.Context(), serverID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "server not found"})
		return
	}

	if server.UserID != userID {
		c.JSON(http.StatusNotFound, gin.H{"error": "server not found"})
		return
	}

	// Check server is in a state where logs are available
	if server.Status != models.ServerStatusRunning &&
		server.Status != models.ServerStatusStarting &&
		server.Status != models.ServerStatusStopping {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":  "logs not available",
			"reason": fmt.Sprintf("server is %s", server.Status),
		})
		return
	}

	// Set SSE headers
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no") // Disable nginx buffering

	// Create context that cancels when client disconnects
	ctx, cancel := context.WithCancel(c.Request.Context())
	defer cancel()

	// Start log streaming from K8s
	podName := "server-" + serverID
	const tailLines int64 = 50
	const containerName = "game"

	logStream, err := h.k8sClient.StreamPodLogs(ctx, h.config.K8sNamespace, podName, containerName, tailLines)
	if err != nil {
		log.Printf("failed to stream logs for server %s: %v", serverID, err)
		c.SSEvent("error", gin.H{
			"message": "Failed to connect to server logs",
			"details": err.Error(),
		})
		c.Writer.Flush()
		return
	}
	defer logStream.Close()

	// Send initial connection success event
	c.SSEvent("connected", gin.H{
		"server_id": serverID,
		"status":    "streaming",
	})
	c.Writer.Flush()

	// Start heartbeat goroutine to prevent proxy timeouts
	heartbeatDone := make(chan struct{})
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				close(heartbeatDone)
				return
			case <-ticker.C:
				c.SSEvent("heartbeat", gin.H{"timestamp": time.Now().UTC().Format(time.RFC3339)})
				c.Writer.Flush()
			}
		}
	}()

	// Stream logs line by line
	scanner := bufio.NewScanner(logStream)
	for scanner.Scan() {
		select {
		case <-ctx.Done():
			log.Printf("log streaming ended for server %s: client disconnected", serverID)
			return
		default:
			line := scanner.Text()
			c.SSEvent("log", gin.H{
				"line":      line,
				"timestamp": time.Now().UTC().Format(time.RFC3339),
			})
			c.Writer.Flush()
		}
	}

	if err := scanner.Err(); err != nil {
		log.Printf("log streaming error for server %s: %v", serverID, err)
		c.SSEvent("error", gin.H{
			"message": "Log stream interrupted",
			"details": err.Error(),
		})
		c.Writer.Flush()
	}

	// Send end event when stream closes
	c.SSEvent("end", gin.H{
		"message": "Log stream ended",
	})
	c.Writer.Flush()
}

// StreamStatus streams real-time status updates for all user's servers via SSE
func (h *ServerHandler) StreamStatus(c *gin.Context) {
	userIDStr := middleware.GetUserID(c)
	if userIDStr == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid user ID"})
		return
	}

	// Set SSE headers
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no") // Disable nginx buffering

	// Create context that cancels when client disconnects
	ctx, cancel := context.WithCancel(c.Request.Context())
	defer cancel()

	// Subscribe to hub for this user's events
	eventCh := h.hub.Subscribe(userID)
	defer h.hub.Unsubscribe(userID, eventCh)

	// Get all user's servers and send initial state
	servers, err := h.db.ListServersByUser(ctx, userID)
	if err != nil {
		log.Printf("failed to list servers for user %s: %v", userID, err)
		c.SSEvent("error", gin.H{
			"message": "Failed to get servers",
			"details": err.Error(),
		})
		c.Writer.Flush()
		return
	}

	// Build initial state for all servers
	initialServers := make([]gin.H, len(servers))
	for i, server := range servers {
		initialServers[i] = gin.H{
			"server_id":      server.ID.String(),
			"status":         server.Status,
			"status_message": server.StatusMessage,
		}
	}

	// Send initial connection event with all server states
	c.SSEvent("connected", gin.H{
		"servers":   initialServers,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
	c.Writer.Flush()

	// Start heartbeat ticker
	heartbeatTicker := time.NewTicker(30 * time.Second)
	defer heartbeatTicker.Stop()

	log.Printf("status streaming started for user %s", userID)

	// Stream events
	for {
		select {
		case <-ctx.Done():
			log.Printf("status streaming ended for user %s: client disconnected", userID)
			return

		case event, ok := <-eventCh:
			if !ok {
				// Channel closed
				return
			}
			c.SSEvent("status", gin.H{
				"server_id":      event.ServerID,
				"status":         event.Status,
				"status_message": event.StatusMessage,
				"timestamp":      event.Timestamp.Format(time.RFC3339),
			})
			c.Writer.Flush()

		case <-heartbeatTicker.C:
			c.SSEvent("heartbeat", gin.H{
				"timestamp": time.Now().UTC().Format(time.RFC3339),
			})
			c.Writer.Flush()
		}
	}
}
