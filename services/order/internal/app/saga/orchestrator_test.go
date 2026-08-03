package saga

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"go.opentelemetry.io/otel/trace/noop"

	"github.com/orderfulfillment/order/internal/contracts"
)

const testOrderID = "11111111-1111-1111-1111-111111111111"

type fakeInventory struct {
	reserveErr                    error
	reserved, released, committed bool
}

func (f *fakeInventory) Reserve(_ context.Context, _ string, _ []ReserveLine) error {
	if f.reserveErr != nil {
		return f.reserveErr
	}
	f.reserved = true
	return nil
}
func (f *fakeInventory) Release(_ context.Context, _ string) error { f.released = true; return nil }
func (f *fakeInventory) Commit(_ context.Context, _ string) error  { f.committed = true; return nil }

type fakePayments struct {
	result PaymentResult
	err    error
}

func (f *fakePayments) Authorize(_ context.Context, _ PaymentRequest) (PaymentResult, error) {
	return f.result, f.err
}

type fakeOrders struct {
	confirmed, cancelled bool
	reason               string
}

func (f *fakeOrders) Confirm(_ context.Context, _ string) error { f.confirmed = true; return nil }
func (f *fakeOrders) Cancel(_ context.Context, _, reason string) error {
	f.cancelled = true
	f.reason = reason
	return nil
}

type memStore struct{ m map[string]Instance }

func newMemStore() *memStore { return &memStore{m: map[string]Instance{}} }
func (s *memStore) Load(_ context.Context, orderID string) (Instance, bool, error) {
	inst, ok := s.m[orderID]
	return inst, ok, nil
}
func (s *memStore) Save(_ context.Context, inst Instance) error { s.m[inst.OrderID] = inst; return nil }

func newOrch(inv InventoryClient, pay PaymentGateway, orders OrderController, store Store) *Orchestrator {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	return NewOrchestrator(inv, pay, orders, store, logger, noop.NewTracerProvider().Tracer("test"))
}

func placedEvent() contracts.OrderPlacedV1 {
	return contracts.OrderPlacedV1{
		OrderID:    testOrderID,
		CustomerID: "22222222-2222-2222-2222-222222222222",
		Items:      []contracts.LineItem{{SKU: "SKU-1", Quantity: 2, UnitPriceMinor: 1500}},
		TotalMinor: 3000,
		Currency:   "USD",
		PlacedAt:   contracts.FormatTime(time.Now()),
	}
}

func TestSaga_HappyPath(t *testing.T) {
	inv := &fakeInventory{}
	orders := &fakeOrders{}
	store := newMemStore()
	orch := newOrch(inv, &fakePayments{result: PaymentResult{Approved: true, Reference: "ref-1"}}, orders, store)

	if err := orch.Handle(context.Background(), placedEvent()); err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if !inv.reserved || !inv.committed || inv.released {
		t.Fatalf("inventory calls wrong: %+v", inv)
	}
	if !orders.confirmed || orders.cancelled {
		t.Fatalf("order calls wrong: %+v", orders)
	}
	if got := store.m[testOrderID].State; got != StateCompleted {
		t.Fatalf("state = %s, want COMPLETED", got)
	}
}

func TestSaga_ReservationRejectedCancels(t *testing.T) {
	inv := &fakeInventory{reserveErr: ErrReservationRejected}
	orders := &fakeOrders{}
	store := newMemStore()
	orch := newOrch(inv, &fakePayments{}, orders, store)

	if err := orch.Handle(context.Background(), placedEvent()); err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if inv.released {
		t.Fatal("nothing was reserved, release must not be called")
	}
	if !orders.cancelled {
		t.Fatal("order must be cancelled on reservation reject")
	}
	if got := store.m[testOrderID].State; got != StateCancelled {
		t.Fatalf("state = %s, want CANCELLED", got)
	}
}

func TestSaga_PaymentDeclinedCompensates(t *testing.T) {
	inv := &fakeInventory{}
	orders := &fakeOrders{}
	store := newMemStore()
	orch := newOrch(inv, &fakePayments{result: PaymentResult{Approved: false, Decline: "declined"}}, orders, store)

	if err := orch.Handle(context.Background(), placedEvent()); err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if !inv.reserved || !inv.released {
		t.Fatalf("should reserve then release: %+v", inv)
	}
	if inv.committed {
		t.Fatal("must not commit on decline")
	}
	if !orders.cancelled || orders.confirmed {
		t.Fatalf("should cancel not confirm: %+v", orders)
	}
	if got := store.m[testOrderID].State; got != StateCancelled {
		t.Fatalf("state = %s, want CANCELLED", got)
	}
}

func TestSaga_TransientReserveErrorRetries(t *testing.T) {
	inv := &fakeInventory{reserveErr: errors.New("connection refused")}
	orders := &fakeOrders{}
	store := newMemStore()
	orch := newOrch(inv, &fakePayments{}, orders, store)

	err := orch.Handle(context.Background(), placedEvent())
	if err == nil {
		t.Fatal("expected transient error to propagate for redelivery")
	}
	if orders.cancelled {
		t.Fatal("transient error must not cancel the order")
	}
	if got := store.m[testOrderID].State; got != StateStarted {
		t.Fatalf("state = %s, want STARTED", got)
	}
}

func TestSaga_IdempotentOnCompleted(t *testing.T) {
	store := newMemStore()
	store.m[testOrderID] = Instance{OrderID: testOrderID, State: StateCompleted}
	inv := &fakeInventory{}
	orch := newOrch(inv, &fakePayments{result: PaymentResult{Approved: true}}, &fakeOrders{}, store)

	if err := orch.Handle(context.Background(), placedEvent()); err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if inv.reserved || inv.committed {
		t.Fatal("a terminal saga must do no work")
	}
}

func TestSaga_ResumesFromReserved(t *testing.T) {
	store := newMemStore()
	store.m[testOrderID] = Instance{OrderID: testOrderID, State: StateReserved, ReservationHeld: true}
	inv := &fakeInventory{}
	orders := &fakeOrders{}
	orch := newOrch(inv, &fakePayments{result: PaymentResult{Approved: true, Reference: "ref"}}, orders, store)

	if err := orch.Handle(context.Background(), placedEvent()); err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if inv.reserved {
		t.Fatal("resuming from RESERVED must skip the reserve step")
	}
	if !inv.committed || !orders.confirmed {
		t.Fatalf("should pay, commit, confirm: inv=%+v orders=%+v", inv, orders)
	}
	if got := store.m[testOrderID].State; got != StateCompleted {
		t.Fatalf("state = %s, want COMPLETED", got)
	}
}
