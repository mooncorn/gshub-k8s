package process

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"

	"github.com/mooncorn/gshub/supervisor/internal/api"
	"github.com/mooncorn/gshub/supervisor/internal/config"
	"go.uber.org/zap"
)

// Status represents the process status
type Status string

const (
	StatusIdle     Status = "idle"
	StatusStarting Status = "starting"
	StatusRunning  Status = "running"
	StatusStopping Status = "stopping"
	StatusStopped  Status = "stopped"
	StatusFailed   Status = "failed"
)

// Manager handles the game process lifecycle
type Manager struct {
	config        *config.Config
	apiClient     *api.Client
	healthChecker *HealthChecker
	logger        *zap.Logger

	cmd      *exec.Cmd
	status   Status
	statusMu sync.RWMutex

	// Channels for coordination
	stopCh   chan struct{}
	doneCh   chan struct{}
	exitCode int

	// For stdout/stderr capture
	stdout io.ReadCloser
	stderr io.ReadCloser
}

// NewManager creates a new process manager
func NewManager(cfg *config.Config, apiClient *api.Client, logger *zap.Logger) (*Manager, error) {
	healthConfig := HealthConfig{
		Type:         cfg.HealthType,
		Port:         cfg.HealthPort,
		Protocol:     cfg.HealthProtocol,
		Pattern:      cfg.HealthPattern,
		InitialDelay: cfg.InitialDelay,
		Timeout:      cfg.HealthTimeout,
		Interval:     cfg.HealthInterval,
	}

	healthChecker, err := NewHealthChecker(healthConfig, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create health checker: %w", err)
	}

	return &Manager{
		config:        cfg,
		apiClient:     apiClient,
		healthChecker: healthChecker,
		logger:        logger,
		status:        StatusIdle,
		stopCh:        make(chan struct{}),
		doneCh:        make(chan struct{}),
	}, nil
}

// Status returns the current process status
func (m *Manager) Status() Status {
	m.statusMu.RLock()
	defer m.statusMu.RUnlock()
	return m.status
}

// setStatus updates the process status
func (m *Manager) setStatus(status Status) {
	m.statusMu.Lock()
	defer m.statusMu.Unlock()
	m.status = status
}

// PID returns the process ID if running, 0 otherwise
func (m *Manager) PID() int {
	m.statusMu.RLock()
	defer m.statusMu.RUnlock()
	if m.cmd != nil && m.cmd.Process != nil {
		return m.cmd.Process.Pid
	}
	return 0
}

