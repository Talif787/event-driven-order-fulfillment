package com.orderfulfillment.payment.application;

import com.orderfulfillment.payment.application.port.PaymentEventPublisher;
import com.orderfulfillment.payment.application.port.PaymentEventPublisher.PaymentEvent;
import com.orderfulfillment.payment.application.port.PaymentGateway;
import com.orderfulfillment.payment.application.port.PaymentGateway.GatewayResult;
import com.orderfulfillment.payment.application.port.PaymentRepository;
import com.orderfulfillment.payment.domain.Payment;
import io.micrometer.core.instrument.MeterRegistry;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.dao.DataIntegrityViolationException;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Transactional;

import java.util.Optional;

/**
 * Charges orders idempotently. One payment exists per order (unique order_id),
 * so a retried charge for the same order returns the original result instead of
 * charging again. The order saga is the sole caller and processes each order on
 * a single partition, so retries arrive sequentially and the order_id lookup
 * covers them; the duplicate-key catch guards the rare concurrent case.
 */
@Service
public class PaymentService {

    private static final Logger log = LoggerFactory.getLogger(PaymentService.class);

    private final PaymentRepository payments;
    private final PaymentGateway gateway;
    private final PaymentEventPublisher events;
    private final MeterRegistry meters;

    public PaymentService(PaymentRepository payments, PaymentGateway gateway,
                          PaymentEventPublisher events, MeterRegistry meters) {
        this.payments = payments;
        this.gateway = gateway;
        this.events = events;
        this.meters = meters;
    }

    @Transactional
    public PaymentResult capture(CaptureCommand cmd) {
        Optional<Payment> existing = payments.findByOrderId(cmd.orderId());
        if (existing.isPresent()) {
            log.info("idempotent capture replay orderId={}", cmd.orderId());
            return PaymentResult.of(existing.get());
        }

        Payment payment = Payment.open(cmd.orderId(), cmd.amountMinor(), cmd.currency());
        GatewayResult result = gateway.charge(cmd.orderId(), cmd.amountMinor(), cmd.currency(), cmd.forceOutcome());

        String eventType;
        if (result.approved()) {
            payment.capture(result.reference());
            eventType = "payment.captured.v1";
        } else {
            payment.decline(result.declineReason());
            eventType = "payment.declined.v1";
        }

        try {
            payment = payments.save(payment);
        } catch (DataIntegrityViolationException duplicate) {
            log.warn("concurrent capture for orderId={}, returning existing", cmd.orderId());
            return payments.findByOrderId(cmd.orderId())
                    .map(PaymentResult::of)
                    .orElseThrow(() -> duplicate);
        }

        events.publish(PaymentEvent.of(eventType, payment));
        meters.counter("payments_total", "outcome", result.approved() ? "captured" : "declined").increment();
        log.info("capture orderId={} status={} reference={}",
                payment.getOrderId(), payment.getStatus(), payment.getProviderReference());
        return PaymentResult.of(payment);
    }

    @Transactional(readOnly = true)
    public PaymentResult getByOrderId(java.util.UUID orderId) {
        return payments.findByOrderId(orderId)
                .map(PaymentResult::of)
                .orElseThrow(() -> new PaymentNotFoundException("no payment for order " + orderId));
    }
}
