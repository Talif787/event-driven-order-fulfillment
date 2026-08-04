package app

import (
	"context"
	"log/slog"

	"go.opentelemetry.io/otel/trace"

	"github.com/orderfulfillment/inventory/internal/domain/inventory"
	"github.com/orderfulfillment/inventory/internal/infra/metrics"
)

// ReleaseReservationHandler releases a held reservation (saga compensation).
type ReleaseReservationHandler struct {
	store  InventoryStore
	logger *slog.Logger
	tracer trace.Tracer
}

func NewReleaseReservationHandler(store InventoryStore, logger *slog.Logger, tracer trace.Tracer) *ReleaseReservationHandler {
	return &ReleaseReservationHandler{store: store, logger: logger, tracer: tracer}
}

func (h *ReleaseReservationHandler) Handle(ctx context.Context, orderID string) (ReservationResult, error) {
	ctx, span := h.tracer.Start(ctx, "ReleaseReservation.Handle")
	defer span.End()

	oid, err := inventory.ParseOrderID(orderID)
	if err != nil {
		return ReservationResult{}, err
	}
	res, err := h.store.Release(ctx, oid.String())
	if err != nil {
		return ReservationResult{}, err
	}
	metrics.Reservations.WithLabelValues("released").Inc()
	h.logger.InfoContext(ctx, "reservation released",
		slog.String("order_id", oid.String()),
		slog.Bool("idempotent", res.Idempotent),
	)
	return res, nil
}

// CommitReservationHandler commits a held reservation (order fulfilled).
type CommitReservationHandler struct {
	store  InventoryStore
	logger *slog.Logger
	tracer trace.Tracer
}

func NewCommitReservationHandler(store InventoryStore, logger *slog.Logger, tracer trace.Tracer) *CommitReservationHandler {
	return &CommitReservationHandler{store: store, logger: logger, tracer: tracer}
}

func (h *CommitReservationHandler) Handle(ctx context.Context, orderID string) (ReservationResult, error) {
	ctx, span := h.tracer.Start(ctx, "CommitReservation.Handle")
	defer span.End()

	oid, err := inventory.ParseOrderID(orderID)
	if err != nil {
		return ReservationResult{}, err
	}
	res, err := h.store.Commit(ctx, oid.String())
	if err != nil {
		return ReservationResult{}, err
	}
	metrics.Reservations.WithLabelValues("committed").Inc()
	h.logger.InfoContext(ctx, "reservation committed",
		slog.String("order_id", oid.String()),
		slog.Bool("idempotent", res.Idempotent),
	)
	return res, nil
}

// GetReservationHandler reads a reservation by order id.
type GetReservationHandler struct {
	store  InventoryStore
	tracer trace.Tracer
}

func NewGetReservationHandler(store InventoryStore, tracer trace.Tracer) *GetReservationHandler {
	return &GetReservationHandler{store: store, tracer: tracer}
}

func (h *GetReservationHandler) Handle(ctx context.Context, orderID string) (ReservationResult, error) {
	ctx, span := h.tracer.Start(ctx, "GetReservation.Handle")
	defer span.End()

	oid, err := inventory.ParseOrderID(orderID)
	if err != nil {
		return ReservationResult{}, err
	}
	return h.store.GetReservation(ctx, oid.String())
}
