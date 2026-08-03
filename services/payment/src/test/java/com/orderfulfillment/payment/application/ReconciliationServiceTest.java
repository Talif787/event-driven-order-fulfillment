package com.orderfulfillment.payment.application;

import com.orderfulfillment.payment.application.port.PaymentEventPublisher;
import com.orderfulfillment.payment.application.port.PaymentGateway;
import com.orderfulfillment.payment.application.port.PaymentGateway.GatewaySettlement;
import com.orderfulfillment.payment.application.port.PaymentRepository;
import com.orderfulfillment.payment.application.port.WebhookEventStore;
import com.orderfulfillment.payment.domain.Payment;
import com.orderfulfillment.payment.domain.PaymentStatus;
import org.junit.jupiter.api.Test;

import java.time.Duration;
import java.util.List;
import java.util.Optional;
import java.util.UUID;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.ArgumentMatchers.argThat;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.never;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.verifyNoInteractions;
import static org.mockito.Mockito.when;

class ReconciliationServiceTest {

    private final PaymentRepository payments = mock(PaymentRepository.class);
    private final WebhookEventStore webhookEvents = mock(WebhookEventStore.class);
    private final PaymentGateway gateway = mock(PaymentGateway.class);
    private final PaymentEventPublisher events = mock(PaymentEventPublisher.class);
    private final ReconciliationService service =
            new ReconciliationService(payments, webhookEvents, gateway, events, Duration.ofMinutes(2));

    private final UUID orderId = UUID.randomUUID();

    private Payment captured() {
        Payment p = Payment.open(orderId, 3000, "USD");
        p.capture("psp_1");
        return p;
    }

    @Test
    void settledWebhookMovesCapturedToSettled() {
        when(webhookEvents.recordIfNew("evt_1", "payment.settled")).thenReturn(true);
        Payment p = captured();
        when(payments.findByProviderReference("psp_1")).thenReturn(Optional.of(p));
        when(payments.save(any())).thenAnswer(inv -> inv.getArgument(0));

        service.handleWebhook(new WebhookCommand("evt_1", "payment.settled", "psp_1", null, null));

        assertEquals(PaymentStatus.SETTLED, p.getStatus());
        verify(events).publish(argThat(e -> e.type().equals("payment.settled.v1")));
    }

    @Test
    void duplicateWebhookIsIgnored() {
        when(webhookEvents.recordIfNew("evt_1", "payment.settled")).thenReturn(false);

        service.handleWebhook(new WebhookCommand("evt_1", "payment.settled", "psp_1", null, null));

        verifyNoInteractions(payments);
        verify(events, never()).publish(any());
    }

    @Test
    void sweepSettlesStaleCapturedFromGateway() {
        Payment p = captured();
        when(payments.findCapturedOlderThan(any())).thenReturn(List.of(p));
        when(gateway.fetchSettlement("psp_1")).thenReturn(GatewaySettlement.SETTLED);
        when(payments.save(any())).thenAnswer(inv -> inv.getArgument(0));

        service.sweepStaleCaptured();

        assertEquals(PaymentStatus.SETTLED, p.getStatus());
        verify(events).publish(argThat(e -> e.type().equals("payment.settled.v1")));
    }
}
