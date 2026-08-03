package com.orderfulfillment.payment.application;

import java.util.UUID;

/** A verified provider webhook, normalized for reconciliation. */
public record WebhookCommand(
        String eventId,
        String type,
        String providerReference,
        UUID orderId,
        String reason) {}
