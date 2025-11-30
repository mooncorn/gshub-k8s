package api

import (
	"encoding/json"
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

// HandleStripeWebhook handles Stripe webhook events
func (h *ServerHandler) HandleStripeWebhook(c *gin.Context) {
	body, err := c.GetRawData()
	if err != nil {
		log.Printf("failed to read request body: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read request body"})
		return
	}

	signature := c.GetHeader("Stripe-Signature")
	if signature == "" {
		log.Printf("missing signature header")
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing signature header"})
		return
	}

	// Verify webhook signature
	event, err := h.stripeService.VerifyWebhookSignature(body, signature)
	if err != nil {
		log.Printf("invalid webhook signature: %v", err)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid signature"})
		return
	}

	// Handle checkout.session.completed event
	if event.Type == "checkout.session.completed" {
		// Unmarshal raw JSON data to get session ID
		var sessionData map[string]interface{}
		if err := json.Unmarshal(event.Data.Raw, &sessionData); err != nil {
			log.Printf("failed to unmarshal webhook event data: %v", err)
			c.JSON(http.StatusOK, gin.H{"status": "received"})
			return
		}

		sessionID, ok := sessionData["id"].(string)
		if !ok {
			log.Printf("missing session ID in webhook event data")
			c.JSON(http.StatusOK, gin.H{"status": "received"})
			return
		}

		// Retrieve full session from Stripe
		sess, err := h.stripeService.RetrieveCheckoutSession(c.Request.Context(), sessionID)
		if err != nil {
			log.Printf("failed to retrieve checkout session: %v", err)
			c.JSON(http.StatusOK, gin.H{"status": "received"})
			return
		}

		if err := h.stripeService.HandleCheckoutSessionCompleted(c.Request.Context(), sess); err != nil {
			log.Printf("failed to handle checkout session completed: %v", err)
			c.JSON(http.StatusOK, gin.H{"status": "received"})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{"status": "received"})
}
