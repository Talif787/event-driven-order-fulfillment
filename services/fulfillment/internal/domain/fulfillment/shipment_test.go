package fulfillment

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func mustCreate(t *testing.T) *Shipment {
	t.Helper()
	s, err := Create(uuid.New(), time.Unix(0, 0).UTC())
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	return s
}

func TestShipmentHappyPath(t *testing.T) {
	s := mustCreate(t)
	if s.Status() != StatusCreated {
		t.Fatalf("want CREATED, got %s", s.Status())
	}
	changed, err := s.Dispatch("UPS", "1Z999", time.Unix(1, 0).UTC())
	if err != nil || !changed {
		t.Fatalf("dispatch: changed=%v err=%v", changed, err)
	}
	if s.Status() != StatusDispatched || s.Carrier() != "UPS" {
		t.Fatalf("unexpected after dispatch: %s %s", s.Status(), s.Carrier())
	}
	changed, err = s.Deliver(time.Unix(2, 0).UTC())
	if err != nil || !changed {
		t.Fatalf("deliver: changed=%v err=%v", changed, err)
	}
	if s.Status() != StatusDelivered {
		t.Fatalf("want DELIVERED, got %s", s.Status())
	}
}

func TestDispatchRequiresCarrier(t *testing.T) {
	s := mustCreate(t)
	if _, err := s.Dispatch("", "", time.Now()); err == nil {
		t.Fatal("expected validation error for empty carrier")
	}
}

func TestDeliverBeforeDispatchIsIllegal(t *testing.T) {
	s := mustCreate(t)
	if _, err := s.Deliver(time.Now()); err == nil {
		t.Fatal("expected invalid transition delivering a CREATED shipment")
	}
}

func TestTransitionsAreIdempotent(t *testing.T) {
	s := mustCreate(t)
	if _, err := s.Dispatch("UPS", "1Z999", time.Now()); err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	changed, err := s.Dispatch("UPS", "1Z999", time.Now())
	if err != nil {
		t.Fatalf("re-dispatch errored: %v", err)
	}
	if changed {
		t.Fatal("re-dispatch should be an idempotent no-op")
	}
}
