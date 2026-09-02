CREATE TABLE IF NOT EXISTS orders.customer_activity_outbox_events (
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

CREATE UNIQUE INDEX IF NOT EXISTS idx_order_activity_outbox_sequence
    ON orders.customer_activity_outbox_events (sequence_number);

CREATE INDEX IF NOT EXISTS idx_order_activity_outbox_pending
    ON orders.customer_activity_outbox_events (available_at, created_at)
    WHERE published_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_order_activity_outbox_aggregate_sequence
    ON orders.customer_activity_outbox_events (aggregate_id, sequence_number)
    WHERE published_at IS NULL;

CREATE TABLE IF NOT EXISTS analytics.customer_activity_inbox (
    event_id UUID PRIMARY KEY,
    topic TEXT NOT NULL,
    partition_id INTEGER NOT NULL,
    message_offset BIGINT NOT NULL,
    received_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (topic, partition_id, message_offset)
);

CREATE TABLE IF NOT EXISTS analytics.customer_activity_events (
    event_id UUID PRIMARY KEY,
    event_type TEXT NOT NULL CHECK (event_type IN (
        'book.viewed', 'book.searched', 'book.added_to_cart',
        'book.removed_from_cart', 'checkout.started',
        'order.confirmed', 'comment.created'
    )),
    schema_version INTEGER NOT NULL CHECK (schema_version > 0),
    actor_id UUID NOT NULL,
    user_id UUID,
    anonymous_id UUID,
    session_id UUID,
    book_id UUID,
    order_id UUID,
    comment_id UUID,
    search_query VARCHAR(200) NOT NULL DEFAULT '',
    quantity INTEGER NOT NULL DEFAULT 0 CHECK (quantity >= 0 AND quantity <= 100),
    source VARCHAR(40) NOT NULL,
    trace_id VARCHAR(64) NOT NULL DEFAULT '',
    occurred_at TIMESTAMPTZ NOT NULL,
    received_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_customer_activity_type_time
    ON analytics.customer_activity_events (event_type, occurred_at DESC);

CREATE INDEX IF NOT EXISTS idx_customer_activity_actor_time
    ON analytics.customer_activity_events (actor_id, occurred_at DESC);

CREATE INDEX IF NOT EXISTS idx_customer_activity_book_time
    ON analytics.customer_activity_events (book_id, occurred_at DESC)
    WHERE book_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_customer_activity_session_time
    ON analytics.customer_activity_events (session_id, occurred_at DESC)
    WHERE session_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_customer_activity_order
    ON analytics.customer_activity_events (order_id)
    WHERE order_id IS NOT NULL;
