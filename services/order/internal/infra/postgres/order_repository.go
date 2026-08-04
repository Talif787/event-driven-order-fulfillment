package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.opentelemetry.io/otel/trace"

	"github.com/orderfulfillment/order/internal/app"
	"github.com/orderfulfillment/order/internal/domain/order"
)

const uniqueViolation = "23505"

// OrderRepository persists the order aggregate using an event store and an
// outbox, both written in a single transaction to guarantee atomic
// state-change-plus-event-publish.
type OrderRepository struct{ pool *pgxpool.Pool }

func NewOrderRepository(pool *pgxpool.Pool) *OrderRepository { return &OrderRepository{pool: pool} }

// Load reconstructs the aggregate by folding its ordered event history.
func (r *OrderRepository) Load(ctx context.Context, id order.OrderID) (*order.Order, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT event_type, payload FROM order_events WHERE order_id = $1 ORDER BY version ASC`, id.String())
	if err != nil {
		return nil, fmt.Errorf("query events: %w", err)
	}
	defer rows.Close()

	var history []order.DomainEvent
	for rows.Next() {
		var eventType string
		var payload []byte
		if err := rows.Scan(&eventType, &payload); err != nil {
			return nil, fmt.Errorf("scan event: %w", err)
		}
		ev, err := decodeEvent(eventType, payload)
		if err != nil {
			return nil, err
		}
		history = append(history, ev)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate events: %w", err)
	}
	if len(history) == 0 {
		return nil, order.ErrNotFound
	}
	return order.Rehydrate(history)
}

// Save appends new events and outbox messages atomically, enforcing optimistic
// concurrency through the unique (order_id, version) constraint.
func (r *OrderRepository) Save(ctx context.Context, agg *order.Order, expectedVersion int64, outbox []app.OutboxMessage) error {
	changes := agg.UncommittedChanges()
	if len(changes) == 0 {
		return nil
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	traceID := traceIDFromContext(ctx)
	for i, ev := range changes {
		eventType, payload, err := encodeEvent(ev)
		if err != nil {
			return err
		}
		version := expectedVersion + int64(i) + 1
		_, err = tx.Exec(ctx,
			`INSERT INTO order_events (event_id, order_id, version, event_type, payload, occurred_at, trace_id)
			 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
			uuid.NewString(), ev.AggregateID().String(), version, eventType, payload, ev.OccurredAt(), traceID)
		if err != nil {
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && pgErr.Code == uniqueViolation {
				return order.ErrConcurrency
			}
			return fmt.Errorf("insert event: %w", err)
		}
	}

	for _, msg := range outbox {
		eventID := msg.EventID
		if eventID == "" {
			eventID = uuid.NewString()
		}
		headers, err := json.Marshal(msg.Headers)
		if err != nil {
			return fmt.Errorf("marshal outbox headers: %w", err)
		}
		_, err = tx.Exec(ctx,
			`INSERT INTO outbox (event_id, aggregate_id, topic, event_type, payload, headers, trace_id)
			 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
			eventID, msg.AggregateID, msg.Topic, msg.EventType, msg.Payload, headers, traceID)
		if err != nil {
			return fmt.Errorf("insert outbox: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	return nil
}

func traceIDFromContext(ctx context.Context) string {
	sc := trace.SpanContextFromContext(ctx)
	if sc.HasTraceID() {
		return sc.TraceID().String()
	}
	return ""
}
