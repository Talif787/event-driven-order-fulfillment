package query

import (
	"context"
	"errors"

	"go.opentelemetry.io/otel/trace"

	"github.com/orderfulfillment/order/internal/app"
	"github.com/orderfulfillment/order/internal/domain/order"
)

// OrderView is the read model returned to callers.
type OrderView struct {
	OrderID    string
	CustomerID string
	Status     string
	TotalMinor int64
	Currency   string
	Version    int64
}

// GetOrderHandler serves single-order reads. It reads the denormalized
// projection first (the CQRS read path) and falls back to folding the event
// store when the projection has not yet caught up. The fallback preserves
// read-your-writes immediately after placement while keeping the projection as
// the primary, cheap read path.
type GetOrderHandler struct {
	projections app.ProjectionReader
	repo        app.Repository
	tracer      trace.Tracer
}

func NewGetOrderHandler(projections app.ProjectionReader, repo app.Repository, tracer trace.Tracer) *GetOrderHandler {
	return &GetOrderHandler{projections: projections, repo: repo, tracer: tracer}
}

func (h *GetOrderHandler) Handle(ctx context.Context, id string) (OrderView, error) {
	ctx, span := h.tracer.Start(ctx, "GetOrderHandler.Handle")
	defer span.End()

	oid, err := order.ParseOrderID(id)
	if err != nil {
		return OrderView{}, err
	}

	view, err := h.projections.GetOrder(ctx, oid.String())
	if err == nil {
		return OrderView{
			OrderID:    view.OrderID,
			CustomerID: view.CustomerID,
			Status:     view.Status,
			TotalMinor: view.TotalMinor,
			Currency:   view.Currency,
			Version:    view.Version,
		}, nil
	}
	if !errors.Is(err, order.ErrNotFound) {
		return OrderView{}, err
	}

	agg, err := h.repo.Load(ctx, oid)
	if err != nil {
		return OrderView{}, err
	}
	return OrderView{
		OrderID:    agg.ID().String(),
		CustomerID: agg.CustomerID().String(),
		Status:     string(agg.Status()),
		TotalMinor: agg.Total().MinorUnits,
		Currency:   agg.Total().Currency,
		Version:    agg.Version(),
	}, nil
}
