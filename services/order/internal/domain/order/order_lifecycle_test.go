package order

import (
	"errors"
	"testing"
	"time"
)

func placedOrder(t *testing.T) *Order {
	t.Helper()
	now := time.Date(2026, 8, 2, 12, 0, 0, 0, time.UTC)
	o, err := PlaceOrder(NewOrderID(), CustomerID{}, sampleItems(t), mustAddress(t), now)
	if err != nil {
		t.Fatalf("PlaceOrder: %v", err)
	}
	o.MarkChangesCommitted()
	return o
}

func TestConfirmFromPending(t *testing.T) {
	o := placedOrder(t)
	if err := o.Confirm(time.Now()); err != nil {
		t.Fatalf("Confirm: %v", err)
	}
	if o.Status() != StatusConfirmed {
		t.Fatalf("status = %s, want CONFIRMED", o.Status())
	}
	changes := o.UncommittedChanges()
	if len(changes) != 1 || changes[0].EventType() != OrderConfirmedType {
		t.Fatalf("expected one OrderConfirmed event, got %+v", changes)
	}
}

func TestConfirmIsIdempotent(t *testing.T) {
	o := placedOrder(t)
	if err := o.Confirm(time.Now()); err != nil {
		t.Fatalf("Confirm: %v", err)
	}
	o.MarkChangesCommitted()
	if err := o.Confirm(time.Now()); err != nil {
		t.Fatalf("re-confirm: %v", err)
	}
	if len(o.UncommittedChanges()) != 0 {
		t.Fatal("re-confirming should raise no new event")
	}
}

func TestCancelFromPending(t *testing.T) {
	o := placedOrder(t)
	if err := o.Cancel("payment declined", time.Now()); err != nil {
		t.Fatalf("Cancel: %v", err)
	}
	if o.Status() != StatusCancelled {
		t.Fatalf("status = %s, want CANCELLED", o.Status())
	}
	changes := o.UncommittedChanges()
	if len(changes) != 1 || changes[0].EventType() != OrderCancelledType {
		t.Fatalf("expected one OrderCancelled event, got %+v", changes)
	}
}

func TestCancelIsIdempotent(t *testing.T) {
	o := placedOrder(t)
	if err := o.Cancel("first", time.Now()); err != nil {
		t.Fatalf("Cancel: %v", err)
	}
	o.MarkChangesCommitted()
	if err := o.Cancel("second", time.Now()); err != nil {
		t.Fatalf("re-cancel: %v", err)
	}
	if len(o.UncommittedChanges()) != 0 {
		t.Fatal("re-cancelling should raise no new event")
	}
}

func TestConfirmAfterCancelIsInvalid(t *testing.T) {
	o := placedOrder(t)
	if err := o.Cancel("gone", time.Now()); err != nil {
		t.Fatalf("Cancel: %v", err)
	}
	if err := o.Confirm(time.Now()); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("want ErrInvalidTransition, got %v", err)
	}
}

func TestCancelAfterConfirmIsInvalid(t *testing.T) {
	o := placedOrder(t)
	if err := o.Confirm(time.Now()); err != nil {
		t.Fatalf("Confirm: %v", err)
	}
	if err := o.Cancel("too late", time.Now()); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("want ErrInvalidTransition, got %v", err)
	}
}
