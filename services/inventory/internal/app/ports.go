package app

import "context"

// ReserveLine is one requested SKU and quantity, in transport-agnostic form.
type ReserveLine struct {
	SKU      string
	Quantity int32
}

// ReservationResult is the outcome of a reservation operation. Idempotent is
// true when the operation returned an already-existing or already-applied
// reservation rather than performing new work.
type ReservationResult struct {
	ReservationID string
	OrderID       string
	Status        string
	Lines         []ReserveLine
	Idempotent    bool
}

// StockView is a read of the ledger for one SKU.
type StockView struct {
	SKU       string
	Available int64
	Reserved  int64
	Version   int64
}

// InventoryStore is the persistence port for stock and reservations. All write
// operations are transactional and idempotent, and enforce optimistic
// concurrency on the stock ledger.
type InventoryStore interface {
	Reserve(ctx context.Context, orderID string, lines []ReserveLine) (ReservationResult, error)
	Release(ctx context.Context, orderID string) (ReservationResult, error)
	Commit(ctx context.Context, orderID string) (ReservationResult, error)
	GetReservation(ctx context.Context, orderID string) (ReservationResult, error)
	GetStock(ctx context.Context, sku string) (StockView, error)
	UpsertStock(ctx context.Context, sku string, available int64) error
}
