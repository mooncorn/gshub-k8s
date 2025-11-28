-- Pending Server Requests Table
-- Created: 2025-11-27
-- Tracks server creation requests before payment is completed

CREATE TABLE IF NOT EXISTS pending_server_requests (
  id                    UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id               UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  display_name          VARCHAR(50),
  subdomain             VARCHAR(63) NOT NULL UNIQUE,
  game                  VARCHAR(20) NOT NULL,
  plan                  VARCHAR(20) NOT NULL,
  stripe_session_id     VARCHAR(255) UNIQUE,
  status                VARCHAR(20) NOT NULL DEFAULT 'awaiting_payment',
  server_id             UUID REFERENCES servers(id),
  created_at            TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
  updated_at            TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
  expires_at            TIMESTAMP WITH TIME ZONE DEFAULT NOW() + INTERVAL '24 hours'
);

-- Indexes for common queries
CREATE INDEX IF NOT EXISTS idx_pending_server_requests_user_id ON pending_server_requests(user_id);
CREATE INDEX IF NOT EXISTS idx_pending_server_requests_status ON pending_server_requests(status);
CREATE INDEX IF NOT EXISTS idx_pending_server_requests_stripe_session ON pending_server_requests(stripe_session_id);
CREATE INDEX IF NOT EXISTS idx_pending_server_requests_server_id ON pending_server_requests(server_id);
CREATE INDEX IF NOT EXISTS idx_pending_server_requests_expires_at ON pending_server_requests(expires_at) WHERE status = 'awaiting_payment';

-- Trigger to auto-update updated_at
CREATE TRIGGER update_pending_server_requests_updated_at
  BEFORE UPDATE ON pending_server_requests
  FOR EACH ROW
  EXECUTE FUNCTION update_updated_at_column();
