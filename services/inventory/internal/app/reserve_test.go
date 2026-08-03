package app

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	"go.opentelemetry.io/otel/trace/noop"

	"github.com/orderfulfillment/inventory/internal/domain/inventory"
)

// fakeStore records calls and returns canned results, so the handler's
// validation and delegation can be tested without a database.
type fakeStore struct {
	reserveCalled bool
	gotOrderID    string
	gotLines      []ReserveLine
	result        ReservationResult
	err           error
}

func (f *fakeStore) Reserve(_ context.Context, orderID string, lines []ReserveLine) (ReservationResult, error) {
	f.reserveCalled = true
	f.gotOrderID = orderID
	f.gotLines = lines
	if f.err != nil {
		return ReservationResult{}, f.err
	}
	return f.result, nil
}
func (f *fakeStore) Release(context.Context, string) (ReservationResult, error) {
	return ReservationResult{}, nil
}
func (f *fakeStore) Commit(context.Context, string) (ReservationResult, error) {
	return ReservationResult{}, nil
}
func (f *fakeStore) GetReservation(context.Context, string) (ReservationResult, error) {
	return ReservationResult{}, nil
}
func (f *fakeStore) GetStock(context.Context, string) (StockView, error) { return StockView{}, nil }
func (f *fakeStore) UpsertStock(context.Context, string, int64) error    { return nil }

func newReserveHandler(store InventoryStore) *ReserveStockHandler {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	tracer := noop.NewTracerProvider().Tracer("test")
	return NewReserveStockHandler(store, logger, tracer)
}

const validOrder = "11111111-1111-1111-1111-111111111111"

func TestReserveValidDelegatesToStore(t *testing.T) {
	store := &fakeStore{result: ReservationResult{ReservationID: "r1", OrderID: validOrder, Status: "HELD"}}
	h := newReserveHandler(store)
	_, err := h.Handle(context.Background(), ReserveStockCommand{
		OrderID: validOrder,
		Lines:   []ReserveLineInput{{SKU: "SKU-1", Quantity: 2}},
	})
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if !store.reserveCalled || store.gotOrderID != validOrder || len(store.gotLines) != 1 {
		t.Fatalf("store not called correctly: %+v", store)
	}
}

func TestReserveInvalidOrderIDDoesNotCallStore(t *testing.T) {
	store := &fakeStore{}
	h := newReserveHandler(store)
	_, err := h.Handle(context.Background(), ReserveStockCommand{
		OrderID: "not-a-uuid",
		Lines:   []ReserveLineInput{{SKU: "SKU-1", Quantity: 2}},
	})
	if !errors.Is(err, inventory.ErrInvalidIdentifier) {
		t.Fatalf("want ErrInvalidIdentifier, got %v", err)
	}
	if store.reserveCalled {
		t.Fatal("store should not be called on invalid order id")
	}
}

func TestReserveRejectsBadQuantity(t *testing.T) {
	_, err := newReserveHandler(&fakeStore{}).Handle(context.Background(), ReserveStockCommand{
		OrderID: validOrder,
		Lines:   []ReserveLineInput{{SKU: "SKU-1", Quantity: 0}},
	})
	if !errors.Is(err, inventory.ErrValidation) {
		t.Fatalf("want ErrValidation, got %v", err)
	}
}

func TestReserveRejectsDuplicateSKU(t *testing.T) {
	_, err := newReserveHandler(&fakeStore{}).Handle(context.Background(), ReserveStockCommand{
		OrderID: validOrder,
		Lines:   []ReserveLineInput{{SKU: "SKU-1", Quantity: 1}, {SKU: "SKU-1", Quantity: 1}},
	})
	if !errors.Is(err, inventory.ErrValidation) {
		t.Fatalf("want ErrValidation, got %v", err)
	}
}

func TestReserveRejectsEmptyLines(t *testing.T) {
	_, err := newReserveHandler(&fakeStore{}).Handle(context.Background(), ReserveStockCommand{OrderID: validOrder})
	if !errors.Is(err, inventory.ErrValidation) {
		t.Fatalf("want ErrValidation, got %v", err)
	}
}
