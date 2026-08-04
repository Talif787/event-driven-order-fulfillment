package http

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/google/uuid"

	"github.com/orderfulfillment/fulfillment/internal/app"
	"github.com/orderfulfillment/fulfillment/internal/domain/fulfillment"
)

// Handlers serve the shipment HTTP API.
type Handlers struct {
	service *app.Service
	logger  *slog.Logger
}

func NewHandlers(service *app.Service, logger *slog.Logger) *Handlers {
	return &Handlers{service: service, logger: logger}
}

// GetShipment returns the shipment for an order.
func (h *Handlers) GetShipment(w http.ResponseWriter, r *http.Request) {
	orderID, err := parseOrderID(r)
	if err != nil {
		writeError(w, r, h.logger, err)
		return
	}
	shipment, err := h.service.Get(r.Context(), orderID)
	if err != nil {
		writeError(w, r, h.logger, err)
		return
	}
	writeJSON(w, http.StatusOK, toShipmentResponse(shipment))
}

// Dispatch advances a shipment to DISPATCHED.
func (h *Handlers) Dispatch(w http.ResponseWriter, r *http.Request) {
	orderID, err := parseOrderID(r)
	if err != nil {
		writeError(w, r, h.logger, err)
		return
	}
	var body dispatchRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, r, h.logger, fmt.Errorf("%w: malformed body", fulfillment.ErrValidation))
		return
	}
	shipment, err := h.service.Dispatch(r.Context(), orderID, body.Carrier, body.TrackingCode)
	if err != nil {
		writeError(w, r, h.logger, err)
		return
	}
	writeJSON(w, http.StatusOK, toShipmentResponse(shipment))
}

// Deliver advances a shipment to DELIVERED.
func (h *Handlers) Deliver(w http.ResponseWriter, r *http.Request) {
	orderID, err := parseOrderID(r)
	if err != nil {
		writeError(w, r, h.logger, err)
		return
	}
	shipment, err := h.service.Deliver(r.Context(), orderID)
	if err != nil {
		writeError(w, r, h.logger, err)
		return
	}
	writeJSON(w, http.StatusOK, toShipmentResponse(shipment))
}

func parseOrderID(r *http.Request) (uuid.UUID, error) {
	id, err := uuid.Parse(r.PathValue("orderId"))
	if err != nil {
		return uuid.Nil, fmt.Errorf("%w: orderId must be a uuid", fulfillment.ErrInvalidIdentifier)
	}
	return id, nil
}
