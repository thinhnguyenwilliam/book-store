ALTER TABLE payments.payments
    DROP CONSTRAINT IF EXISTS payments_status_check;

ALTER TABLE payments.payments
    ADD CONSTRAINT payments_status_check
    CHECK (status IN ('pending', 'succeeded', 'failed', 'refund_pending', 'refunded'));

ALTER TABLE payments.payments
    ADD COLUMN IF NOT EXISTS provider TEXT NOT NULL DEFAULT 'wallet',
    ADD COLUMN IF NOT EXISTS provider_reference TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS provider_transaction_id TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS checkout_url TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS expires_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS paid_at TIMESTAMPTZ;

CREATE UNIQUE INDEX IF NOT EXISTS idx_payments_provider_transaction
    ON payments.payments (provider, provider_transaction_id)
    WHERE provider_transaction_id <> '';

DROP INDEX IF EXISTS payments.idx_payments_pending_reconciliation;

CREATE INDEX idx_payments_pending_reconciliation
    ON payments.payments (updated_at, id)
    WHERE status IN ('pending', 'refund_pending') AND provider <> 'wallet';

CREATE TABLE IF NOT EXISTS payments.payment_allocations (
    payment_id UUID NOT NULL,
    seller_id TEXT NOT NULL,
    amount_cents BIGINT NOT NULL CHECK (amount_cents > 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (payment_id, seller_id)
);

CREATE TABLE IF NOT EXISTS payments.webhook_events (
    id UUID PRIMARY KEY,
    provider TEXT NOT NULL,
    event_id TEXT NOT NULL,
    provider_reference TEXT NOT NULL DEFAULT '',
    signature_valid BOOLEAN NOT NULL,
    payload JSONB NOT NULL,
    processing_error TEXT NOT NULL DEFAULT '',
    processed_at TIMESTAMPTZ,
    received_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (provider, event_id)
);

CREATE INDEX IF NOT EXISTS idx_payment_webhook_reference
    ON payments.webhook_events (provider, provider_reference, received_at DESC);

CREATE TABLE IF NOT EXISTS payments.outbox_events (
    id UUID PRIMARY KEY,
    aggregate_id UUID NOT NULL,
    event_type TEXT NOT NULL,
    trace_id VARCHAR(32) NOT NULL DEFAULT '',
    payload JSONB NOT NULL,
    attempts INTEGER NOT NULL DEFAULT 0,
    available_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    processing_at TIMESTAMPTZ,
    published_at TIMESTAMPTZ,
    last_error TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_payment_outbox_pending
    ON payments.outbox_events (available_at, created_at)
    WHERE published_at IS NULL;

CREATE TABLE IF NOT EXISTS payments.settlement_reconciliations (
    id UUID PRIMARY KEY,
    payment_id UUID NOT NULL,
    provider TEXT NOT NULL,
    provider_transaction_id TEXT NOT NULL DEFAULT '',
    local_status TEXT NOT NULL,
    provider_status TEXT NOT NULL,
    expected_amount_cents BIGINT NOT NULL,
    provider_amount_cents BIGINT NOT NULL,
    matched BOOLEAN NOT NULL,
    mismatch_reason TEXT NOT NULL DEFAULT '',
    checked_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_settlement_payment_checked
    ON payments.settlement_reconciliations (payment_id, checked_at DESC);
