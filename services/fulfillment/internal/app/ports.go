package app

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/orderfulfillment/fulfillment/internal/domain/fulfillment"
)

// ShipmentRepository persists shipment aggregates.
type ShipmentRepository interface {
	// GetByOrderID returns the shipment for an order, or ErrShipmentNotFound.
	GetByOrderID(ctx context.Context, orderID uuid.UUID) (*fulfillment.Shipment, error)
	// Insert stores a new shipment, returning ErrShipmentExists on a duplicate
	// order id (the unique constraint), which makes creation idempotent.
	Insert(ctx context.Context, s *fulfillment.Shipment) error
	// Update persists an advanced shipment under optimistic concurrency,
	// returning ErrConcurrency if the row changed since it was read.
	Update(ctx context.Context, s *fulfillment.Shipment) error
}

// Event is a fulfillment integration event ready to publish. Key is the
// partition key (the order id) so a given order's events keep their order.
type Event struct {
	Type    string
	Key     string
	Payload []byte
}

// EventPublisher publishes fulfillment events to the backbone.
type EventPublisher interface {
	Publish(ctx context.Context, events ...Event) error
}

// Clock supplies the current time, injected so tests are deterministic.
type Clock interface {
	Now() time.Time
}

// SystemClock is the production clock.
type SystemClock struct{}

func (SystemClock) Now() time.Time { return time.Now().UTC() }
