package postgres

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/orderfulfillment/order/internal/app"
)

// RelayStore drains the transactional outbox to the event backbone.
type RelayStore struct{ pool *pgxpool.Pool }

func NewRelayStore(pool *pgxpool.Pool) *RelayStore { return &RelayStore{pool: pool} }

// DrainBatch locks up to batchSize unpublished outbox rows, hands them to
// publish, and marks them published, all within one transaction. Row locking
// with SKIP LOCKED lets multiple relay instances process disjoint rows
// concurrently. If publish fails the transaction rolls back and the rows remain
// unpublished for the next cycle, giving at-least-once delivery.
func (s *RelayStore) DrainBatch(ctx context.Context, batchSize int, publish func(context.Context, []app.OutboxRecord) error) (int, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	rows, err := tx.Query(ctx,
		`SELECT id, event_id::text, aggregate_id::text, topic, event_type, payload, headers
		 FROM outbox
		 WHERE published_at IS NULL
		 ORDER BY id ASC
		 LIMIT $1
		 FOR UPDATE SKIP LOCKED`, batchSize)
	if err != nil {
		return 0, fmt.Errorf("select outbox: %w", err)
	}

	var records []app.OutboxRecord
	var ids []int64
	for rows.Next() {
		var rec app.OutboxRecord
		var headersRaw []byte
		if err := rows.Scan(&rec.ID, &rec.EventID, &rec.AggregateID, &rec.Topic, &rec.EventType, &rec.Payload, &headersRaw); err != nil {
			rows.Close()
			return 0, fmt.Errorf("scan outbox: %w", err)
		}
		rec.Headers = map[string]string{}
		if len(headersRaw) > 0 {
			if err := json.Unmarshal(headersRaw, &rec.Headers); err != nil {
				rows.Close()
				return 0, fmt.Errorf("decode outbox headers: %w", err)
			}
		}
		records = append(records, rec)
		ids = append(ids, rec.ID)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, fmt.Errorf("iterate outbox: %w", err)
	}

	if len(records) == 0 {
		return 0, tx.Commit(ctx)
	}

	if err := publish(ctx, records); err != nil {
		return 0, err
	}

	if _, err := tx.Exec(ctx, `UPDATE outbox SET published_at = now() WHERE id = ANY($1)`, ids); err != nil {
		return 0, fmt.Errorf("mark published: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("commit tx: %w", err)
	}
	return len(records), nil
}
