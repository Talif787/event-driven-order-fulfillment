-- One shipment per order (order_id unique) makes creation idempotent: a
-- redelivered order.confirmed returns the existing shipment instead of a
-- duplicate. version drives optimistic concurrency on the advance transitions.
CREATE TABLE shipments (
    id             UUID         NOT NULL,
    order_id       UUID         NOT NULL,
    status         TEXT         NOT NULL,
    carrier        TEXT         NOT NULL DEFAULT '',
    tracking_code  TEXT         NOT NULL DEFAULT '',
    failure_reason TEXT         NOT NULL DEFAULT '',
    version        BIGINT       NOT NULL DEFAULT 0,
    created_at     TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ  NOT NULL DEFAULT now(),
    CONSTRAINT pk_shipments PRIMARY KEY (id),
    CONSTRAINT uq_shipments_order UNIQUE (order_id)
);

CREATE INDEX ix_shipments_status ON shipments (status);
