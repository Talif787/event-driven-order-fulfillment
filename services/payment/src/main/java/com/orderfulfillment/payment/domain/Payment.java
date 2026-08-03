package com.orderfulfillment.payment.domain;

import java.util.UUID;

/**
 * Payment aggregate. State changes go through guarded transitions rather than
 * setters. Each transition is idempotent: re-applying the current state is a
 * no-op that returns false (no event to publish), a valid move returns true,
 * and an illegal move throws. This lets both the synchronous charge and the
 * asynchronous webhook path retry safely.
 */
public class Payment {

    private final UUID id;
    private final UUID orderId;
    private final long amountMinor;
    private final String currency;
    private PaymentStatus status;
    private String providerReference;
    private String failureReason;
    private long version;

    private Payment(UUID id, UUID orderId, long amountMinor, String currency,
                    PaymentStatus status, String providerReference, String failureReason, long version) {
        this.id = id;
        this.orderId = orderId;
        this.amountMinor = amountMinor;
        this.currency = currency;
        this.status = status;
        this.providerReference = providerReference;
        this.failureReason = failureReason;
        this.version = version;
    }

    /** Opens a new payment for an order in the PENDING state. */
    public static Payment open(UUID orderId, long amountMinor, String currency) {
        if (orderId == null) {
            throw new IllegalArgumentException("orderId is required");
        }
        if (amountMinor <= 0) {
            throw new IllegalArgumentException("amountMinor must be positive");
        }
        if (currency == null || currency.length() != 3) {
            throw new IllegalArgumentException("currency must be a 3 letter code");
        }
        return new Payment(UUID.randomUUID(), orderId, amountMinor, currency,
                PaymentStatus.PENDING, "", "", 0);
    }

    /** Rebuilds an aggregate from persisted state. Used by the persistence adapter. */
    public static Payment rehydrate(UUID id, UUID orderId, long amountMinor, String currency,
                                    PaymentStatus status, String providerReference, String failureReason, long version) {
        return new Payment(id, orderId, amountMinor, currency, status, providerReference, failureReason, version);
    }

    /** PENDING to CAPTURED. Idempotent when already captured. */
    public boolean capture(String reference) {
        if (status == PaymentStatus.CAPTURED) {
            return false;
        }
        if (status != PaymentStatus.PENDING) {
            throw new IllegalPaymentTransitionException("cannot capture a " + status + " payment");
        }
        this.status = PaymentStatus.CAPTURED;
        this.providerReference = reference == null ? "" : reference;
        return true;
    }

    /** PENDING to DECLINED. Idempotent when already declined. */
    public boolean decline(String reason) {
        if (status == PaymentStatus.DECLINED) {
            return false;
        }
        if (status != PaymentStatus.PENDING) {
            throw new IllegalPaymentTransitionException("cannot decline a " + status + " payment");
        }
        this.status = PaymentStatus.DECLINED;
        this.failureReason = reason == null ? "" : reason;
        return true;
    }

    /** CAPTURED to SETTLED. Idempotent when already settled. */
    public boolean settle() {
        if (status == PaymentStatus.SETTLED) {
            return false;
        }
        if (status != PaymentStatus.CAPTURED) {
            throw new IllegalPaymentTransitionException("cannot settle a " + status + " payment");
        }
        this.status = PaymentStatus.SETTLED;
        return true;
    }

    /** CAPTURED or SETTLED to REFUNDED. Idempotent when already refunded. */
    public boolean refund() {
        if (status == PaymentStatus.REFUNDED) {
            return false;
        }
        if (status != PaymentStatus.CAPTURED && status != PaymentStatus.SETTLED) {
            throw new IllegalPaymentTransitionException("cannot refund a " + status + " payment");
        }
        this.status = PaymentStatus.REFUNDED;
        return true;
    }

    /** CAPTURED to FAILED (settlement failure). Idempotent when already failed. */
    public boolean fail(String reason) {
        if (status == PaymentStatus.FAILED) {
            return false;
        }
        if (status != PaymentStatus.CAPTURED) {
            throw new IllegalPaymentTransitionException("cannot fail a " + status + " payment");
        }
        this.status = PaymentStatus.FAILED;
        this.failureReason = reason == null ? "" : reason;
        return true;
    }

    public boolean isApproved() {
        return status == PaymentStatus.CAPTURED || status == PaymentStatus.SETTLED;
    }

    public UUID getId() { return id; }
    public UUID getOrderId() { return orderId; }
    public long getAmountMinor() { return amountMinor; }
    public String getCurrency() { return currency; }
    public PaymentStatus getStatus() { return status; }
    public String getProviderReference() { return providerReference; }
    public String getFailureReason() { return failureReason; }
    public long getVersion() { return version; }
}
