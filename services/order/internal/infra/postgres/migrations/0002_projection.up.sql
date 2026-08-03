CREATE TABLE IF NOT EXISTS order_projections (
    order_id    UUID        NOT NULL,
    customer_id UUID        NOT NULL,
    status      TEXT        NOT NULL,
    total_minor BIGINT      NOT NULL,
    currency    TEXT        NOT NULL,
    version     BIGINT      NOT NULL,
    placed_at   TIMESTAMPTZ NOT NULL,
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (order_id)
);

CREATE INDEX IF NOT EXISTS ix_order_projections_customer ON order_projections (customer_id);
CREATE INDEX IF NOT EXISTS ix_order_projections_status ON order_projections (status);
