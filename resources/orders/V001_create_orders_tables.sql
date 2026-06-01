-- +migrate Up
CREATE TABLE IF NOT EXISTS orders (
    id          BIGSERIAL    PRIMARY KEY,
    table_no    TEXT         NOT NULL,
    status      TEXT         NOT NULL DEFAULT 'pending',
    total_cents BIGINT       NOT NULL DEFAULT 0 CHECK (total_cents >= 0),
    note        TEXT         NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_orders_status ON orders (status);

CREATE TABLE IF NOT EXISTS order_items (
    id               BIGSERIAL   PRIMARY KEY,
    order_id         BIGINT      NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    menu_item_id     BIGINT      NOT NULL,
    name             TEXT        NOT NULL,
    unit_price_cents BIGINT      NOT NULL DEFAULT 0 CHECK (unit_price_cents >= 0),
    quantity         INTEGER     NOT NULL DEFAULT 1 CHECK (quantity > 0),
    subtotal_cents   BIGINT      NOT NULL DEFAULT 0 CHECK (subtotal_cents >= 0)
);

CREATE INDEX IF NOT EXISTS idx_order_items_order_id ON order_items (order_id);

-- +migrate Down
DROP TABLE IF EXISTS order_items;
DROP TABLE IF EXISTS orders;
