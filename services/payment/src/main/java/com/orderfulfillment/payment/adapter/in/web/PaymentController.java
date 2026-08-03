package com.orderfulfillment.payment.adapter.in.web;

import com.orderfulfillment.payment.adapter.in.web.dto.CapturePaymentRequest;
import com.orderfulfillment.payment.adapter.in.web.dto.PaymentResponse;
import com.orderfulfillment.payment.application.CaptureCommand;
import com.orderfulfillment.payment.application.PaymentResult;
import com.orderfulfillment.payment.application.PaymentService;
import jakarta.validation.Valid;
import org.springframework.http.HttpStatus;
import org.springframework.http.ResponseEntity;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.PathVariable;
import org.springframework.web.bind.annotation.PostMapping;
import org.springframework.web.bind.annotation.RequestBody;
import org.springframework.web.bind.annotation.RequestHeader;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RestController;

import java.util.UUID;

@RestController
@RequestMapping("/v1/payments")
public class PaymentController {

    private final PaymentService service;

    public PaymentController(PaymentService service) {
        this.service = service;
    }

    /**
     * Charges an order. Idempotent on order id: a retry returns the original
     * result. The optional X-Force-Outcome header (approve or decline) drives
     * the simulated gateway for demos and tests.
     */
    @PostMapping
    public ResponseEntity<PaymentResponse> capture(
            @Valid @RequestBody CapturePaymentRequest request,
            @RequestHeader(value = "X-Force-Outcome", required = false) String forceOutcome) {
        PaymentResult result = service.capture(new CaptureCommand(
                request.orderId(), request.amountMinor(), request.currency(), forceOutcome));
        return ResponseEntity.status(HttpStatus.CREATED).body(PaymentResponse.of(result));
    }

    @GetMapping("/{orderId}")
    public PaymentResponse getByOrderId(@PathVariable UUID orderId) {
        return PaymentResponse.of(service.getByOrderId(orderId));
    }
}
