package com.orderfulfillment.payment.adapter.out.persistence;

import jakarta.persistence.Column;
import jakarta.persistence.Entity;
import jakarta.persistence.Id;
import jakarta.persistence.Table;

import java.time.Instant;

@Entity
@Table(name = "webhook_events")
public class WebhookEventEntity {

    @Id
    @Column(name = "event_id")
    private String eventId;

    @Column(name = "event_type", nullable = false)
    private String eventType;

    @Column(name = "received_at", nullable = false)
    private Instant receivedAt;

    protected WebhookEventEntity() {
    }

    WebhookEventEntity(String eventId, String eventType) {
        this.eventId = eventId;
        this.eventType = eventType;
        this.receivedAt = Instant.now();
    }
}
