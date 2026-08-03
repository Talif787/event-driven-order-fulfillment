package com.orderfulfillment.payment.application.port;

import java.util.UUID;

/**
 * Port to the payment provider (PSP). The charge is synchronous (authorize and
 * capture); settlement is confirmed later, either by a webhook or by the
 * reconciliation sweep calling fetchSettlement.
 */
public interface PaymentGateway {

    GatewayResult charge(UUID orderId, long amountMinor, String currency, String forceOutcome);

    GatewaySettlement fetchSettlement(String providerReference);

    record GatewayResult(boolean approved, String reference, String declineReason) {}

    enum GatewaySettlement { PENDING, SETTLED, FAILED }
}
