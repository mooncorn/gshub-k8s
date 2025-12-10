package stripe

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/google/uuid"
	"github.com/mooncorn/gshub/api/config"
	"github.com/mooncorn/gshub/api/internal/database"
	"github.com/mooncorn/gshub/api/internal/models"
	"github.com/mooncorn/gshub/api/internal/services/k8s"
	"github.com/mooncorn/gshub/api/internal/services/portalloc"
	"github.com/stripe/stripe-go/v84"
	"github.com/stripe/stripe-go/v84/checkout/session"
	"github.com/stripe/stripe-go/v84/subscription"
	"github.com/stripe/stripe-go/v84/webhook"
)

type Service struct {
	db               *database.DB
	config           *config.Config
	k8sClient        *k8s.Client
	portAllocService *portalloc.Service
	k8sNamespace     string
}

// WebhookError represents an error that occurred during webhook processing
// StatusCode determines the HTTP response code
type WebhookError struct {
	StatusCode int    // HTTP status code to return
	Message    string // User-facing error message
	Err        error  // Internal error for logging
}

func (e *WebhookError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Err)
	}
	return e.Message
}

// NewWebhookError creates a new WebhookError with a specific HTTP status code
func NewWebhookError(statusCode int, message string, err error) *WebhookError {
	return &WebhookError{
		StatusCode: statusCode,
		Message:    message,
		Err:        err,
	}
}

// Common webhook errors
var (
	ErrMalformedRequest  = NewWebhookError(http.StatusBadRequest, "malformed request", nil)
	ErrInvalidSignature  = NewWebhookError(http.StatusUnauthorized, "invalid webhook signature", nil)
	ErrProcessingFailure = NewWebhookError(http.StatusInternalServerError, "failed to process webhook", nil)
	ErrMissingEventData  = NewWebhookError(http.StatusBadRequest, "missing or invalid event data", nil)
)

func NewService(db *database.DB, cfg *config.Config, k8sClient *k8s.Client, portAllocService *portalloc.Service, k8sNamespace string) *Service {
	stripe.Key = cfg.StripeSecretKey
	return &Service{
		db:               db,
		config:           cfg,
		k8sClient:        k8sClient,
		portAllocService: portAllocService,
		k8sNamespace:     k8sNamespace,
	}
}

// CreateCheckoutSession creates a Stripe Checkout Session with pending request metadata
func (s *Service) CreateCheckoutSession(ctx context.Context, userID uuid.UUID, pendingRequestID uuid.UUID, priceID string, email string) (string, string, error) {
	// Create checkout session parameters
	params := &stripe.CheckoutSessionParams{
		Mode:          stripe.String(string(stripe.CheckoutSessionModeSubscription)),
		SuccessURL:    stripe.String(s.config.FrontendURL + "/"),
		CancelURL:     stripe.String(s.config.FrontendURL + "/servers/new"),
		CustomerEmail: stripe.String(email),
		LineItems: []*stripe.CheckoutSessionLineItemParams{
			{
				Price:    stripe.String(priceID),
				Quantity: stripe.Int64(1),
			},
		},
		Metadata: map[string]string{
			"pending_request_id": pendingRequestID.String(),
			"user_id":            userID.String(),
		},
	}

	sess, err := session.New(params)
	if err != nil {
		return "", "", fmt.Errorf("failed to create checkout session: %w", err)
	}

	return sess.ID, sess.URL, nil
}

// RetrieveCheckoutSession retrieves a Stripe checkout session by ID
func (s *Service) RetrieveCheckoutSession(ctx context.Context, sessionID string) (*stripe.CheckoutSession, error) {
	sess, err := session.Get(sessionID, &stripe.CheckoutSessionParams{})
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve checkout session: %w", err)
	}
	return sess, nil
}

