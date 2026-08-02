package http

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/orderfulfillment/order/internal/domain/order"
	"github.com/orderfulfillment/order/internal/infra/logging"
)

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

// writeError maps domain and application errors to a stable HTTP envelope. It
// never leaks internal detail on 5xx responses.
func writeError(w http.ResponseWriter, r *http.Request, logger *slog.Logger, err error) {
	correlationID, _ := logging.CorrelationID(r.Context())
	status, code, message := classify(err)

	if status >= http.StatusInternalServerError {
		logger.ErrorContext(r.Context(), "request failed", slog.String("error", err.Error()), slog.Int("status", status))
		message = "an internal error occurred"
	} else {
		logger.WarnContext(r.Context(), "request rejected", slog.String("error", err.Error()), slog.Int("status", status))
	}

	writeJSON(w, status, errorEnvelope{Error: errorBody{Code: code, Message: message, CorrelationID: correlationID}})
}

func classify(err error) (int, string, string) {
	switch {
	case errors.Is(err, order.ErrValidation), errors.Is(err, order.ErrInvalidIdentifier):
		return http.StatusBadRequest, "VALIDATION_ERROR", err.Error()
	case errors.Is(err, order.ErrNotFound):
		return http.StatusNotFound, "NOT_FOUND", "order not found"
	case errors.Is(err, order.ErrConcurrency):
		return http.StatusConflict, "CONFLICT", "the resource was modified concurrently"
	default:
		return http.StatusInternalServerError, "INTERNAL", err.Error()
	}
}
