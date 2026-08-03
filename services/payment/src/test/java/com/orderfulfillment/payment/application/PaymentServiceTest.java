package com.orderfulfillment.payment.application;

import com.orderfulfillment.payment.application.port.PaymentEventPublisher;
import com.orderfulfillment.payment.application.port.PaymentGateway;
import com.orderfulfillment.payment.application.port.PaymentGateway.GatewayResult;
import com.orderfulfillment.payment.application.port.PaymentRepository;
import com.orderfulfillment.payment.domain.Payment;
import org.junit.jupiter.api.Test;

import java.util.Optional;
import java.util.UUID;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertTrue;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.ArgumentMatchers.anyLong;
import static org.mockito.ArgumentMatchers.argThat;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.never;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.verifyNoInteractions;
import static org.mockito.Mockito.when;

class PaymentServiceTest {

    private final PaymentRepository payments = mock(PaymentRepository.class);
    private final PaymentGateway gateway = mock(PaymentGateway.class);
    private final PaymentEventPublisher events = mock(PaymentEventPublisher.class);
    private final PaymentService service = new PaymentService(payments, gateway, events);

    private final UUID orderId = UUID.randomUUID();

    @Test
    void approvedChargeCaptures() {
        when(payments.findByOrderId(orderId)).thenReturn(Optional.empty());
        when(gateway.charge(any(), anyLong(), any(), any()))
                .thenReturn(new GatewayResult(true, "psp_1", ""));
        when(payments.save(any())).thenAnswer(inv -> inv.getArgument(0));

        PaymentResult result = service.capture(new CaptureCommand(orderId, 3000, "USD", null));

        assertTrue(result.approved());
        assertEquals("CAPTURED", result.status());
        verify(events).publish(argThat(e -> e.type().equals("payment.captured.v1")));
    }

    @Test
    void declinedChargeDeclines() {
        when(payments.findByOrderId(orderId)).thenReturn(Optional.empty());
        when(gateway.charge(any(), anyLong(), any(), any()))
                .thenReturn(new GatewayResult(false, "", "card declined"));
        when(payments.save(any())).thenAnswer(inv -> inv.getArgument(0));

        PaymentResult result = service.capture(new CaptureCommand(orderId, 3000, "USD", null));

        assertFalse(result.approved());
        assertEquals("DECLINED", result.status());
        verify(events).publish(argThat(e -> e.type().equals("payment.declined.v1")));
    }

    @Test
    void idempotentReplayReturnsExistingWithoutCharging() {
        Payment existing = Payment.open(orderId, 3000, "USD");
        existing.capture("psp_x");
        when(payments.findByOrderId(orderId)).thenReturn(Optional.of(existing));

        PaymentResult result = service.capture(new CaptureCommand(orderId, 3000, "USD", null));

        assertTrue(result.approved());
        verifyNoInteractions(gateway);
        verify(events, never()).publish(any());
    }
}
