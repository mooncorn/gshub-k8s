package podmonitor

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"

	"github.com/mooncorn/gshub/api/internal/database"
	"github.com/mooncorn/gshub/api/internal/models"
	"github.com/mooncorn/gshub/api/internal/services/broadcast"
	"github.com/mooncorn/gshub/api/internal/services/k8s"
	"go.uber.org/zap"
)

const (
	// CrashLoopThreshold is the restart count that indicates a crash loop
	CrashLoopThreshold = 5
)

// PodMonitor watches K8s pods for container-level issues
type PodMonitor struct {
	db        *database.DB
	k8sClient *k8s.Client
	hub       *broadcast.Hub
	logger    *zap.Logger
	namespace string
	ticker    *time.Ticker
	done      chan struct{}
	interval  time.Duration
}

// NewPodMonitor creates a new pod monitor
func NewPodMonitor(db *database.DB, k8sClient *k8s.Client, hub *broadcast.Hub, logger *zap.Logger, namespace string) *PodMonitor {
	return &PodMonitor{
		db:        db,
		k8sClient: k8sClient,
		hub:       hub,
		logger:    logger,
		namespace: namespace,
		done:      make(chan struct{}),
		interval:  30 * time.Second,
	}
}

// Start begins the monitoring loop
func (m *PodMonitor) Start(ctx context.Context) {
	m.ticker = time.NewTicker(m.interval)
	go m.loop(ctx)
	m.logger.Info("Pod monitor started", zap.Duration("interval", m.interval))
}

// Stop gracefully stops the monitoring loop
func (m *PodMonitor) Stop() {
	if m.ticker != nil {
		m.ticker.Stop()
	}
	close(m.done)
	m.logger.Info("Pod monitor stopped")
}

// loop runs the monitoring loop
func (m *PodMonitor) loop(ctx context.Context) {
	for {
		select {
		case <-m.done:
			return
		case <-m.ticker.C:
			m.checkPods(ctx)
		}
	}
}

// checkPods examines all running server pods for issues
func (m *PodMonitor) checkPods(ctx context.Context) {
	// Get all running and starting servers
	runningServers, err := m.db.GetServersByStatus(ctx, string(models.ServerStatusRunning))
	if err != nil {
		m.logger.Error("failed to get running servers", zap.Error(err))
		return
	}

	startingServers, err := m.db.GetServersByStatus(ctx, string(models.ServerStatusStarting))
	if err != nil {
		m.logger.Error("failed to get starting servers", zap.Error(err))
		return
	}

	servers := append(runningServers, startingServers...)

	for _, server := range servers {
		serverID := server.ID.String()
		labelSelector := "server=" + serverID

		pod, err := m.k8sClient.GetPodByLabel(ctx, m.namespace, labelSelector)
		if err != nil {
			// Pod not found - could be scaling, stopping, or deleted
			continue
		}

		// Check container statuses
		for _, cs := range pod.Status.ContainerStatuses {
			if cs.Name != "supervisor" {
				continue
			}

			// Detect crash loop (high restart count)
			if cs.RestartCount >= CrashLoopThreshold {
				m.handleCrashLoop(ctx, &server, int(cs.RestartCount))
			}

			// Detect OOM kill from last termination state
			if cs.LastTerminationState.Terminated != nil {
				if cs.LastTerminationState.Terminated.Reason == "OOMKilled" {
					m.handleOOMKill(ctx, &server)
				}
			}

			// Detect waiting states (CrashLoopBackOff, ImagePullBackOff, etc.)
			if cs.State.Waiting != nil {
				reason := cs.State.Waiting.Reason
				if reason == "CrashLoopBackOff" || reason == "ImagePullBackOff" || reason == "ErrImagePull" {
					m.handleWaitingState(ctx, &server, reason, cs.State.Waiting.Message)
				}
			}
		}

		// Also check init container issues
		for _, cs := range pod.Status.InitContainerStatuses {
			if cs.State.Waiting != nil {
				reason := cs.State.Waiting.Reason
				if reason == "ImagePullBackOff" || reason == "ErrImagePull" {
					m.handleWaitingState(ctx, &server, reason, cs.State.Waiting.Message)
				}
			}
		}

		// Check pod phase
		if pod.Status.Phase == corev1.PodFailed {
			m.handlePodFailed(ctx, &server, pod.Status.Reason, pod.Status.Message)
		}
	}
}

