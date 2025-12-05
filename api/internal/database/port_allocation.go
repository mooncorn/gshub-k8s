package database

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// Node represents a Kubernetes node available for game server scheduling
type Node struct {
	ID        uuid.UUID
	Name      string
	PublicIP  string
	IsActive  bool
	CreatedAt time.Time
	UpdatedAt time.Time
}

// PortAllocation represents a port slot on a node
type PortAllocation struct {
	ID          uuid.UUID
	NodeID      uuid.UUID
	ServerID    *uuid.UUID
	Port        int
	Protocol    string
	PortName    *string
	AllocatedAt *time.Time
	CreatedAt   time.Time
}

// AllocatedPort contains node info with the allocated port
type AllocatedPort struct {
	NodeName  string
	NodeIP    string
	Port      int
	Protocol  string
	PortName  string
}

// PortRequirement specifies a port needed for a game server
type PortRequirement struct {
	Name     string // "game", "query", "rcon"
	Protocol string // "TCP" or "UDP"
}

// UpsertNode creates or updates a node record
func (db *DB) UpsertNode(ctx context.Context, node *Node) error {
	query := `
		INSERT INTO nodes (name, public_ip, is_active)
		VALUES ($1, $2, $3)
		ON CONFLICT (name) DO UPDATE SET
			public_ip = EXCLUDED.public_ip,
			is_active = EXCLUDED.is_active,
			updated_at = NOW()
		RETURNING id, created_at, updated_at
	`
	err := db.Pool.QueryRow(ctx, query, node.Name, node.PublicIP, node.IsActive).
		Scan(&node.ID, &node.CreatedAt, &node.UpdatedAt)
	if err != nil {
		return fmt.Errorf("failed to upsert node: %w", err)
	}
	return nil
}

// GetNodeByName retrieves a node by its Kubernetes name
func (db *DB) GetNodeByName(ctx context.Context, name string) (*Node, error) {
	query := `
		SELECT id, name, public_ip, is_active, created_at, updated_at
		FROM nodes
		WHERE name = $1
	`
	var node Node
	err := db.Pool.QueryRow(ctx, query, name).Scan(
		&node.ID, &node.Name, &node.PublicIP, &node.IsActive,
		&node.CreatedAt, &node.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get node: %w", err)
	}
	return &node, nil
}

// GetAllNodes retrieves all nodes
func (db *DB) GetAllNodes(ctx context.Context) ([]Node, error) {
	query := `
		SELECT id, name, public_ip, is_active, created_at, updated_at
		FROM nodes
		ORDER BY name
	`
	rows, err := db.Pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to get nodes: %w", err)
	}
	defer rows.Close()

	var nodes []Node
	for rows.Next() {
		var node Node
		if err := rows.Scan(
			&node.ID, &node.Name, &node.PublicIP, &node.IsActive,
			&node.CreatedAt, &node.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan node: %w", err)
		}
		nodes = append(nodes, node)
	}
	return nodes, nil
}

// SetNodeActive updates the is_active status of a node
func (db *DB) SetNodeActive(ctx context.Context, nodeName string, isActive bool) error {
	query := `UPDATE nodes SET is_active = $2, updated_at = NOW() WHERE name = $1`
	_, err := db.Pool.Exec(ctx, query, nodeName, isActive)
	if err != nil {
		return fmt.Errorf("failed to set node active status: %w", err)
	}
	return nil
}

// InitializeNodePorts creates port allocation slots for a node
// Only creates ports that don't already exist
func (db *DB) InitializeNodePorts(ctx context.Context, nodeID uuid.UUID, minPort, maxPort int) error {
	// Insert ports for both TCP and UDP using CROSS JOIN
	query := `
		INSERT INTO port_allocations (node_id, port, protocol)
		SELECT $1::uuid, ports.port, protocols.protocol
		FROM generate_series($2::int, $3::int) AS ports(port)
		CROSS JOIN (VALUES ('TCP'), ('UDP')) AS protocols(protocol)
		ON CONFLICT (node_id, port, protocol) DO NOTHING
	`
	_, err := db.Pool.Exec(ctx, query, nodeID, minPort, maxPort)
	if err != nil {
		return fmt.Errorf("failed to initialize node ports: %w", err)
	}
	return nil
}

