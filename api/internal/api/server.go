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

	// Validate resource capacity before proceeding to checkout
	catalog, err := h.k8sClient.LoadGameCatalog(c.Request.Context(), h.config.K8sNamespace, h.config.K8sGameCatalogName)
	if err != nil {
		log.Printf("failed to load game catalog: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load game configuration"})
		return
	}

	gameConfig, err := catalog.GetGameConfig(req.Game)
	if err != nil {
		log.Printf("game not found in catalog: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	planConfig, err := gameConfig.GetPlanConfig(req.Plan)
	if err != nil {
		log.Printf("plan not found in catalog: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Build port requirements from game config
	portReqs := make([]portalloc.PortRequirement, len(gameConfig.Ports))
	for i, p := range gameConfig.Ports {
		portReqs[i] = portalloc.PortRequirement{Name: p.Name, Protocol: p.Protocol}
	}

	// Build resource requirements (with sidecar overhead)
	cpuMillicores := parseCPUToMillicores(planConfig.CPU) + 100       // +100m for sidecar
	memBytes := parseMemoryToBytes(planConfig.Memory) + 128*1024*1024 // +128Mi for sidecar
	resourceReq := &portalloc.ResourceRequirement{
		CPUMillicores: cpuMillicores,
		MemoryBytes:   memBytes,
	}

	// Check capacity before proceeding to checkout
	hasCapacity, err := h.portAllocService.HasCapacity(c.Request.Context(), portReqs, resourceReq)
	if err != nil {
		log.Printf("failed to check capacity: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to check server availability"})
		return
	}
	if !hasCapacity {
		log.Printf("no capacity available for game=%s plan=%s", req.Game, req.Plan)
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "No server capacity available at this time. Please try again later.",
		})
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

	// Load game catalog to get default env
	var gameConfigInfo *models.GameConfigInfo
	catalog, err := h.k8sClient.LoadGameCatalog(c.Request.Context(), h.config.K8sNamespace, h.config.K8sGameCatalogName)
	if err == nil {
		if gameConfig, err := catalog.GetGameConfig(string(server.Game)); err == nil {
			if planConfig, err := gameConfig.GetPlanConfig(string(server.Plan)); err == nil {
				// Merge game + plan defaults for display
				defaultEnv := k8s.MergeEnvVars(gameConfig.Env, planConfig.Env, nil)
				effectiveEnv := k8s.MergeEnvVars(gameConfig.Env, planConfig.Env, server.EnvOverrides)
				gameConfigInfo = &models.GameConfigInfo{
					DefaultEnv:   defaultEnv,
					EffectiveEnv: effectiveEnv,
				}
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"server":      server,
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

// RestartServer restarts a server with updated environment variables.
// This deletes the deployment and transitions to pending so the reconciler
// creates a new deployment with the latest env vars from the database.
func (h *ServerHandler) RestartServer(c *gin.Context) {
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

	// Only restart from running or stopped states
	if server.Status != models.ServerStatusRunning && server.Status != models.ServerStatusStopped {
		c.JSON(http.StatusBadRequest, gin.H{"error": "server must be running or stopped to restart"})
		return
	}

	// Delete deployment (keeps PVC with data intact)
	deployName := "server-" + serverID
	if err := h.k8sClient.DeleteGameDeployment(c.Request.Context(), h.config.K8sNamespace, deployName); err != nil {
		log.Printf("RestartServer: failed to delete deployment for server %s: %v", serverID, err)
		// Continue anyway - deployment might not exist
	}

	// Release port allocation (will be reallocated on next reconcile)
	if err := h.portAllocService.ReleasePorts(c.Request.Context(), server.ID); err != nil {
		log.Printf("RestartServer: failed to release ports for server %s: %v", serverID, err)
		// Continue anyway
	}

	// Transition to pending - reconciler creates new deployment with updated env
	transitioned, err := h.db.TransitionServerStatusFrom(
		c.Request.Context(), serverID,
		[]models.ServerStatus{models.ServerStatusRunning, models.ServerStatusStopped},
		models.ServerStatusPending,
		"Restarting server with updated configuration...",
	)
	if err != nil {
		log.Printf("failed to transition to pending: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
		return
	}
	if !transitioned {
		c.JSON(http.StatusBadRequest, gin.H{"error": "server cannot be restarted from current state"})
		return
	}

	// Broadcast status update
	h.hub.Publish(server.UserID, broadcast.StatusEvent{
		ServerID:  serverID,
		Status:    string(models.ServerStatusPending),
		Timestamp: time.Now().UTC(),
	})

	c.JSON(http.StatusAccepted, gin.H{"status": "restarting", "message": "server is restarting"})
}

// triggerServerStart attempts to start a server.
// If a deployment already exists, it scales it to 1 (fast restart).
// Otherwise, it leaves the server in "pending" for the reconciler to create the deployment.
func (h *ServerHandler) triggerServerStart(server *models.Server) {
	ctx := context.Background()
	serverID := server.ID.String()
	deployName := "server-" + serverID

	// Check if deployment already exists (fast restart case)
	exists, err := h.k8sClient.DeploymentExists(ctx, h.config.K8sNamespace, deployName)
	if err != nil {
		log.Printf("triggerServerStart: failed to check deployment existence for server %s: %v", serverID, err)
		return // Reconciler will retry
	}

	if exists {
		// Fast path: Just scale up existing deployment
		if err := h.k8sClient.ScaleGameDeployment(ctx, h.config.K8sNamespace, deployName, 1); err != nil {
			log.Printf("triggerServerStart: failed to scale deployment for server %s: %v", serverID, err)
			return
		}

		// Transition to starting - supervisor will report running via internal API
		transitioned, err := h.db.TransitionServerStatus(ctx, serverID,
			models.ServerStatusPending, models.ServerStatusStarting,
			"Starting game server...")
		if err != nil {
			log.Printf("triggerServerStart: failed to transition to starting for server %s: %v", serverID, err)
			return
		}
		if transitioned {
			log.Printf("triggerServerStart: scaled deployment to 1 for server %s (fast restart)", serverID)

			// Broadcast status update
			h.hub.Publish(server.UserID, broadcast.StatusEvent{
				ServerID:  serverID,
				Status:    string(models.ServerStatusStarting),
				Timestamp: time.Now().UTC(),
			})
		}
		return
	}

	// Slow path: No deployment exists, reconciler will create one
	// Just leave server in "pending" state for reconciler
	log.Printf("triggerServerStart: no deployment exists for server %s, reconciler will create", serverID)
}

// triggerServerStop scales the deployment to 0 to stop the server.
// The supervisor will receive SIGTERM and report "stopped" via internal API.
// A fallback goroutine ensures the server is marked stopped if supervisor fails.
func (h *ServerHandler) triggerServerStop(serverID string) {
	ctx := context.Background()
	deployName := "server-" + serverID

	// Scale to 0 - supervisor receives SIGTERM and reports status via internal API
	if err := h.k8sClient.ScaleGameDeployment(ctx, h.config.K8sNamespace, deployName, 0); err != nil {
		log.Printf("triggerServerStop: failed to scale deployment for server %s: %v", serverID, err)
		return
	}
	log.Printf("triggerServerStop: scaled deployment to 0 for server %s", serverID)

	// Start background fallback: mark as stopped if still "stopping" after timeout
	go h.ensureStoppedState(serverID)
}

// ensureStoppedState is a fallback that marks server as stopped if supervisor
// fails to report (e.g., pod was killed before graceful shutdown completed)
func (h *ServerHandler) ensureStoppedState(serverID string) {
	time.Sleep(90 * time.Second) // Wait longer than typical grace period

	ctx := context.Background()
	server, err := h.db.GetServerByID(ctx, serverID)
	if err != nil {
		return
	}

	// If still in "stopping" state, force transition to "stopped"
	if server.Status == models.ServerStatusStopping {
		// Verify deployment is actually scaled to 0
		deployName := "server-" + serverID
		deploy, err := h.k8sClient.GetGameDeployment(ctx, h.config.K8sNamespace, deployName)
		if err != nil || deploy == nil || (deploy.Spec.Replicas != nil && *deploy.Spec.Replicas == 0) {
			transitioned, _ := h.db.TransitionServerStatus(ctx, serverID,
				models.ServerStatusStopping, models.ServerStatusStopped,
				"Server stopped (fallback)")
			if transitioned {
				h.db.MarkServerStopped(ctx, serverID)
				log.Printf("ensureStoppedState: fallback marked server %s as stopped", serverID)

				// Broadcast status update
				h.hub.Publish(server.UserID, broadcast.StatusEvent{
					ServerID:  serverID,
					Status:    string(models.ServerStatusStopped),
					Timestamp: time.Now().UTC(),
				})
			}
		}
	}
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
	// Find the pod by label since Deployment pods have generated suffixes
	labelSelector := "server=" + serverID
	pod, err := h.k8sClient.GetPodByLabel(ctx, h.config.K8sNamespace, labelSelector)
	if err != nil {
		log.Printf("failed to find pod for server %s: %v", serverID, err)
		c.SSEvent("error", gin.H{
			"message": "Failed to find server pod",
			"details": err.Error(),
		})
		c.Writer.Flush()
		return
	}

	const tailLines int64 = 50
	const containerName = "supervisor"

	logStream, err := h.k8sClient.StreamPodLogs(ctx, h.config.K8sNamespace, pod.Name, containerName, tailLines)
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

// parseCPUToMillicores converts a CPU string (e.g., "1", "500m") to millicores
func parseCPUToMillicores(cpu string) int {
	q := resource.MustParse(cpu)
	return int(q.MilliValue())
}

// parseMemoryToBytes converts a memory string (e.g., "2Gi", "512Mi") to bytes
func parseMemoryToBytes(memory string) int64 {
	q := resource.MustParse(memory)
	return q.Value()
}
