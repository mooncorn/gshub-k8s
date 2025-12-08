package api

import (
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/mooncorn/gshub/api/config"
	"github.com/mooncorn/gshub/api/internal/api/middleware"
	"github.com/mooncorn/gshub/api/internal/database"
	"github.com/mooncorn/gshub/api/internal/models"
	stripeservice "github.com/mooncorn/gshub/api/internal/services/stripe"
)

type BillingHandler struct {
	db            *database.DB
	config        *config.Config
	stripeService *stripeservice.Service
}

func NewBillingHandler(db *database.DB, cfg *config.Config, stripeSvc *stripeservice.Service) *BillingHandler {
	return &BillingHandler{
		db:            db,
		config:        cfg,
		stripeService: stripeSvc,
	}
}

// GetBilling returns subscription information for all user servers
func (h *BillingHandler) GetBilling(c *gin.Context) {
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

	// Get all servers for user
	servers, err := h.db.ListServersByUser(c.Request.Context(), userID)
	if err != nil {
		log.Printf("failed to list servers: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list servers"})
		return
	}

	// Build subscription info for each server
	subscriptions := make([]models.ServerSubscription, 0, len(servers))
	for _, server := range servers {
		sub := models.ServerSubscription{
			ServerID:    server.ID.String(),
			DisplayName: server.DisplayName,
			Game:        server.Game,
			Plan:        server.Plan,
			Status:      server.Status,
			ExpiredAt:   server.ExpiredAt,
			DeleteAfter: server.DeleteAfter,
		}

		// Fetch Stripe subscription details if available
		if server.StripeSubscriptionID != nil && *server.StripeSubscriptionID != "" {
			stripeSub, err := h.stripeService.GetSubscription(c.Request.Context(), *server.StripeSubscriptionID)
			if err != nil {
				log.Printf("failed to get subscription for server %s: %v", server.ID, err)
				// Continue without subscription details
			} else {
				// Get current period end from the first subscription item
				var currentPeriodEnd int64
				if stripeSub.Items != nil && len(stripeSub.Items.Data) > 0 {
					currentPeriodEnd = stripeSub.Items.Data[0].CurrentPeriodEnd
				}

				sub.Subscription = &models.SubscriptionInfo{
					SubscriptionID:    stripeSub.ID,
					Status:            string(stripeSub.Status),
					CurrentPeriodEnd:  time.Unix(currentPeriodEnd, 0),
					CancelAtPeriodEnd: stripeSub.CancelAtPeriodEnd,
				}
				if stripeSub.CanceledAt > 0 {
					canceledAt := time.Unix(stripeSub.CanceledAt, 0)
					sub.Subscription.CanceledAt = &canceledAt
				}
				if stripeSub.CancelAt > 0 {
					cancelsAt := time.Unix(stripeSub.CancelAt, 0)
					sub.Subscription.CancelsAt = &cancelsAt
				}
			}
		}

		subscriptions = append(subscriptions, sub)
	}

	c.JSON(http.StatusOK, models.BillingResponse{
		Subscriptions: subscriptions,
	})
}

// CancelSubscription cancels a subscription at period end
func (h *BillingHandler) CancelSubscription(c *gin.Context) {
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

	// Verify server has active subscription
	if server.StripeSubscriptionID == nil || *server.StripeSubscriptionID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "server has no active subscription"})
		return
	}

	// Cancel subscription at period end
	sub, err := h.stripeService.CancelSubscriptionAtPeriodEnd(c.Request.Context(), *server.StripeSubscriptionID)
	if err != nil {
		log.Printf("failed to cancel subscription: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to cancel subscription"})
		return
	}

	// Get current period end from the first subscription item
	var currentPeriodEnd int64
	if sub.Items != nil && len(sub.Items.Data) > 0 {
		currentPeriodEnd = sub.Items.Data[0].CurrentPeriodEnd
	}

	c.JSON(http.StatusOK, gin.H{
		"status":               "cancelled",
		"message":              "Subscription will be cancelled at the end of the billing period",
		"cancel_at_period_end": sub.CancelAtPeriodEnd,
		"current_period_end":   time.Unix(currentPeriodEnd, 0),
	})
}

// ResubscribeServer creates a new checkout session for an expired server
func (h *BillingHandler) ResubscribeServer(c *gin.Context) {
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

	// Verify server is in expired state
	if server.Status != models.ServerStatusExpired {
		c.JSON(http.StatusBadRequest, gin.H{"error": "server is not expired"})
		return
	}

	// Get user email
	user, err := h.db.GetUserByID(c.Request.Context(), userID)
	if err != nil {
		log.Printf("failed to get user: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get user"})
		return
	}

	// Get price ID for game+plan combination
	priceID, err := h.config.GetPriceID(string(server.Game), string(server.Plan))
	if err != nil {
		log.Printf("failed to get price ID: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get price"})
		return
	}

	// Create checkout session for resubscription
	sessionID, checkoutURL, err := h.stripeService.CreateResubscribeCheckoutSession(
		c.Request.Context(),
		server.ID,
		userID,
		priceID,
		user.Email,
	)
	if err != nil {
		log.Printf("failed to create resubscribe checkout session: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create checkout session"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"session_id":   sessionID,
		"checkout_url": checkoutURL,
	})
}

// ResumeSubscription resumes a subscription that was scheduled to cancel
func (h *BillingHandler) ResumeSubscription(c *gin.Context) {
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

	// Verify server has active subscription
	if server.StripeSubscriptionID == nil || *server.StripeSubscriptionID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "server has no active subscription"})
		return
	}

	// Resume subscription
	_, err = h.stripeService.ResumeSubscription(c.Request.Context(), *server.StripeSubscriptionID)
	if err != nil {
		log.Printf("failed to resume subscription: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to resume subscription"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "resumed",
		"message": "Subscription has been resumed",
	})
}
