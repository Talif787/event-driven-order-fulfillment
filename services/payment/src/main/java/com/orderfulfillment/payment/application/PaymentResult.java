package com.orderfulfillment.payment.application;

import com.orderfulfillment.payment.domain.Payment;

import java.util.UUID;

/** Outcome of a capture, returned to the caller (the order saga). */
public record PaymentResult(
        UUID paymentId,
        UUID orderId,
        String status,
        String providerReference,
        boolean approved,
        String declineReason) {

    public static PaymentResult of(Payment p) {
        return new PaymentResult(p.getId(), p.getOrderId(), p.getStatus().name(),
                p.getProviderReference(), p.isApproved(), p.getFailureReason());
    }
}
