package com.orderfulfillment.payment.application;

/** Thrown when a webhook references a payment that does not exist. */
public class PaymentNotFoundException extends RuntimeException {
    public PaymentNotFoundException(String message) {
        super(message);
    }
}
