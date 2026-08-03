package com.orderfulfillment.payment.application;

import com.orderfulfillment.payment.application.port.PaymentEventPublisher;
import com.orderfulfillment.payment.application.port.PaymentEventPublisher.PaymentEvent;
import com.orderfulfillment.payment.application.port.PaymentGateway;
import com.orderfulfillment.payment.application.port.PaymentGateway.GatewaySettlement;
import com.orderfulfillment.payment.application.port.PaymentRepository;
import com.orderfulfillment.payment.application.port.WebhookEventStore;
import com.orderfulfillment.payment.domain.Payment;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.scheduling.annotation.Scheduled;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Transactional;

import java.time.Duration;
import java.time.Instant;
import java.util.Optional;

/**
 * Reconciles payment state with the provider. Two paths converge on the same
 * transitions: inbound webhooks (fast path) and a periodic sweep that catches
 * captured payments whose settlement webhook never arrived (safety net). Both
 * are idempotent: webhooks dedupe on the provider event id, the sweep relies on
 * the aggregate's idempotent transitions, and neither re-emits an event when the
 * state did not actually change.
 */
@Service
public class ReconciliationService {

    private static final Logger log = LoggerFactory.getLogger(ReconciliationService.class);

    private final PaymentRepository payments;
    private final WebhookEventStore webhookEvents;
    private final PaymentGateway gateway;
    private final PaymentEventPublisher events;
    private final Duration staleAfter;

    public ReconciliationService(PaymentRepository payments,
                                 WebhookEventStore webhookEvents,
                                 PaymentGateway gateway,
                                 PaymentEventPublisher events,
                                 @Value("${payment.reconciliation.stale-after}") Duration staleAfter) {
        this.payments = payments;
        this.webhookEvents = webhookEvents;
        this.gateway = gateway;
        this.events = events;
        this.staleAfter = staleAfter;
    }

    @Transactional
    public void handleWebhook(WebhookCommand cmd) {
        if (!webhookEvents.recordIfNew(cmd.eventId(), cmd.type())) {
            log.info("duplicate webhook eventId={} ignored", cmd.eventId());
            return;
        }

        Payment payment = locate(cmd)
                .orElseThrow(() -> new PaymentNotFoundException(
                        "no payment for reference " + cmd.providerReference()));

        boolean changed;
        String eventType;
        switch (cmd.type()) {
            case "payment.settled" -> {
                changed = payment.settle();
                eventType = "payment.settled.v1";
            }
            case "payment.refunded" -> {
                changed = payment.refund();
                eventType = "payment.refunded.v1";
            }
            case "payment.failed" -> {
                changed = payment.fail(cmd.reason());
                eventType = "payment.failed.v1";
            }
            default -> {
                log.info("unhandled webhook type={} ignored", cmd.type());
                return;
            }
        }

        if (changed) {
            payments.save(payment);
            events.publish(PaymentEvent.of(eventType, payment));
            log.info("reconciled orderId={} via webhook to {}", payment.getOrderId(), payment.getStatus());
        }
    }

    private Optional<Payment> locate(WebhookCommand cmd) {
        if (cmd.providerReference() != null && !cmd.providerReference().isBlank()) {
            Optional<Payment> byRef = payments.findByProviderReference(cmd.providerReference());
            if (byRef.isPresent()) {
                return byRef;
            }
        }
        if (cmd.orderId() != null) {
            return payments.findByOrderId(cmd.orderId());
        }
        return Optional.empty();
    }

    @Scheduled(fixedDelayString = "${payment.reconciliation.poll-interval-ms}")
    @Transactional
    public void sweepStaleCaptured() {
        Instant threshold = Instant.now().minus(staleAfter);
        for (Payment payment : payments.findCapturedOlderThan(threshold)) {
            GatewaySettlement settlement = gateway.fetchSettlement(payment.getProviderReference());
            switch (settlement) {
                case SETTLED -> {
                    if (payment.settle()) {
                        payments.save(payment);
                        events.publish(PaymentEvent.of("payment.settled.v1", payment));
                        log.info("swept orderId={} to SETTLED", payment.getOrderId());
                    }
                }
                case FAILED -> {
                    if (payment.fail("settlement failed")) {
                        payments.save(payment);
                        events.publish(PaymentEvent.of("payment.failed.v1", payment));
                        log.info("swept orderId={} to FAILED", payment.getOrderId());
                    }
                }
                case PENDING -> {
                    // Provider has not settled yet; leave for the next sweep.
                }
            }
        }
    }
}
