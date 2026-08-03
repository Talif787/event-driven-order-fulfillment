package com.orderfulfillment.payment.adapter.out.persistence;

import com.orderfulfillment.payment.domain.PaymentStatus;
import org.springframework.data.jpa.repository.JpaRepository;

import java.time.Instant;
import java.util.List;
import java.util.Optional;
import java.util.UUID;

interface PaymentJpaRepository extends JpaRepository<PaymentEntity, UUID> {
    Optional<PaymentEntity> findByOrderId(UUID orderId);

    Optional<PaymentEntity> findByProviderReference(String providerReference);

    List<PaymentEntity> findByStatusAndUpdatedAtBefore(PaymentStatus status, Instant threshold);
}
