package process

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"regexp"
	"sync"
	"time"

	"go.uber.org/zap"
)

// HealthChecker monitors the health of the game process
type HealthChecker struct {
	config  HealthConfig
	healthy bool
	mu      sync.RWMutex
	logger  *zap.Logger

	// For log pattern matching
	logReader io.Reader
	pattern   *regexp.Regexp
}

// HealthConfig holds health check configuration
type HealthConfig struct {
	Type         string        // "port", "log-pattern", "none"
	Port         int           // For port checks
	Protocol     string        // "TCP" or "UDP"
	Pattern      string        // Regex pattern for log-pattern type
	InitialDelay time.Duration // Wait before first check
	Timeout      time.Duration // Max time to become healthy
	Interval     time.Duration // Check frequency
}

// NewHealthChecker creates a new health checker
func NewHealthChecker(config HealthConfig, logger *zap.Logger) (*HealthChecker, error) {
	hc := &HealthChecker{
		config:  config,
		healthy: false,
		logger:  logger,
	}

	if config.Type == "log-pattern" && config.Pattern != "" {
		pattern, err := regexp.Compile(config.Pattern)
		if err != nil {
			return nil, fmt.Errorf("invalid health check pattern: %w", err)
		}
		hc.pattern = pattern
	}

	return hc, nil
}

// SetLogReader sets the log reader for log-pattern health checks
func (hc *HealthChecker) SetLogReader(reader io.Reader) {
	hc.logReader = reader
}

// IsHealthy returns current health status
func (hc *HealthChecker) IsHealthy() bool {
	hc.mu.RLock()
	defer hc.mu.RUnlock()
	return hc.healthy
}

// setHealthy updates the health status
func (hc *HealthChecker) setHealthy(healthy bool) {
	hc.mu.Lock()
	defer hc.mu.Unlock()
	hc.healthy = healthy
}

// WaitForHealthy blocks until the process becomes healthy or times out
func (hc *HealthChecker) WaitForHealthy(ctx context.Context) error {
	if hc.config.Type == "none" {
		// No health check configured, consider immediately healthy
		hc.setHealthy(true)
		return nil
	}

	// Wait for initial delay
	hc.logger.Info("waiting for initial delay before health checks",
		zap.Duration("delay", hc.config.InitialDelay))

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(hc.config.InitialDelay):
	}

	deadline := time.Now().Add(hc.config.Timeout)

	for {
		if time.Now().After(deadline) {
			return fmt.Errorf("health check timeout after %v", hc.config.Timeout)
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		var healthy bool
		var err error

		switch hc.config.Type {
		case "port":
			healthy, err = hc.checkPort()
		case "log-pattern":
			healthy, err = hc.checkLogPattern()
		default:
			return fmt.Errorf("unknown health check type: %s", hc.config.Type)
		}

		if err != nil {
			hc.logger.Debug("health check failed", zap.Error(err))
		}

		if healthy {
			hc.setHealthy(true)
			hc.logger.Info("health check passed")
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(hc.config.Interval):
		}
	}
}

// checkPort performs a TCP or UDP port check
func (hc *HealthChecker) checkPort() (bool, error) {
	address := fmt.Sprintf("localhost:%d", hc.config.Port)

	switch hc.config.Protocol {
	case "TCP":
		conn, err := net.DialTimeout("tcp", address, 5*time.Second)
		if err != nil {
			return false, err
		}
		conn.Close()
		return true, nil

	case "UDP":
		// UDP is connectionless, so we just check if we can create a connection
		// For a more robust check, we'd need game-specific probe packets
		conn, err := net.DialTimeout("udp", address, 5*time.Second)
		if err != nil {
			return false, err
		}
		conn.Close()
		return true, nil

	default:
		return false, fmt.Errorf("unknown protocol: %s", hc.config.Protocol)
	}
}

// checkLogPattern scans logs for a pattern match
func (hc *HealthChecker) checkLogPattern() (bool, error) {
	if hc.logReader == nil {
		return false, fmt.Errorf("log reader not set")
	}

	if hc.pattern == nil {
		return false, fmt.Errorf("pattern not compiled")
	}

	// This is a simplified check - in reality we'd want to
	// continuously scan logs and set healthy when pattern matches
	return hc.IsHealthy(), nil
}

// StartLogScanner starts scanning logs for health pattern
// This runs in the background and sets healthy=true when pattern is found
func (hc *HealthChecker) StartLogScanner(ctx context.Context) {
	if hc.config.Type != "log-pattern" || hc.logReader == nil || hc.pattern == nil {
		return
	}

	go func() {
		scanner := bufio.NewScanner(hc.logReader)
		for scanner.Scan() {
			select {
			case <-ctx.Done():
				return
			default:
			}

			line := scanner.Text()
			if hc.pattern.MatchString(line) {
				hc.logger.Info("health pattern matched in logs",
					zap.String("pattern", hc.config.Pattern),
					zap.String("line", line))
				hc.setHealthy(true)
				return
			}
		}
	}()
}

// RunContinuousChecks runs health checks continuously after initial healthy state
func (hc *HealthChecker) RunContinuousChecks(ctx context.Context, onUnhealthy func()) {
	if hc.config.Type == "none" {
		return
	}

	ticker := time.NewTicker(hc.config.Interval)
	defer ticker.Stop()

	failCount := 0
	maxFailures := 3 // Mark unhealthy after 3 consecutive failures

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			var healthy bool
			var err error

			switch hc.config.Type {
			case "port":
				healthy, err = hc.checkPort()
			case "log-pattern":
				// For log-pattern, we rely on the log scanner
				healthy = hc.IsHealthy()
			}

			if err != nil || !healthy {
				failCount++
				hc.logger.Warn("health check failed",
					zap.Error(err),
					zap.Int("fail_count", failCount))

				if failCount >= maxFailures {
					hc.setHealthy(false)
					if onUnhealthy != nil {
						onUnhealthy()
					}
				}
			} else {
				failCount = 0
				hc.setHealthy(true)
			}
		}
	}
}
