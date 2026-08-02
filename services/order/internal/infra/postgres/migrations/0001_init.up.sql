CREATE TABLE IF NOT EXISTS order_events (
    event_id    UUID        NOT NULL,
    order_id    UUID        NOT NULL,
    version     BIGINT      NOT NULL,
    event_type  TEXT        NOT NULL,
    payload     JSONB       NOT NULL,
    occurred_at TIMESTAMPTZ NOT NULL,
    trace_id    TEXT        NOT NULL DEFAULT '',
    recorded_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (event_id),
    CONSTRAINT uq_order_events_order_version UNIQUE (order_id, version)
);

CREATE INDEX IF NOT EXISTS ix_order_events_order_id ON order_events (order_id, version);

CREATE TABLE IF NOT EXISTS outbox (
    id           BIGINT      GENERATED ALWAYS AS IDENTITY,
    event_id     UUID        NOT NULL,
    aggregate_id UUID        NOT NULL,
    topic        TEXT        NOT NULL,
    event_type   TEXT        NOT NULL,
    payload      JSONB       NOT NULL,
    headers      JSONB       NOT NULL DEFAULT '{}'::jsonb,
    trace_id     TEXT        NOT NULL DEFAULT '',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    published_at TIMESTAMPTZ,
    PRIMARY KEY (id),
    CONSTRAINT uq_outbox_event_id UNIQUE (event_id)
);

CREATE INDEX IF NOT EXISTS ix_outbox_unpublished ON outbox (created_at) WHERE published_at IS NULL;

CREATE TABLE IF NOT EXISTS idempotency_keys (
    key        TEXT        NOT NULL,
    order_id   UUID        NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (key)
);
