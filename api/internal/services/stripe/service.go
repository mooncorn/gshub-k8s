package stripe

import (
	"context"

	"github.com/mooncorn/gshub/api/config"
	"github.com/mooncorn/gshub/api/internal/database"
	"github.com/stripe/stripe-go/v84"
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

// CreateCheckoutSession creates a Stripe Checkout Session
func (s *Service) CreateCheckoutSession(ctx context.Context, userID, email, priceID string) (*stripe.CheckoutSession, error) {
	return nil, nil
}

// CreateCustomerPortalSession creates a Stripe Customer Portal session
func (s *Service) CreateCustomerPortalSession(ctx context.Context, userID string) (*stripe.BillingPortalSession, error) {
	return nil, nil
}

// HandleCheckoutSessionCompleted handles the checkout.session.completed webhook
func (s *Service) HandleCheckoutSessionCompleted(ctx context.Context, sess *stripe.CheckoutSession) error {
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
