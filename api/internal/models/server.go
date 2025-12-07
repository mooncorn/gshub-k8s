package models

import (
	"time"

	"github.com/google/uuid"
)

// Server represents a game server instance
type Server struct {
	ID                   uuid.UUID      `json:"id"`
	UserID               uuid.UUID      `json:"user_id"`
	DisplayName          string         `json:"display_name"`
	Game                 GameType       `json:"game"`
	Subdomain            string         `json:"subdomain"`
	Plan                 ServerPlan     `json:"plan"`
	Status               ServerStatus   `json:"status"`
	StatusMessage        *string        `json:"status_message,omitempty"`
	CreationError        *string        `json:"creation_error,omitempty"`
	LastReconciled       *time.Time     `json:"last_reconciled,omitempty"`
	Volumes              []ServerVolume `json:"volumes,omitempty"`
	Ports                []ServerPort   `json:"ports,omitempty"`
	StripeSubscriptionID *string        `json:"stripe_subscription_id,omitempty"`
	CreatedAt            time.Time      `json:"created_at"`
	UpdatedAt            time.Time      `json:"updated_at"`
	StoppedAt            *time.Time     `json:"stopped_at,omitempty"`
	ExpiredAt            *time.Time     `json:"expired_at,omitempty"`
	DeleteAfter          *time.Time     `json:"delete_after,omitempty"`
}

// ServerPort represents a single port configuration
type ServerPort struct {
	ID            uuid.UUID `json:"id"`
	ServerID      string    `json:"server_id"`
	Name          string    `json:"name"`                // "game", "query", "rcon"
	ContainerPort int       `json:"container_port"`      // Port in container
	HostPort      *int      `json:"host_port,omitempty"` // Allocated host port
	NodeIP        *string   `json:"node_ip,omitempty"`   // Public IP of hosting node
	Protocol      string    `json:"protocol"`            // "TCP" or "UDP"
	CreatedAt     time.Time `json:"created_at"`
}

// ServerVolume represents a single volume mount
type ServerVolume struct {
	ID        uuid.UUID `json:"id"`
	ServerID  string    `json:"server_id"`
	Name      string    `json:"name"`       // "data", "logs", "config"
	MountPath string    `json:"mount_path"` // "/data", "/logs"
	SubPath   string    `json:"sub_path"`   // Subdirectory in PVC
	CreatedAt time.Time `json:"created_at"`
}

// Server lifecycle status constants
type ServerStatus string

const (
	ServerStatusPending  ServerStatus = "pending"  // Server created in DB, K8s resources not yet created
	ServerStatusStarting ServerStatus = "starting" // K8s GameServer created, waiting for pod Ready
	ServerStatusRunning  ServerStatus = "running"  // K8s pod is running and healthy
	ServerStatusStopping ServerStatus = "stopping" // Stop requested, waiting for K8s deletion
	ServerStatusStopped  ServerStatus = "stopped"  // User stopped the server (pod deleted, PVC preserved)
	ServerStatusExpired  ServerStatus = "expired"  // Subscription expired, server stopped
	ServerStatusFailed   ServerStatus = "failed"   // Something went wrong during creation/runtime
	ServerStatusDeleting ServerStatus = "deleting" // Hard delete in progress, PVC being deleted
	ServerStatusDeleted  ServerStatus = "deleted"  // All resources cleaned up, ready for DB deletion
)

// Game type constants
type GameType string

const (
	GameMinecraft GameType = "minecraft"
	GameValheim   GameType = "valheim"
	GameRust      GameType = "rust"
	GameARK       GameType = "ark"
)

// Server plan constants (for future billing tiers)
type ServerPlan string

const (
	PlanSmall  ServerPlan = "small"
	PlanMedium ServerPlan = "medium"
	PlanLarge  ServerPlan = "large"
)

// CreateServerRequest is the payload for creating a new server
type CreateServerRequest struct {
	DisplayName string `json:"display_name" binding:"omitempty,min=3,max=50"` // Optional
	Subdomain   string `json:"subdomain" binding:"required,min=3,max=50,dns"`
	Game        string `json:"game" binding:"required,oneof=minecraft valheim"`
	Plan        string `json:"plan" binding:"required,oneof=small medium large"`
}

// UpdateServerRequest is the payload for updating server details
type UpdateServerRequest struct {
	DisplayName *string `json:"display_name,omitempty" binding:"omitempty,min=3,max=50"`
}

// ServerListResponse is the response for listing servers
type ServerListResponse struct {
	Servers []Server `json:"servers"`
	Total   int      `json:"total"`
}
