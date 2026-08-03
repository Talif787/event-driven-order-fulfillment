package com.orderfulfillment.payment.adapter.out.persistence;

import com.orderfulfillment.payment.domain.Payment;
import com.orderfulfillment.payment.domain.PaymentStatus;
import jakarta.persistence.Column;
import jakarta.persistence.Entity;
import jakarta.persistence.EnumType;
import jakarta.persistence.Enumerated;
import jakarta.persistence.Id;
import jakarta.persistence.PostLoad;
import jakarta.persistence.PostPersist;
import jakarta.persistence.PrePersist;
import jakarta.persistence.PreUpdate;
import jakarta.persistence.Table;
import jakarta.persistence.Version;
import org.springframework.data.domain.Persistable;

import java.time.Instant;
import java.util.UUID;

@Entity
@Table(name = "payments")
public class PaymentEntity implements Persistable<UUID> {

    @Id
    private UUID id;

    @Column(name = "order_id", nullable = false, unique = true, updatable = false)
    private UUID orderId;

    @Column(name = "amount_minor", nullable = false, updatable = false)
    private long amountMinor;

    @Column(nullable = false, updatable = false)
    private String currency;

    @Enumerated(EnumType.STRING)
    @Column(nullable = false)
    private PaymentStatus status;

    @Column(name = "provider_reference", nullable = false)
    private String providerReference;

    @Column(name = "failure_reason", nullable = false)
    private String failureReason;

    @Version
    @Column(nullable = false)
    private long version;

    @Column(name = "created_at", updatable = false)
    private Instant createdAt;

    @Column(name = "updated_at")
    private Instant updatedAt;

    @jakarta.persistence.Transient
    private boolean isNew = true;

    protected PaymentEntity() {
    }

    static PaymentEntity fromDomain(Payment p) {
        PaymentEntity e = new PaymentEntity();
        e.id = p.getId();
        e.orderId = p.getOrderId();
        e.amountMinor = p.getAmountMinor();
        e.currency = p.getCurrency();
        e.status = p.getStatus();
        e.providerReference = p.getProviderReference();
        e.failureReason = p.getFailureReason();
        e.version = p.getVersion();
        return e;
    }

    void applyFrom(Payment p) {
        this.status = p.getStatus();
        this.providerReference = p.getProviderReference();
        this.failureReason = p.getFailureReason();
    }

    Payment toDomain() {
        return Payment.rehydrate(id, orderId, amountMinor, currency, status,
                providerReference, failureReason, version);
    }

    @PrePersist
    void onInsert() {
        Instant now = Instant.now();
        this.createdAt = now;
        this.updatedAt = now;
    }

    @PreUpdate
    void onUpdate() {
        this.updatedAt = Instant.now();
    }

    @PostLoad
    @PostPersist
    void markNotNew() {
        this.isNew = false;
    }

    @Override
    public UUID getId() {
        return id;
    }

    @Override
    public boolean isNew() {
        return isNew;
    }
}
