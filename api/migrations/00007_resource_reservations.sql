-- Resource Reservation System
-- Tracks CPU/Memory capacity per node and reservations per server

-- Add resource capacity columns to nodes table
ALTER TABLE nodes
    ADD COLUMN IF NOT EXISTS allocatable_cpu_millicores INT,
    ADD COLUMN IF NOT EXISTS allocatable_memory_bytes BIGINT;

-- Add resource reservation columns to servers table
ALTER TABLE servers
    ADD COLUMN IF NOT EXISTS reserved_cpu_millicores INT,
    ADD COLUMN IF NOT EXISTS reserved_memory_bytes BIGINT;

-- Index for efficient resource queries during allocation
-- Only counts servers that are actively reserving resources (not expired/deleted/failed)
CREATE INDEX IF NOT EXISTS idx_servers_node_resources
    ON servers(node_name, reserved_cpu_millicores, reserved_memory_bytes)
    WHERE node_name IS NOT NULL AND status NOT IN ('deleted', 'expired', 'failed');

COMMENT ON COLUMN nodes.allocatable_cpu_millicores IS 'K8s allocatable CPU in millicores (1000 = 1 core)';
COMMENT ON COLUMN nodes.allocatable_memory_bytes IS 'K8s allocatable memory in bytes';
COMMENT ON COLUMN servers.reserved_cpu_millicores IS 'Reserved CPU for this server in millicores';
COMMENT ON COLUMN servers.reserved_memory_bytes IS 'Reserved memory for this server in bytes';
