package database

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/mooncorn/gshub/api/internal/models"
)

func (db *DB) CreateUser(ctx context.Context, email, passwordHash string) (*models.User, error) {
	user := &models.User{}

	err := db.Pool.QueryRow(ctx, `
          INSERT INTO users (email, password_hash)
          VALUES ($1, $2)
          RETURNING id, email, stripe_customer_id, created_at, updated_at
      `, email, passwordHash).Scan(
		&user.ID,
		&user.Email,
		&user.StripeCustomerID,
		&user.CreatedAt,
		&user.UpdatedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	return user, nil
}

func (db *DB) GetUserByEmail(ctx context.Context, email string) (*models.User, error) {
	user := &models.User{}

	err := db.Pool.QueryRow(ctx, `
          SELECT id, email, password_hash, stripe_customer_id, created_at, updated_at
          FROM users
          WHERE email = $1
      `, email).Scan(
		&user.ID,
		&user.Email,
		&user.PasswordHash,
		&user.StripeCustomerID,
		&user.CreatedAt,
		&user.UpdatedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("user not found: %w", err)
	}

	return user, nil
}

func (db *DB) GetUserByID(ctx context.Context, userID uuid.UUID) (*models.User, error) {
	user := &models.User{}

	err := db.Pool.QueryRow(ctx, `
          SELECT id, email, stripe_customer_id, created_at, updated_at
          FROM users
          WHERE id = $1
      `, userID).Scan(
		&user.ID,
		&user.Email,
		&user.StripeCustomerID,
		&user.CreatedAt,
		&user.UpdatedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("user not found: %w", err)
	}

	return user, nil
}
