-- Add env_overrides column to store per-server environment customizations
-- Uses JSONB for flexibility and efficient querying
-- NULL means use catalog defaults, non-null completely overrides defaults (full override mode)
ALTER TABLE servers ADD COLUMN env_overrides JSONB;
