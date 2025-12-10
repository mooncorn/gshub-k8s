package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"go.uber.org/zap"
)

// Status represents the server status values
type Status string

const (
	StatusStarting Status = "starting"
	StatusRunning  Status = "running"
	StatusStopping Status = "stopping"
	StatusStopped  Status = "stopped"
	StatusFailed   Status = "failed"
)

// StatusUpdateRequest is sent to report status changes
type StatusUpdateRequest struct {
	Status     Status `json:"status"`
	Message    string `json:"message,omitempty"`
	ProcessPID int    `json:"process_pid,omitempty"`
}

// HeartbeatRequest is sent periodically while running
type HeartbeatRequest struct {
	ProcessPID int     `json:"process_pid"`
	MemoryMB   int64   `json:"memory_mb,omitempty"`
	CPUPercent float64 `json:"cpu_percent,omitempty"`
}

// Client communicates with the gshub API internal endpoint
type Client struct {
	httpClient  *http.Client
	baseURL     string
	serverID    string
	authToken   string
	logger      *zap.Logger
}

// NewClient creates a new API client
func NewClient(baseURL, serverID, authToken string, logger *zap.Logger) *Client {
	return &Client{
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		baseURL:   baseURL,
		serverID:  serverID,
		authToken: authToken,
		logger:    logger,
	}
}

// ReportStatus sends a status update to the API
func (c *Client) ReportStatus(ctx context.Context, status Status, message string, pid int) error {
	req := StatusUpdateRequest{
		Status:     status,
		Message:    message,
		ProcessPID: pid,
	}

	url := fmt.Sprintf("%s/internal/servers/%s/status", c.baseURL, c.serverID)
	return c.post(ctx, url, req)
}

// SendHeartbeat sends a heartbeat to the API
func (c *Client) SendHeartbeat(ctx context.Context, pid int, memoryMB int64, cpuPercent float64) error {
	req := HeartbeatRequest{
		ProcessPID: pid,
		MemoryMB:   memoryMB,
		CPUPercent: cpuPercent,
	}

	url := fmt.Sprintf("%s/internal/servers/%s/heartbeat", c.baseURL, c.serverID)
	return c.post(ctx, url, req)
}

// post sends a POST request with JSON body
func (c *Client) post(ctx context.Context, url string, body interface{}) error {
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("failed to marshal request body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(jsonBody))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.authToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	return nil
}

// ReportStatusWithRetry sends a status update with retries
func (c *Client) ReportStatusWithRetry(ctx context.Context, status Status, message string, pid int, maxRetries int) {
	for i := 0; i <= maxRetries; i++ {
		err := c.ReportStatus(ctx, status, message, pid)
		if err == nil {
			c.logger.Info("reported status",
				zap.String("status", string(status)),
				zap.String("message", message),
				zap.Int("pid", pid))
			return
		}

		c.logger.Warn("failed to report status, retrying",
			zap.Error(err),
			zap.Int("attempt", i+1),
			zap.Int("max_retries", maxRetries))

		if i < maxRetries {
			select {
			case <-ctx.Done():
				return
			case <-time.After(time.Duration(i+1) * time.Second):
				// Exponential backoff
			}
		}
	}

	c.logger.Error("failed to report status after retries",
		zap.String("status", string(status)),
		zap.Int("max_retries", maxRetries))
}
