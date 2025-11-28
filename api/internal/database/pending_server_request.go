package database

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/mooncorn/gshub/api/internal/models"
)

// CreatePendingServerRequest creates a new pending server request
func (db *DB) CreatePendingServerRequest(
	ctx context.Context,
	userID uuid.UUID,
	displayName *string,
	subdomain string,
	game string,
	plan string,
) (*uuid.UUID, error) {
	var id uuid.UUID

	query := `
		INSERT INTO pending_server_requests
		(user_id, display_name, subdomain, game, plan, status)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id
	`

	err := db.Pool.QueryRow(ctx, query, userID, displayName, subdomain, game, plan).Scan(&id)
	if err != nil {
		return nil, fmt.Errorf("failed to create pending server request: %w", err)
	}

	return &id, nil
}

// GetPendingServerRequest retrieves a pending server request by ID
func (db *DB) GetPendingServerRequest(ctx context.Context, id uuid.UUID) (*models.PendingServerRequest, error) {
	query := `
		SELECT
			id, user_id, display_name, subdomain, game, plan,
			stripe_session_id, status, server_id, created_at, updated_at, expires_at
		FROM pending_server_requests
		WHERE id = $1
	`

	row := db.Pool.QueryRow(ctx, query, id)
	psr := &models.PendingServerRequest{}

	err := row.Scan(
		&psr.ID, &psr.UserID, &psr.DisplayName, &psr.Subdomain, &psr.Game, &psr.Plan,
		&psr.StripeSessionID, &psr.Status, &psr.ServerID, &psr.CreatedAt, &psr.UpdatedAt, &psr.ExpiresAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get pending server request: %w", err)
	}

	return psr, nil
}

// GetPendingServerRequestByStripeSession retrieves a pending server request by Stripe session ID
func (db *DB) GetPendingServerRequestByStripeSession(ctx context.Context, sessionID string) (*models.PendingServerRequest, error) {
	query := `
		SELECT
			id, user_id, display_name, subdomain, game, plan,
			stripe_session_id, status, server_id, created_at, updated_at, expires_at
		FROM pending_server_requests
		WHERE stripe_session_id = $1
	`

	row := db.Pool.QueryRow(ctx, query, sessionID)
	psr := &models.PendingServerRequest{}

	err := row.Scan(
		&psr.ID, &psr.UserID, &psr.DisplayName, &psr.Subdomain, &psr.Game, &psr.Plan,
		&psr.StripeSessionID, &psr.Status, &psr.ServerID, &psr.CreatedAt, &psr.UpdatedAt, &psr.ExpiresAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get pending server request by stripe session: %w", err)
	}

	return psr, nil
}

// UpdatePendingServerRequestWithSession updates the Stripe session ID
func (db *DB) UpdatePendingServerRequestWithSession(ctx context.Context, id uuid.UUID, sessionID string) error {
	query := `
		UPDATE pending_server_requests
		SET stripe_session_id = $1, updated_at = NOW()
		WHERE id = $2
	`

	_, err := db.Pool.Exec(ctx, query, sessionID, id)
	if err != nil {
		return fmt.Errorf("failed to update pending server request with session: %w", err)
	}

	return nil
}

// MarkPendingServerRequestCompleted marks a pending request as completed and links the server
func (db *DB) MarkPendingServerRequestCompleted(ctx context.Context, id uuid.UUID, serverID uuid.UUID) error {
	query := `
		UPDATE pending_server_requests
		SET status = $1, server_id = $2, updated_at = NOW()
		WHERE id = $3
	`

	_, err := db.Pool.Exec(ctx, query, models.PendingStatusCompleted, serverID, id)
	if err != nil {
		return fmt.Errorf("failed to mark pending server request as completed: %w", err)
	}

	return nil
}

// MarkPendingServerRequestFailed marks a pending request as failed
func (db *DB) MarkPendingServerRequestFailed(ctx context.Context, id uuid.UUID) error {
	query := `
		UPDATE pending_server_requests
		SET status = $1, updated_at = NOW()
		WHERE id = $2
	`

	_, err := db.Pool.Exec(ctx, query, models.PendingStatusFailed, id)
	if err != nil {
		return fmt.Errorf("failed to mark pending server request as failed: %w", err)
	}

	return nil
}

// SubdomainExists checks if a subdomain is already taken (in servers or pending requests)
func (db *DB) SubdomainExists(ctx context.Context, subdomain string) (bool, error) {
	query := `
		SELECT EXISTS(
			SELECT 1 FROM servers WHERE subdomain = $1
			UNION
			SELECT 1 FROM pending_server_requests WHERE subdomain = $1
		)
	`

	var exists bool
	err := db.Pool.QueryRow(ctx, query, subdomain).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("failed to check subdomain existence: %w", err)
	}

	return exists, nil
}
