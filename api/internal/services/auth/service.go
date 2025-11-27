package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/mooncorn/gshub/api/config"
	"github.com/mooncorn/gshub/api/internal/database"
	"github.com/mooncorn/gshub/api/internal/models"
	"golang.org/x/crypto/bcrypt"
)

type Service struct {
	db     *database.DB
	config *config.Config
}

func NewService(db *database.DB, cfg *config.Config) *Service {
	return &Service{
		db:     db,
		config: cfg,
	}
}

type Claims struct {
	UserID string `json:"user_id"`
	Email  string `json:"email"`
	jwt.RegisteredClaims
}

// HashPassword hashes a password using bcrypt
func (s *Service) HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

// ComparePassword compares a password with its hash
func (s *Service) ComparePassword(hash, password string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
}

// GenerateAccessToken generates a JWT access token
func (s *Service) GenerateAccessToken(user *models.User) (string, error) {
	claims := &Claims{
		UserID: user.ID.String(),
		Email:  user.Email,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(s.config.JWTAccessExpiry)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(s.config.JWTSecret))
}

// GenerateRefreshToken generates a random refresh token
func (s *Service) GenerateRefreshToken() (string, error) {
	b := make([]byte, 32)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

// SaveRefreshToken saves a refresh token to the database
func (s *Service) SaveRefreshToken(ctx context.Context, userID string, token string) error {
	parsedUserID, err := uuid.Parse(userID)
	if err != nil {
		return fmt.Errorf("invalid user ID format: %w", err)
	}
	expiresAt := time.Now().Add(s.config.JWTRefreshExpiry)
	return s.db.CreateRefreshToken(ctx, parsedUserID, token, expiresAt)
}

// ValidateRefreshToken validates a refresh token and returns the user ID
func (s *Service) ValidateRefreshToken(ctx context.Context, token string) (string, error) {
	refreshToken, err := s.db.GetRefreshToken(ctx, token)
	if err != nil {
		return "", fmt.Errorf("invalid refresh token")
	}

	if time.Now().After(refreshToken.ExpiresAt) {
		return "", fmt.Errorf("refresh token expired")
	}

	return refreshToken.UserID.String(), nil
}

// DeleteRefreshToken removes a refresh token
func (s *Service) DeleteRefreshToken(ctx context.Context, token string) error {
	return s.db.DeleteRefreshToken(ctx, token)
}

// DeleteUserRefreshTokens removes all refresh tokens for a user
func (s *Service) DeleteUserRefreshTokens(ctx context.Context, userID string) error {
	parsedUserID, err := uuid.Parse(userID)
	if err != nil {
		return fmt.Errorf("invalid user ID format: %w", err)
	}
	return s.db.DeleteUserRefreshTokens(ctx, parsedUserID)
}

// GenerateVerificationToken generates and saves an email verification token
func (s *Service) GenerateVerificationToken(ctx context.Context, userID string) (string, error) {
	parsedUserID, err := uuid.Parse(userID)
	if err != nil {
		return "", fmt.Errorf("invalid user ID format: %w", err)
	}

	token, err := s.GenerateRefreshToken()
	if err != nil {
		return "", err
	}
	expiresAt := time.Now().Add(24 * time.Hour)
	_, err = s.db.CreateEmailVerificationToken(ctx, parsedUserID, token, expiresAt)
	if err != nil {
		return "", err
	}

	return token, nil
}

// ValidateVerificationToken validates an email verification token and returns the user ID
func (s *Service) ValidateVerificationToken(ctx context.Context, token string) (string, error) {
	userID, expiresAt, err := s.db.GetEmailVerificationToken(ctx, token)
	if err != nil {
		return "", fmt.Errorf("invalid verification token")
	}

	if time.Now().After(expiresAt) {
		return "", fmt.Errorf("verification token expired")
	}

	// Delete the token after validation (single use)
	if err := s.db.DeleteEmailVerificationToken(ctx, token); err != nil {
		return "", fmt.Errorf("failed to consume verification token")
	}

	return userID.String(), nil
}

// GeneratePasswordResetToken generates and saves a password reset token
func (s *Service) GeneratePasswordResetToken(ctx context.Context, userID string) (string, error) {
	parsedUserID, err := uuid.Parse(userID)
	if err != nil {
		return "", fmt.Errorf("invalid user ID format: %w", err)
	}

	token, err := s.GenerateRefreshToken()
	if err != nil {
		return "", fmt.Errorf("failed to generate refresh token: %w", err)
	}
	expiresAt := time.Now().Add(1 * time.Hour)
	_, err = s.db.CreatePasswordResetToken(ctx, parsedUserID, token, expiresAt)
	if err != nil {
		return "", err
	}

	return token, nil
}

// ValidatePasswordResetToken validates a password reset token and returns the user ID
func (s *Service) ValidatePasswordResetToken(ctx context.Context, token string) (string, error) {
	userID, expiresAt, used, err := s.db.GetPasswordResetToken(ctx, token)
	if err != nil {
		return "", fmt.Errorf("invalid reset token")
	}

	if used {
		return "", fmt.Errorf("reset token already used")
	}

	if time.Now().After(expiresAt) {
		return "", fmt.Errorf("reset token expired")
	}

	return userID.String(), nil
}

// MarkPasswordResetTokenUsed marks a token as used
func (s *Service) MarkPasswordResetTokenUsed(ctx context.Context, token string) error {
	return s.db.MarkPasswordResetTokenUsed(ctx, token)
}

// CreateUser creates a new user with hashed password
func (s *Service) CreateUser(ctx context.Context, email, password string) (*models.User, error) {
	passwordHash, err := s.HashPassword(password)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	return s.db.CreateUser(ctx, email, passwordHash)
}

// GetUserByEmail retrieves a user by email
func (s *Service) GetUserByEmail(ctx context.Context, email string) (*models.User, error) {
	return s.db.GetUserByEmail(ctx, email)
}

// GetUserByID retrieves a user by ID
func (s *Service) GetUserByID(ctx context.Context, userID string) (*models.User, error) {
	parsedUserID, err := uuid.Parse(userID)
	if err != nil {
		return nil, fmt.Errorf("invalid user ID format: %w", err)
	}

	return s.db.GetUserByID(ctx, parsedUserID)
}

// VerifyEmail marks a user's email as verified
func (s *Service) VerifyEmail(ctx context.Context, userID string) error {
	parsedUserID, err := uuid.Parse(userID)
	if err != nil {
		return fmt.Errorf("invalid user ID format: %w", err)
	}

	return s.db.MarkEmailVerified(ctx, parsedUserID)
}

// UpdatePassword updates a user's password
func (s *Service) UpdatePassword(ctx context.Context, userID string, newPassword string) error {
	passwordHash, err := s.HashPassword(newPassword)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	parsedUserID, err := uuid.Parse(userID)
	if err != nil {
		return fmt.Errorf("invalid user ID format: %w", err)
	}

	return s.db.UpdateUserPassword(ctx, parsedUserID, passwordHash)
}
