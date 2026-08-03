package com.orderfulfillment.payment.security;

/** Thrown when an inbound webhook fails HMAC signature verification. */
public class InvalidSignatureException extends RuntimeException {
    public InvalidSignatureException(String message) {
        super(message);
    }
}
