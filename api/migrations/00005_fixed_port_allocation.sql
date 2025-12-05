-- Fixed Port Allocation System
-- Enables persistent port assignments for GameServers across restarts
-- Created: 2025-12-05

-- Nodes table: tracks Kubernetes nodes available for game server scheduling
CREATE TABLE IF NOT EXISTS nodes (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name            VARCHAR(255) UNIQUE NOT NULL,  -- K8s node name (kubernetes.io/hostname)
    public_ip       VARCHAR(45) NOT NULL,          -- Node's public IP (from platform.io/public-ip label)
    is_active       BOOLEAN DEFAULT TRUE,          -- Can schedule new servers on this node
    created_at      TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at      TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_nodes_name ON nodes(name);
CREATE INDEX IF NOT EXISTS idx_nodes_active ON nodes(is_active) WHERE is_active = TRUE;

-- Port allocations: pre-populated pool of ports per node
-- Each row represents a single (node, port, protocol) slot
-- server_id = NULL means the port is available
CREATE TABLE IF NOT EXISTS port_allocations (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    node_id         UUID NOT NULL REFERENCES nodes(id) ON DELETE CASCADE,
    server_id       UUID REFERENCES servers(id) ON DELETE SET NULL,  -- NULL = available
    port            INT NOT NULL,                   -- Host port number (e.g., 25501)
    protocol        VARCHAR(10) NOT NULL,           -- TCP or UDP
    port_name       VARCHAR(50),                    -- "game", "query", etc. (set when allocated)
    allocated_at    TIMESTAMP WITH TIME ZONE,       -- When this port was allocated to a server
    created_at      TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE(node_id, port, protocol)                 -- Each (node, port, protocol) combo is unique
);

CREATE INDEX IF NOT EXISTS idx_port_allocations_node ON port_allocations(node_id);
CREATE INDEX IF NOT EXISTS idx_port_allocations_server ON port_allocations(server_id) WHERE server_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_port_allocations_available ON port_allocations(node_id) WHERE server_id IS NULL;

-- Add node_name column to servers table for hard binding to a specific node
ALTER TABLE servers ADD COLUMN IF NOT EXISTS node_name VARCHAR(255);
CREATE INDEX IF NOT EXISTS idx_servers_node_name ON servers(node_name) WHERE node_name IS NOT NULL;

-- Trigger to auto-update nodes.updated_at
CREATE TRIGGER update_nodes_updated_at
    BEFORE UPDATE ON nodes
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();
