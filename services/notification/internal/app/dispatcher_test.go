package app

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/orderfulfillment/notification/internal/contracts"
	"github.com/orderfulfillment/notification/internal/domain/notification"
)

type fakeStore struct {
	seen     map[string]bool
	recorded []*notification.Notification
}

func newFakeStore() *fakeStore { return &fakeStore{seen: map[string]bool{}} }

func (f *fakeStore) RecordIfNew(_ context.Context, n *notification.Notification) (bool, error) {
	if f.seen[n.DedupeKey()] {
		return false, nil
	}
	f.seen[n.DedupeKey()] = true
	f.recorded = append(f.recorded, n)
	return true, nil
}

type fakeSender struct{ sent []*notification.Notification }

func (f *fakeSender) Send(_ context.Context, n *notification.Notification) error {
	f.sent = append(f.sent, n)
	return nil
}

type fixedClock struct{ t time.Time }

func (c fixedClock) Now() time.Time { return c.t }

func newDispatcher() (*Dispatcher, *fakeStore, *fakeSender) {
	store := newFakeStore()
	sender := &fakeSender{}
	d := NewDispatcher(store, sender, fixedClock{t: time.Unix(0, 0).UTC()},
		slog.New(slog.NewTextHandler(io.Discard, nil)))
	return d, store, sender
}

func TestConfirmedRendersAndSends(t *testing.T) {
	d, store, sender := newDispatcher()
	orderID := uuid.New().String()
	payload := []byte(`{"orderId":"` + orderID + `","confirmedAt":"2026-01-01T00:00:00.000Z"}`)
	if err := d.Handle(context.Background(), contracts.TypeOrderConfirmed, payload); err != nil {
		t.Fatalf("handle: %v", err)
	}
	if len(store.recorded) != 1 || len(sender.sent) != 1 {
		t.Fatalf("want one recorded and one sent, got %d/%d", len(store.recorded), len(sender.sent))
	}
	if sender.sent[0].Subject() != "Order confirmed" {
		t.Fatalf("unexpected subject %q", sender.sent[0].Subject())
	}
}

func TestDuplicateIsNotResent(t *testing.T) {
	d, _, sender := newDispatcher()
	payload := []byte(`{"orderId":"` + uuid.New().String() + `"}`)
	if err := d.Handle(context.Background(), contracts.TypeShipmentDelivered, payload); err != nil {
		t.Fatalf("first: %v", err)
	}
	if err := d.Handle(context.Background(), contracts.TypeShipmentDelivered, payload); err != nil {
		t.Fatalf("second: %v", err)
	}
	if len(sender.sent) != 1 {
		t.Fatalf("duplicate should not resend, sent %d", len(sender.sent))
	}
}

func TestUnknownTypeIsNoop(t *testing.T) {
	d, store, sender := newDispatcher()
	if err := d.Handle(context.Background(), "some.unknown.v1", []byte(`{}`)); err != nil {
		t.Fatalf("handle: %v", err)
	}
	if len(store.recorded) != 0 || len(sender.sent) != 0 {
		t.Fatal("unknown event type should record and send nothing")
	}
}

func TestPaymentCapturedIncludesAmount(t *testing.T) {
	d, _, sender := newDispatcher()
	orderID := uuid.New().String()
	payload := []byte(`{"orderId":"` + orderID + `","amountMinor":1299,"currency":"USD","status":"CAPTURED"}`)
	if err := d.Handle(context.Background(), contracts.TypePaymentCaptured, payload); err != nil {
		t.Fatalf("handle: %v", err)
	}
	if len(sender.sent) != 1 {
		t.Fatalf("want one sent, got %d", len(sender.sent))
	}
	if body := sender.sent[0].Body(); !strings.Contains(body, "12.99 USD") {
		t.Fatalf("want formatted amount in body, got %q", body)
	}
}
