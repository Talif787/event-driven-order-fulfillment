-- One row per delivered notification. dedupe_key is the natural idempotency key
-- (event type plus order id): each logical event fires once per order, so a
-- redelivered event collides on this key and is skipped instead of resent.
CREATE TABLE notifications (
    id          UUID         NOT NULL,
    dedupe_key  TEXT         NOT NULL,
    event_type  TEXT         NOT NULL,
    order_id    UUID         NOT NULL,
    channel     TEXT         NOT NULL,
    recipient   TEXT         NOT NULL,
    subject     TEXT         NOT NULL,
    body        TEXT         NOT NULL,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT now(),
    CONSTRAINT pk_notifications PRIMARY KEY (id),
    CONSTRAINT uq_notifications_dedupe UNIQUE (dedupe_key)
);

CREATE INDEX ix_notifications_order ON notifications (order_id);
