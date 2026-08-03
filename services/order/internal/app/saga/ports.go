// Package saga contains the order fulfillment saga: an orchestrated
// distributed transaction that reserves stock, takes payment, and then either
// confirms the order or compensates by releasing stock and cancelling.
package saga

import (
	"context"
	"errors"
)

// ErrReservationRejected is a business decline from inventory (insufficient
// stock or unknown SKU), as opposed to a transient/transport error. The
// orchestrator cancels the order on this error rather than retrying.
var ErrReservationRejected = errors.New("reservation rejected")

// ReserveLine is one SKU and quantity to hold.
type ReserveLine struct {
	SKU      string
	Quantity int32
}

// InventoryClient is the synchronous port to the Inventory service. All three
// operations are idempotent, so the orchestrator can safely retry them.
type InventoryClient interface {
	Reserve(ctx context.Context, orderID string, lines []ReserveLine) error
	Release(ctx context.Context, orderID string) error
	Commit(ctx context.Context, orderID string) error
}

// PaymentRequest is an authorization request for an order total.
type PaymentRequest struct {
	OrderID     string
	AmountMinor int64
	Currency    string
}

// PaymentResult is the outcome of an authorization. Approved false is a clean
// decline (compensate); a non-nil error from Authorize is transient (retry).
type PaymentResult struct {
	Reference string
	Approved  bool
	Decline   string
}

// PaymentGateway is the payment port. Phase 3.5 ships a stub; Phase 4 replaces
// it with the real Payment service without changing this interface.
type PaymentGateway interface {
	Authorize(ctx context.Context, req PaymentRequest) (PaymentResult, error)
}

// OrderController drives the order aggregate's terminal transitions. It is
// backed by the confirm and cancel command handlers.
type OrderController interface {
	Confirm(ctx context.Context, orderID string) error
	Cancel(ctx context.Context, orderID, reason string) error
}

// Store persists saga instances so progress survives restarts and redelivery.
type Store interface {
	Load(ctx context.Context, orderID string) (Instance, bool, error)
	Save(ctx context.Context, inst Instance) error
}
