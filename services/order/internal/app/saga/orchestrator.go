package saga

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"go.opentelemetry.io/otel/trace"

	"github.com/orderfulfillment/order/internal/contracts"
)

// Orchestrator runs the order fulfillment saga. It consumes order.placed events
// and coordinates the Inventory and Payment steps, confirming the order on
// success and compensating (release stock, cancel order) on failure.
type Orchestrator struct {
	inventory InventoryClient
	payments  PaymentGateway
	orders    OrderController
	store     Store
	logger    *slog.Logger
	tracer    trace.Tracer
}

func NewOrchestrator(inventory InventoryClient, payments PaymentGateway, orders OrderController, store Store, logger *slog.Logger, tracer trace.Tracer) *Orchestrator {
	return &Orchestrator{inventory: inventory, payments: payments, orders: orders, store: store, logger: logger, tracer: tracer}
}

// Handle runs or resumes the saga for a placed order. It is idempotent and
// crash-safe: state is persisted after each step, every downstream operation is
// idempotent, and a returned error leaves the Kafka offset uncommitted so the
// event is redelivered and the saga resumes from where it stopped. A nil return
// means the saga reached a terminal state (completed or cancelled) or made
// progress that is safe to commit.
func (o *Orchestrator) Handle(ctx context.Context, event contracts.OrderPlacedV1) error {
	ctx, span := o.tracer.Start(ctx, "Saga.Handle")
	defer span.End()

	inst, found, err := o.store.Load(ctx, event.OrderID)
	if err != nil {
		return fmt.Errorf("load saga: %w", err)
	}
	if !found {
		inst = NewInstance(event.OrderID)
		if err := o.store.Save(ctx, inst); err != nil {
			return fmt.Errorf("save saga: %w", err)
		}
	}
	if inst.Terminal() {
		return nil
	}

	lines := make([]ReserveLine, 0, len(event.Items))
	for _, it := range event.Items {
		lines = append(lines, ReserveLine{SKU: it.SKU, Quantity: it.Quantity})
	}

	// Step 1: reserve stock.
	if inst.State == StateStarted {
		err := o.inventory.Reserve(ctx, event.OrderID, lines)
		if errors.Is(err, ErrReservationRejected) {
			return o.cancel(ctx, &inst, "inventory reservation rejected")
		}
		if err != nil {
			return fmt.Errorf("reserve stock: %w", err)
		}
		inst.State = StateReserved
		inst.ReservationHeld = true
		if err := o.store.Save(ctx, inst); err != nil {
			return fmt.Errorf("save saga: %w", err)
		}
		o.logger.InfoContext(ctx, "saga reserved", slog.String("order_id", event.OrderID))
	}

	// Step 2: authorize payment. A clean decline compensates; a transport error
	// is retried.
	if inst.State == StateReserved {
		res, err := o.payments.Authorize(ctx, PaymentRequest{
			OrderID: event.OrderID, AmountMinor: event.TotalMinor, Currency: event.Currency,
		})
		if err != nil {
			return fmt.Errorf("authorize payment: %w", err)
		}
		if !res.Approved {
			if err := o.inventory.Release(ctx, event.OrderID); err != nil {
				return fmt.Errorf("release after decline: %w", err)
			}
			inst.ReservationHeld = false
			reason := "payment declined"
			if res.Decline != "" {
				reason = "payment declined: " + res.Decline
			}
			return o.cancel(ctx, &inst, reason)
		}
		inst.State = StatePaid
		inst.PaymentRef = res.Reference
		if err := o.store.Save(ctx, inst); err != nil {
			return fmt.Errorf("save saga: %w", err)
		}
		o.logger.InfoContext(ctx, "saga paid", slog.String("order_id", event.OrderID), slog.String("payment_ref", res.Reference))
	}

	// Step 3: commit the held stock and confirm the order.
	if inst.State == StatePaid {
		if err := o.inventory.Commit(ctx, event.OrderID); err != nil {
			return fmt.Errorf("commit stock: %w", err)
		}
		if err := o.orders.Confirm(ctx, event.OrderID); err != nil {
			return fmt.Errorf("confirm order: %w", err)
		}
		inst.State = StateCompleted
		inst.ReservationHeld = false
		if err := o.store.Save(ctx, inst); err != nil {
			return fmt.Errorf("save saga: %w", err)
		}
		o.logger.InfoContext(ctx, "saga completed", slog.String("order_id", event.OrderID))
	}

	return nil
}

func (o *Orchestrator) cancel(ctx context.Context, inst *Instance, reason string) error {
	if err := o.orders.Cancel(ctx, inst.OrderID, reason); err != nil {
		return fmt.Errorf("cancel order: %w", err)
	}
	inst.State = StateCancelled
	inst.Reason = reason
	if err := o.store.Save(ctx, *inst); err != nil {
		return fmt.Errorf("save saga: %w", err)
	}
	o.logger.InfoContext(ctx, "saga cancelled", slog.String("order_id", inst.OrderID), slog.String("reason", reason))
	return nil
}
