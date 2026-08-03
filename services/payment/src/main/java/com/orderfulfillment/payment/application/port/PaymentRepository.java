package com.orderfulfillment.payment.application.port;

import com.orderfulfillment.payment.domain.Payment;

import java.time.Instant;
import java.util.List;
import java.util.Optional;
import java.util.UUID;

/** Persistence port for payment aggregates. */
public interface PaymentRepository {
    Optional<Payment> findByOrderId(UUID orderId);

    Optional<Payment> findByProviderReference(String providerReference);

    Payment save(Payment payment);

    /** Captured payments not yet settled, last updated before the threshold. */
    List<Payment> findCapturedOlderThan(Instant threshold);
}
