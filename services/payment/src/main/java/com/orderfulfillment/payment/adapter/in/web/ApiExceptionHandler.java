package com.orderfulfillment.payment.adapter.in.web;

import com.orderfulfillment.payment.adapter.in.web.dto.ErrorResponse;
import com.orderfulfillment.payment.application.PaymentNotFoundException;
import com.orderfulfillment.payment.domain.IllegalPaymentTransitionException;
import com.orderfulfillment.payment.security.InvalidSignatureException;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.dao.OptimisticLockingFailureException;
import org.springframework.http.HttpStatus;
import org.springframework.http.ResponseEntity;
import org.springframework.web.bind.MethodArgumentNotValidException;
import org.springframework.web.bind.annotation.ExceptionHandler;
import org.springframework.web.bind.annotation.RestControllerAdvice;

/** Maps domain and application errors to the shared HTTP error envelope. */
@RestControllerAdvice
public class ApiExceptionHandler {

    private static final Logger log = LoggerFactory.getLogger(ApiExceptionHandler.class);

    @ExceptionHandler(PaymentNotFoundException.class)
    public ResponseEntity<ErrorResponse> notFound(PaymentNotFoundException e) {
        return build(HttpStatus.NOT_FOUND, "NOT_FOUND", e.getMessage());
    }

    @ExceptionHandler(IllegalPaymentTransitionException.class)
    public ResponseEntity<ErrorResponse> illegalTransition(IllegalPaymentTransitionException e) {
        return build(HttpStatus.CONFLICT, "INVALID_STATE", e.getMessage());
    }

    @ExceptionHandler(OptimisticLockingFailureException.class)
    public ResponseEntity<ErrorResponse> concurrency(OptimisticLockingFailureException e) {
        return build(HttpStatus.CONFLICT, "CONFLICT", "the payment was modified concurrently, please retry");
    }

    @ExceptionHandler(InvalidSignatureException.class)
    public ResponseEntity<ErrorResponse> invalidSignature(InvalidSignatureException e) {
        return build(HttpStatus.UNAUTHORIZED, "INVALID_SIGNATURE", e.getMessage());
    }

    @ExceptionHandler({IllegalArgumentException.class, MethodArgumentNotValidException.class})
    public ResponseEntity<ErrorResponse> validation(Exception e) {
        return build(HttpStatus.BAD_REQUEST, "VALIDATION_ERROR", e.getMessage());
    }

    @ExceptionHandler(Exception.class)
    public ResponseEntity<ErrorResponse> internal(Exception e) {
        log.error("unhandled error", e);
        return build(HttpStatus.INTERNAL_SERVER_ERROR, "INTERNAL", "an internal error occurred");
    }

    private ResponseEntity<ErrorResponse> build(HttpStatus status, String code, String message) {
        return ResponseEntity.status(status).body(ErrorResponse.of(code, message));
    }
}
