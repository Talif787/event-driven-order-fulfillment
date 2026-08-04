package app

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/orderfulfillment/fulfillment/internal/contracts"
	"github.com/orderfulfillment/fulfillment/internal/domain/fulfillment"
)

type fakeRepo struct {
	byOrder map[uuid.UUID]*fulfillment.Shipment
}

func newFakeRepo() *fakeRepo { return &fakeRepo{byOrder: map[uuid.UUID]*fulfillment.Shipment{}} }

func (f *fakeRepo) GetByOrderID(_ context.Context, orderID uuid.UUID) (*fulfillment.Shipment, error) {
	s, ok := f.byOrder[orderID]
	if !ok {
		return nil, fulfillment.ErrShipmentNotFound
	}
	return s, nil
}

func (f *fakeRepo) Insert(_ context.Context, s *fulfillment.Shipment) error {
	if _, ok := f.byOrder[s.OrderID()]; ok {
		return fulfillment.ErrShipmentExists
	}
	f.byOrder[s.OrderID()] = s
	return nil
}

func (f *fakeRepo) Update(_ context.Context, s *fulfillment.Shipment) error {
	f.byOrder[s.OrderID()] = s
	return nil
}

type fakePublisher struct{ events []Event }

func (p *fakePublisher) Publish(_ context.Context, events ...Event) error {
	p.events = append(p.events, events...)
	return nil
}

type fixedClock struct{ t time.Time }

func (c fixedClock) Now() time.Time { return c.t }

func newService() (*Service, *fakeRepo, *fakePublisher) {
	repo := newFakeRepo()
	pub := &fakePublisher{}
	svc := NewService(repo, pub, fixedClock{t: time.Unix(0, 0).UTC()}, slogDiscard())
	return svc, repo, pub
}

func TestCreateForOrderIsIdempotent(t *testing.T) {
	svc, repo, pub := newService()
	orderID := uuid.New()

	first, err := svc.CreateForOrder(context.Background(), orderID)
	if err != nil {
		t.Fatalf("first create: %v", err)
	}
	second, err := svc.CreateForOrder(context.Background(), orderID)
	if err != nil {
		t.Fatalf("second create: %v", err)
	}
	if first.ID() != second.ID() {
		t.Fatal("idempotent create should return the same shipment")
	}
	if len(repo.byOrder) != 1 {
		t.Fatalf("expected exactly one shipment, got %d", len(repo.byOrder))
	}
	if !hasEventType(pub.events, contracts.TypeShipmentCreated) {
		t.Fatal("expected a shipment.created event")
	}
}

func TestDispatchThenDeliverPublishes(t *testing.T) {
	svc, _, pub := newService()
	orderID := uuid.New()
	if _, err := svc.CreateForOrder(context.Background(), orderID); err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := svc.Dispatch(context.Background(), orderID, "UPS", "1Z999"); err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	if _, err := svc.Deliver(context.Background(), orderID); err != nil {
		t.Fatalf("deliver: %v", err)
	}
	if !hasEventType(pub.events, contracts.TypeShipmentDispatched) {
		t.Fatal("expected a shipment.dispatched event")
	}
	if !hasEventType(pub.events, contracts.TypeShipmentDelivered) {
		t.Fatal("expected a shipment.delivered event")
	}
}

func TestDispatchUnknownOrderIsNotFound(t *testing.T) {
	svc, _, _ := newService()
	if _, err := svc.Dispatch(context.Background(), uuid.New(), "UPS", "1Z999"); err == nil {
		t.Fatal("expected not found dispatching an unknown order")
	}
}

func hasEventType(events []Event, eventType string) bool {
	for _, e := range events {
		if e.Type == eventType {
			return true
		}
	}
	return false
}
