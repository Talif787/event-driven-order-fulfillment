-- Stock ledger, one row per SKU. available is sellable stock; reserved is held
-- but not yet shipped. version drives optimistic concurrency: every mutating
-- write increments it and is guarded by the previously observed value.
CREATE TABLE IF NOT EXISTS stock_items (
    sku        TEXT        NOT NULL,
    available  BIGINT      NOT NULL CHECK (available >= 0),
    reserved   BIGINT      NOT NULL DEFAULT 0 CHECK (reserved >= 0),
    version    BIGINT      NOT NULL DEFAULT 0,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (sku)
);

-- One reservation per order. order_id is unique, which makes reservation
-- creation idempotent under concurrent requests for the same order.
CREATE TABLE IF NOT EXISTS reservations (
    id         UUID        NOT NULL,
    order_id   UUID        NOT NULL,
    status     TEXT        NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (id),
    CONSTRAINT uq_reservations_order UNIQUE (order_id)
);

-- The held lines for each reservation.
CREATE TABLE IF NOT EXISTS reservation_lines (
    reservation_id UUID    NOT NULL REFERENCES reservations (id) ON DELETE CASCADE,
    sku            TEXT    NOT NULL,
    quantity       INTEGER NOT NULL CHECK (quantity > 0),
    PRIMARY KEY (reservation_id, sku)
);

CREATE INDEX IF NOT EXISTS ix_reservations_order ON reservations (order_id);
