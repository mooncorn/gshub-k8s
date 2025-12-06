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
		SuccessURL:    stripe.String(s.config.FrontendURL + "/servers/checkout/success?session_id={CHECKOUT_SESSION_ID}"),
		CancelURL:     stripe.String(s.config.FrontendURL + "/servers"),
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

	// 1. Delete GameServer from K8s (if running)
	gsName := "server-" + serverID
	if err := s.k8sClient.DeleteGameServer(ctx, s.k8sNamespace, gsName); err != nil {
		log.Printf("Failed to delete GameServer (may not exist): event_id=%s server_id=%s error=%v", event.ID, serverID, err)
		// Continue - GameServer may not exist if server was already stopped
	} else {
		log.Printf("Deleted GameServer: event_id=%s server_id=%s", event.ID, serverID)
	}

	// 2. Release port allocations
	if err := s.portAllocService.ReleasePorts(ctx, server.ID); err != nil {
		log.Printf("Failed to release ports: event_id=%s server_id=%s error=%v", event.ID, serverID, err)
		// Continue - ports may not be allocated
	} else {
		log.Printf("Released ports: event_id=%s server_id=%s", event.ID, serverID)
	}

	// 3. Mark server as expired (clears node_name and resource reservations, sets 7-day grace period)
	// PVC is NOT deleted here - it remains for the 7-day grace period
	err = s.db.MarkServerExpired(ctx, serverID)
	if err != nil {
		return fmt.Errorf("failed to mark server expired: event_id=%s server_id=%s subscription_id=%s error=%w", event.ID, server.ID, sub.ID, err)
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
