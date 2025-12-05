package portalloc

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/mooncorn/gshub/api/internal/database"
	"go.uber.org/zap"
)

// Service manages port allocations for game servers
type Service struct {
	db     *database.DB
	logger *zap.Logger
}

// NewService creates a new port allocation service
func NewService(db *database.DB, logger *zap.Logger) *Service {
	return &Service{
		db:     db,
		logger: logger,
	}
}

// PortRequirement specifies a port needed for a game server
type PortRequirement struct {
	Name     string // "game", "query", "rcon"
	Protocol string // "TCP" or "UDP"
}

// AllocatedPort contains node info with the allocated port
type AllocatedPort struct {
	NodeName string
	NodeIP   string
	Port     int
	Protocol string
	PortName string
}

// AllocatePorts allocates ports for a server on an available node
// Returns allocated ports or error if no capacity
func (s *Service) AllocatePorts(ctx context.Context, serverID uuid.UUID, requirements []PortRequirement) ([]AllocatedPort, error) {
	// Convert to database requirements
	dbReqs := make([]database.PortRequirement, len(requirements))
	for i, req := range requirements {
		dbReqs[i] = database.PortRequirement{
			Name:     req.Name,
			Protocol: req.Protocol,
		}
	}

	node, dbPorts, err := s.db.AllocatePortsForServer(ctx, serverID, dbReqs)
	if err != nil {
		s.logger.Error("failed to allocate ports",
			zap.String("server_id", serverID.String()),
			zap.Error(err),
		)
		return nil, fmt.Errorf("failed to allocate ports: %w", err)
	}

	// Convert to service-level types
	ports := make([]AllocatedPort, len(dbPorts))
	for i, p := range dbPorts {
		ports[i] = AllocatedPort{
			NodeName: p.NodeName,
			NodeIP:   p.NodeIP,
			Port:     p.Port,
			Protocol: p.Protocol,
			PortName: p.PortName,
		}
	}

	s.logger.Info("allocated ports for server",
		zap.String("server_id", serverID.String()),
		zap.String("node", node.Name),
		zap.Int("port_count", len(ports)),
	)

	return ports, nil
}

// GetServerPorts retrieves current port allocations for a server
func (s *Service) GetServerPorts(ctx context.Context, serverID uuid.UUID) ([]AllocatedPort, error) {
	dbPorts, err := s.db.GetServerPortAllocations(ctx, serverID)
	if err != nil {
		return nil, fmt.Errorf("failed to get server ports: %w", err)
	}

	ports := make([]AllocatedPort, len(dbPorts))
	for i, p := range dbPorts {
		ports[i] = AllocatedPort{
			NodeName: p.NodeName,
			NodeIP:   p.NodeIP,
			Port:     p.Port,
			Protocol: p.Protocol,
			PortName: p.PortName,
		}
	}

	return ports, nil
}

// ReleasePorts releases all ports allocated to a server
func (s *Service) ReleasePorts(ctx context.Context, serverID uuid.UUID) error {
	if err := s.db.ReleaseServerPorts(ctx, serverID); err != nil {
		s.logger.Error("failed to release ports",
			zap.String("server_id", serverID.String()),
			zap.Error(err),
		)
		return fmt.Errorf("failed to release ports: %w", err)
	}

	s.logger.Info("released ports for server",
		zap.String("server_id", serverID.String()),
	)

	return nil
}

// HasAllocatedPorts checks if a server already has port allocations
func (s *Service) HasAllocatedPorts(ctx context.Context, serverID uuid.UUID) (bool, error) {
	ports, err := s.GetServerPorts(ctx, serverID)
	if err != nil {
		return false, err
	}
	return len(ports) > 0, nil
}
