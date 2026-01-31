-- 000002_group_chat_features.up.sql
-- Group Chat Features: Starred messages, message search, archived conversations
-- =============================================================================

-- =============================================================================
-- Starred Messages
-- =============================================================================

CREATE TABLE starred_messages (
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    message_id UUID NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
    starred_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, message_id)
);

-- Index for fetching user's starred messages
CREATE INDEX idx_starred_messages_user ON starred_messages(user_id, starred_at DESC);

-- =============================================================================
-- Archived Conversations
-- =============================================================================

ALTER TABLE conversations ADD COLUMN IF NOT EXISTS archived_at TIMESTAMPTZ;

-- Index for filtering archived conversations
CREATE INDEX idx_conversations_archived ON conversations(archived_at) WHERE archived_at IS NOT NULL;

-- =============================================================================
-- Full-Text Search on Messages
-- =============================================================================

-- Add tsvector column for full-text search
ALTER TABLE messages ADD COLUMN IF NOT EXISTS search_vector tsvector;

-- Create index for full-text search
CREATE INDEX IF NOT EXISTS idx_messages_search ON messages USING GIN(search_vector);

-- Function to update search vector
CREATE OR REPLACE FUNCTION messages_search_vector_update() RETURNS TRIGGER AS $$
BEGIN
    NEW.search_vector := to_tsvector('english', COALESCE(NEW.body_text, ''));
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Trigger to automatically update search vector
DROP TRIGGER IF EXISTS messages_search_vector_trigger ON messages;
CREATE TRIGGER messages_search_vector_trigger
    BEFORE INSERT OR UPDATE ON messages
    FOR EACH ROW EXECUTE FUNCTION messages_search_vector_update();

-- Update existing messages
UPDATE messages SET search_vector = to_tsvector('english', COALESCE(body_text, ''));

-- =============================================================================
-- Unread Messages Tracking
-- =============================================================================

-- Track last read message per user per conversation
CREATE TABLE IF NOT EXISTS conversation_read_status (
    conversation_id UUID NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    last_read_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_read_message_id UUID REFERENCES messages(id) ON DELETE SET NULL,
    PRIMARY KEY (conversation_id, user_id)
);

-- Index for quick lookup
CREATE INDEX idx_read_status_user ON conversation_read_status(user_id);
