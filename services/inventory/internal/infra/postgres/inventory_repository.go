package postgres

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/orderfulfillment/inventory/internal/app"
	"github.com/orderfulfillment/inventory/internal/domain/inventory"
)

// maxOCCRetries bounds how many times a transaction retries after an optimistic
// concurrency conflict before giving up with ErrConcurrency.
const maxOCCRetries = 5

// rowQuerier is satisfied by both *pgxpool.Pool and pgx.Tx, so read helpers can
// run either inside a transaction or directly against the pool.
type rowQuerier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

// InventoryRepository persists stock and reservations with optimistic
// concurrency on the stock ledger and order-keyed idempotency on reservations.
type InventoryRepository struct{ pool *pgxpool.Pool }

func NewInventoryRepository(pool *pgxpool.Pool) *InventoryRepository {
	return &InventoryRepository{pool: pool}
}

// Reserve holds stock for an order. It is idempotent (a repeat call for the same
// order returns the existing reservation) and retries on optimistic concurrency
// conflicts. The whole set of lines is all-or-nothing.
func (r *InventoryRepository) Reserve(ctx context.Context, orderID string, lines []app.ReserveLine) (app.ReservationResult, error) {
	sorted := append([]app.ReserveLine(nil), lines...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].SKU < sorted[j].SKU })

	for attempt := 0; attempt < maxOCCRetries; attempt++ {
		result, retry, err := r.reserveOnce(ctx, orderID, sorted)
		if err != nil {
			return app.ReservationResult{}, err
		}
		if !retry {
			return result, nil
		}
	}
	return app.ReservationResult{}, inventory.ErrConcurrency
}