// handleCrashLoop handles servers in a crash loop
func (m *PodMonitor) handleCrashLoop(ctx context.Context, server *models.Server, restartCount int) {
	serverID := server.ID.String()
	message := fmt.Sprintf("Server crash loop detected (%d restarts). Check server logs for errors.", restartCount)

	m.logger.Warn("crash loop detected",
		zap.String("server_id", serverID),
		zap.Int("restart_count", restartCount))

	// Update restart count in database
	if err := m.db.UpdateServerRestartCount(ctx, serverID, restartCount); err != nil {
		m.logger.Error("failed to update restart count", zap.Error(err), zap.String("server_id", serverID))
	}

	// Only transition to failed if still running (avoid race with other handlers)
	transitioned, _ := m.db.TransitionServerStatus(ctx, serverID,
		models.ServerStatusRunning, models.ServerStatusFailed, message)

	if transitioned {
		// Broadcast to user
		m.hub.Publish(server.UserID, broadcast.StatusEvent{
			ServerID:      serverID,
			Status:        string(models.ServerStatusFailed),
			StatusMessage: &message,
			Timestamp:     time.Now().UTC(),
		})
	}
}

// handleOOMKill handles servers that were killed due to out of memory
func (m *PodMonitor) handleOOMKill(ctx context.Context, server *models.Server) {
	serverID := server.ID.String()
	message := "Server ran out of memory (OOM killed). Consider upgrading to a larger plan."

	m.logger.Warn("OOM kill detected", zap.String("server_id", serverID))

	// Record OOM event
	if err := m.db.RecordOOMEvent(ctx, serverID); err != nil {
		m.logger.Error("failed to record OOM event", zap.Error(err), zap.String("server_id", serverID))
	}

	// Transition to failed
	transitioned, _ := m.db.TransitionServerStatus(ctx, serverID,
		models.ServerStatusRunning, models.ServerStatusFailed, message)

	if transitioned {
		m.hub.Publish(server.UserID, broadcast.StatusEvent{
			ServerID:      serverID,
			Status:        string(models.ServerStatusFailed),
			StatusMessage: &message,
			Timestamp:     time.Now().UTC(),
		})
	}
}

// handleWaitingState handles pods stuck in waiting states
func (m *PodMonitor) handleWaitingState(ctx context.Context, server *models.Server, reason, waitMessage string) {
	serverID := server.ID.String()
	message := fmt.Sprintf("%s: %s", reason, waitMessage)

	m.logger.Warn("pod in waiting state",
		zap.String("server_id", serverID),
		zap.String("reason", reason))

	// Try to transition from starting to failed
	transitioned, _ := m.db.TransitionServerStatus(ctx, serverID,
		models.ServerStatusStarting, models.ServerStatusFailed, message)

	if transitioned {
		m.hub.Publish(server.UserID, broadcast.StatusEvent{
			ServerID:      serverID,
			Status:        string(models.ServerStatusFailed),
			StatusMessage: &message,
			Timestamp:     time.Now().UTC(),
		})
	}
}

// handlePodFailed handles pods that have failed
func (m *PodMonitor) handlePodFailed(ctx context.Context, server *models.Server, reason, podMessage string) {
	serverID := server.ID.String()
	message := fmt.Sprintf("Pod failed: %s - %s", reason, podMessage)

	m.logger.Warn("pod failed",
		zap.String("server_id", serverID),
		zap.String("reason", reason))

	// Try to transition from either running or starting to failed
	transitioned, _ := m.db.TransitionServerStatus(ctx, serverID,
		models.ServerStatusRunning, models.ServerStatusFailed, message)

	if !transitioned {
		transitioned, _ = m.db.TransitionServerStatus(ctx, serverID,
			models.ServerStatusStarting, models.ServerStatusFailed, message)
	}

	if transitioned {
		m.hub.Publish(server.UserID, broadcast.StatusEvent{
			ServerID:      serverID,
			Status:        string(models.ServerStatusFailed),
			StatusMessage: &message,
			Timestamp:     time.Now().UTC(),
		})
	}
}
