CREATE SCHEMA IF NOT EXISTS analytics;

CREATE TABLE IF NOT EXISTS orders.outbox_events (
    id UUID PRIMARY KEY,
    sequence_number BIGSERIAL NOT NULL,
    aggregate_id UUID NOT NULL,
    event_type TEXT NOT NULL,
    trace_id VARCHAR(64) NOT NULL DEFAULT '',
    payload JSONB NOT NULL,
    attempts INTEGER NOT NULL DEFAULT 0,
    available_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    processing_at TIMESTAMPTZ,
    published_at TIMESTAMPTZ,
    last_error TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

ALTER TABLE orders.outbox_events
    ADD COLUMN IF NOT EXISTS sequence_number BIGSERIAL;

CREATE UNIQUE INDEX IF NOT EXISTS idx_order_outbox_sequence
    ON orders.outbox_events (sequence_number);

CREATE INDEX IF NOT EXISTS idx_order_outbox_pending
    ON orders.outbox_events (available_at, created_at)
    WHERE published_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_order_outbox_aggregate_sequence
    ON orders.outbox_events (aggregate_id, sequence_number)
    WHERE published_at IS NULL;

-- A monotonic database sequence lets every dispatcher serialize events per
-- aggregate, including existing RabbitMQ outboxes when multiple replicas run.
ALTER TABLE auth.outbox_events
    ADD COLUMN IF NOT EXISTS sequence_number BIGSERIAL;
ALTER TABLE payments.outbox_events
    ADD COLUMN IF NOT EXISTS sequence_number BIGSERIAL;
ALTER TABLE chat.outbox_events
    ADD COLUMN IF NOT EXISTS sequence_number BIGSERIAL;

CREATE UNIQUE INDEX IF NOT EXISTS idx_auth_outbox_sequence
    ON auth.outbox_events (sequence_number);
CREATE UNIQUE INDEX IF NOT EXISTS idx_payment_outbox_sequence
    ON payments.outbox_events (sequence_number);
CREATE UNIQUE INDEX IF NOT EXISTS idx_chat_outbox_sequence
    ON chat.outbox_events (sequence_number);

CREATE INDEX IF NOT EXISTS idx_auth_outbox_aggregate_sequence
    ON auth.outbox_events (aggregate_id, sequence_number)
    WHERE published_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_payment_outbox_aggregate_sequence
    ON payments.outbox_events (aggregate_id, sequence_number)
    WHERE published_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_chat_outbox_aggregate_sequence
    ON chat.outbox_events (aggregate_id, sequence_number)
    WHERE published_at IS NULL;

-- Idempotent inbox: a replay or duplicate delivery cannot update the read model twice.
CREATE TABLE IF NOT EXISTS analytics.kafka_inbox_events (
    event_id UUID PRIMARY KEY,
    topic TEXT NOT NULL,
    partition_id INTEGER NOT NULL,
    message_offset BIGINT NOT NULL,
    event_type TEXT NOT NULL,
    aggregate_id UUID NOT NULL,
    received_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    processed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (topic, partition_id, message_offset)
);

CREATE TABLE IF NOT EXISTS analytics.order_lifecycle (
    order_id UUID PRIMARY KEY,
    user_id UUID NOT NULL,
    status TEXT NOT NULL,
    total_cents BIGINT NOT NULL CHECK (total_cents >= 0),
    currency CHAR(3) NOT NULL,
    failure_stage TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL,
    stock_reserved_at TIMESTAMPTZ,
    payment_pending_at TIMESTAMPTZ,
    payment_succeeded_at TIMESTAMPTZ,
    payment_failed_at TIMESTAMPTZ,
    confirmed_at TIMESTAMPTZ,
    cancelled_at TIMESTAMPTZ,
    compensation_pending_at TIMESTAMPTZ,
    last_event_id UUID NOT NULL,
    last_event_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_order_lifecycle_created
    ON analytics.order_lifecycle (created_at DESC, order_id DESC);

CREATE INDEX IF NOT EXISTS idx_order_lifecycle_status_created
    ON analytics.order_lifecycle (status, created_at DESC);
