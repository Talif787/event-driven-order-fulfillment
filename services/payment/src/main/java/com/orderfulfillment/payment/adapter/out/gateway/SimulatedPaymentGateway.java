package com.orderfulfillment.payment.adapter.out.gateway;

import com.orderfulfillment.payment.application.port.PaymentGateway;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.stereotype.Component;

import java.util.UUID;

/**
 * Stand-in for a real PSP. The charge is deterministic and configurable so both
 * the approve and decline paths are demoable, and fetchSettlement reports the
 * provider as settled, which is what lets the reconciliation sweep resolve a
 * captured payment whose settlement webhook never arrived. Swap this class for a
 * real provider client (Stripe, Adyen) without touching the application layer.
 */
@Component
public class SimulatedPaymentGateway implements PaymentGateway {

    private final String defaultOutcome;

    public SimulatedPaymentGateway(@Value("${payment.gateway.default-outcome}") String defaultOutcome) {
        this.defaultOutcome = defaultOutcome;
    }

    @Override
    public GatewayResult charge(UUID orderId, long amountMinor, String currency, String forceOutcome) {
        String outcome = (forceOutcome != null && !forceOutcome.isBlank()) ? forceOutcome : defaultOutcome;
        if ("decline".equalsIgnoreCase(outcome)) {
            return new GatewayResult(false, "", "card declined by issuer");
        }
        return new GatewayResult(true, "psp_" + UUID.randomUUID(), "");
    }

    @Override
    public GatewaySettlement fetchSettlement(String providerReference) {
        return GatewaySettlement.SETTLED;
    }
}
