package cleanup

import (
	"context"
	"fmt"
	"time"

	"github.com/mooncorn/gshub/api/internal/database"
	"github.com/mooncorn/gshub/api/internal/services/k8s"
	"go.uber.org/zap"
)

// Config holds configuration for the cleanup service
type Config struct {
	// Interval is how often to run cleanup (default: 1 hour)
	Interval time.Duration
	// Namespace is the K8s namespace to clean up resources in
	Namespace string
}

// DefaultConfig returns the default configuration
func DefaultConfig() Config {
	return Config{
		Interval: 1 * time.Hour,
	}
}

// Service handles cleanup of expired servers
type Service struct {
	db        *database.DB
	k8sClient *k8s.Client
	config    Config
	logger    *zap.Logger
	stopCh    chan struct{}
}

// NewService creates a new cleanup service
func NewService(db *database.DB, k8sClient *k8s.Client, config Config, logger *zap.Logger) *Service {
	return &Service{
		db:        db,
		k8sClient: k8sClient,
		config:    config,
		logger:    logger,
		stopCh:    make(chan struct{}),
	}
}

// Start begins the cleanup service
func (s *Service) Start(ctx context.Context) {
	// Run initial cleanup
	s.runCleanup(ctx)

	go func() {
		ticker := time.NewTicker(s.config.Interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				s.runCleanup(ctx)
			case <-s.stopCh:
				s.logger.Info("cleanup service stopped")
				return
			case <-ctx.Done():
				s.logger.Info("cleanup service context cancelled")
				return
			}
		}
	}()

	s.logger.Info("cleanup service started",
		zap.Duration("interval", s.config.Interval),
	)
}

// Stop stops the cleanup service
func (s *Service) Stop() {
	close(s.stopCh)
}

// runCleanup finds and cleans up expired servers past their grace period
func (s *Service) runCleanup(ctx context.Context) {
	servers, err := s.db.GetExpiredServersForCleanup(ctx)
	if err != nil {
		s.logger.Error("failed to get expired servers for cleanup", zap.Error(err))
		return
	}

	if len(servers) == 0 {
		return
	}

	s.logger.Info("cleaning up expired servers", zap.Int("count", len(servers)))

	successCount := 0
	failureCount := 0

	for _, server := range servers {
		serverID := server.ID.String()
		pvcName := fmt.Sprintf("server-%s", serverID)

		// Delete PVC from K8s
		if err := s.k8sClient.DeletePVC(ctx, s.config.Namespace, pvcName); err != nil {
			s.logger.Error("failed to delete PVC",
				zap.String("server_id", serverID),
				zap.String("pvc_name", pvcName),
				zap.Error(err),
			)
			failureCount++
			continue
		}

		s.logger.Info("deleted PVC",
			zap.String("server_id", serverID),
			zap.String("pvc_name", pvcName),
		)

		// Hard delete server record from database
		if err := s.db.HardDeleteServer(ctx, serverID); err != nil {
			s.logger.Error("failed to hard delete server",
				zap.String("server_id", serverID),
				zap.Error(err),
			)
			failureCount++
			continue
		}

		s.logger.Info("hard deleted server record",
			zap.String("server_id", serverID),
		)

		successCount++
	}

	s.logger.Info("cleanup cycle complete",
		zap.Int("succeeded", successCount),
		zap.Int("failed", failureCount),
	)
}
