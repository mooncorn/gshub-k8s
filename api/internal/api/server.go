package api

import (
	"log"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/mooncorn/gshub/api/config"
	"github.com/mooncorn/gshub/api/internal/api/middleware"
	"github.com/mooncorn/gshub/api/internal/database"
	"github.com/mooncorn/gshub/api/internal/models"
	"github.com/mooncorn/gshub/api/internal/services/k8s"
	stripeservice "github.com/mooncorn/gshub/api/internal/services/stripe"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

type ServerHandler struct {
	db            *database.DB
	k8sClient     *k8s.Client
	config        *config.Config
	stripeService *stripeservice.Service
}

func NewServerHandler(db *database.DB, k8sClient *k8s.Client, cfg *config.Config, stripeSvc *stripeservice.Service) *ServerHandler {
	return &ServerHandler{
		db:            db,
		k8sClient:     k8sClient,
		config:        cfg,
		stripeService: stripeSvc,
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

	// Try to get K8s GameServer status if server is running
	var k8sState *string
	if server.Status == models.ServerStatusRunning || server.Status == models.ServerStatusPending {
		gsName := "server-" + serverID
		gs, err := h.k8sClient.GetGameServer(c.Request.Context(), h.config.K8sNamespace, gsName)
		if err == nil && gs != nil {
			state := string(gs.Status.State)
			k8sState = &state
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"server":    server,
		"k8s_state": k8sState,
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

	// Check if server can be stopped
	if server.Status != models.ServerStatusRunning && server.Status != models.ServerStatusPending {
		c.JSON(http.StatusBadRequest, gin.H{"error": "server is not running"})
		return
	}

	// Delete GameServer from K8s
	gsName := "server-" + serverID
	err = h.k8sClient.DeleteGameServer(c.Request.Context(), h.config.K8sNamespace, gsName)
	if err != nil {
		log.Printf("failed to delete GameServer: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to stop server"})
		return
	}

	// Mark server as stopped in database
	err = h.db.MarkServerStopped(c.Request.Context(), serverID)
	if err != nil {
		log.Printf("failed to mark server stopped: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update server status"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "stopped", "message": "server stopped successfully"})
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

	// Check if server can be started
	if server.Status != models.ServerStatusStopped && server.Status != models.ServerStatusFailed {
		c.JSON(http.StatusBadRequest, gin.H{"error": "server cannot be started from current state"})
		return
	}

	// Set status to pending so reconciler will pick it up
	err = h.db.UpdateServerStatus(c.Request.Context(), serverID, string(models.ServerStatusPending), "")
	if err != nil {
		log.Printf("failed to update server status: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to start server"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "starting", "message": "server is starting"})
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
