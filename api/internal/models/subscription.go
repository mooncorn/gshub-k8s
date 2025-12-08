package models

import "time"

// SubscriptionInfo contains Stripe subscription details
type SubscriptionInfo struct {
	SubscriptionID    string     `json:"subscription_id"`
	Status            string     `json:"status"` // active, past_due, canceled, etc.
	CurrentPeriodEnd  time.Time  `json:"current_period_end"`
	CancelAtPeriodEnd bool       `json:"cancel_at_period_end"`
	CanceledAt        *time.Time `json:"canceled_at,omitempty"`
	CancelsAt         *time.Time `json:"cancels_at,omitempty"`
}

// ServerSubscription combines server info with subscription details for billing page
type ServerSubscription struct {
	ServerID     string            `json:"server_id"`
	DisplayName  string            `json:"display_name"`
	Game         GameType          `json:"game"`
	Plan         ServerPlan        `json:"plan"`
	Status       ServerStatus      `json:"status"`
	Subscription *SubscriptionInfo `json:"subscription,omitempty"`
	ExpiredAt    *time.Time        `json:"expired_at,omitempty"`
	DeleteAfter  *time.Time        `json:"delete_after,omitempty"`
}

// BillingResponse is the response for the billing page
type BillingResponse struct {
	Subscriptions []ServerSubscription `json:"subscriptions"`
}
