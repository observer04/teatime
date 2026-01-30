-- 000001_init_schema.down.sql
-- Rollback initial schema
-- =============================================================================

DROP TRIGGER IF EXISTS update_conversations_updated_at ON conversations;
DROP TRIGGER IF EXISTS update_credentials_updated_at ON credentials;
DROP TRIGGER IF EXISTS update_users_updated_at ON users;
DROP FUNCTION IF EXISTS update_updated_at_column();

DROP TABLE IF EXISTS reports;
DROP TABLE IF EXISTS blocks;
DROP TABLE IF EXISTS message_receipts;
DROP TABLE IF EXISTS messages;
DROP TABLE IF EXISTS conversation_members;
DROP TABLE IF EXISTS conversations;
DROP TABLE IF EXISTS oauth_identities;
DROP TABLE IF EXISTS refresh_tokens;
DROP TABLE IF EXISTS credentials;
DROP TABLE IF EXISTS users;

DROP TYPE IF EXISTS member_role;
DROP TYPE IF EXISTS conversation_type;
