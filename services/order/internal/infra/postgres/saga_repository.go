package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/orderfulfillment/order/internal/app/saga"
)

// SagaRepository persists saga instances so the orchestrator can resume after a
// restart and deduplicate redelivered events.
type SagaRepository struct{ pool *pgxpool.Pool }

func NewSagaRepository(pool *pgxpool.Pool) *SagaRepository { return &SagaRepository{pool: pool} }

// Load returns the saga instance for an order, or found=false when none exists.
func (r *SagaRepository) Load(ctx context.Context, orderID string) (saga.Instance, bool, error) {
	var inst saga.Instance
	var state string
	err := r.pool.QueryRow(ctx,
		`SELECT order_id::text, state, reservation_held, payment_ref, reason
		 FROM saga_instances WHERE order_id = $1`, orderID).
		Scan(&inst.OrderID, &state, &inst.ReservationHeld, &inst.PaymentRef, &inst.Reason)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return saga.Instance{}, false, nil
		}
		return saga.Instance{}, false, fmt.Errorf("load saga: %w", err)
	}
	inst.State = saga.State(state)
	return inst, true, nil
}

// Save upserts the saga instance.
func (r *SagaRepository) Save(ctx context.Context, inst saga.Instance) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO saga_instances (order_id, state, reservation_held, payment_ref, reason, updated_at)
		 VALUES ($1, $2, $3, $4, $5, now())
		 ON CONFLICT (order_id) DO UPDATE SET
		     state            = EXCLUDED.state,
		     reservation_held = EXCLUDED.reservation_held,
		     payment_ref      = EXCLUDED.payment_ref,
		     reason           = EXCLUDED.reason,
		     updated_at       = now()`,
		inst.OrderID, string(inst.State), inst.ReservationHeld, inst.PaymentRef, inst.Reason)
	if err != nil {
		return fmt.Errorf("save saga: %w", err)
	}
	return nil
}
