package com.orderfulfillment.payment.adapter.out.persistence;

import com.orderfulfillment.payment.application.port.PaymentRepository;
import com.orderfulfillment.payment.domain.Payment;
import com.orderfulfillment.payment.domain.PaymentStatus;
import org.springframework.stereotype.Repository;

import java.time.Instant;
import java.util.List;
import java.util.Optional;
import java.util.UUID;

/**
 * Bridges the domain repository port to Spring Data JPA. On save it loads the
 * managed entity within the current transaction and copies the mutable fields
 * onto it, so Hibernate's @Version optimistic locking uses the version the
 * aggregate was read at. A missing row is a new insert.
 */
@Repository
public class PaymentRepositoryAdapter implements PaymentRepository {

    private final PaymentJpaRepository jpa;

    public PaymentRepositoryAdapter(PaymentJpaRepository jpa) {
        this.jpa = jpa;
    }

    @Override
    public Optional<Payment> findByOrderId(UUID orderId) {
        return jpa.findByOrderId(orderId).map(PaymentEntity::toDomain);
    }

    @Override
    public Optional<Payment> findByProviderReference(String providerReference) {
        return jpa.findByProviderReference(providerReference).map(PaymentEntity::toDomain);
    }

    @Override
    public Payment save(Payment payment) {
        PaymentEntity entity = jpa.findById(payment.getId())
                .map(existing -> {
                    existing.applyFrom(payment);
                    return existing;
                })
                .orElseGet(() -> PaymentEntity.fromDomain(payment));
        return jpa.save(entity).toDomain();
    }

    @Override
    public List<Payment> findCapturedOlderThan(Instant threshold) {
        return jpa.findByStatusAndUpdatedAtBefore(PaymentStatus.CAPTURED, threshold)
                .stream().map(PaymentEntity::toDomain).toList();
    }
}
