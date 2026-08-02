package http

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/orderfulfillment/order/internal/app/command"
	"github.com/orderfulfillment/order/internal/app/query"
	"github.com/orderfulfillment/order/internal/domain/order"
)

const maxBodyBytes = 1 << 20 // 1 MiB

// Handlers holds the HTTP entry points and their application dependencies.
type Handlers struct {
	place  *command.PlaceOrderHandler
	get    *query.GetOrderHandler
	logger *slog.Logger
}

func NewHandlers(place *command.PlaceOrderHandler, get *query.GetOrderHandler, logger *slog.Logger) *Handlers {
	return &Handlers{place: place, get: get, logger: logger}
}

func (h *Handlers) PlaceOrder(w http.ResponseWriter, r *http.Request) {
	idempotencyKey := r.Header.Get("Idempotency-Key")
	if idempotencyKey == "" {
		writeError(w, r, h.logger, wrapValidation("Idempotency-Key header is required"))
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	var req placeOrderRequest
	if err := dec.Decode(&req); err != nil {
		writeError(w, r, h.logger, wrapValidation("malformed request body: "+err.Error()))
		return
	}

	customerID := req.CustomerID
	if p, ok := principalFromContext(r.Context()); ok {
		customerID = p.CustomerID
	}

	cmd := command.PlaceOrderCommand{
		CustomerID:     customerID,
		IdempotencyKey: idempotencyKey,
		Lines:          toCommandLines(req.Items),
		Ship: command.ShipTo{
			Line1: req.ShipTo.Line1, Line2: req.ShipTo.Line2, City: req.ShipTo.City,
			Region: req.ShipTo.Region, PostalCode: req.ShipTo.PostalCode, Country: req.ShipTo.Country,
		},
	}

	res, err := h.place.Handle(r.Context(), cmd)
	if err != nil {
		writeError(w, r, h.logger, err)
		return
	}
	writeJSON(w, http.StatusAccepted, placeOrderResponse{OrderID: res.OrderID, Status: res.Status})
}

func (h *Handlers) GetOrder(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	view, err := h.get.Handle(r.Context(), id)
	if err != nil {
		writeError(w, r, h.logger, err)
		return
	}
	writeJSON(w, http.StatusOK, orderResponse{
		OrderID: view.OrderID, CustomerID: view.CustomerID, Status: view.Status,
		TotalMinor: view.TotalMinor, Currency: view.Currency, Version: view.Version,
	})
}

func toCommandLines(items []itemDTO) []command.PlaceOrderLine {
	lines := make([]command.PlaceOrderLine, 0, len(items))
	for _, it := range items {
		lines = append(lines, command.PlaceOrderLine{
			SKU: it.SKU, Quantity: it.Quantity, UnitPriceMinor: it.UnitPriceMinor, Currency: it.Currency,
		})
	}
	return lines
}

func wrapValidation(msg string) error {
	return &validationError{msg: msg}
}

type validationError struct{ msg string }

func (e *validationError) Error() string { return e.msg }
func (e *validationError) Is(target error) bool {
	return target == order.ErrValidation
}

