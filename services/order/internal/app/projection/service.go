package projection

import (
	"context"
	"fmt"
	"log/slog"

	"go.opentelemetry.io/otel/trace"

	"github.com/orderfulfillment/order/internal/app"
	"github.com/orderfulfillment/order/internal/contracts"
	"github.com/orderfulfillment/order/internal/domain/order"
)

// Service applies integration events to the order read model. It is the first
// consumer of the order event stream and the template for future projections.
type Service struct {
	store  app.ProjectionStore
	logger *slog.Logger
	tracer trace.Tracer
}

func NewService(store app.ProjectionStore, logger *slog.Logger, tracer trace.Tracer) *Service {
	return &Service{store: store, logger: logger, tracer: tracer}
}

// Apply routes an event by type. Unmodeled types are ignored so the projector
// tolerates events it does not yet handle.
func (s *Service) Apply(ctx context.Context, eventType string, payload []byte) error {
	ctx, span := s.tracer.Start(ctx, "Projection.Apply")
	defer span.End()

	switch eventType {
	case contracts.TypeOrderPlaced:
		return s.applyOrderPlaced(ctx, payload)
	case contracts.TypeOrderConfirmed:
		return s.applyOrderConfirmed(ctx, payload)
	case contracts.TypeOrderCancelled:
		return s.applyOrderCancelled(ctx, payload)
	default:
		s.logger.DebugContext(ctx, "ignoring unmodeled event", slog.String("event_type", eventType))
		return nil
	}
}

func (s *Service) applyOrderPlaced(ctx context.Context, payload []byte) error {
	event, err := contracts.UnmarshalOrderPlaced(payload)
	if err != nil {
		return err
	}
	placedAt, err := event.ParsePlacedAt()
	if err != nil {
		return err
	}
	if err := s.store.UpsertOrderPlaced(ctx, app.OrderProjection{
		OrderID:    event.OrderID,
		CustomerID: event.CustomerID,
		Status:     string(order.StatusPending),
		TotalMinor: event.TotalMinor,
		Currency:   event.Currency,
		Version:    1,
		PlacedAt:   placedAt,
	}); err != nil {
		return fmt.Errorf("apply order.placed: %w", err)
	}
	s.logger.InfoContext(ctx, "projection updated", slog.String("order_id", event.OrderID))
	return nil
}

func (s *Service) applyOrderConfirmed(ctx context.Context, payload []byte) error {
	event, err := contracts.UnmarshalOrderConfirmed(payload)
	if err != nil {
		return err
	}
	if err := s.store.UpdateStatus(ctx, event.OrderID, string(order.StatusConfirmed)); err != nil {
		return fmt.Errorf("apply order.confirmed: %w", err)
	}
	s.logger.InfoContext(ctx, "projection status updated",
		slog.String("order_id", event.OrderID), slog.String("status", string(order.StatusConfirmed)))
	return nil
}

func (s *Service) applyOrderCancelled(ctx context.Context, payload []byte) error {
	event, err := contracts.UnmarshalOrderCancelled(payload)
	if err != nil {
		return err
	}
	if err := s.store.UpdateStatus(ctx, event.OrderID, string(order.StatusCancelled)); err != nil {
		return fmt.Errorf("apply order.cancelled: %w", err)
	}
	s.logger.InfoContext(ctx, "projection status updated",
		slog.String("order_id", event.OrderID), slog.String("status", string(order.StatusCancelled)))
	return nil
}
