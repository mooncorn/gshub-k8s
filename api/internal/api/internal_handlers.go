package api

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mooncorn/gshub/api/internal/database"
	"github.com/mooncorn/gshub/api/internal/models"
	"github.com/mooncorn/gshub/api/internal/services/broadcast"
	"go.uber.org/zap"
)

// Helper to convert string pointer
func stringPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// InternalHandler handles internal API requests from supervisors
type InternalHandler struct {
	db     *database.DB
	hub    *broadcast.Hub
	logger *zap.Logger
}

// NewInternalHandler creates a new internal handler
func NewInternalHandler(db *database.DB, hub *broadcast.Hub, logger *zap.Logger) *InternalHandler {
	return &InternalHandler{
		db:     db,
		hub:    hub,
		logger: logger,
	}
}

// RegisterInternalRoutes registers internal API routes
func (h *InternalHandler) RegisterInternalRoutes(r *gin.Engine) {
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "healthy"})
	})

	internal := r.Group("/internal")
	internal.Use(h.authMiddleware())
	{
		internal.POST("/servers/:id/status", h.UpdateStatus)
		internal.POST("/servers/:id/heartbeat", h.Heartbeat)
	}
}

// authMiddleware validates the supervisor auth token
func (h *InternalHandler) authMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		serverID := c.Param("id")
		if serverID == "" {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "server ID required"})
			return
		}

		// Extract bearer token
		authHeader := c.GetHeader("Authorization")
		if len(authHeader) < 8 || authHeader[:7] != "Bearer " {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid authorization header"})
			return
		}
		token := authHeader[7:]

		// Validate token
		valid, err := h.db.ValidateServerAuthToken(c.Request.Context(), serverID, token)
		if err != nil {
			h.logger.Error("failed to validate auth token", zap.Error(err), zap.String("server_id", serverID))
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
			return
		}

		if !valid {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			return
		}

		c.Set("server_id", serverID)
		c.Next()
	}
}

// StatusUpdateRequest represents a status update from the supervisor
type StatusUpdateRequest struct {
	Status     string `json:"status" binding:"required"`
	Message    string `json:"message"`
	ProcessPID int    `json:"process_pid"`
}

// UpdateStatus handles status updates from supervisors
func (h *InternalHandler) UpdateStatus(c *gin.Context) {
	serverID := c.GetString("server_id")

	var req StatusUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	// Map supervisor status to model status
	var toStatus models.ServerStatus
	switch req.Status {
	case "starting":
		toStatus = models.ServerStatusStarting
	case "running":
		toStatus = models.ServerStatusRunning
	case "stopping":
		toStatus = models.ServerStatusStopping
	case "stopped":
		toStatus = models.ServerStatusStopped
	case "failed":
		toStatus = models.ServerStatusFailed
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid status"})
		return
	}

	// Get current server to find user ID for broadcasting
	server, err := h.db.GetServerByID(c.Request.Context(), serverID)
	if err != nil {
		h.logger.Error("failed to get server", zap.Error(err), zap.String("server_id", serverID))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	// Transition status (allow from any status for flexibility)
	err = h.db.UpdateServerStatusAny(c.Request.Context(), serverID, toStatus, req.Message)
	if err != nil {
		h.logger.Error("failed to update status", zap.Error(err), zap.String("server_id", serverID))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update status"})
		return
	}

	h.logger.Info("server status updated",
		zap.String("server_id", serverID),
		zap.String("status", req.Status),
		zap.String("message", req.Message),
		zap.Int("pid", req.ProcessPID))

	// Broadcast status update to connected clients
	h.hub.Publish(server.UserID, broadcast.StatusEvent{
		ServerID:      serverID,
		Status:        string(toStatus),
		StatusMessage: stringPtr(req.Message),
		Timestamp:     time.Now().UTC(),
	})

	c.JSON(http.StatusOK, gin.H{"status": "updated"})
}

// HeartbeatRequest represents a heartbeat from the supervisor
type HeartbeatRequest struct {
	ProcessPID int     `json:"process_pid"`
	MemoryMB   int64   `json:"memory_mb"`
	CPUPercent float64 `json:"cpu_percent"`
}

// Heartbeat handles heartbeat requests from supervisors
func (h *InternalHandler) Heartbeat(c *gin.Context) {
	serverID := c.GetString("server_id")

	var req HeartbeatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	// Update heartbeat timestamp
	if err := h.db.UpdateServerHeartbeat(c.Request.Context(), serverID); err != nil {
		h.logger.Error("failed to update heartbeat", zap.Error(err), zap.String("server_id", serverID))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update heartbeat"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
