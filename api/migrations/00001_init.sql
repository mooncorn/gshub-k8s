-- Game Server Hosting Platform - Initial Schema
-- Created: 2025-11-25

-- Enable UUID extension for generating UUIDs
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- Users table - stores user accounts
CREATE TABLE IF NOT EXISTS users (
  id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  email               VARCHAR(255) UNIQUE NOT NULL,
  password_hash       VARCHAR(255) NOT NULL,
  email_verified      BOOLEAN DEFAULT FALSE,
  stripe_customer_id  VARCHAR(255),
  created_at          TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
  updated_at          TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Create index on email for faster lookups during login
CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);

-- Refresh tokens table
CREATE TABLE IF NOT EXISTS refresh_tokens (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token       VARCHAR(255) UNIQUE NOT NULL,
    expires_at  TIMESTAMP WITH TIME ZONE NOT NULL,
    created_at  TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_refresh_tokens_user_id ON refresh_tokens(user_id);
CREATE INDEX IF NOT EXISTS idx_refresh_tokens_token ON refresh_tokens(token);

-- Password reset tokens table
CREATE TABLE IF NOT EXISTS password_reset_tokens (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token       VARCHAR(255) UNIQUE NOT NULL,
    expires_at  TIMESTAMP WITH TIME ZONE NOT NULL,
    used        BOOLEAN DEFAULT FALSE,
    created_at  TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Email verification tokens table
CREATE TABLE IF NOT EXISTS email_verification_tokens (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token       VARCHAR(255) UNIQUE NOT NULL,
    expires_at  TIMESTAMP WITH TIME ZONE NOT NULL,
    created_at  TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_email_verification_tokens_user_id ON email_verification_tokens(user_id);
CREATE INDEX IF NOT EXISTS idx_email_verification_tokens_token ON email_verification_tokens(token);


CREATE INDEX IF NOT EXISTS idx_password_reset_tokens_user_id ON password_reset_tokens(user_id);
CREATE INDEX IF NOT EXISTS idx_password_reset_tokens_token ON password_reset_tokens(token);


-- Servers table - stores game server instances
CREATE TABLE IF NOT EXISTS servers (
  id                      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id                 UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  display_name            VARCHAR(255) NOT NULL,
  game                    VARCHAR(50) NOT NULL,
  subdomain               VARCHAR(63) UNIQUE,
  plan                    VARCHAR(50) NOT NULL,
  status                  VARCHAR(20) DEFAULT 'pending',
  status_message          TEXT,
  node_ip                 VARCHAR(45),
  stripe_subscription_id  VARCHAR(255),
  created_at              TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
  updated_at              TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
  stopped_at              TIMESTAMP WITH TIME ZONE,
  expired_at              TIMESTAMP WITH TIME ZONE,
  delete_after            TIMESTAMP WITH TIME ZONE
);

-- Indexes for common queries
CREATE INDEX IF NOT EXISTS idx_servers_user_id ON servers(user_id);
CREATE INDEX IF NOT EXISTS idx_servers_status ON servers(status);
CREATE INDEX IF NOT EXISTS idx_servers_delete_after ON servers(delete_after) WHERE delete_after IS NOT NULL;

-- Stores multiple ports per server
CREATE TABLE IF NOT EXISTS server_ports (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  server_id       UUID NOT NULL REFERENCES servers(id) ON DELETE CASCADE,
  name            VARCHAR(50) NOT NULL,  -- e.g., "game", "query", "rcon"
  container_port  INT NOT NULL,          -- Port inside container
  host_port       INT,                   -- Allocated port on node (from Agones)
  protocol        VARCHAR(10) NOT NULL,  -- TCP or UDP
  created_at      TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_server_ports_server_id ON server_ports(server_id);

-- Stores multiple volume mounts per server
CREATE TABLE IF NOT EXISTS server_volumes (
  id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  server_id    UUID NOT NULL REFERENCES servers(id) ON DELETE CASCADE,
  name         VARCHAR(50) NOT NULL,  -- e.g., "data", "logs", "config"
  mount_path   VARCHAR(255) NOT NULL, -- e.g., "/data", "/logs"
  sub_path     VARCHAR(255) NOT NULL, -- Subdirectory in PVC: "data", "logs"
  created_at   TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_server_volumes_server_id ON server_volumes(server_id);

-- Function to automatically update updated_at timestamp
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
  NEW.updated_at = NOW();
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Triggers to auto-update updated_at
CREATE TRIGGER update_users_updated_at
  BEFORE UPDATE ON users
  FOR EACH ROW
  EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_servers_updated_at
  BEFORE UPDATE ON servers
  FOR EACH ROW
  EXECUTE FUNCTION update_updated_at_column();
