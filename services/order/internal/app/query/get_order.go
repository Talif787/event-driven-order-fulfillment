package query

import (
	"context"

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

// GetOrderHandler serves order reads by folding the event stream.
type GetOrderHandler struct {
	repo   app.Repository
	tracer trace.Tracer
}

func NewGetOrderHandler(repo app.Repository, tracer trace.Tracer) *GetOrderHandler {
	return &GetOrderHandler{repo: repo, tracer: tracer}
}

func (h *GetOrderHandler) Handle(ctx context.Context, id string) (OrderView, error) {
	ctx, span := h.tracer.Start(ctx, "GetOrderHandler.Handle")
	defer span.End()

	oid, err := order.ParseOrderID(id)
	if err != nil {
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
