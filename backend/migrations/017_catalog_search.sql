CREATE TABLE IF NOT EXISTS catalog.outbox_events (
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

CREATE UNIQUE INDEX IF NOT EXISTS idx_catalog_outbox_sequence
    ON catalog.outbox_events (sequence_number);

CREATE INDEX IF NOT EXISTS idx_catalog_outbox_pending
    ON catalog.outbox_events (available_at, created_at)
    WHERE published_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_catalog_outbox_aggregate_sequence
    ON catalog.outbox_events (aggregate_id, sequence_number)
    WHERE published_at IS NULL;
