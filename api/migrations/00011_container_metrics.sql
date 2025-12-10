-- Container metrics tracking for crash loop and OOM detection
-- +goose Up

ALTER TABLE servers ADD COLUMN restart_count INT DEFAULT 0;
ALTER TABLE servers ADD COLUMN last_restart_at TIMESTAMP WITH TIME ZONE;
ALTER TABLE servers ADD COLUMN last_oom_at TIMESTAMP WITH TIME ZONE;

-- Index for finding servers with high restart counts (only for active servers)
CREATE INDEX idx_servers_restart_count ON servers(restart_count)
    WHERE status IN ('running', 'starting');

-- +goose Down

DROP INDEX IF EXISTS idx_servers_restart_count;
ALTER TABLE servers DROP COLUMN IF EXISTS last_oom_at;
ALTER TABLE servers DROP COLUMN IF EXISTS last_restart_at;
ALTER TABLE servers DROP COLUMN IF EXISTS restart_count;
