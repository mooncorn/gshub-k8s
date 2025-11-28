package models

import (
	"time"

	"github.com/google/uuid"
)

// PendingServerRequest represents a server creation request waiting for payment
type PendingServerRequest struct {
	ID              uuid.UUID     `json:"id"`
	UserID          uuid.UUID     `json:"user_id"`
	DisplayName     *string       `json:"display_name,omitempty"`
	Subdomain       string        `json:"subdomain"`
	Game            string        `json:"game"`
	Plan            string        `json:"plan"`
	StripeSessionID *string       `json:"stripe_session_id,omitempty"`
	Status          PaymentStatus `json:"status"` // awaiting_payment, completed, failed, expired
	ServerID        *uuid.UUID    `json:"server_id,omitempty"`
	CreatedAt       time.Time     `json:"created_at"`
	UpdatedAt       time.Time     `json:"updated_at"`
	ExpiresAt       time.Time     `json:"expires_at"`
}

type PaymentStatus string

// PendingServerRequest status constants
const (
	PendingStatusAwaitingPayment PaymentStatus = "awaiting_payment"
	PendingStatusCompleted       PaymentStatus = "completed"
	PendingStatusFailed          PaymentStatus = "failed"
	PendingStatusExpired         PaymentStatus = "expired"
)
