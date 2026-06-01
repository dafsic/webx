-- +migrate Up
CREATE TABLE IF NOT EXISTS menu_items (
    id          BIGSERIAL   PRIMARY KEY,
    name        TEXT        NOT NULL,
    description TEXT        NOT NULL DEFAULT '',
    price_cents BIGINT      NOT NULL DEFAULT 0 CHECK (price_cents >= 0),
    available   BOOLEAN     NOT NULL DEFAULT TRUE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_menu_items_available ON menu_items (available);

-- +migrate Down
DROP TABLE IF EXISTS menu_items;
