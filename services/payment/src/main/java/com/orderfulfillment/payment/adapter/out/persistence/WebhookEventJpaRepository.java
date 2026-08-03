package com.orderfulfillment.payment.adapter.out.persistence;

import org.springframework.data.jpa.repository.JpaRepository;

interface WebhookEventJpaRepository extends JpaRepository<WebhookEventEntity, String> {
}
