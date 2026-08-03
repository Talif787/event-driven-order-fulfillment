package com.orderfulfillment.payment.application;

import java.util.UUID;

/**
 * Request to charge an order. forceOutcome is an optional demo/test override
 * ("approve" or "decline") passed through to the simulated gateway; null means
 * use the gateway default.
 */
public record CaptureCommand(UUID orderId, long amountMinor, String currency, String forceOutcome) {}
