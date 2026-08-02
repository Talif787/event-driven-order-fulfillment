package order

import (
	"errors"
	"testing"
	"time"
)

func mustSKU(t *testing.T, s string) SKU {
	t.Helper()
	v, err := NewSKU(s)
	if err != nil {
		t.Fatalf("NewSKU(%q): %v", s, err)
	}
	return v
}

func mustQty(t *testing.T, n int32) Quantity {
	t.Helper()
	v, err := NewQuantity(n)
	if err != nil {
		t.Fatalf("NewQuantity(%d): %v", n, err)
	}
	return v
}

func mustMoney(t *testing.T, cur string, minor int64) Money {
	t.Helper()
	v, err := NewMoney(cur, minor)
	if err != nil {
		t.Fatalf("NewMoney(%s,%d): %v", cur, minor, err)
	}
	return v
}

func mustAddress(t *testing.T) Address {
	t.Helper()
	a, err := NewAddress("1 Main St", "", "Boston", "MA", "02118", "US")
	if err != nil {
		t.Fatalf("NewAddress: %v", err)
	}
	return a
}

func sampleItems(t *testing.T) []LineItem {
	t.Helper()
	return []LineItem{
		{SKU: mustSKU(t, "SKU-1"), Quantity: mustQty(t, 2), UnitPrice: mustMoney(t, "USD", 1500)},
		{SKU: mustSKU(t, "SKU-2"), Quantity: mustQty(t, 1), UnitPrice: mustMoney(t, "USD", 500)},
	}
}

func TestPlaceOrder_ComputesTotalAndEmitsEvent(t *testing.T) {
	now := time.Date(2026, 8, 2, 12, 0, 0, 0, time.UTC)
	o, err := PlaceOrder(NewOrderID(), CustomerID{}, sampleItems(t), mustAddress(t), now)
	if err != nil {
		t.Fatalf("PlaceOrder: %v", err)
	}
	if o.Status() != StatusPending {
		t.Fatalf("status = %s, want PENDING", o.Status())
	}
	if o.Total().MinorUnits != 3500 {
		t.Fatalf("total = %d, want 3500", o.Total().MinorUnits)
	}
	if o.Version() != 1 {
		t.Fatalf("version = %d, want 1", o.Version())
	}
	changes := o.UncommittedChanges()
	if len(changes) != 1 || changes[0].EventType() != OrderPlacedType {
		t.Fatalf("expected one OrderPlaced event, got %+v", changes)
	}
}

func TestPlaceOrder_RejectsEmptyItems(t *testing.T) {
	_, err := PlaceOrder(NewOrderID(), CustomerID{}, nil, mustAddress(t), time.Now())
	if !errors.Is(err, ErrValidation) {
		t.Fatalf("err = %v, want ErrValidation", err)
	}
}

func TestPlaceOrder_RejectsDuplicateSKU(t *testing.T) {
	items := []LineItem{
		{SKU: mustSKU(t, "SKU-1"), Quantity: mustQty(t, 1), UnitPrice: mustMoney(t, "USD", 100)},
		{SKU: mustSKU(t, "SKU-1"), Quantity: mustQty(t, 1), UnitPrice: mustMoney(t, "USD", 100)},
	}
	_, err := PlaceOrder(NewOrderID(), CustomerID{}, items, mustAddress(t), time.Now())
	if !errors.Is(err, ErrValidation) {
		t.Fatalf("err = %v, want ErrValidation", err)
	}
}

func TestRehydrate_RebuildsState(t *testing.T) {
	id := NewOrderID()
	now := time.Now().UTC()
	history := []DomainEvent{OrderPlaced{
		OrderID: id, Items: sampleItems(t), ShipTo: mustAddress(t),
		Total: mustMoney(t, "USD", 3500), PlacedAt: now,
	}}
	o, err := Rehydrate(history)
	if err != nil {
		t.Fatalf("Rehydrate: %v", err)
	}
	if o.ID().String() != id.String() || o.Status() != StatusPending || o.Version() != 1 {
		t.Fatalf("unexpected rehydrated state: id=%s status=%s version=%d", o.ID(), o.Status(), o.Version())
	}
	if len(o.UncommittedChanges()) != 0 {
		t.Fatalf("rehydrated aggregate must have no uncommitted changes")
	}
}

func TestRehydrate_EmptyHistoryIsNotFound(t *testing.T) {
	_, err := Rehydrate(nil)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}
