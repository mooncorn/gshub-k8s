-- Add supervisor fields for auth token and heartbeat tracking
ALTER TABLE servers ADD COLUMN auth_token VARCHAR(64);
ALTER TABLE servers ADD COLUMN last_heartbeat TIMESTAMP WITH TIME ZONE;

-- Index for efficient heartbeat queries
CREATE INDEX idx_servers_last_heartbeat ON servers(last_heartbeat)
    WHERE status IN ('starting', 'running');

-- Index for auth token lookups
CREATE INDEX idx_servers_auth_token ON servers(id, auth_token)
    WHERE auth_token IS NOT NULL;