// Start spawns the game process and waits for it to become healthy
func (m *Manager) Start(ctx context.Context) error {
	if m.Status() != StatusIdle && m.Status() != StatusStopped && m.Status() != StatusFailed {
		return fmt.Errorf("cannot start: process is in %s state", m.Status())
	}

	m.setStatus(StatusStarting)
	m.stopCh = make(chan struct{})
	m.doneCh = make(chan struct{})

	// Report starting status
	m.apiClient.ReportStatusWithRetry(ctx, api.StatusStarting, "Starting game process", 0, 3)

	// Build command
	if len(m.config.StartCommand) == 0 {
		return fmt.Errorf("start command is empty")
	}

	// Expand environment variables in command arguments (e.g., ${MEMORY} -> 1536M)
	expandedCmd := make([]string, len(m.config.StartCommand))
	for i, arg := range m.config.StartCommand {
		expandedCmd[i] = os.ExpandEnv(arg)
	}

	m.cmd = exec.CommandContext(ctx, expandedCmd[0], expandedCmd[1:]...)

	// Set working directory
	if m.config.WorkDir != "" {
		m.cmd.Dir = m.config.WorkDir
	}

	// Inherit environment and add supervisor-specific vars
	m.cmd.Env = os.Environ()

	// Capture stdout and stderr
	var err error
	m.stdout, err = m.cmd.StdoutPipe()
	if err != nil {
		m.setStatus(StatusFailed)
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	m.stderr, err = m.cmd.StderrPipe()
	if err != nil {
		m.setStatus(StatusFailed)
		return fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	// Set up process group for clean shutdown
	m.cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}

	// Start the process
	m.logger.Info("starting game process",
		zap.Strings("command", expandedCmd),
		zap.String("work_dir", m.config.WorkDir))

	if err := m.cmd.Start(); err != nil {
		m.setStatus(StatusFailed)
		m.apiClient.ReportStatusWithRetry(ctx, api.StatusFailed, fmt.Sprintf("Failed to start: %v", err), 0, 3)
		return fmt.Errorf("failed to start process: %w", err)
	}

	m.logger.Info("game process started", zap.Int("pid", m.cmd.Process.Pid))

	// Start log forwarding
	go m.forwardLogs("stdout", m.stdout)
	go m.forwardLogs("stderr", m.stderr)

	// Set up log reader for health checker if using log-pattern
	if m.config.HealthType == "log-pattern" {
		// Create a pipe to tee stdout to both the logger and health checker
		// For simplicity, we'll use a separate approach
		m.healthChecker.SetLogReader(m.stdout)
		m.healthChecker.StartLogScanner(ctx)
	}

	// Start a goroutine to wait for process exit
	go m.waitForExit()

	// Wait for health check to pass
	healthCtx, healthCancel := context.WithTimeout(ctx, m.config.HealthTimeout+m.config.InitialDelay)
	defer healthCancel()

	if err := m.healthChecker.WaitForHealthy(healthCtx); err != nil {
		m.logger.Error("health check failed", zap.Error(err))
		m.setStatus(StatusFailed)
		m.apiClient.ReportStatusWithRetry(ctx, api.StatusFailed, fmt.Sprintf("Health check failed: %v", err), m.PID(), 3)
		// Kill the process since it's not healthy
		m.Stop(ctx, false)
		return fmt.Errorf("health check failed: %w", err)
	}

	m.setStatus(StatusRunning)
	m.apiClient.ReportStatusWithRetry(ctx, api.StatusRunning, "Game server is running", m.PID(), 3)

	m.logger.Info("game process is healthy and running", zap.Int("pid", m.PID()))

	return nil
}

// Stop gracefully stops the game process
func (m *Manager) Stop(ctx context.Context, graceful bool) error {
	if m.Status() != StatusRunning && m.Status() != StatusStarting {
		return fmt.Errorf("cannot stop: process is in %s state", m.Status())
	}

	m.setStatus(StatusStopping)
	close(m.stopCh)

	m.apiClient.ReportStatusWithRetry(ctx, api.StatusStopping, "Stopping game process", m.PID(), 3)

	if m.cmd == nil || m.cmd.Process == nil {
		m.setStatus(StatusStopped)
		return nil
	}

	pid := m.cmd.Process.Pid

	if graceful {
		m.logger.Info("sending SIGTERM for graceful shutdown", zap.Int("pid", pid))

		// Send SIGTERM to the process group
		if err := syscall.Kill(-pid, syscall.SIGTERM); err != nil {
			m.logger.Warn("failed to send SIGTERM", zap.Error(err))
		}

		// Wait for graceful shutdown or timeout
		select {
		case <-m.doneCh:
			m.logger.Info("process exited gracefully")
		case <-time.After(m.config.GracePeriod):
			m.logger.Warn("grace period exceeded, sending SIGKILL",
				zap.Duration("grace_period", m.config.GracePeriod))
			if err := syscall.Kill(-pid, syscall.SIGKILL); err != nil {
				m.logger.Warn("failed to send SIGKILL", zap.Error(err))
			}
			<-m.doneCh
		case <-ctx.Done():
			m.logger.Warn("context cancelled, sending SIGKILL")
			syscall.Kill(-pid, syscall.SIGKILL)
			<-m.doneCh
		}
	} else {
		m.logger.Info("sending SIGKILL for immediate shutdown", zap.Int("pid", pid))
		if err := syscall.Kill(-pid, syscall.SIGKILL); err != nil {
			m.logger.Warn("failed to send SIGKILL", zap.Error(err))
		}
		<-m.doneCh
	}

	m.setStatus(StatusStopped)

	// Use a dedicated context for the final status report to ensure it completes
	// even if the parent context is cancelled during shutdown
	reportCtx, reportCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer reportCancel()
	m.apiClient.ReportStatusWithRetry(reportCtx, api.StatusStopped, "Game process stopped", 0, 3)

	return nil
}

// waitForExit waits for the process to exit and updates status
func (m *Manager) waitForExit() {
	defer close(m.doneCh)

	if m.cmd == nil {
		return
	}

	err := m.cmd.Wait()
	m.exitCode = m.cmd.ProcessState.ExitCode()

	m.logger.Info("game process exited",
		zap.Int("exit_code", m.exitCode),
		zap.Error(err))

	// Update status based on current state and exit code
	currentStatus := m.Status()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if currentStatus == StatusStopping {
		// Expected shutdown via Stop() - status already reported by Stop()
		m.setStatus(StatusStopped)
	} else if currentStatus == StatusRunning {
		if m.exitCode == 0 {
			// Clean exit (e.g., game server shutdown command)
			m.setStatus(StatusStopped)
			m.apiClient.ReportStatusWithRetry(ctx, api.StatusStopped, "Game process stopped", 0, 3)
		} else {
			// Unexpected crash
			m.setStatus(StatusFailed)
			m.apiClient.ReportStatusWithRetry(ctx, api.StatusFailed,
				fmt.Sprintf("Process crashed with exit code %d", m.exitCode), 0, 3)
		}
	} else if currentStatus == StatusStarting {
		// Process exited during startup - report failure
		m.setStatus(StatusFailed)
		m.apiClient.ReportStatusWithRetry(ctx, api.StatusFailed,
			fmt.Sprintf("Process exited during startup with exit code %d", m.exitCode), 0, 3)
	}
}

// Wait blocks until the process exits
func (m *Manager) Wait() {
	<-m.doneCh
}

// WaitWithContext blocks until the process exits or context is cancelled
func (m *Manager) WaitWithContext(ctx context.Context) error {
	select {
	case <-m.doneCh:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// ExitCode returns the exit code of the process (-1 if not exited)
func (m *Manager) ExitCode() int {
	select {
	case <-m.doneCh:
		return m.exitCode
	default:
		return -1
	}
}

// forwardLogs reads from a reader and logs each line
func (m *Manager) forwardLogs(name string, reader io.Reader) {
	buf := make([]byte, 4096)
	for {
		n, err := reader.Read(buf)
		if n > 0 {
			// Log the output
			m.logger.Debug("game output",
				zap.String("stream", name),
				zap.ByteString("data", buf[:n]))
			// Also write to our stdout/stderr for docker logs
			if name == "stdout" {
				os.Stdout.Write(buf[:n])
			} else {
				os.Stderr.Write(buf[:n])
			}
		}
		if err != nil {
			if err != io.EOF {
				m.logger.Debug("log forwarding ended", zap.String("stream", name), zap.Error(err))
			}
			return
		}
	}
}

// IsRunning returns true if the process is currently running
func (m *Manager) IsRunning() bool {
	status := m.Status()
	return status == StatusRunning || status == StatusStarting
}

// IsHealthy returns true if the process is healthy
func (m *Manager) IsHealthy() bool {
	return m.healthChecker.IsHealthy()
}

// StartContinuousHealthCheck starts continuous health monitoring after startup
// The onStatusChange callback is invoked when the game process becomes unhealthy
func (m *Manager) StartContinuousHealthCheck(ctx context.Context, onStatusChange func(status, message string)) {
	m.healthChecker.RunContinuousChecks(ctx, func() {
		// Game became unhealthy
		m.logger.Warn("game process became unhealthy during continuous monitoring")
		m.setStatus(StatusFailed)
		if onStatusChange != nil {
			onStatusChange("failed", "Game process health check failed")
		}
	})
}
