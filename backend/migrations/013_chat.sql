CREATE SCHEMA IF NOT EXISTS chat;

CREATE TABLE IF NOT EXISTS chat.conversations (
    id UUID PRIMARY KEY,
    customer_id UUID NOT NULL,
    type VARCHAR(20) NOT NULL DEFAULT 'support'
        CHECK (type IN ('support')),
    status VARCHAR(20) NOT NULL DEFAULT 'open'
        CHECK (status IN ('open', 'closed')),
    last_message_sequence BIGINT NOT NULL DEFAULT 0
        CHECK (last_message_sequence >= 0),
    last_message_preview VARCHAR(300) NOT NULL DEFAULT '',
    last_message_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_chat_one_open_support_per_customer
    ON chat.conversations (customer_id)
    WHERE type = 'support' AND status = 'open';

CREATE INDEX IF NOT EXISTS idx_chat_conversations_cursor
    ON chat.conversations (updated_at DESC, id DESC);

CREATE TABLE IF NOT EXISTS chat.conversation_members (
    conversation_id UUID NOT NULL,
    user_id UUID NOT NULL,
    member_role VARCHAR(20) NOT NULL
        CHECK (member_role IN ('customer', 'admin')),
    last_read_sequence BIGINT NOT NULL DEFAULT 0
        CHECK (last_read_sequence >= 0),
    joined_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    left_at TIMESTAMPTZ,
    PRIMARY KEY (conversation_id, user_id),
    CONSTRAINT fk_chat_member_conversation
        FOREIGN KEY (conversation_id) REFERENCES chat.conversations(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_chat_members_user
    ON chat.conversation_members (user_id, conversation_id)
    WHERE left_at IS NULL;

CREATE TABLE IF NOT EXISTS chat.messages (
    id UUID PRIMARY KEY,
    conversation_id UUID NOT NULL,
    sender_id UUID NOT NULL,
    sender_name VARCHAR(120) NOT NULL,
    client_message_id UUID NOT NULL,
    sequence_number BIGINT NOT NULL CHECK (sequence_number > 0),
    content TEXT NOT NULL CHECK (char_length(content) BETWEEN 1 AND 4000),
    message_type VARCHAR(20) NOT NULL DEFAULT 'text'
        CHECK (message_type IN ('text')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    edited_at TIMESTAMPTZ,
    deleted_at TIMESTAMPTZ,
    CONSTRAINT fk_chat_message_conversation
        FOREIGN KEY (conversation_id) REFERENCES chat.conversations(id) ON DELETE RESTRICT,
    CONSTRAINT uq_chat_message_sequence UNIQUE (conversation_id, sequence_number),
    CONSTRAINT uq_chat_message_idempotency UNIQUE (sender_id, client_message_id)
);

CREATE INDEX IF NOT EXISTS idx_chat_messages_cursor
    ON chat.messages (conversation_id, sequence_number DESC);

CREATE TABLE IF NOT EXISTS chat.outbox_events (
    id UUID PRIMARY KEY,
    aggregate_id UUID NOT NULL,
    event_type VARCHAR(100) NOT NULL,
    trace_id VARCHAR(32) NOT NULL DEFAULT '',
    payload JSONB NOT NULL,
    attempts INTEGER NOT NULL DEFAULT 0,
    available_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    processing_at TIMESTAMPTZ,
    published_at TIMESTAMPTZ,
    last_error TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_chat_outbox_pending
    ON chat.outbox_events (available_at, created_at)
    WHERE published_at IS NULL;
