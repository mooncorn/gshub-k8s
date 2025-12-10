package k8s

import (
	"context"
	"fmt"

	"gopkg.in/yaml.v3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GameCatalog represents the structure of the game catalog ConfigMap
type GameCatalog struct {
	Games map[string]GameConfig `yaml:"games"`
}

// GameConfig holds configuration for a specific game
type GameConfig struct {
	Name              string                `yaml:"name"`
	Image             string                `yaml:"image"`             // Legacy: game server image (used with Agones)
	SupervisorImage   string                `yaml:"supervisorImage"`   // Supervisor image (includes game server)
	Ports             []GamePort            `yaml:"ports"`
	Volumes           []GameVolume          `yaml:"volumes"`
	Env               map[string]string     `yaml:"env"`
	HealthCheck       *HealthCheckConfig    `yaml:"healthCheck"`
	Process           *ProcessConfig        `yaml:"process"`           // Supervisor process configuration
	SupervisorOverhead *ResourceOverhead    `yaml:"supervisorOverhead"` // Additional resources for supervisor
	Plans             map[string]PlanConfig `yaml:"plans"`
}

// ProcessConfig holds configuration for the supervisor process management
type ProcessConfig struct {
	StartCommand []string `yaml:"startCommand"` // Command to start the game server
	WorkDir      string   `yaml:"workDir"`      // Working directory for the game process
	GracePeriod  int      `yaml:"gracePeriod"`  // Seconds to wait for graceful shutdown
	StopCommand  []string `yaml:"stopCommand"`  // Optional command to stop gracefully (e.g., RCON)
}

// ResourceOverhead holds additional resource requirements for the supervisor
type ResourceOverhead struct {
	CPU    string `yaml:"cpu"`    // e.g., "50m"
	Memory string `yaml:"memory"` // e.g., "64Mi"
}

// HealthCheckConfig holds configuration for sidecar health checks
type HealthCheckConfig struct {
	Type         string `yaml:"type"`         // "port", "delay", "log-pattern"
	Port         string `yaml:"port"`         // Port number to check
	Protocol     string `yaml:"protocol"`     // "TCP" or "UDP"
	Pattern      string `yaml:"pattern"`      // Regex pattern for log-pattern type
	InitialDelay string `yaml:"initialDelay"` // Delay before starting checks (e.g., "10s" or "10" for seconds)
	Timeout      string `yaml:"timeout"`      // Timeout for readiness (e.g., "30s" or "30" for seconds)
	Interval     string `yaml:"interval"`     // Check interval (e.g., "10" for seconds)
}

type GamePort struct {
	Name     string `yaml:"name"`
	Port     int32  `yaml:"port"`
	Protocol string `yaml:"protocol"`
}

type GameVolume struct {
	Name      string `yaml:"name"`
	MountPath string `yaml:"mount_path"`
	SubPath   string `yaml:"sub_path"`
}

// PlanConfig holds configuration for a specific plan (size)
type PlanConfig struct {
	Name    string            `yaml:"name"`
	CPU     string            `yaml:"cpu"`
	Memory  string            `yaml:"memory"`
	Storage string            `yaml:"storage"`
	Env     map[string]string `yaml:"env"` // Plan-level environment variables
}

// LoadGameCatalog reads the game-catalog ConfigMap from Kubernetes
func (c *Client) LoadGameCatalog(ctx context.Context, namespace, configMapName string) (*GameCatalog, error) {
	cm, err := c.clientset.CoreV1().ConfigMaps(namespace).Get(ctx, configMapName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get ConfigMap: %w", err)
	}

	catalogYAML, ok := cm.Data["games.yaml"]
	if !ok {
		return nil, fmt.Errorf("games.yaml not found in ConfigMap")
	}

	var catalog GameCatalog
	if err := yaml.Unmarshal([]byte(catalogYAML), &catalog); err != nil {
		return nil, fmt.Errorf("failed to parse games.yaml: %w", err)
	}

	return &catalog, nil
}

// GetGameConfig retrieves configuration for a specific game
func (catalog *GameCatalog) GetGameConfig(game string) (*GameConfig, error) {
	config, ok := catalog.Games[game]
	if !ok {
		return nil, fmt.Errorf("game %s not found in catalog", game)
	}
	return &config, nil
}

// GetPlanConfig retrieves configuration for a specific plan
func (game *GameConfig) GetPlanConfig(plan string) (*PlanConfig, error) {
	config, ok := game.Plans[plan]
	if !ok {
		return nil, fmt.Errorf("plan %s not found for game", plan)
	}
	return &config, nil
}

// MergeEnvVars performs a three-layer merge of environment variables.
// Priority (highest wins): userOverrides > planEnv > gameEnv
func MergeEnvVars(gameEnv, planEnv, userOverrides map[string]string) map[string]string {
	if userOverrides != nil {
		// Full override mode: user overrides completely replace defaults
		result := make(map[string]string, len(userOverrides))
		for k, v := range userOverrides {
			result[k] = v
		}
		return result
	}

	// Merge game + plan defaults (plan wins on conflict)
	result := make(map[string]string, len(gameEnv)+len(planEnv))
	for k, v := range gameEnv {
		result[k] = v
	}
	for k, v := range planEnv {
		result[k] = v
	}
	return result
}
