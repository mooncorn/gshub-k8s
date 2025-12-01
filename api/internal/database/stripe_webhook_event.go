package database

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/mooncorn/gshub/api/internal/models"
)

// GetStripeWebhookEvent retrieves a webhook event by Stripe event ID
func (db *DB) GetStripeWebhookEvent(ctx context.Context, stripeEventID string) (*models.StripeWebhookEvent, error) {
	query := `
		SELECT id, stripe_event_id, event_type, status, error_message, processed_at, created_at
		FROM stripe_webhook_events
		WHERE stripe_event_id = $1
	`

	row := db.Pool.QueryRow(ctx, query, stripeEventID)
	event := &models.StripeWebhookEvent{}

	err := row.Scan(
		&event.ID, &event.StripeEventID, &event.EventType, &event.Status,
		&event.ErrorMessage, &event.ProcessedAt, &event.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get stripe webhook event: %w", err)
	}

	return event, nil
}

// CreateStripeWebhookEvent creates a new processed webhook event record
func (db *DB) CreateStripeWebhookEvent(
	ctx context.Context,
	stripeEventID string,
	eventType string,
	status models.WebhookStatus,
	errorMessage *string,
) (*uuid.UUID, error) {
	var id uuid.UUID

	query := `
		INSERT INTO stripe_webhook_events
		(stripe_event_id, event_type, status, error_message, processed_at)
		VALUES ($1, $2, $3, $4, NOW())
		RETURNING id
	`

	err := db.Pool.QueryRow(ctx, query, stripeEventID, eventType, status, errorMessage).Scan(&id)
	if err != nil {
		return nil, fmt.Errorf("failed to create stripe webhook event: %w", err)
	}

	return &id, nil
}

// UpdateStripeWebhookEventStatus updates the status of a webhook event
func (db *DB) UpdateStripeWebhookEventStatus(
	ctx context.Context,
	stripeEventID string,
	status models.WebhookStatus,
	errorMessage *string,
) error {
	query := `
		UPDATE stripe_webhook_events
		SET status = $1, error_message = $2, processed_at = NOW()
		WHERE stripe_event_id = $3
	`

	_, err := db.Pool.Exec(ctx, query, status, errorMessage, stripeEventID)
	if err != nil {
		return fmt.Errorf("failed to update stripe webhook event status: %w", err)
	}

	return nil
}
