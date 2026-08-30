-- Foreign keys are intentionally limited to tables owned by the same service.
-- Cross-service references (user_id, order_id in another schema, seller_id)
-- remain logical IDs and are validated through gRPC/events.

CREATE INDEX IF NOT EXISTS idx_stock_reservations_book_id
    ON catalog.stock_reservations (book_id);

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'fk_order_items_order'
          AND conrelid = 'orders.order_items'::regclass
    ) THEN
        ALTER TABLE orders.order_items
            ADD CONSTRAINT fk_order_items_order
            FOREIGN KEY (order_id) REFERENCES orders.orders(id)
            ON DELETE CASCADE NOT VALID;
    END IF;
END $$;
ALTER TABLE orders.order_items VALIDATE CONSTRAINT fk_order_items_order;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'fk_stock_reservations_book'
          AND conrelid = 'catalog.stock_reservations'::regclass
    ) THEN
        ALTER TABLE catalog.stock_reservations
            ADD CONSTRAINT fk_stock_reservations_book
            FOREIGN KEY (book_id) REFERENCES catalog.books(id)
            ON DELETE RESTRICT NOT VALID;
    END IF;
END $$;
ALTER TABLE catalog.stock_reservations VALIDATE CONSTRAINT fk_stock_reservations_book;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'fk_payment_allocations_payment'
          AND conrelid = 'payments.payment_allocations'::regclass
    ) THEN
        ALTER TABLE payments.payment_allocations
            ADD CONSTRAINT fk_payment_allocations_payment
            FOREIGN KEY (payment_id) REFERENCES payments.payments(id)
            ON DELETE RESTRICT NOT VALID;
    END IF;
END $$;
ALTER TABLE payments.payment_allocations VALIDATE CONSTRAINT fk_payment_allocations_payment;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'fk_ledger_entries_transaction'
          AND conrelid = 'payments.ledger_entries'::regclass
    ) THEN
        ALTER TABLE payments.ledger_entries
            ADD CONSTRAINT fk_ledger_entries_transaction
            FOREIGN KEY (transaction_id) REFERENCES payments.ledger_transactions(id)
            ON DELETE RESTRICT NOT VALID;
    END IF;
END $$;
ALTER TABLE payments.ledger_entries VALIDATE CONSTRAINT fk_ledger_entries_transaction;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'fk_ledger_entries_wallet'
          AND conrelid = 'payments.ledger_entries'::regclass
    ) THEN
        ALTER TABLE payments.ledger_entries
            ADD CONSTRAINT fk_ledger_entries_wallet
            FOREIGN KEY (wallet_id) REFERENCES payments.wallets(id)
            ON DELETE RESTRICT NOT VALID;
    END IF;
END $$;
ALTER TABLE payments.ledger_entries VALIDATE CONSTRAINT fk_ledger_entries_wallet;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'fk_settlement_reconciliations_payment'
          AND conrelid = 'payments.settlement_reconciliations'::regclass
    ) THEN
        ALTER TABLE payments.settlement_reconciliations
            ADD CONSTRAINT fk_settlement_reconciliations_payment
            FOREIGN KEY (payment_id) REFERENCES payments.payments(id)
            ON DELETE RESTRICT NOT VALID;
    END IF;
END $$;
ALTER TABLE payments.settlement_reconciliations
    VALIDATE CONSTRAINT fk_settlement_reconciliations_payment;
