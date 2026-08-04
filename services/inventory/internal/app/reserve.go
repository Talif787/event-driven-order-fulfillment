package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"go.opentelemetry.io/otel/trace"

	"github.com/orderfulfillment/inventory/internal/domain/inventory"
	"github.com/orderfulfillment/inventory/internal/infra/metrics"
)

// ReserveLineInput is one requested line on the reserve command.
type ReserveLineInput struct {
	SKU      string
	Quantity int32
}

// ReserveStockCommand requests a hold on stock for an order.
type ReserveStockCommand struct {
	OrderID string
	Lines   []ReserveLineInput
}

// ReserveStockHandler validates a reservation request and delegates the
// transactional hold to the store.
type ReserveStockHandler struct {
	store  InventoryStore
	logger *slog.Logger
	tracer trace.Tracer
}

func NewReserveStockHandler(store InventoryStore, logger *slog.Logger, tracer trace.Tracer) *ReserveStockHandler {
	return &ReserveStockHandler{store: store, logger: logger, tracer: tracer}
}

func (h *ReserveStockHandler) Handle(ctx context.Context, cmd ReserveStockCommand) (ReservationResult, error) {
	ctx, span := h.tracer.Start(ctx, "ReserveStock.Handle")
	defer span.End()

	oid, err := inventory.ParseOrderID(cmd.OrderID)
	if err != nil {
		return ReservationResult{}, err
	}
	if len(cmd.Lines) == 0 {
		return ReservationResult{}, fmt.Errorf("%w: at least one line is required", inventory.ErrValidation)
	}

	lines := make([]ReserveLine, 0, len(cmd.Lines))
	seen := make(map[string]bool, len(cmd.Lines))
	for _, in := range cmd.Lines {
		sku, err := inventory.NewSKU(in.SKU)
		if err != nil {
			return ReservationResult{}, err
		}
		qty, err := inventory.NewQuantity(in.Quantity)
		if err != nil {
			return ReservationResult{}, err
		}
		if seen[sku.String()] {
			return ReservationResult{}, fmt.Errorf("%w: duplicate sku %s", inventory.ErrValidation, sku)
		}
		seen[sku.String()] = true
		lines = append(lines, ReserveLine{SKU: sku.String(), Quantity: qty.Value()})
	}

	res, err := h.store.Reserve(ctx, oid.String(), lines)
	if err != nil {
		if errors.Is(err, inventory.ErrInsufficientStock) {
			metrics.Reservations.WithLabelValues("rejected").Inc()
		}
		return ReservationResult{}, err
	}
	metrics.Reservations.WithLabelValues(strings.ToLower(res.Status)).Inc()
	h.logger.InfoContext(ctx, "stock reserved",
		slog.String("order_id", oid.String()),
		slog.String("status", res.Status),
		slog.Bool("idempotent", res.Idempotent),
	)
	return res, nil
}
