package com.orderfulfillment.payment.adapter.in.web.dto;

import jakarta.validation.constraints.NotBlank;
import jakarta.validation.constraints.NotNull;
import jakarta.validation.constraints.Positive;
import jakarta.validation.constraints.Size;

import java.util.UUID;

public record CapturePaymentRequest(
        @NotNull UUID orderId,
        @Positive long amountMinor,
        @NotBlank @Size(min = 3, max = 3) String currency) {
}
