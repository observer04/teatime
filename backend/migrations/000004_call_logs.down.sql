-- 000002_call_logs.down.sql
-- Rollback call history tables

DROP INDEX IF EXISTS idx_call_logs_created;
DROP INDEX IF EXISTS idx_call_participants_user;
DROP INDEX IF EXISTS idx_call_logs_initiator;
DROP INDEX IF EXISTS idx_call_logs_conversation;

DROP TABLE IF EXISTS call_participants;
DROP TABLE IF EXISTS call_logs;

DROP TYPE IF EXISTS call_type;
DROP TYPE IF EXISTS call_status;
