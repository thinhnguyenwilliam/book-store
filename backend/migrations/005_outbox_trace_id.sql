ALTER TABLE auth.outbox_events
    ADD COLUMN IF NOT EXISTS trace_id VARCHAR(32) NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_outbox_trace_id
    ON auth.outbox_events (trace_id)
    WHERE trace_id <> '';
