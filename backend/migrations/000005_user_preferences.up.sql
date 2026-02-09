-- Add user preferences and status tracking
ALTER TABLE users 
ADD COLUMN IF NOT EXISTS show_online_status BOOLEAN NOT NULL DEFAULT true,
ADD COLUMN IF NOT EXISTS read_receipts_enabled BOOLEAN NOT NULL DEFAULT true,
ADD COLUMN IF NOT EXISTS last_seen_at TIMESTAMPTZ;

-- Index for efficient online status queries
CREATE INDEX IF NOT EXISTS idx_users_last_seen ON users(last_seen_at) WHERE show_online_status = true;
