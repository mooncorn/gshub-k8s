package config

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config holds all supervisor configuration loaded from environment variables
type Config struct {
	// Server identification
	ServerID  string
	AuthToken string

	// API connection
	APIEndpoint string

	// Process configuration
	StartCommand []string
	WorkDir      string
	GracePeriod  time.Duration

	// Health check configuration
	HealthType     string // "port", "log-pattern", "none"
	HealthPort     int
	HealthProtocol string // "TCP" or "UDP"
	HealthPattern  string // regex pattern for log-pattern type
	InitialDelay   time.Duration
	HealthTimeout  time.Duration
	HealthInterval time.Duration

	// Heartbeat configuration
	HeartbeatInterval time.Duration

	// Health server configuration (for K8s probes)
	HealthServerPort int
}

// Load reads configuration from environment variables
func Load() (*Config, error) {
	cfg := &Config{
		// Defaults
		GracePeriod:       30 * time.Second,
		HealthType:        "none",
		HealthProtocol:    "TCP",
		InitialDelay:      15 * time.Second,
		HealthTimeout:     120 * time.Second,
		HealthInterval:    10 * time.Second,
		HeartbeatInterval: 30 * time.Second,
		HealthServerPort:  8080,
	}

	// Required fields
	cfg.ServerID = os.Getenv("GSHUB_SERVER_ID")
	if cfg.ServerID == "" {
		return nil, fmt.Errorf("GSHUB_SERVER_ID is required")
	}

	cfg.AuthToken = os.Getenv("GSHUB_AUTH_TOKEN")
	if cfg.AuthToken == "" {
		return nil, fmt.Errorf("GSHUB_AUTH_TOKEN is required")
	}

	cfg.APIEndpoint = os.Getenv("GSHUB_API_ENDPOINT")
	if cfg.APIEndpoint == "" {
		return nil, fmt.Errorf("GSHUB_API_ENDPOINT is required")
	}

	// Start command (JSON array)
	startCmdJSON := os.Getenv("GSHUB_START_COMMAND")
	if startCmdJSON == "" {
		return nil, fmt.Errorf("GSHUB_START_COMMAND is required")
	}
	if err := json.Unmarshal([]byte(startCmdJSON), &cfg.StartCommand); err != nil {
		return nil, fmt.Errorf("invalid GSHUB_START_COMMAND JSON: %w", err)
	}
	if len(cfg.StartCommand) == 0 {
		return nil, fmt.Errorf("GSHUB_START_COMMAND must have at least one element")
	}

	// Optional fields
	if workDir := os.Getenv("GSHUB_WORK_DIR"); workDir != "" {
		cfg.WorkDir = workDir
	}

	if gracePeriod := os.Getenv("GSHUB_GRACE_PERIOD"); gracePeriod != "" {
		seconds, err := strconv.Atoi(gracePeriod)
		if err != nil {
			return nil, fmt.Errorf("invalid GSHUB_GRACE_PERIOD: %w", err)
		}
		cfg.GracePeriod = time.Duration(seconds) * time.Second
	}

	// Health check configuration
	if healthType := os.Getenv("GSHUB_HEALTH_TYPE"); healthType != "" {
		cfg.HealthType = healthType
	}

	if healthPort := os.Getenv("GSHUB_HEALTH_PORT"); healthPort != "" {
		port, err := strconv.Atoi(healthPort)
		if err != nil {
			return nil, fmt.Errorf("invalid GSHUB_HEALTH_PORT: %w", err)
		}
		cfg.HealthPort = port
	}

	if healthProtocol := os.Getenv("GSHUB_HEALTH_PROTOCOL"); healthProtocol != "" {
		cfg.HealthProtocol = healthProtocol
	}

	if healthPattern := os.Getenv("GSHUB_HEALTH_PATTERN"); healthPattern != "" {
		cfg.HealthPattern = healthPattern
	}

	if initialDelay := os.Getenv("GSHUB_HEALTH_INITIAL_DELAY"); initialDelay != "" {
		seconds, err := strconv.Atoi(initialDelay)
		if err != nil {
			return nil, fmt.Errorf("invalid GSHUB_HEALTH_INITIAL_DELAY: %w", err)
		}
		cfg.InitialDelay = time.Duration(seconds) * time.Second
	}

	if healthTimeout := os.Getenv("GSHUB_HEALTH_TIMEOUT"); healthTimeout != "" {
		seconds, err := strconv.Atoi(healthTimeout)
		if err != nil {
			return nil, fmt.Errorf("invalid GSHUB_HEALTH_TIMEOUT: %w", err)
		}
		cfg.HealthTimeout = time.Duration(seconds) * time.Second
	}

	if healthInterval := os.Getenv("GSHUB_HEALTH_INTERVAL"); healthInterval != "" {
		seconds, err := strconv.Atoi(healthInterval)
		if err != nil {
			return nil, fmt.Errorf("invalid GSHUB_HEALTH_INTERVAL: %w", err)
		}
		cfg.HealthInterval = time.Duration(seconds) * time.Second
	}

	if heartbeatInterval := os.Getenv("GSHUB_HEARTBEAT_INTERVAL"); heartbeatInterval != "" {
		seconds, err := strconv.Atoi(heartbeatInterval)
		if err != nil {
			return nil, fmt.Errorf("invalid GSHUB_HEARTBEAT_INTERVAL: %w", err)
		}
		cfg.HeartbeatInterval = time.Duration(seconds) * time.Second
	}

	if healthServerPort := os.Getenv("GSHUB_HEALTH_SERVER_PORT"); healthServerPort != "" {
		port, err := strconv.Atoi(healthServerPort)
		if err != nil {
			return nil, fmt.Errorf("invalid GSHUB_HEALTH_SERVER_PORT: %w", err)
		}
		cfg.HealthServerPort = port
	}

	return cfg, nil
}
