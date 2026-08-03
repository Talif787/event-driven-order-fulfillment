package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/orderfulfillment/order/internal/app"
	"github.com/orderfulfillment/order/internal/domain/order"
)

// ProjectionRepository maintains and serves the denormalized order read model.
type ProjectionRepository struct{ pool *pgxpool.Pool }

func NewProjectionRepository(pool *pgxpool.Pool) *ProjectionRepository {
	return &ProjectionRepository{pool: pool}
}

// UpsertOrderPlaced writes or refreshes the read model. The version guard makes
// the write idempotent and safe against out-of-order redelivery: an older event
// never overwrites newer state.
func (r *ProjectionRepository) UpsertOrderPlaced(ctx context.Context, p app.OrderProjection) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO order_projections (order_id, customer_id, status, total_minor, currency, version, placed_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, now())
		 ON CONFLICT (order_id) DO UPDATE SET
		     customer_id = EXCLUDED.customer_id,
		     status      = EXCLUDED.status,
		     total_minor = EXCLUDED.total_minor,
		     currency    = EXCLUDED.currency,
		     version     = EXCLUDED.version,
		     placed_at   = EXCLUDED.placed_at,
		     updated_at  = now()
		 WHERE EXCLUDED.version >= order_projections.version`,
		p.OrderID, p.CustomerID, p.Status, p.TotalMinor, p.Currency, p.Version, p.PlacedAt)
	if err != nil {
		return fmt.Errorf("upsert projection: %w", err)
	}
	return nil
}

// UpdateStatus advances the read-model status for lifecycle events that follow
// placement (confirmed, cancelled). The order.placed event for an aggregate
// always precedes these on the same partition, so the row exists by the time
// this runs; if it does not (projection lag), the update is a harmless no-op.
func (r *ProjectionRepository) UpdateStatus(ctx context.Context, orderID, status string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE order_projections SET status = $2, updated_at = now() WHERE order_id = $1`,
		orderID, status)
	if err != nil {
		return fmt.Errorf("update projection status: %w", err)
	}
	return nil
}

// GetOrder returns the read model for an order, or order.ErrNotFound when the
// projection has not yet been built.
func (r *ProjectionRepository) GetOrder(ctx context.Context, orderID string) (app.OrderProjection, error) {
	var p app.OrderProjection
	err := r.pool.QueryRow(ctx,
		`SELECT order_id::text, customer_id::text, status, total_minor, currency, version, placed_at
		 FROM order_projections WHERE order_id = $1`, orderID).
		Scan(&p.OrderID, &p.CustomerID, &p.Status, &p.TotalMinor, &p.Currency, &p.Version, &p.PlacedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return app.OrderProjection{}, order.ErrNotFound
		}
		return app.OrderProjection{}, fmt.Errorf("query projection: %w", err)
	}
	return p, nil
}
