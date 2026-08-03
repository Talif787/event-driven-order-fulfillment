package com.orderfulfillment.payment.domain;

/** Thrown when a payment is asked to make a transition its status forbids. */
public class IllegalPaymentTransitionException extends RuntimeException {
    public IllegalPaymentTransitionException(String message) {
        super(message);
    }
}
