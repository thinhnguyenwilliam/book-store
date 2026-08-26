CREATE SCHEMA IF NOT EXISTS orders;
CREATE SCHEMA IF NOT EXISTS payments;

ALTER TABLE catalog.books
    ADD COLUMN IF NOT EXISTS seller_id UUID;

CREATE INDEX IF NOT EXISTS idx_books_seller_id
    ON catalog.books (seller_id);

CREATE TABLE IF NOT EXISTS catalog.stock_reservations (
    id UUID PRIMARY KEY,
    order_id UUID NOT NULL,
    book_id UUID NOT NULL,
    quantity INTEGER NOT NULL CHECK (quantity > 0),
    status TEXT NOT NULL CHECK (status IN ('reserved', 'committed', 'released')),
    idempotency_key TEXT NOT NULL UNIQUE,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (order_id, book_id)
);

CREATE INDEX IF NOT EXISTS idx_stock_reservations_expiry
    ON catalog.stock_reservations (expires_at)
    WHERE status = 'reserved';

CREATE TABLE IF NOT EXISTS orders.cart_items (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL,
    book_id UUID NOT NULL,
    quantity INTEGER NOT NULL CHECK (quantity > 0 AND quantity <= 100),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (user_id, book_id)
);

CREATE INDEX IF NOT EXISTS idx_cart_items_user
    ON orders.cart_items (user_id, created_at);

CREATE TABLE IF NOT EXISTS orders.orders (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL,
    status TEXT NOT NULL,
    total_cents BIGINT NOT NULL CHECK (total_cents >= 0),
    currency CHAR(3) NOT NULL DEFAULT 'VND',
    payment_id UUID,
    failure_reason TEXT NOT NULL DEFAULT '',
    idempotency_key TEXT NOT NULL,
    reservation_expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (user_id, idempotency_key)
);

ALTER TABLE orders.orders
    ADD COLUMN IF NOT EXISTS reservation_expires_at TIMESTAMPTZ;

UPDATE orders.orders
SET reservation_expires_at = created_at + INTERVAL '15 minutes'
WHERE reservation_expires_at IS NULL;

ALTER TABLE orders.orders
    ALTER COLUMN reservation_expires_at SET NOT NULL;

CREATE INDEX IF NOT EXISTS idx_orders_user_created
    ON orders.orders (user_id, created_at DESC, id DESC);

CREATE INDEX IF NOT EXISTS idx_orders_reconciliation
    ON orders.orders (reservation_expires_at, created_at)
    WHERE status IN ('pending', 'stock_reserved', 'payment_pending', 'compensation_pending');

CREATE INDEX IF NOT EXISTS idx_orders_payment_reconciliation
    ON orders.orders (updated_at, created_at)
    WHERE status = 'payment_pending';

CREATE TABLE IF NOT EXISTS orders.order_items (
    id UUID PRIMARY KEY,
    order_id UUID NOT NULL,
    book_id UUID NOT NULL,
    seller_id TEXT NOT NULL,
    title TEXT NOT NULL,
    unit_price_cents BIGINT NOT NULL CHECK (unit_price_cents >= 0),
    quantity INTEGER NOT NULL CHECK (quantity > 0),
    subtotal_cents BIGINT NOT NULL CHECK (subtotal_cents >= 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_order_items_order
    ON orders.order_items (order_id);

CREATE TABLE IF NOT EXISTS payments.wallets (
    id UUID PRIMARY KEY,
    owner_id TEXT NOT NULL UNIQUE,
    balance_cents BIGINT NOT NULL DEFAULT 0,
    currency CHAR(3) NOT NULL DEFAULT 'VND',
    allow_negative BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS payments.payments (
    id UUID PRIMARY KEY,
    order_id UUID NOT NULL UNIQUE,
    buyer_id UUID NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('succeeded', 'failed', 'refunded')),
    amount_cents BIGINT NOT NULL CHECK (amount_cents > 0),
    platform_fee_cents BIGINT NOT NULL CHECK (platform_fee_cents >= 0),
    currency CHAR(3) NOT NULL DEFAULT 'VND',
    failure_reason TEXT NOT NULL DEFAULT '',
    idempotency_key TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (buyer_id, idempotency_key)
);

CREATE TABLE IF NOT EXISTS payments.ledger_transactions (
    id UUID PRIMARY KEY,
    kind TEXT NOT NULL,
    reference_id TEXT NOT NULL,
    idempotency_key TEXT NOT NULL UNIQUE,
    description TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS payments.ledger_entries (
    id UUID PRIMARY KEY,
    transaction_id UUID NOT NULL,
    wallet_id UUID NOT NULL,
    amount_cents BIGINT NOT NULL CHECK (amount_cents <> 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_ledger_entries_transaction
    ON payments.ledger_entries (transaction_id);

CREATE INDEX IF NOT EXISTS idx_ledger_entries_wallet
    ON payments.ledger_entries (wallet_id, created_at DESC);
