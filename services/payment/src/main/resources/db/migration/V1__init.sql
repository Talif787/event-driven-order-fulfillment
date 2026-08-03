-- Payment records. One payment per order (order_id UNIQUE) makes the capture
-- endpoint idempotent: a retried charge for the same order returns the existing
-- record instead of charging twice. version drives optimistic locking so a
-- settlement webhook and the reconciliation sweeper cannot clobber each other.
CREATE TABLE payments (
    id                 UUID         NOT NULL,
    order_id           UUID         NOT NULL,
    amount_minor       BIGINT       NOT NULL,
    currency           TEXT         NOT NULL,
    status             TEXT         NOT NULL,
    provider_reference TEXT         NOT NULL DEFAULT '',
    failure_reason     TEXT         NOT NULL DEFAULT '',
    version            BIGINT       NOT NULL DEFAULT 0,
    created_at         TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at         TIMESTAMPTZ  NOT NULL DEFAULT now(),
    CONSTRAINT pk_payments PRIMARY KEY (id),
    CONSTRAINT uq_payments_order UNIQUE (order_id)
);

CREATE INDEX ix_payments_status ON payments (status);
CREATE INDEX ix_payments_reference ON payments (provider_reference);

-- Processed webhook deliveries, keyed on the provider event id, so a redelivered
-- webhook is recognized and skipped (idempotent reconciliation).
CREATE TABLE webhook_events (
    event_id     TEXT         NOT NULL,
    event_type   TEXT         NOT NULL,
    received_at  TIMESTAMPTZ  NOT NULL DEFAULT now(),
    CONSTRAINT pk_webhook_events PRIMARY KEY (event_id)
);
