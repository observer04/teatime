-- 000002_group_chat_features.down.sql
-- Rollback: Group Chat Features
-- =============================================================================

-- Drop read status tracking
DROP TABLE IF EXISTS conversation_read_status;

-- Drop search trigger and function
DROP TRIGGER IF EXISTS messages_search_vector_trigger ON messages;
DROP FUNCTION IF EXISTS messages_search_vector_update();

-- Drop search vector column
ALTER TABLE messages DROP COLUMN IF EXISTS search_vector;

-- Drop archived column
ALTER TABLE conversations DROP COLUMN IF EXISTS archived_at;

-- Drop starred messages
DROP TABLE IF EXISTS starred_messages;
