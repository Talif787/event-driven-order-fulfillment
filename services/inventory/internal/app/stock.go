package app

import (
	"context"
	"fmt"
	"log/slog"

	"go.opentelemetry.io/otel/trace"

	"github.com/orderfulfillment/inventory/internal/domain/inventory"
)

// GetStockHandler reads the ledger for one SKU.
type GetStockHandler struct {
	store  InventoryStore
	tracer trace.Tracer
}

func NewGetStockHandler(store InventoryStore, tracer trace.Tracer) *GetStockHandler {
	return &GetStockHandler{store: store, tracer: tracer}
}

func (h *GetStockHandler) Handle(ctx context.Context, sku string) (StockView, error) {
	ctx, span := h.tracer.Start(ctx, "GetStock.Handle")
	defer span.End()

	s, err := inventory.NewSKU(sku)
	if err != nil {
		return StockView{}, err
	}
	return h.store.GetStock(ctx, s.String())
}

// SetStockCommand seeds or adjusts available stock for a SKU (admin operation).
type SetStockCommand struct {
	SKU       string
	Available int64
}

// SetStockHandler upserts the available quantity for a SKU.
type SetStockHandler struct {
	store  InventoryStore
	logger *slog.Logger
	tracer trace.Tracer
}

func NewSetStockHandler(store InventoryStore, logger *slog.Logger, tracer trace.Tracer) *SetStockHandler {
	return &SetStockHandler{store: store, logger: logger, tracer: tracer}
}

func (h *SetStockHandler) Handle(ctx context.Context, cmd SetStockCommand) error {
	ctx, span := h.tracer.Start(ctx, "SetStock.Handle")
	defer span.End()

	s, err := inventory.NewSKU(cmd.SKU)
	if err != nil {
		return err
	}
	if cmd.Available < 0 {
		return fmt.Errorf("%w: available must be non-negative", inventory.ErrValidation)
	}
	if err := h.store.UpsertStock(ctx, s.String(), cmd.Available); err != nil {
		return err
	}
	h.logger.InfoContext(ctx, "stock set", slog.String("sku", s.String()), slog.Int64("available", cmd.Available))
	return nil
}
