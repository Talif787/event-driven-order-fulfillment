package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// IdempotencyStore persists idempotency keys with a unique constraint so a
// retried request maps to the original order rather than creating a duplicate.
type IdempotencyStore struct{ pool *pgxpool.Pool }

func NewIdempotencyStore(pool *pgxpool.Pool) *IdempotencyStore { return &IdempotencyStore{pool: pool} }

func (s *IdempotencyStore) Reserve(ctx context.Context, key, orderID string) (string, bool, error) {
	tag, err := s.pool.Exec(ctx,
		`INSERT INTO idempotency_keys (key, order_id) VALUES ($1, $2) ON CONFLICT (key) DO NOTHING`,
		key, orderID)
	if err != nil {
		return "", false, fmt.Errorf("insert idempotency key: %w", err)
	}
	if tag.RowsAffected() == 1 {
		return "", false, nil
	}

	var existing string
	err = s.pool.QueryRow(ctx, `SELECT order_id FROM idempotency_keys WHERE key = $1`, key).Scan(&existing)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", false, fmt.Errorf("idempotency key vanished after conflict: %s", key)
		}
		return "", false, fmt.Errorf("read existing idempotency key: %w", err)
	}
	return existing, true, nil
}
