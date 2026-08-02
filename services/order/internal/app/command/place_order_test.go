package command

import (
	"context"
	"log/slog"
	"io"
	"testing"
	"time"

	"go.opentelemetry.io/otel/trace/noop"

	"github.com/orderfulfillment/order/internal/app"
	"github.com/orderfulfillment/order/internal/domain/order"
)

type fakeRepo struct {
	saved     map[string]*order.Order
	saveCalls int
}

func newFakeRepo() *fakeRepo { return &fakeRepo{saved: map[string]*order.Order{}} }

func (r *fakeRepo) Load(_ context.Context, id order.OrderID) (*order.Order, error) {
	o, ok := r.saved[id.String()]
	if !ok {
		return nil, order.ErrNotFound
	}
	return o, nil
}

func (r *fakeRepo) Save(_ context.Context, agg *order.Order, _ int64, _ []app.OutboxMessage) error {
	r.saveCalls++
	r.saved[agg.ID().String()] = agg
	return nil
}

type fakeIdem struct{ keys map[string]string }

func newFakeIdem() *fakeIdem { return &fakeIdem{keys: map[string]string{}} }

func (s *fakeIdem) Reserve(_ context.Context, key, orderID string) (string, bool, error) {
	if existing, ok := s.keys[key]; ok {
		return existing, true, nil
	}
	s.keys[key] = orderID
	return "", false, nil
}

type fixedIDs struct{ id order.OrderID }

func (g fixedIDs) NewOrderID() order.OrderID { return g.id }

type fixedClock struct{ t time.Time }

func (c fixedClock) Now() time.Time { return c.t }

func newHandler(repo app.Repository, idem app.IdempotencyStore, ids app.IDGenerator) *PlaceOrderHandler {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	return NewPlaceOrderHandler(repo, idem, ids, fixedClock{t: time.Now().UTC()}, logger, noop.NewTracerProvider().Tracer("test"))
}

func validCommand(key string) PlaceOrderCommand {
	return PlaceOrderCommand{
		CustomerID:     "11111111-1111-1111-1111-111111111111",
		IdempotencyKey: key,
		Lines:          []PlaceOrderLine{{SKU: "SKU-1", Quantity: 2, UnitPriceMinor: 1500, Currency: "USD"}},
		Ship:           ShipTo{Line1: "1 Main St", City: "Boston", Region: "MA", PostalCode: "02118", Country: "US"},
	}
}

func TestPlaceOrder_Success(t *testing.T) {
	repo := newFakeRepo()
	h := newHandler(repo, newFakeIdem(), fixedIDs{id: order.NewOrderID()})

	res, err := h.Handle(context.Background(), validCommand("idem-key-0001"))
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if res.Idempotent {
		t.Fatalf("first call should not be idempotent replay")
	}
	if repo.saveCalls != 1 {
		t.Fatalf("saveCalls = %d, want 1", repo.saveCalls)
	}
}

func TestPlaceOrder_IdempotentReplayCreatesNoDuplicate(t *testing.T) {
	repo := newFakeRepo()
	idem := newFakeIdem()
	h := newHandler(repo, idem, fixedIDs{id: order.NewOrderID()})

	first, err := h.Handle(context.Background(), validCommand("idem-key-0001"))
	if err != nil {
		t.Fatalf("first Handle: %v", err)
	}
	second, err := h.Handle(context.Background(), validCommand("idem-key-0001"))
	if err != nil {
		t.Fatalf("second Handle: %v", err)
	}
	if !second.Idempotent {
		t.Fatalf("second call should be an idempotent replay")
	}
	if first.OrderID != second.OrderID {
		t.Fatalf("replay returned different order id: %s vs %s", first.OrderID, second.OrderID)
	}
	if repo.saveCalls != 1 {
		t.Fatalf("saveCalls = %d, want 1 (no duplicate persist)", repo.saveCalls)
	}
}

func TestPlaceOrder_InvalidInputRejected(t *testing.T) {
	h := newHandler(newFakeRepo(), newFakeIdem(), fixedIDs{id: order.NewOrderID()})
	cmd := validCommand("idem-key-0001")
	cmd.Lines = nil
	if _, err := h.Handle(context.Background(), cmd); err == nil {
		t.Fatal("expected validation error for empty lines")
	}
}
