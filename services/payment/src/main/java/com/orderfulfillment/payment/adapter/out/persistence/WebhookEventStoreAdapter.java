package com.orderfulfillment.payment.adapter.out.persistence;

import com.orderfulfillment.payment.application.port.WebhookEventStore;
import org.springframework.stereotype.Repository;

@Repository
public class WebhookEventStoreAdapter implements WebhookEventStore {

    private final WebhookEventJpaRepository jpa;

    public WebhookEventStoreAdapter(WebhookEventJpaRepository jpa) {
        this.jpa = jpa;
    }

    @Override
    public boolean recordIfNew(String eventId, String eventType) {
        if (jpa.existsById(eventId)) {
            return false;
        }
        jpa.save(new WebhookEventEntity(eventId, eventType));
        return true;
    }
}
