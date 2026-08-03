package com.orderfulfillment.payment.adapter.in.web.dto;

import com.orderfulfillment.payment.application.PaymentResult;

import java.util.UUID;

public record PaymentResponse(
        UUID paymentId,
        UUID orderId,
        String status,
        String providerReference,
        boolean approved,
        String declineReason) {

    public static PaymentResponse of(PaymentResult r) {
        return new PaymentResponse(r.paymentId(), r.orderId(), r.status(),
                r.providerReference(), r.approved(), r.declineReason());
    }
}
