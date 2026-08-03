package com.orderfulfillment.payment.adapter.in.web;

import com.fasterxml.jackson.databind.ObjectMapper;
import com.orderfulfillment.payment.adapter.in.web.dto.WebhookRequest;
import com.orderfulfillment.payment.application.ReconciliationService;
import com.orderfulfillment.payment.application.WebhookCommand;
import com.orderfulfillment.payment.security.InvalidSignatureException;
import com.orderfulfillment.payment.security.WebhookSignatureVerifier;
import org.springframework.http.ResponseEntity;
import org.springframework.web.bind.annotation.PostMapping;
import org.springframework.web.bind.annotation.RequestBody;
import org.springframework.web.bind.annotation.RequestHeader;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RestController;

/**
 * Receives provider webhooks. The signature is verified over the raw body
 * before parsing; reconciliation then dedupes on the event id and applies the
 * idempotent transition.
 */
@RestController
@RequestMapping("/v1/payments/webhooks")
public class WebhookController {

    private final WebhookSignatureVerifier verifier;
    private final ReconciliationService reconciliation;
    private final ObjectMapper mapper;

    public WebhookController(WebhookSignatureVerifier verifier,
                             ReconciliationService reconciliation,
                             ObjectMapper mapper) {
        this.verifier = verifier;
        this.reconciliation = reconciliation;
        this.mapper = mapper;
    }

    @PostMapping
    public ResponseEntity<Void> receive(
            @RequestBody String rawBody,
            @RequestHeader(value = "X-Signature", required = false) String signature) {
        if (!verifier.verify(rawBody, signature)) {
            throw new InvalidSignatureException("invalid webhook signature");
        }
        WebhookRequest event = parse(rawBody);
        reconciliation.handleWebhook(new WebhookCommand(
                event.eventId(), event.type(), event.providerReference(), event.orderId(), event.reason()));
        return ResponseEntity.accepted().build();
    }

    private WebhookRequest parse(String rawBody) {
        try {
            return mapper.readValue(rawBody, WebhookRequest.class);
        } catch (Exception e) {
            throw new IllegalArgumentException("malformed webhook body");
        }
    }
}