// AllocatePortsForServer allocates ports for a server on an available node
// Uses SELECT FOR UPDATE to prevent race conditions
// Returns the node and allocated ports
func (db *DB) AllocatePortsForServer(ctx context.Context, serverID uuid.UUID, requirements []PortRequirement) (*Node, []AllocatedPort, error) {
	tx, err := db.Pool.Begin(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// Count required ports per protocol
	tcpCount, udpCount := 0, 0
	for _, req := range requirements {
		switch req.Protocol {
		case "TCP":
			tcpCount++
		case "UDP":
			udpCount++
		}
	}

	// Find a node with enough available ports for both protocols
	// Lock the node row to prevent concurrent allocations
	nodeQuery := `
		SELECT n.id, n.name, n.public_ip
		FROM nodes n
		WHERE n.is_active = TRUE
		AND (
			SELECT COUNT(*) FROM port_allocations pa
			WHERE pa.node_id = n.id AND pa.server_id IS NULL AND pa.protocol = 'TCP'
		) >= $1
		AND (
			SELECT COUNT(*) FROM port_allocations pa
			WHERE pa.node_id = n.id AND pa.server_id IS NULL AND pa.protocol = 'UDP'
		) >= $2
		ORDER BY (
			SELECT COUNT(*) FROM port_allocations pa
			WHERE pa.node_id = n.id AND pa.server_id IS NULL
		) DESC
		LIMIT 1
		FOR UPDATE OF n
	`

	var node Node
	err = tx.QueryRow(ctx, nodeQuery, tcpCount, udpCount).Scan(&node.ID, &node.Name, &node.PublicIP)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil, fmt.Errorf("no node with available capacity")
		}
		return nil, nil, fmt.Errorf("failed to find available node: %w", err)
	}

	// Allocate ports for each requirement
	var allocatedPorts []AllocatedPort
	for _, req := range requirements {
		// Get an available port for this protocol and lock it
		portQuery := `
			SELECT id, port
			FROM port_allocations
			WHERE node_id = $1 AND protocol = $2 AND server_id IS NULL
			ORDER BY port ASC
			LIMIT 1
			FOR UPDATE
		`

		var portID uuid.UUID
		var port int
		err = tx.QueryRow(ctx, portQuery, node.ID, req.Protocol).Scan(&portID, &port)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to get available %s port: %w", req.Protocol, err)
		}

		// Assign the port to the server
		updateQuery := `
			UPDATE port_allocations
			SET server_id = $1, port_name = $2, allocated_at = NOW()
			WHERE id = $3
		`
		_, err = tx.Exec(ctx, updateQuery, serverID, req.Name, portID)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to allocate port: %w", err)
		}

		allocatedPorts = append(allocatedPorts, AllocatedPort{
			NodeName: node.Name,
			NodeIP:   node.PublicIP,
			Port:     port,
			Protocol: req.Protocol,
			PortName: req.Name,
		})
	}

	// Update server's node_name
	serverUpdateQuery := `UPDATE servers SET node_name = $1 WHERE id = $2`
	_, err = tx.Exec(ctx, serverUpdateQuery, node.Name, serverID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to update server node_name: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return &node, allocatedPorts, nil
}

// GetServerPortAllocations retrieves all port allocations for a server
func (db *DB) GetServerPortAllocations(ctx context.Context, serverID uuid.UUID) ([]AllocatedPort, error) {
	query := `
		SELECT n.name, n.public_ip, pa.port, pa.protocol, pa.port_name
		FROM port_allocations pa
		JOIN nodes n ON n.id = pa.node_id
		WHERE pa.server_id = $1
		ORDER BY pa.port_name
	`

	rows, err := db.Pool.Query(ctx, query, serverID)
	if err != nil {
		return nil, fmt.Errorf("failed to get server port allocations: %w", err)
	}
	defer rows.Close()

	var ports []AllocatedPort
	for rows.Next() {
		var port AllocatedPort
		var portName *string
		if err := rows.Scan(&port.NodeName, &port.NodeIP, &port.Port, &port.Protocol, &portName); err != nil {
			return nil, fmt.Errorf("failed to scan port allocation: %w", err)
		}
		if portName != nil {
			port.PortName = *portName
		}
		ports = append(ports, port)
	}

	return ports, nil
}

// ReleaseServerPorts releases all ports allocated to a server
func (db *DB) ReleaseServerPorts(ctx context.Context, serverID uuid.UUID) error {
	query := `
		UPDATE port_allocations
		SET server_id = NULL, port_name = NULL, allocated_at = NULL
		WHERE server_id = $1
	`
	_, err := db.Pool.Exec(ctx, query, serverID)
	if err != nil {
		return fmt.Errorf("failed to release server ports: %w", err)
	}
	return nil
}

// GetNodePortStats returns port usage statistics for a node
func (db *DB) GetNodePortStats(ctx context.Context, nodeName string) (total, used int, err error) {
	query := `
		SELECT
			COUNT(*) as total,
			COUNT(*) FILTER (WHERE server_id IS NOT NULL) as used
		FROM port_allocations pa
		JOIN nodes n ON n.id = pa.node_id
		WHERE n.name = $1
	`
	err = db.Pool.QueryRow(ctx, query, nodeName).Scan(&total, &used)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to get node port stats: %w", err)
	}
	return total, used, nil
}

// DeleteNode removes a node and all its port allocations (cascades)
func (db *DB) DeleteNode(ctx context.Context, nodeName string) error {
	query := `DELETE FROM nodes WHERE name = $1`
	_, err := db.Pool.Exec(ctx, query, nodeName)
	if err != nil {
		return fmt.Errorf("failed to delete node: %w", err)
	}
	return nil
}
