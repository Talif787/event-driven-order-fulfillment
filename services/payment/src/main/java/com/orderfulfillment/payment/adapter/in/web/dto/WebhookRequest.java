package com.orderfulfillment.payment.adapter.in.web.dto;

import com.fasterxml.jackson.annotation.JsonIgnoreProperties;

import java.util.UUID;

/**
 * Provider webhook payload. eventId dedupes redeliveries; type selects the
 * transition; providerReference or orderId locates the payment.
 */
@JsonIgnoreProperties(ignoreUnknown = true)
public record WebhookRequest(
        String eventId,
        String type,
        String providerReference,
        UUID orderId,
        String reason) {
}