// VerifyWebhookSignature verifies and constructs a Stripe webhook event
func (s *Service) VerifyWebhookSignature(body []byte, signature string) (*stripe.Event, error) {
	// TODO: Remove IgnoreAPIVersionMismatch once webhook is updated to 2025-11-17.clover
	event, err := webhook.ConstructEventWithOptions(
		body,
		signature,
		s.config.StripeWebhookSecret,
		webhook.ConstructEventOptions{
			IgnoreAPIVersionMismatch: true,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to verify webhook signature: %w", err)
	}
	return &event, nil
}

// HandleStripeEvent dispatches webhook events to appropriate handlers
func (s *Service) HandleStripeEvent(ctx context.Context, event *stripe.Event) error {
	log.Printf("Processing Stripe event: event_id=%s event_type=%s", event.ID, event.Type)

	switch event.Type {
	case "checkout.session.completed":
		return s.handleCheckoutSessionCompleted(ctx, event)
	case "customer.subscription.updated":
		return s.handleSubscriptionUpdated(ctx, event)
	case "customer.subscription.deleted":
		return s.handleSubscriptionDeleted(ctx, event)
	default:
		// Log unknown event type but don't fail
		log.Printf("Received unhandled Stripe event type: event_id=%s event_type=%s", event.ID, event.Type)
		return nil
	}
}

// handleCheckoutSessionCompleted is the internal handler for checkout.session.completed events
func (s *Service) handleCheckoutSessionCompleted(ctx context.Context, event *stripe.Event) error {
	// Unmarshal the event data into a checkout session
	var sess stripe.CheckoutSession
	if err := json.Unmarshal(event.Data.Raw, &sess); err != nil {
		return fmt.Errorf("failed to unmarshal checkout session from webhook event: %w", err)
	}

	log.Printf("Processing checkout session: event_id=%s session_id=%s", event.ID, sess.ID)

	// Check if this is a resubscription
	if resubscribeServerID, ok := sess.Metadata["resubscribe_server_id"]; ok {
		log.Printf("Processing resubscription: event_id=%s server_id=%s", event.ID, resubscribeServerID)
		return s.handleResubscribeCheckout(ctx, event.ID, &sess, resubscribeServerID)
	}

	// Handle new server creation
	return s.CompleteCheckoutSession(ctx, event.ID, &sess)
}

// handleSubscriptionUpdated is the internal handler for customer.subscription.updated events
func (s *Service) handleSubscriptionUpdated(ctx context.Context, event *stripe.Event) error {
	var sub stripe.Subscription
	if err := json.Unmarshal(event.Data.Raw, &sub); err != nil {
		return fmt.Errorf("failed to unmarshal subscription from webhook event: %w", err)
	}

	log.Printf("Processing subscription update: event_id=%s subscription_id=%s status=%s", event.ID, sub.ID, sub.Status)

	// Find server by subscription ID
	server, err := s.db.GetServerByStripeSubscriptionID(ctx, sub.ID)
	if err != nil {
		log.Printf("Failed to find server for subscription: event_id=%s subscription_id=%s error=%v", event.ID, sub.ID, err)
		return nil // Don't fail webhook if server not found; it may have been created before we stored subscription IDs
	}

	// Log status change but don't act on subscription.updated alone
	// The actual action happens when subscription.deleted is received
	log.Printf("Subscription status change: event_id=%s server_id=%s subscription_id=%s status=%s", event.ID, server.ID, sub.ID, sub.Status)
	return nil
}

// handleSubscriptionDeleted is the internal handler for customer.subscription.deleted events
func (s *Service) handleSubscriptionDeleted(ctx context.Context, event *stripe.Event) error {
	var sub stripe.Subscription
	if err := json.Unmarshal(event.Data.Raw, &sub); err != nil {
		return fmt.Errorf("failed to unmarshal subscription from webhook event: %w", err)
	}

	log.Printf("Processing subscription deletion: event_id=%s subscription_id=%s", event.ID, sub.ID)

	// Find server by subscription ID
	server, err := s.db.GetServerByStripeSubscriptionID(ctx, sub.ID)
	if err != nil {
		log.Printf("Failed to find server for subscription deletion: event_id=%s subscription_id=%s error=%v", event.ID, sub.ID, err)
		return nil // Don't fail webhook if server not found; it may have been created before we stored subscription IDs
	}

	serverID := server.ID.String()

	// 1. Atomically transition to expired from any active state
	// This prevents race conditions with concurrent stop/start operations
	transitioned, err := s.db.TransitionServerStatusFrom(ctx, serverID,
		[]models.ServerStatus{
			models.ServerStatusPending,
			models.ServerStatusStarting,
			models.ServerStatusRunning,
			models.ServerStatusStopping,
			models.ServerStatusStopped,
		},
		models.ServerStatusExpired,
		"Subscription cancelled",
	)
	if err != nil {
		return fmt.Errorf("failed to transition server to expired: event_id=%s server_id=%s error=%w", event.ID, serverID, err)
	}

	if !transitioned {
		// Server was already expired/failed/deleted - that's fine
		log.Printf("Server already in terminal state: event_id=%s server_id=%s status=%s", event.ID, serverID, server.Status)
		return nil
	}

	// 2. Set expiration metadata (timestamps, clear resource reservations)
	if err := s.db.MarkServerExpired(ctx, serverID); err != nil {
		log.Printf("Failed to set expiration metadata: event_id=%s server_id=%s error=%v", event.ID, serverID, err)
		// Continue - status is already expired, timestamps are secondary
	}

	// 3. Delete Deployment from K8s (idempotent - may not exist if stopped)
	deployName := "server-" + serverID
	if err := s.k8sClient.DeleteGameDeployment(ctx, s.k8sNamespace, deployName); err != nil {
		log.Printf("Failed to delete Deployment (may not exist): event_id=%s server_id=%s error=%v", event.ID, serverID, err)
	} else {
		log.Printf("Deleted Deployment: event_id=%s server_id=%s", event.ID, serverID)
	}

	// 4. Release port allocations (idempotent - may not be allocated)
	if err := s.portAllocService.ReleasePorts(ctx, server.ID); err != nil {
		log.Printf("Failed to release ports: event_id=%s server_id=%s error=%v", event.ID, serverID, err)
	} else {
		log.Printf("Released ports: event_id=%s server_id=%s", event.ID, serverID)
	}

	log.Printf("Server marked as expired: event_id=%s server_id=%s subscription_id=%s delete_after=+7days", event.ID, server.ID, sub.ID)
	return nil
}

// CompleteCheckoutSession completes a checkout session and creates the associated server
func (s *Service) CompleteCheckoutSession(ctx context.Context, eventID string, sess *stripe.CheckoutSession) error {
	// Verify payment status
	if sess.PaymentStatus != stripe.CheckoutSessionPaymentStatusPaid {
		return fmt.Errorf("payment not completed: status %s", sess.PaymentStatus)
	}

	// Extract pending_request_id from metadata
	pendingRequestIDStr, ok := sess.Metadata["pending_request_id"]
	if !ok {
		return fmt.Errorf("pending_request_id not found in metadata")
	}

	pendingRequestID, err := uuid.Parse(pendingRequestIDStr)
	if err != nil {
		return fmt.Errorf("invalid pending_request_id: %w", err)
	}

	log.Printf("Processing checkout session completion: event_id=%s session_id=%s pending_request_id=%s", eventID, sess.ID, pendingRequestID)

	// Extract subscription ID from the checkout session
	// For subscription mode, the subscription ID is available in the subscription field
	if sess.Subscription == nil {
		return fmt.Errorf("subscription not found in checkout session")
	}

	subscriptionID := sess.Subscription.ID
	log.Printf("Checkout session subscription: event_id=%s session_id=%s subscription_id=%s", eventID, sess.ID, subscriptionID)

	// Start transaction to ensure atomicity of all operations
	tx, err := s.db.Pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// Create a temporary DB struct with the transaction
	txDB := &database.DB{Pool: tx}

	// Retrieve pending server request within transaction
	pendingReq, err := txDB.GetPendingServerRequest(ctx, pendingRequestID)
	if err != nil {
		return fmt.Errorf("failed to get pending server request: %w", err)
	}

	// Check if already processed
	if pendingReq.Status != models.PendingStatusAwaitingPayment {
		log.Printf("Pending request already processed: event_id=%s pending_request_id=%s status=%s", eventID, pendingRequestID, pendingReq.Status)
		return nil // Idempotent: return success if already processed
	}

	// Create the server from pending request
	serverParams := &database.CreateServerParams{
		UserID:               pendingReq.UserID,
		DisplayName:          *pendingReq.DisplayName,
		Subdomain:            pendingReq.Subdomain,
		Game:                 models.GameType(pendingReq.Game),
		Plan:                 models.ServerPlan(pendingReq.Plan),
		StripeSubscriptionID: &subscriptionID,
	}

	createdServer, err := txDB.CreateServer(ctx, serverParams)
	if err != nil {
		return fmt.Errorf("failed to create server: %w", err)
	}

	// Mark pending request as completed with server ID
	err = txDB.MarkPendingServerRequestCompleted(ctx, pendingRequestID, createdServer.ID)
	if err != nil {
		return fmt.Errorf("failed to mark pending request as completed: %w", err)
	}

	// Commit transaction
	if err = tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	log.Printf("Server created successfully: event_id=%s server_id=%s pending_request_id=%s", eventID, createdServer.ID, pendingRequestID)
	return nil
}

// GetSubscription retrieves subscription details from Stripe
func (s *Service) GetSubscription(ctx context.Context, subscriptionID string) (*stripe.Subscription, error) {
	sub, err := subscription.Get(subscriptionID, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve subscription: %w", err)
	}
	return sub, nil
}

// CancelSubscriptionAtPeriodEnd cancels a subscription at the end of the billing period
func (s *Service) CancelSubscriptionAtPeriodEnd(ctx context.Context, subscriptionID string) (*stripe.Subscription, error) {
	params := &stripe.SubscriptionParams{
		CancelAtPeriodEnd: stripe.Bool(true),
	}
	sub, err := subscription.Update(subscriptionID, params)
	if err != nil {
		return nil, fmt.Errorf("failed to cancel subscription: %w", err)
	}
	return sub, nil
}

// ResumeSubscription removes the cancel_at_period_end flag to resume a subscription
func (s *Service) ResumeSubscription(ctx context.Context, subscriptionID string) (*stripe.Subscription, error) {
	params := &stripe.SubscriptionParams{
		CancelAtPeriodEnd: stripe.Bool(false),
	}
	sub, err := subscription.Update(subscriptionID, params)
	if err != nil {
		return nil, fmt.Errorf("failed to resume subscription: %w", err)
	}
	return sub, nil
}

// CreateResubscribeCheckoutSession creates a new checkout session for resubscribing an expired server
func (s *Service) CreateResubscribeCheckoutSession(ctx context.Context, serverID uuid.UUID, userID uuid.UUID, priceID string, email string) (string, string, error) {
	params := &stripe.CheckoutSessionParams{
		Mode:          stripe.String(string(stripe.CheckoutSessionModeSubscription)),
		SuccessURL:    stripe.String(s.config.FrontendURL + "/settings/billing?resubscribed=true"),
		CancelURL:     stripe.String(s.config.FrontendURL + "/settings/billing"),
		CustomerEmail: stripe.String(email),
		LineItems: []*stripe.CheckoutSessionLineItemParams{
			{
				Price:    stripe.String(priceID),
				Quantity: stripe.Int64(1),
			},
		},
		Metadata: map[string]string{
			"resubscribe_server_id": serverID.String(),
			"user_id":               userID.String(),
		},
	}

	sess, err := session.New(params)
	if err != nil {
		return "", "", fmt.Errorf("failed to create resubscribe checkout session: %w", err)
	}

	return sess.ID, sess.URL, nil
}

// handleResubscribeCheckout processes a checkout session for resubscribing an expired server
func (s *Service) handleResubscribeCheckout(ctx context.Context, eventID string, sess *stripe.CheckoutSession, serverIDStr string) error {
	if sess.PaymentStatus != stripe.CheckoutSessionPaymentStatusPaid {
		return fmt.Errorf("payment not completed: status %s", sess.PaymentStatus)
	}

	serverID, err := uuid.Parse(serverIDStr)
	if err != nil {
		return fmt.Errorf("invalid server ID: %w", err)
	}

	if sess.Subscription == nil {
		return fmt.Errorf("subscription not found in checkout session")
	}

	subscriptionID := sess.Subscription.ID

	// Reactivate the server with the new subscription ID
	if err := s.db.ReactivateServer(ctx, serverID.String(), subscriptionID); err != nil {
		return fmt.Errorf("failed to reactivate server: %w", err)
	}

	log.Printf("Server reactivated: event_id=%s server_id=%s subscription_id=%s", eventID, serverID, subscriptionID)
	return nil
}
