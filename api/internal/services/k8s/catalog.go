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
	Name    string                `yaml:"name"`
	Image   string                `yaml:"image"`
	Ports   []GamePort            `yaml:"ports"`
	Volumes []GameVolume          `yaml:"volumes"`
	Env     map[string]string     `yaml:"env"`
	Plans   map[string]PlanConfig `yaml:"plans"`
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
	Name    string `yaml:"name"`
	CPU     string `yaml:"cpu"`
	Memory  string `yaml:"memory"`
	Storage string `yaml:"storage"`
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
