package com.orderfulfillment.payment.domain;

/**
 * Lifecycle of a payment.
 *
 * <p>The synchronous charge moves PENDING to CAPTURED (approved) or DECLINED.
 * Asynchronous provider webhooks then reconcile a captured payment to SETTLED,
 * REFUNDED, or FAILED. DECLINED, REFUNDED, and FAILED are terminal.
 */
public enum PaymentStatus {
    PENDING,
    CAPTURED,
    DECLINED,
    SETTLED,
    REFUNDED,
    FAILED
}
