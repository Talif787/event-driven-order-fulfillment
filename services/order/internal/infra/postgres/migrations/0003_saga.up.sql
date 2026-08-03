-- Durable saga instances for the order fulfillment orchestration. One row per
-- order, upserted as the saga advances, so the orchestrator resumes correctly
-- after a restart and ignores redelivered events for terminal sagas.
CREATE TABLE IF NOT EXISTS saga_instances (
    order_id         UUID        NOT NULL,
    state            TEXT        NOT NULL,
    reservation_held BOOLEAN     NOT NULL DEFAULT false,
    payment_ref      TEXT        NOT NULL DEFAULT '',
    reason           TEXT        NOT NULL DEFAULT '',
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (order_id)
);

CREATE INDEX IF NOT EXISTS ix_saga_instances_state ON saga_instances (state);
