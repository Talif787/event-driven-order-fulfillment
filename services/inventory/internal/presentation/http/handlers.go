package http

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/orderfulfillment/inventory/internal/app"
	"github.com/orderfulfillment/inventory/internal/domain/inventory"
)

// Handlers is the HTTP adapter over the application use cases. The use cases are
// transport-agnostic, so a gRPC adapter for the saga can be added alongside this
// one without touching the domain or application layers.
type Handlers struct {
	reserve        *app.ReserveStockHandler
	release        *app.ReleaseReservationHandler
	commit         *app.CommitReservationHandler
	getReservation *app.GetReservationHandler
	getStock       *app.GetStockHandler
	setStock       *app.SetStockHandler
	logger         *slog.Logger
}

func NewHandlers(
	reserve *app.ReserveStockHandler,
	release *app.ReleaseReservationHandler,
	commit *app.CommitReservationHandler,
	getReservation *app.GetReservationHandler,
	getStock *app.GetStockHandler,
	setStock *app.SetStockHandler,
	logger *slog.Logger,
) *Handlers {
	return &Handlers{
		reserve:        reserve,
		release:        release,
		commit:         commit,
		getReservation: getReservation,
		getStock:       getStock,
		setStock:       setStock,
		logger:         logger,
	}
}

// Reserve holds stock for an order. Returns 201 on creation and 200 when the
// reservation already existed (idempotent replay).
func (h *Handlers) Reserve(w http.ResponseWriter, r *http.Request) {
	var req reserveRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, r, h.logger, err)
		return
	}
	cmd := app.ReserveStockCommand{OrderID: req.OrderID, Lines: toReserveInputs(req.Lines)}
	res, err := h.reserve.Handle(r.Context(), cmd)
	if err != nil {
		writeError(w, r, h.logger, err)
		return
	}
	status := http.StatusCreated
	if res.Idempotent {
		status = http.StatusOK
	}
	writeJSON(w, status, toReservationResponse(res))
}

// GetReservation returns the reservation for an order id.
func (h *Handlers) GetReservation(w http.ResponseWriter, r *http.Request) {
	res, err := h.getReservation.Handle(r.Context(), r.PathValue("orderId"))
	if err != nil {
		writeError(w, r, h.logger, err)
		return
	}
	writeJSON(w, http.StatusOK, toReservationResponse(res))
}

// Release returns held stock to available and marks the reservation RELEASED.
func (h *Handlers) Release(w http.ResponseWriter, r *http.Request) {
	res, err := h.release.Handle(r.Context(), r.PathValue("orderId"))
	if err != nil {
		writeError(w, r, h.logger, err)
		return
	}
	writeJSON(w, http.StatusOK, toReservationResponse(res))
}

// Commit finalizes held stock and marks the reservation COMMITTED.
func (h *Handlers) Commit(w http.ResponseWriter, r *http.Request) {
	res, err := h.commit.Handle(r.Context(), r.PathValue("orderId"))
	if err != nil {
		writeError(w, r, h.logger, err)
		return
	}
	writeJSON(w, http.StatusOK, toReservationResponse(res))
}

// GetStock returns the ledger for one SKU.
func (h *Handlers) GetStock(w http.ResponseWriter, r *http.Request) {
	view, err := h.getStock.Handle(r.Context(), r.PathValue("sku"))
	if err != nil {
		writeError(w, r, h.logger, err)
		return
	}
	writeJSON(w, http.StatusOK, stockResponse{
		SKU: view.SKU, Available: view.Available, Reserved: view.Reserved, Version: view.Version,
	})
}

// SetStock seeds or overwrites available stock for a SKU (admin operation).
func (h *Handlers) SetStock(w http.ResponseWriter, r *http.Request) {
	var req setStockRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, r, h.logger, err)
		return
	}
	cmd := app.SetStockCommand{SKU: r.PathValue("sku"), Available: req.Available}
	if err := h.setStock.Handle(r.Context(), cmd); err != nil {
		writeError(w, r, h.logger, err)
		return
	}
	view, err := h.getStock.Handle(r.Context(), r.PathValue("sku"))
	if err != nil {
		writeError(w, r, h.logger, err)
		return
	}
	writeJSON(w, http.StatusOK, stockResponse{
		SKU: view.SKU, Available: view.Available, Reserved: view.Reserved, Version: view.Version,
	})
}

func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return fmt.Errorf("%w: malformed request body", inventory.ErrValidation)
	}
	return nil
}

func toReserveInputs(lines []reserveLineDTO) []app.ReserveLineInput {
	out := make([]app.ReserveLineInput, 0, len(lines))
	for _, l := range lines {
		out = append(out, app.ReserveLineInput{SKU: l.SKU, Quantity: l.Quantity})
	}
	return out
}

func toReservationResponse(res app.ReservationResult) reservationResponse {
	lines := make([]reserveLineDTO, 0, len(res.Lines))
	for _, l := range res.Lines {
		lines = append(lines, reserveLineDTO{SKU: l.SKU, Quantity: l.Quantity})
	}
	return reservationResponse{
		ReservationID: res.ReservationID,
		OrderID:       res.OrderID,
		Status:        res.Status,
		Lines:         lines,
	}
}
