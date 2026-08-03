package com.orderfulfillment.payment.application.port;

/** Records processed webhook deliveries so redelivered events are skipped. */
public interface WebhookEventStore {
    /** Returns true if the event id was newly recorded, false if already seen. */
    boolean recordIfNew(String eventId, String eventType);
}
