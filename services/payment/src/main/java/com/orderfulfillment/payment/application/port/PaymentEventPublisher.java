package com.orderfulfillment.payment.application.port;

import com.orderfulfillment.payment.domain.Payment;

import java.util.UUID;

/** Port that publishes payment integration events to the event backbone. */
public interface PaymentEventPublisher {

    void publish(PaymentEvent event);

    record PaymentEvent(
            String type,
            UUID paymentId,
            UUID orderId,
            long amountMinor,
            String currency,
            String status,
            String providerReference) {

        public static PaymentEvent of(String type, Payment p) {
            return new PaymentEvent(type, p.getId(), p.getOrderId(), p.getAmountMinor(),
                    p.getCurrency(), p.getStatus().name(), p.getProviderReference());
        }
    }
}
