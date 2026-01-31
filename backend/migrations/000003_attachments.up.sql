-- Attachments table for file uploads
CREATE TABLE IF NOT EXISTS attachments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    uploader_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    conversation_id UUID NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    bucket VARCHAR(255) NOT NULL,
    object_key VARCHAR(512) NOT NULL,
    filename VARCHAR(255) NOT NULL,
    mime_type VARCHAR(100) NOT NULL,
    size_bytes BIGINT NOT NULL,
    sha256 VARCHAR(64),
    status VARCHAR(20) NOT NULL DEFAULT 'uploading', -- uploading, ready, error
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    completed_at TIMESTAMP WITH TIME ZONE,
    CONSTRAINT attachments_status_check CHECK (status IN ('uploading', 'ready', 'error'))
);

CREATE INDEX idx_attachments_uploader ON attachments(uploader_id);
CREATE INDEX idx_attachments_conversation ON attachments(conversation_id);
CREATE INDEX idx_attachments_created ON attachments(created_at DESC);

-- Add attachment_id to messages table (nullable for backward compatibility)
ALTER TABLE messages ADD COLUMN attachment_id UUID REFERENCES attachments(id) ON DELETE SET NULL;

CREATE INDEX idx_messages_attachment ON messages(attachment_id) WHERE attachment_id IS NOT NULL;

-- Message attachments join table (for multiple attachments per message in the future)
CREATE TABLE IF NOT EXISTS message_attachments (
    message_id UUID NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
    attachment_id UUID NOT NULL REFERENCES attachments(id) ON DELETE CASCADE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (message_id, attachment_id)
);

CREATE INDEX idx_message_attachments_message ON message_attachments(message_id);
CREATE INDEX idx_message_attachments_attachment ON message_attachments(attachment_id);
