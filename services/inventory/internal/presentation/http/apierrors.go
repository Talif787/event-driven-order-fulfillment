package http

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/orderfulfillment/inventory/internal/domain/inventory"
	"github.com/orderfulfillment/inventory/internal/infra/logging"
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
	case errors.Is(err, inventory.ErrValidation), errors.Is(err, inventory.ErrInvalidIdentifier):
		return http.StatusBadRequest, "VALIDATION_ERROR", err.Error()
	case errors.Is(err, inventory.ErrSKUNotFound), errors.Is(err, inventory.ErrReservationNotFound):
		return http.StatusNotFound, "NOT_FOUND", err.Error()
	case errors.Is(err, inventory.ErrInsufficientStock):
		return http.StatusConflict, "INSUFFICIENT_STOCK", err.Error()
	case errors.Is(err, inventory.ErrConcurrency):
		return http.StatusConflict, "CONFLICT", "the resource was modified concurrently, please retry"
	case errors.Is(err, inventory.ErrInvalidTransition):
		return http.StatusConflict, "INVALID_STATE", err.Error()
	default:
		return http.StatusInternalServerError, "INTERNAL", err.Error()
	}
}
