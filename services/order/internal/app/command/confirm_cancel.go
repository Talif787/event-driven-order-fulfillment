package command

import (
	"context"
	"fmt"
	"log/slog"

	"go.opentelemetry.io/otel/trace"

	"github.com/orderfulfillment/order/internal/app"
	"github.com/orderfulfillment/order/internal/contracts"
	"github.com/orderfulfillment/order/internal/domain/order"
	"github.com/orderfulfillment/order/internal/infra/tracing"
)

// ConfirmOrderHandler confirms a pending order (the saga success terminal). It
// appends OrderConfirmed and publishes order.confirmed.v1 through the outbox, in
// one transaction. It is idempotent: confirming an already-confirmed order is a
// no-op.
type ConfirmOrderHandler struct {
	repo   app.Repository
	clock  app.Clock
	logger *slog.Logger
	tracer trace.Tracer
}

func NewConfirmOrderHandler(repo app.Repository, clock app.Clock, logger *slog.Logger, tracer trace.Tracer) *ConfirmOrderHandler {
	return &ConfirmOrderHandler{repo: repo, clock: clock, logger: logger, tracer: tracer}
}

func (h *ConfirmOrderHandler) Handle(ctx context.Context, orderID string) error {
	ctx, span := h.tracer.Start(ctx, "ConfirmOrderHandler.Handle")
	defer span.End()

	oid, err := order.ParseOrderID(orderID)
	if err != nil {
		return err
	}
	agg, err := h.repo.Load(ctx, oid)
	if err != nil {
		return err
	}
	if err := agg.Confirm(h.clock.Now()); err != nil {
		return err
	}
	changes := agg.UncommittedChanges()
	if len(changes) == 0 {
		return nil // already confirmed
	}
	outbox, err := lifecycleOutbox(changes)
	if err != nil {
		return err
	}
	for i := range outbox {
		tracing.InjectToMap(ctx, outbox[i].Headers)
	}
	expected := agg.Version() - int64(len(changes))
	if err := h.repo.Save(ctx, agg, expected, outbox); err != nil {
		return fmt.Errorf("persist confirm: %w", err)
	}
	agg.MarkChangesCommitted()
	h.logger.InfoContext(ctx, "order confirmed", slog.String("order_id", oid.String()))
	return nil
}

// CancelOrderHandler cancels a pending order (the saga compensation terminal).
// It appends OrderCancelled and publishes order.cancelled.v1 through the outbox.
// It is idempotent: cancelling an already-cancelled order is a no-op.
type CancelOrderHandler struct {
	repo   app.Repository
	clock  app.Clock
	logger *slog.Logger
	tracer trace.Tracer
}

func NewCancelOrderHandler(repo app.Repository, clock app.Clock, logger *slog.Logger, tracer trace.Tracer) *CancelOrderHandler {
	return &CancelOrderHandler{repo: repo, clock: clock, logger: logger, tracer: tracer}
}

func (h *CancelOrderHandler) Handle(ctx context.Context, orderID, reason string) error {
	ctx, span := h.tracer.Start(ctx, "CancelOrderHandler.Handle")
	defer span.End()

	oid, err := order.ParseOrderID(orderID)
	if err != nil {
		return err
	}
	agg, err := h.repo.Load(ctx, oid)
	if err != nil {
		return err
	}
	if err := agg.Cancel(reason, h.clock.Now()); err != nil {
		return err
	}
	changes := agg.UncommittedChanges()
	if len(changes) == 0 {
		return nil // already cancelled
	}
	outbox, err := lifecycleOutbox(changes)
	if err != nil {
		return err
	}
	for i := range outbox {
		tracing.InjectToMap(ctx, outbox[i].Headers)
	}
	expected := agg.Version() - int64(len(changes))
	if err := h.repo.Save(ctx, agg, expected, outbox); err != nil {
		return fmt.Errorf("persist cancel: %w", err)
	}
	agg.MarkChangesCommitted()
	h.logger.InfoContext(ctx, "order cancelled", slog.String("order_id", oid.String()), slog.String("reason", reason))
	return nil
}

// lifecycleOutbox maps confirmed and cancelled domain events to their
// integration-event outbox messages.
func lifecycleOutbox(changes []order.DomainEvent) ([]app.OutboxMessage, error) {
	messages := make([]app.OutboxMessage, 0, len(changes))
	for _, ev := range changes {
		switch e := ev.(type) {
		case order.OrderConfirmed:
			raw, err := contracts.OrderConfirmedV1{
				OrderID:     e.OrderID.String(),
				ConfirmedAt: contracts.FormatTime(e.ConfirmedAt),
			}.Marshal()
			if err != nil {
				return nil, err
			}
			messages = append(messages, app.OutboxMessage{
				AggregateID: e.OrderID.String(),
				Topic:       contracts.TopicOrderEvents,
				EventType:   string(order.OrderConfirmedType),
				Payload:     raw,
				Headers:     map[string]string{contracts.HeaderContentType: "application/json"},
			})
		case order.OrderCancelled:
			raw, err := contracts.OrderCancelledV1{
				OrderID:     e.OrderID.String(),
				Reason:      e.Reason,
				CancelledAt: contracts.FormatTime(e.CancelledAt),
			}.Marshal()
			if err != nil {
				return nil, err
			}
			messages = append(messages, app.OutboxMessage{
				AggregateID: e.OrderID.String(),
				Topic:       contracts.TopicOrderEvents,
				EventType:   string(order.OrderCancelledType),
				Payload:     raw,
				Headers:     map[string]string{contracts.HeaderContentType: "application/json"},
			})
		}
	}
	return messages, nil
}
