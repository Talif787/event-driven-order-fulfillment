package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/orderfulfillment/fulfillment/internal/domain/fulfillment"
)

// ShipmentRepository is the pgx-backed shipment store.
type ShipmentRepository struct{ pool *pgxpool.Pool }

func NewShipmentRepository(pool *pgxpool.Pool) *ShipmentRepository {
	return &ShipmentRepository{pool: pool}
}

func (r *ShipmentRepository) GetByOrderID(ctx context.Context, orderID uuid.UUID) (*fulfillment.Shipment, error) {
	const q = `SELECT id, order_id, status, carrier, tracking_code, failure_reason, version, created_at, updated_at
	           FROM shipments WHERE order_id = $1`
	var (
		id, oid              uuid.UUID
		status               string
		carrier, tracking    string
		failureReason        string
		version              int64
		createdAt, updatedAt time.Time
	)
	err := r.pool.QueryRow(ctx, q, orderID).Scan(
		&id, &oid, &status, &carrier, &tracking, &failureReason, &version, &createdAt, &updatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fulfillment.ErrShipmentNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("query shipment: %w", err)
	}
	return fulfillment.Rehydrate(id, oid, fulfillment.ShipmentStatus(status),
		carrier, tracking, failureReason, version, createdAt, updatedAt), nil
}

func (r *ShipmentRepository) Insert(ctx context.Context, s *fulfillment.Shipment) error {
	const q = `INSERT INTO shipments
	           (id, order_id, status, carrier, tracking_code, failure_reason, version, created_at, updated_at)
	           VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`
	_, err := r.pool.Exec(ctx, q,
		s.ID(), s.OrderID(), string(s.Status()), s.Carrier(), s.TrackingCode(),
		s.FailureReason(), s.Version(), s.CreatedAt(), s.UpdatedAt())
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return fulfillment.ErrShipmentExists
		}
		return fmt.Errorf("insert shipment: %w", err)
	}
	return nil
}

func (r *ShipmentRepository) Update(ctx context.Context, s *fulfillment.Shipment) error {
	const q = `UPDATE shipments
	           SET status = $1, carrier = $2, tracking_code = $3, failure_reason = $4,
	               version = version + 1, updated_at = $5
	           WHERE id = $6 AND version = $7`
	tag, err := r.pool.Exec(ctx, q,
		string(s.Status()), s.Carrier(), s.TrackingCode(), s.FailureReason(),
		s.UpdatedAt(), s.ID(), s.Version())
	if err != nil {
		return fmt.Errorf("update shipment: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fulfillment.ErrConcurrency
	}
	return nil
}
