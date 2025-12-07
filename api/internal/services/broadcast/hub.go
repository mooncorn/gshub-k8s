package broadcast

import (
	"sync"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// StatusEvent represents a server status change event
type StatusEvent struct {
	ServerID      string    `json:"server_id"`
	Status        string    `json:"status"`
	StatusMessage *string   `json:"status_message,omitempty"`
	Timestamp     time.Time `json:"timestamp"`
}

// Hub manages SSE client subscriptions and broadcasts status events
type Hub struct {
	mu          sync.RWMutex
	subscribers map[uuid.UUID]map[chan StatusEvent]struct{} // userID -> set of channels
	logger      *zap.Logger
	bufferSize  int
}

// NewHub creates a new broadcast hub
func NewHub(logger *zap.Logger) *Hub {
	return &Hub{
		subscribers: make(map[uuid.UUID]map[chan StatusEvent]struct{}),
		logger:      logger,
		bufferSize:  10, // Buffer to handle burst events
	}
}

// Subscribe creates a new subscription for a user and returns a channel to receive events
func (h *Hub) Subscribe(userID uuid.UUID) chan StatusEvent {
	h.mu.Lock()
	defer h.mu.Unlock()

	ch := make(chan StatusEvent, h.bufferSize)

	if h.subscribers[userID] == nil {
		h.subscribers[userID] = make(map[chan StatusEvent]struct{})
	}
	h.subscribers[userID][ch] = struct{}{}

	h.logger.Debug("client subscribed",
		zap.String("user_id", userID.String()),
		zap.Int("total_subscribers", len(h.subscribers[userID])),
	)

	return ch
}

// Unsubscribe removes a subscription for a user
func (h *Hub) Unsubscribe(userID uuid.UUID, ch chan StatusEvent) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if subs, ok := h.subscribers[userID]; ok {
		if _, exists := subs[ch]; exists {
			delete(subs, ch)
			close(ch)

			// Clean up empty user entry
			if len(subs) == 0 {
				delete(h.subscribers, userID)
			}

			h.logger.Debug("client unsubscribed",
				zap.String("user_id", userID.String()),
			)
		}
	}
}

// Publish sends an event to all subscribers for a specific user
// Non-blocking: drops events if client buffer is full
func (h *Hub) Publish(userID uuid.UUID, event StatusEvent) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	subs, ok := h.subscribers[userID]
	if !ok {
		return // No subscribers for this user
	}

	for ch := range subs {
		select {
		case ch <- event:
			// Event sent successfully
		default:
			// Buffer full, drop event (client is slow)
			h.logger.Warn("dropping event, client buffer full",
				zap.String("user_id", userID.String()),
				zap.String("server_id", event.ServerID),
				zap.String("status", event.Status),
			)
		}
	}
}

// SubscriberCount returns the number of active subscribers for a user
func (h *Hub) SubscriberCount(userID uuid.UUID) int {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if subs, ok := h.subscribers[userID]; ok {
		return len(subs)
	}
	return 0
}

// TotalSubscriberCount returns the total number of active subscribers across all users
func (h *Hub) TotalSubscriberCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()

	count := 0
	for _, subs := range h.subscribers {
		count += len(subs)
	}
	return count
}
