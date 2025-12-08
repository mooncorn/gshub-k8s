package portalloc

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/mooncorn/gshub/api/internal/database"
	"github.com/mooncorn/gshub/api/internal/services/k8s"
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

// ResourceRequirement specifies CPU/memory needed for a game server
type ResourceRequirement struct {
	CPUMillicores int   // CPU in millicores (1000 = 1 core)
	MemoryBytes   int64 // Memory in bytes
}

// AllocatedPort contains node info with the allocated port
type AllocatedPort struct {
	NodeName string
	NodeIP   string
	Port     int
	Protocol string
	PortName string
}

// AllocatePorts allocates ports and resources for a server on an available node
// Returns allocated ports or error if no capacity
// If resourceReq is nil, resource checking is skipped (for backward compatibility)
func (s *Service) AllocatePorts(ctx context.Context, serverID uuid.UUID, requirements []PortRequirement, resourceReq *ResourceRequirement) ([]AllocatedPort, error) {
	// Convert to database requirements
	dbReqs := make([]database.PortRequirement, len(requirements))
	for i, req := range requirements {
		dbReqs[i] = database.PortRequirement{
			Name:     req.Name,
			Protocol: req.Protocol,
		}
	}

	// Convert resource requirement if provided, applying overhead factor
	// to reserve capacity for system processes (kubelet, containerd, OS)
	var dbResourceReq *database.ResourceRequirement
	if resourceReq != nil {
		dbResourceReq = &database.ResourceRequirement{
			CPUMillicores: int(float64(resourceReq.CPUMillicores) * k8s.ResourceOverheadFactor),
			MemoryBytes:   int64(float64(resourceReq.MemoryBytes) * k8s.ResourceOverheadFactor),
		}
	}

	node, dbPorts, err := s.db.AllocatePortsForServer(ctx, serverID, dbReqs, dbResourceReq)
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

// HasCapacity checks if there's available capacity for a server with given requirements
// This is a read-only check that does not allocate any resources
// Used for optimistic validation before checkout
func (s *Service) HasCapacity(ctx context.Context, requirements []PortRequirement, resourceReq *ResourceRequirement) (bool, error) {
	// Count required ports by protocol
	tcpCount := 0
	udpCount := 0
	for _, req := range requirements {
		switch req.Protocol {
		case "TCP":
			tcpCount++
		case "UDP":
			udpCount++
		}
	}

	// Apply overhead factor to resource requirements
	cpuMillicores := 0
	var memoryBytes int64 = 0
	if resourceReq != nil {
		cpuMillicores = int(float64(resourceReq.CPUMillicores) * k8s.ResourceOverheadFactor)
		memoryBytes = int64(float64(resourceReq.MemoryBytes) * k8s.ResourceOverheadFactor)
	}

	hasCapacity, err := s.db.CheckResourceCapacity(ctx, tcpCount, udpCount, cpuMillicores, memoryBytes)
	if err != nil {
		s.logger.Error("failed to check resource capacity",
			zap.Error(err),
		)
		return false, fmt.Errorf("failed to check resource capacity: %w", err)
	}

	s.logger.Debug("capacity check result",
		zap.Bool("has_capacity", hasCapacity),
		zap.Int("tcp_ports", tcpCount),
		zap.Int("udp_ports", udpCount),
		zap.Int("cpu_millicores", cpuMillicores),
		zap.Int64("memory_bytes", memoryBytes),
	)

	return hasCapacity, nil
}
