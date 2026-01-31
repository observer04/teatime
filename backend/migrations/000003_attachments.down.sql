-- Drop message_attachments table
DROP TABLE IF EXISTS message_attachments;

-- Drop attachment_id column from messages
DROP INDEX IF EXISTS idx_messages_attachment;
ALTER TABLE messages DROP COLUMN IF EXISTS attachment_id;

-- Drop attachments table
DROP INDEX IF EXISTS idx_attachments_created;
DROP INDEX IF EXISTS idx_attachments_conversation;
DROP INDEX IF EXISTS idx_attachments_uploader;
DROP TABLE IF EXISTS attachments;
