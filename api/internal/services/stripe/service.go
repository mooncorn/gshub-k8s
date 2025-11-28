package stripe

import (
	"context"
	"fmt"
	"log"

	"github.com/google/uuid"
	"github.com/stripe/stripe-go/v84"
	"github.com/stripe/stripe-go/v84/checkout/session"
	"github.com/stripe/stripe-go/v84/webhook"
	"github.com/mooncorn/gshub/api/config"
	"github.com/mooncorn/gshub/api/internal/database"
	"github.com/mooncorn/gshub/api/internal/models"
)

type Service struct {
	db     *database.DB
	config *config.Config
}

func NewService(db *database.DB, cfg *config.Config) *Service {
	stripe.Key = cfg.StripeSecretKey
	return &Service{
		db:     db,
		config: cfg,
	}
}

// CreateCheckoutSession creates a Stripe Checkout Session with pending request metadata
func (s *Service) CreateCheckoutSession(ctx context.Context, userID uuid.UUID, pendingRequestID uuid.UUID, priceID string, email string) (string, string, error) {
	// Create checkout session parameters
	params := &stripe.CheckoutSessionParams{
		Mode:       stripe.String(string(stripe.CheckoutSessionModeSubscription)),
		SuccessURL: stripe.String(s.config.FrontendURL + "/servers/checkout/success?session_id={CHECKOUT_SESSION_ID}"),
		CancelURL:  stripe.String(s.config.FrontendURL + "/servers"),
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
	event, err := webhook.ConstructEvent(body, signature, s.config.StripeWebhookSecret)
	if err != nil {
		return nil, fmt.Errorf("failed to verify webhook signature: %w", err)
	}
	return &event, nil
}

// HandleCheckoutSessionCompleted handles the checkout.session.completed webhook
func (s *Service) HandleCheckoutSessionCompleted(ctx context.Context, sess *stripe.CheckoutSession) error {
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

	// Retrieve pending server request
	pendingReq, err := s.db.GetPendingServerRequest(ctx, pendingRequestID)
	if err != nil {
		return fmt.Errorf("failed to get pending server request: %w", err)
	}

	// Check if already processed
	if pendingReq.Status != models.PendingStatusAwaitingPayment {
		log.Printf("Pending request %s already processed with status: %s", pendingRequestID, pendingReq.Status)
		return nil // Idempotent: return success if already processed
	}

	// Create the server from pending request
	serverParams := &database.CreateServerParams{
		UserID:      pendingReq.UserID,
		DisplayName: *pendingReq.DisplayName,
		Subdomain:   pendingReq.Subdomain,
		Game:        models.GameType(pendingReq.Game),
		Plan:        models.ServerPlan(pendingReq.Plan),
	}

	createdServer, err := s.db.CreateServer(ctx, serverParams)
	if err != nil {
		return fmt.Errorf("failed to create server: %w", err)
	}

	// Mark pending request as completed with server ID
	err = s.db.MarkPendingServerRequestCompleted(ctx, pendingRequestID, createdServer.ID)
	if err != nil {
		return fmt.Errorf("failed to mark pending request as completed: %w", err)
	}

	log.Printf("Server created successfully: %s for pending request %s", createdServer.ID, pendingRequestID)
	return nil
}

// HandleSubscriptionUpdated handles subscription update webhooks
func (s *Service) HandleSubscriptionUpdated(ctx context.Context, sub *stripe.Subscription) error {
	return nil
}

// HandleSubscriptionDeleted handles subscription deletion webhooks
func (s *Service) HandleSubscriptionDeleted(ctx context.Context, sub *stripe.Subscription) error {
	return nil
}
