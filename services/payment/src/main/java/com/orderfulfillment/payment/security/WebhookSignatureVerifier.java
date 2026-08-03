package com.orderfulfillment.payment.security;

import org.springframework.beans.factory.annotation.Value;
import org.springframework.stereotype.Component;

import javax.crypto.Mac;
import javax.crypto.spec.SecretKeySpec;
import java.nio.charset.StandardCharsets;
import java.security.GeneralSecurityException;
import java.security.MessageDigest;
import java.util.HexFormat;

/**
 * Verifies inbound webhooks with an HMAC-SHA256 signature over the raw request
 * body, using a shared secret. The comparison is constant time to avoid leaking
 * the expected signature through timing. Verification runs on the raw bytes
 * before the body is parsed, matching how real providers sign payloads.
 */
@Component
public class WebhookSignatureVerifier {

    private final byte[] secret;

    public WebhookSignatureVerifier(@Value("${payment.webhook.secret}") String secret) {
        this.secret = secret.getBytes(StandardCharsets.UTF_8);
    }

    public boolean verify(String rawBody, String signatureHeader) {
        if (signatureHeader == null || signatureHeader.isBlank()) {
            return false;
        }
        String expected = hmacSha256Hex(rawBody);
        return MessageDigest.isEqual(
                expected.getBytes(StandardCharsets.UTF_8),
                signatureHeader.trim().getBytes(StandardCharsets.UTF_8));
    }

    private String hmacSha256Hex(String rawBody) {
        try {
            Mac mac = Mac.getInstance("HmacSHA256");
            mac.init(new SecretKeySpec(secret, "HmacSHA256"));
            byte[] digest = mac.doFinal(rawBody.getBytes(StandardCharsets.UTF_8));
            return HexFormat.of().formatHex(digest);
        } catch (GeneralSecurityException e) {
            throw new IllegalStateException("compute webhook signature", e);
        }
    }
}
