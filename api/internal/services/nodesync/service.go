package nodesync

import (
	"context"
	"fmt"
	"time"

	"github.com/mooncorn/gshub/api/internal/database"
	"github.com/mooncorn/gshub/api/internal/services/k8s"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
)

// Config holds configuration for the node sync service
type Config struct {
	// PortRangeMin is the minimum port number for game servers
	PortRangeMin int
	// PortRangeMax is the maximum port number for game servers
	PortRangeMax int
	// SyncInterval is how often to sync nodes (0 = no periodic sync)
	SyncInterval time.Duration
	// NodeRoleLabel is the label key to identify game server nodes
	NodeRoleLabel string
	// PublicIPLabel is the label key containing the node's public IP
	PublicIPLabel string
}

// DefaultConfig returns the default configuration
func DefaultConfig() Config {
	return Config{
		PortRangeMin:  25501,
		PortRangeMax:  25999,
		SyncInterval:  5 * time.Minute,
		NodeRoleLabel: "node-role.kubernetes.io/gameserver",
		PublicIPLabel: "platform.io/public-ip",
	}
}

// Service synchronizes Kubernetes nodes with the database
type Service struct {
	db        *database.DB
	k8sClient *k8s.Client
	config    Config
	logger    *zap.Logger
	stopCh    chan struct{}
}

// NewService creates a new node sync service
func NewService(db *database.DB, k8sClient *k8s.Client, config Config, logger *zap.Logger) *Service {
	return &Service{
		db:        db,
		k8sClient: k8sClient,
		config:    config,
		logger:    logger,
		stopCh:    make(chan struct{}),
	}
}

// Start begins periodic node synchronization
func (s *Service) Start(ctx context.Context) {
	// Initial sync
	if err := s.SyncNodes(ctx); err != nil {
		s.logger.Error("initial node sync failed", zap.Error(err))
	}

	if s.config.SyncInterval <= 0 {
		s.logger.Info("periodic node sync disabled")
		return
	}

	go func() {
		ticker := time.NewTicker(s.config.SyncInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				if err := s.SyncNodes(ctx); err != nil {
					s.logger.Error("periodic node sync failed", zap.Error(err))
				}
			case <-s.stopCh:
				s.logger.Info("node sync stopped")
				return
			case <-ctx.Done():
				s.logger.Info("node sync context cancelled")
				return
			}
		}
	}()

	s.logger.Info("node sync started",
		zap.Duration("interval", s.config.SyncInterval),
	)
}

// Stop stops the periodic synchronization
func (s *Service) Stop() {
	close(s.stopCh)
}

// SyncNodes fetches nodes from Kubernetes and updates the database
func (s *Service) SyncNodes(ctx context.Context) error {
	nodes, err := s.k8sClient.ListNodes(ctx)
	if err != nil {
		return fmt.Errorf("failed to list nodes: %w", err)
	}

	// Track which nodes we see from K8s
	seenNodes := make(map[string]bool)

	for _, node := range nodes {
		// Check if this node has the gameserver role label
		if _, hasRole := node.Labels[s.config.NodeRoleLabel]; !hasRole {
			continue
		}

		// Get public IP from label
		publicIP, hasIP := node.Labels[s.config.PublicIPLabel]
		if !hasIP || publicIP == "" {
			s.logger.Warn("node missing public IP label",
				zap.String("node", node.Name),
				zap.String("label", s.config.PublicIPLabel),
			)
			continue
		}

		seenNodes[node.Name] = true

		// Check if node is ready
		isReady := isNodeReady(&node)

		// Extract allocatable resources from K8s node
		var cpuMillicores *int
		var memoryBytes *int64
		if cpuQuantity, ok := node.Status.Allocatable[corev1.ResourceCPU]; ok {
			val := int(cpuQuantity.MilliValue())
			cpuMillicores = &val
		}
		if memQuantity, ok := node.Status.Allocatable[corev1.ResourceMemory]; ok {
			val := memQuantity.Value()
			memoryBytes = &val
		}

		// Upsert node in database
		dbNode := &database.Node{
			Name:                     node.Name,
			PublicIP:                 publicIP,
			IsActive:                 isReady,
			AllocatableCPUMillicores: cpuMillicores,
			AllocatableMemoryBytes:   memoryBytes,
		}

		if err := s.db.UpsertNode(ctx, dbNode); err != nil {
			s.logger.Error("failed to upsert node",
				zap.String("node", node.Name),
				zap.Error(err),
			)
			continue
		}

		// Initialize port allocations for this node
		if err := s.db.InitializeNodePorts(ctx, dbNode.ID, s.config.PortRangeMin, s.config.PortRangeMax); err != nil {
			s.logger.Error("failed to initialize ports for node",
				zap.String("node", node.Name),
				zap.Error(err),
			)
			continue
		}

		s.logger.Debug("synced node",
			zap.String("node", node.Name),
			zap.String("public_ip", publicIP),
			zap.Bool("is_active", isReady),
			zap.Intp("cpu_millicores", cpuMillicores),
			zap.Int64p("memory_bytes", memoryBytes),
		)
	}

	// Mark nodes that are no longer in K8s as inactive
	dbNodes, err := s.db.GetAllNodes(ctx)
	if err != nil {
		return fmt.Errorf("failed to get database nodes: %w", err)
	}

	for _, dbNode := range dbNodes {
		if !seenNodes[dbNode.Name] && dbNode.IsActive {
			s.logger.Info("marking missing node as inactive",
				zap.String("node", dbNode.Name),
			)
			if err := s.db.SetNodeActive(ctx, dbNode.Name, false); err != nil {
				s.logger.Error("failed to mark node inactive",
					zap.String("node", dbNode.Name),
					zap.Error(err),
				)
			}
		}
	}

	s.logger.Info("node sync completed",
		zap.Int("nodes_synced", len(seenNodes)),
	)

	return nil
}

// isNodeReady checks if a Kubernetes node is in Ready condition
func isNodeReady(node *corev1.Node) bool {
	for _, condition := range node.Status.Conditions {
		if condition.Type == corev1.NodeReady {
			return condition.Status == corev1.ConditionTrue
		}
	}
	return false
}
