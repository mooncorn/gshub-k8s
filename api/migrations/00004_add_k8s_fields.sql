-- Add K8s pod tracking fields to servers table
-- Created: 2025-11-30

ALTER TABLE servers ADD COLUMN IF NOT EXISTS pod_ip VARCHAR(45);
ALTER TABLE servers ADD COLUMN IF NOT EXISTS creation_error TEXT;
ALTER TABLE servers ADD COLUMN IF NOT EXISTS last_reconciled TIMESTAMP WITH TIME ZONE;

-- Index for querying pending servers during reconciliation
CREATE INDEX IF NOT EXISTS idx_servers_status_last_reconciled ON servers(status, last_reconciled) WHERE status = 'pending';
