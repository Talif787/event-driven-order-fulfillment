package app

import (
	"context"
	"time"

	"github.com/orderfulfillment/order/internal/domain/order"
)

// OutboxMessage is a change-data event staged for asynchronous publication.
// It is written in the same transaction as the aggregate's events so that
// state change and event emission are atomic (transactional outbox pattern).
type OutboxMessage struct {
	AggregateID string
	EventID     string
	Topic       string
	EventType   string
	Payload     []byte
	Headers     map[string]string
}

// Repository loads and persists order aggregates. Save appends uncommitted
// events and the outbox messages atomically, enforcing optimistic concurrency
// via expectedVersion (the aggregate version observed before the new events).
type Repository interface {
	Load(ctx context.Context, id order.OrderID) (*order.Order, error)
	Save(ctx context.Context, agg *order.Order, expectedVersion int64, outbox []OutboxMessage) error
}

// IdempotencyStore records idempotency keys so a retried submission returns
// the original order id instead of creating a duplicate.
type IdempotencyStore interface {
	// Reserve claims the key for orderID. If the key already exists it returns
	// the previously stored order id and found=true; otherwise found=false.
	Reserve(ctx context.Context, key string, orderID string) (existingOrderID string, found bool, err error)
}

// IDGenerator produces new identifiers (abstracted for deterministic tests).
type IDGenerator interface{ NewOrderID() order.OrderID }

// Clock abstracts wall-clock time.
type Clock interface{ Now() time.Time }
