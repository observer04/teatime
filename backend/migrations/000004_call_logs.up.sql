-- 000002_call_logs.up.sql
-- Call history for tracking video/audio calls

-- Call status enum
CREATE TYPE call_status AS ENUM ('ringing', 'active', 'ended', 'missed', 'declined', 'cancelled');
CREATE TYPE call_type AS ENUM ('video', 'audio');

-- Call logs table
CREATE TABLE call_logs (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    conversation_id UUID NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    initiator_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    call_type call_type NOT NULL DEFAULT 'video',
    status call_status NOT NULL DEFAULT 'ringing',
    started_at TIMESTAMPTZ, -- NULL until call is answered
    ended_at TIMESTAMPTZ,
    duration_seconds INTEGER DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Call participants (who joined the call)
CREATE TABLE call_participants (
    call_id UUID NOT NULL REFERENCES call_logs(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    joined_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    left_at TIMESTAMPTZ,
    PRIMARY KEY (call_id, user_id)
);

-- Indexes for call history lookup
CREATE INDEX idx_call_logs_conversation ON call_logs(conversation_id, created_at DESC);
CREATE INDEX idx_call_logs_initiator ON call_logs(initiator_id, created_at DESC);
CREATE INDEX idx_call_participants_user ON call_participants(user_id, joined_at DESC);

-- Index for finding calls involving a user (as initiator or participant)
CREATE INDEX idx_call_logs_created ON call_logs(created_at DESC);