func (r *InventoryRepository) reserveOnce(ctx context.Context, orderID string, lines []app.ReserveLine) (app.ReservationResult, bool, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return app.ReservationResult{}, false, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Idempotency: a reservation already exists for this order.
	if existing, found, err := r.findReservationByOrder(ctx, tx, orderID); err != nil {
		return app.ReservationResult{}, false, err
	} else if found {
		existing.Idempotent = true
		return existing, false, nil
	}

	// Load each SKU and apply the reservation in the domain aggregate, which
	// enforces the insufficient-stock invariant.
	items := make([]*inventory.StockItem, 0, len(lines))
	for _, ln := range lines {
		sku, err := inventory.NewSKU(ln.SKU)
		if err != nil {
			return app.ReservationResult{}, false, err
		}
		available, reserved, version, found, err := r.loadStockItem(ctx, tx, ln.SKU)
		if err != nil {
			return app.ReservationResult{}, false, err
		}
		if !found {
			return app.ReservationResult{}, false, fmt.Errorf("%w: %s", inventory.ErrSKUNotFound, ln.SKU)
		}
		item := inventory.RehydrateStockItem(sku, available, reserved, version)
		if err := item.Reserve(int64(ln.Quantity)); err != nil {
			return app.ReservationResult{}, false, err
		}
		items = append(items, item)
	}

	// Persist each ledger change with an optimistic-concurrency guard on version.
	for _, item := range items {
		ct, err := tx.Exec(ctx,
			`UPDATE stock_items SET available = $1, reserved = $2, version = version + 1, updated_at = now()
			 WHERE sku = $3 AND version = $4`,
			item.Available(), item.Reserved(), item.SKU().String(), item.Version())
		if err != nil {
			return app.ReservationResult{}, false, fmt.Errorf("update stock: %w", err)
		}
		if ct.RowsAffected() == 0 {
			return app.ReservationResult{}, true, nil // version moved under us; retry
		}
	}

	// Create the reservation. A unique violation means a concurrent request won
	// the race for this order, so treat it as idempotent.
	resID := uuid.NewString()
	if _, err := tx.Exec(ctx,
		`INSERT INTO reservations (id, order_id, status) VALUES ($1, $2, $3)`,
		resID, orderID, string(inventory.StatusHeld)); err != nil {
		if isUniqueViolation(err) {
			_ = tx.Rollback(ctx)
			existing, found, ferr := r.findReservationByOrder(ctx, r.pool, orderID)
			if ferr != nil {
				return app.ReservationResult{}, false, ferr
			}
			if found {
				existing.Idempotent = true
				return existing, false, nil
			}
		}
		return app.ReservationResult{}, false, fmt.Errorf("insert reservation: %w", err)
	}
	for _, ln := range lines {
		if _, err := tx.Exec(ctx,
			`INSERT INTO reservation_lines (reservation_id, sku, quantity) VALUES ($1, $2, $3)`,
			resID, ln.SKU, ln.Quantity); err != nil {
			return app.ReservationResult{}, false, fmt.Errorf("insert reservation line: %w", err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return app.ReservationResult{}, false, fmt.Errorf("commit tx: %w", err)
	}

	return app.ReservationResult{
		ReservationID: resID,
		OrderID:       orderID,
		Status:        string(inventory.StatusHeld),
		Lines:         append([]app.ReserveLine(nil), lines...),
	}, false, nil
}

// Release returns held stock to available and marks the reservation RELEASED.
func (r *InventoryRepository) Release(ctx context.Context, orderID string) (app.ReservationResult, error) {
	return r.transition(ctx, orderID, inventory.StatusReleased)
}

// Commit finalizes held stock (it leaves the warehouse) and marks the
// reservation COMMITTED.
func (r *InventoryRepository) Commit(ctx context.Context, orderID string) (app.ReservationResult, error) {
	return r.transition(ctx, orderID, inventory.StatusCommitted)
}

func (r *InventoryRepository) transition(ctx context.Context, orderID string, target inventory.ReservationStatus) (app.ReservationResult, error) {
	for attempt := 0; attempt < maxOCCRetries; attempt++ {
		result, retry, err := r.transitionOnce(ctx, orderID, target)
		if err != nil {
			return app.ReservationResult{}, err
		}
		if !retry {
			return result, nil
		}
	}
	return app.ReservationResult{}, inventory.ErrConcurrency
}

func (r *InventoryRepository) transitionOnce(ctx context.Context, orderID string, target inventory.ReservationStatus) (app.ReservationResult, bool, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return app.ReservationResult{}, false, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var resIDStr, statusStr string
	err = tx.QueryRow(ctx, `SELECT id::text, status FROM reservations WHERE order_id = $1`, orderID).Scan(&resIDStr, &statusStr)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return app.ReservationResult{}, false, fmt.Errorf("%w: order %s", inventory.ErrReservationNotFound, orderID)
		}
		return app.ReservationResult{}, false, fmt.Errorf("load reservation: %w", err)
	}

	lines, err := r.loadLines(ctx, tx, resIDStr)
	if err != nil {
		return app.ReservationResult{}, false, err
	}

	res, err := rehydrateReservation(resIDStr, orderID, statusStr, lines)
	if err != nil {
		return app.ReservationResult{}, false, err
	}

	var changed bool
	switch target {
	case inventory.StatusReleased:
		changed, err = res.Release()
	case inventory.StatusCommitted:
		changed, err = res.Commit()
	default:
		err = fmt.Errorf("%w: unsupported target %s", inventory.ErrInvalidTransition, target)
	}
	if err != nil {
		return app.ReservationResult{}, false, err
	}
	if !changed {
		// Already in the target state: idempotent, no ledger change.
		return app.ReservationResult{
			ReservationID: resIDStr, OrderID: orderID, Status: string(target), Lines: lines, Idempotent: true,
		}, false, nil
	}

	for _, ln := range lines {
		sku, err := inventory.NewSKU(ln.SKU)
		if err != nil {
			return app.ReservationResult{}, false, err
		}
		available, reserved, version, found, err := r.loadStockItem(ctx, tx, ln.SKU)
		if err != nil {
			return app.ReservationResult{}, false, err
		}
		if !found {
			return app.ReservationResult{}, false, fmt.Errorf("%w: %s", inventory.ErrSKUNotFound, ln.SKU)
		}
		item := inventory.RehydrateStockItem(sku, available, reserved, version)
		switch target {
		case inventory.StatusReleased:
			err = item.Release(int64(ln.Quantity))
		case inventory.StatusCommitted:
			err = item.Commit(int64(ln.Quantity))
		}
		if err != nil {
			return app.ReservationResult{}, false, err
		}
		ct, err := tx.Exec(ctx,
			`UPDATE stock_items SET available = $1, reserved = $2, version = version + 1, updated_at = now()
			 WHERE sku = $3 AND version = $4`,
			item.Available(), item.Reserved(), ln.SKU, version)
		if err != nil {
			return app.ReservationResult{}, false, fmt.Errorf("update stock: %w", err)
		}
		if ct.RowsAffected() == 0 {
			return app.ReservationResult{}, true, nil // version moved; retry
		}
	}

	if _, err := tx.Exec(ctx,
		`UPDATE reservations SET status = $1, updated_at = now() WHERE id = $2`,
		string(target), resIDStr); err != nil {
		return app.ReservationResult{}, false, fmt.Errorf("update reservation: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return app.ReservationResult{}, false, fmt.Errorf("commit tx: %w", err)
	}

	return app.ReservationResult{
		ReservationID: resIDStr, OrderID: orderID, Status: string(target), Lines: lines,
	}, false, nil
}

// GetReservation returns the reservation for an order, or ErrReservationNotFound.
func (r *InventoryRepository) GetReservation(ctx context.Context, orderID string) (app.ReservationResult, error) {
	res, found, err := r.findReservationByOrder(ctx, r.pool, orderID)
	if err != nil {
		return app.ReservationResult{}, err
	}
	if !found {
		return app.ReservationResult{}, fmt.Errorf("%w: order %s", inventory.ErrReservationNotFound, orderID)
	}
	return res, nil
}

// GetStock returns the ledger for one SKU, or ErrSKUNotFound.
func (r *InventoryRepository) GetStock(ctx context.Context, sku string) (app.StockView, error) {
	v := app.StockView{SKU: sku}
	err := r.pool.QueryRow(ctx,
		`SELECT available, reserved, version FROM stock_items WHERE sku = $1`, sku).
		Scan(&v.Available, &v.Reserved, &v.Version)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return app.StockView{}, fmt.Errorf("%w: %s", inventory.ErrSKUNotFound, sku)
		}
		return app.StockView{}, fmt.Errorf("query stock: %w", err)
	}
	return v, nil
}

