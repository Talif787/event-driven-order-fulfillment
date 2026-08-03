package com.orderfulfillment.payment.adapter.in.web.dto;

/** Stable error envelope shared with the other services: {"error":{code,message}}. */
public record ErrorResponse(ErrorBody error) {

    public record ErrorBody(String code, String message) {
    }

    public static ErrorResponse of(String code, String message) {
        return new ErrorResponse(new ErrorBody(code, message));
    }
}
