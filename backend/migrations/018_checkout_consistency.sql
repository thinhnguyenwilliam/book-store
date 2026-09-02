ALTER TABLE orders.order_items
    ADD COLUMN IF NOT EXISTS cart_item_id UUID,
    ADD COLUMN IF NOT EXISTS cart_updated_at TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS idx_order_items_cart_snapshot
    ON orders.order_items (cart_item_id)
    WHERE cart_item_id IS NOT NULL;
