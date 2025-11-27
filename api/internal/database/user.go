package database

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/mooncorn/gshub/api/internal/models"
)

// CreateUser inserts a new user and returns the user model
func (db *DB) CreateUser(ctx context.Context, email, passwordHash string) (*models.User, error) {
	query := `
		INSERT INTO users (email, password_hash)
		VALUES ($1, $2)
		RETURNING id, email, password_hash, email_verified, stripe_customer_id, created_at, updated_at
	`

	var user models.User
	err := db.Pool.QueryRow(ctx, query, email, passwordHash).Scan(
		&user.ID,
		&user.Email,
		&user.PasswordHash,
		&user.EmailVerified,
		&user.StripeCustomerID,
		&user.CreatedAt,
		&user.UpdatedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	return &user, nil
}

// GetUserByEmail retrieves a user by email address
func (db *DB) GetUserByEmail(ctx context.Context, email string) (*models.User, error) {
	query := `
		SELECT id, email, password_hash, email_verified, stripe_customer_id, created_at, updated_at
		FROM users
		WHERE email = $1
	`

	var user models.User
	err := db.Pool.QueryRow(ctx, query, email).Scan(
		&user.ID,
		&user.Email,
		&user.PasswordHash,
		&user.EmailVerified,
		&user.StripeCustomerID,
		&user.CreatedAt,
		&user.UpdatedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("user not found: %w", err)
	}

	return &user, nil
}

// GetUserByID retrieves a user by ID
func (db *DB) GetUserByID(ctx context.Context, userID uuid.UUID) (*models.User, error) {
	query := `
		SELECT id, email, password_hash, email_verified, stripe_customer_id, created_at, updated_at
		FROM users
		WHERE id = $1
	`

	var user models.User
	err := db.Pool.QueryRow(ctx, query, userID).Scan(
		&user.ID,
		&user.Email,
		&user.PasswordHash,
		&user.EmailVerified,
		&user.StripeCustomerID,
		&user.CreatedAt,
		&user.UpdatedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("user not found: %w", err)
	}

	return &user, nil
}

// MarkEmailVerified sets email_verified to true for a user
func (db *DB) MarkEmailVerified(ctx context.Context, userID uuid.UUID) error {
	query := `
		UPDATE users
		SET email_verified = true,
		    updated_at = NOW()
		WHERE id = $1
	`

	_, err := db.Pool.Exec(ctx, query, userID)
	if err != nil {
		return fmt.Errorf("failed to mark email verified: %w", err)
	}

	return nil
}

// UpdateUserPassword updates a user's password hash
func (db *DB) UpdateUserPassword(ctx context.Context, userID uuid.UUID, passwordHash string) error {
	query := `
		UPDATE users
		SET password_hash = $2,
		    updated_at = NOW()
		WHERE id = $1
	`

	_, err := db.Pool.Exec(ctx, query, userID, passwordHash)
	if err != nil {
		return fmt.Errorf("failed to update password: %w", err)
	}

	return nil
}

// CreateRefreshToken creates a refresh token in the database
// Return models.RefreshToken
func (db *DB) CreateRefreshToken(ctx context.Context, userID uuid.UUID, token string, expiresAt time.Time) error {
	query := `
		INSERT INTO refresh_tokens (user_id, token, expires_at)
		VALUES ($1, $2, $3)
	`

	_, err := db.Pool.Exec(ctx, query, userID, token, expiresAt)
	if err != nil {
		return fmt.Errorf("failed to save refresh token: %w", err)
	}

	return nil
}

// GetRefreshToken retrieves a refresh token with its user ID and expiry
func (db *DB) GetRefreshToken(ctx context.Context, token string) (*models.RefreshToken, error) {
	query := `
		SELECT id, user_id, token, expires_at, created_at
		FROM refresh_tokens
		WHERE token = $1
	`

	var refreshToken models.RefreshToken
	err := db.Pool.QueryRow(ctx, query, token).Scan(
		&refreshToken.ID,
		&refreshToken.UserID,
		&refreshToken.Token,
		&refreshToken.ExpiresAt,
		&refreshToken.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("refresh token not found: %w", err)
	}

	return &refreshToken, nil
}

// DeleteRefreshToken removes a specific refresh token
func (db *DB) DeleteRefreshToken(ctx context.Context, token string) error {
	query := `DELETE FROM refresh_tokens WHERE token = $1`

	_, err := db.Pool.Exec(ctx, query, token)
	if err != nil {
		return fmt.Errorf("failed to delete refresh token: %w", err)
	}

	return nil
}

// DeleteUserRefreshTokens removes all refresh tokens for a user
func (db *DB) DeleteUserRefreshTokens(ctx context.Context, userID uuid.UUID) error {
	query := `DELETE FROM refresh_tokens WHERE user_id = $1`

	_, err := db.Pool.Exec(ctx, query, userID)
	if err != nil {
		return fmt.Errorf("failed to delete user refresh tokens: %w", err)
	}

	return nil
}

// CreateEmailVerificationToken creates an email verification token
func (db *DB) CreateEmailVerificationToken(ctx context.Context, userID uuid.UUID, token string, expiresAt time.Time) (*models.EmailVerificationToken, error) {
	query := `
		INSERT INTO email_verification_tokens (user_id, token, expires_at)
		VALUES ($1, $2, $3)
		RETURNING id, user_id, token, expires_at, created_at
	`

	var emailToken models.EmailVerificationToken
	err := db.Pool.QueryRow(ctx, query, userID, token, expiresAt).Scan(
		&emailToken.ID,
		&emailToken.UserID,
		&emailToken.Token,
		&emailToken.ExpiresAt,
		&emailToken.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to save verification token: %w", err)
	}

	return &emailToken, nil
}

// GetEmailVerificationToken retrieves an email verification token with its user ID and expiry
// TODO: Return models.EmailVerificationToken
func (db *DB) GetEmailVerificationToken(ctx context.Context, token string) (userID uuid.UUID, expiresAt time.Time, err error) {
	query := `
		SELECT user_id, expires_at
		FROM email_verification_tokens
		WHERE token = $1
	`

	err = db.Pool.QueryRow(ctx, query, token).Scan(&userID, &expiresAt)
	if err != nil {
		return uuid.Nil, time.Time{}, fmt.Errorf("verification token not found: %w", err)
	}

	return userID, expiresAt, nil
}

// DeleteEmailVerificationToken deletes an email verification token
func (db *DB) DeleteEmailVerificationToken(ctx context.Context, token string) error {
	query := `DELETE FROM email_verification_tokens WHERE token = $1`

	_, err := db.Pool.Exec(ctx, query, token)
	if err != nil {
		return fmt.Errorf("failed to delete verification token: %w", err)
	}

	return nil
}

// CreatePasswordResetToken creates a password reset token
func (db *DB) CreatePasswordResetToken(ctx context.Context, userID uuid.UUID, token string, expiresAt time.Time) (*models.PasswordResetToken, error) {
	query := `
		INSERT INTO password_reset_tokens (user_id, token, expires_at, used)
		VALUES ($1, $2, $3, false)
		RETURNING id, user_id, token, expires_at, used, created_at
	`

	var resetToken models.PasswordResetToken
	err := db.Pool.QueryRow(ctx, query, userID, token, expiresAt).Scan(
		&resetToken.ID,
		&resetToken.UserID,
		&resetToken.Token,
		&resetToken.ExpiresAt,
		&resetToken.Used,
		&resetToken.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to save reset token: %w", err)
	}

	return &resetToken, nil
}

// GetPasswordResetToken retrieves a password reset token with its details
// TODO: Return models.PasswordResetToken
func (db *DB) GetPasswordResetToken(ctx context.Context, token string) (userID uuid.UUID, expiresAt time.Time, used bool, err error) {
	query := `
		SELECT user_id, expires_at, used
		FROM password_reset_tokens
		WHERE token = $1
	`

	err = db.Pool.QueryRow(ctx, query, token).Scan(&userID, &expiresAt, &used)
	if err != nil {
		return uuid.Nil, time.Time{}, false, fmt.Errorf("reset token not found: %w", err)
	}

	return userID, expiresAt, used, nil
}

// MarkPasswordResetTokenUsed marks a password reset token as used
func (db *DB) MarkPasswordResetTokenUsed(ctx context.Context, token string) error {
	query := `
		UPDATE password_reset_tokens
		SET used = true
		WHERE token = $1
	`

	_, err := db.Pool.Exec(ctx, query, token)
	if err != nil {
		return fmt.Errorf("failed to mark reset token as used: %w", err)
	}

	return nil
}