// UpsertStock seeds or overwrites available stock for a SKU (admin operation).
func (r *InventoryRepository) UpsertStock(ctx context.Context, sku string, available int64) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO stock_items (sku, available, reserved, version) VALUES ($1, $2, 0, 0)
		 ON CONFLICT (sku) DO UPDATE SET available = EXCLUDED.available, version = stock_items.version + 1, updated_at = now()`,
		sku, available)
	if err != nil {
		return fmt.Errorf("upsert stock: %w", err)
	}
	return nil
}

func (r *InventoryRepository) loadStockItem(ctx context.Context, q rowQuerier, sku string) (available, reserved, version int64, found bool, err error) {
	err = q.QueryRow(ctx,
		`SELECT available, reserved, version FROM stock_items WHERE sku = $1`, sku).
		Scan(&available, &reserved, &version)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, 0, 0, false, nil
		}
		return 0, 0, 0, false, fmt.Errorf("load stock: %w", err)
	}
	return available, reserved, version, true, nil
}

func (r *InventoryRepository) findReservationByOrder(ctx context.Context, q rowQuerier, orderID string) (app.ReservationResult, bool, error) {
	var resID, status string
	err := q.QueryRow(ctx, `SELECT id::text, status FROM reservations WHERE order_id = $1`, orderID).Scan(&resID, &status)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return app.ReservationResult{}, false, nil
		}
		return app.ReservationResult{}, false, fmt.Errorf("find reservation: %w", err)
	}
	lines, err := r.loadLines(ctx, q, resID)
	if err != nil {
		return app.ReservationResult{}, false, err
	}
	return app.ReservationResult{ReservationID: resID, OrderID: orderID, Status: status, Lines: lines}, true, nil
}

func (r *InventoryRepository) loadLines(ctx context.Context, q rowQuerier, reservationID string) ([]app.ReserveLine, error) {
	rows, err := q.Query(ctx,
		`SELECT sku, quantity FROM reservation_lines WHERE reservation_id = $1 ORDER BY sku`, reservationID)
	if err != nil {
		return nil, fmt.Errorf("load lines: %w", err)
	}
	defer rows.Close()
	var out []app.ReserveLine
	for rows.Next() {
		var sku string
		var qty int32
		if err := rows.Scan(&sku, &qty); err != nil {
			return nil, fmt.Errorf("scan line: %w", err)
		}
		out = append(out, app.ReserveLine{SKU: sku, Quantity: qty})
	}
	return out, rows.Err()
}

func rehydrateReservation(id, orderID, status string, lines []app.ReserveLine) (*inventory.Reservation, error) {
	rid, err := inventory.ParseReservationID(id)
	if err != nil {
		return nil, err
	}
	oid, err := inventory.ParseOrderID(orderID)
	if err != nil {
		return nil, err
	}
	domainLines := make([]inventory.ReservationLine, 0, len(lines))
	for _, ln := range lines {
		sku, err := inventory.NewSKU(ln.SKU)
		if err != nil {
			return nil, err
		}
		qty, err := inventory.NewQuantity(ln.Quantity)
		if err != nil {
			return nil, err
		}
		domainLines = append(domainLines, inventory.ReservationLine{SKU: sku, Quantity: qty})
	}
	return inventory.RehydrateReservation(rid, oid, inventory.ReservationStatus(status), domainLines), nil
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
