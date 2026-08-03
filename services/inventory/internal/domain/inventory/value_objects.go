package inventory

import (
	"fmt"
	"strings"

	"github.com/google/uuid"
)

// SKU is a validated stock keeping unit.
type SKU struct{ value string }

func NewSKU(s string) (SKU, error) {
	s = strings.TrimSpace(s)
	if s == "" || len(s) > 64 {
		return SKU{}, fmt.Errorf("%w: sku must be 1..64 chars", ErrValidation)
	}
	return SKU{value: s}, nil
}

func (s SKU) String() string { return s.value }

// Quantity is a positive reservation quantity.
type Quantity struct{ value int32 }

func NewQuantity(v int32) (Quantity, error) {
	if v <= 0 || v > 100_000 {
		return Quantity{}, fmt.Errorf("%w: quantity must be 1..100000", ErrValidation)
	}
	return Quantity{value: v}, nil
}

func (q Quantity) Value() int32 { return q.value }

// OrderID is the identifier of the order a reservation belongs to. It is the
// idempotency key for reservations: one order maps to at most one reservation.
type OrderID struct{ value uuid.UUID }

func ParseOrderID(s string) (OrderID, error) {
	v, err := uuid.Parse(strings.TrimSpace(s))
	if err != nil {
		return OrderID{}, fmt.Errorf("%w: order id %q", ErrInvalidIdentifier, s)
	}
	return OrderID{value: v}, nil
}

func (id OrderID) String() string { return id.value.String() }

// ReservationID identifies a reservation aggregate.
type ReservationID struct{ value uuid.UUID }

func NewReservationID() ReservationID { return ReservationID{value: uuid.New()} }

func ParseReservationID(s string) (ReservationID, error) {
	v, err := uuid.Parse(strings.TrimSpace(s))
	if err != nil {
		return ReservationID{}, fmt.Errorf("%w: reservation id %q", ErrInvalidIdentifier, s)
	}
	return ReservationID{value: v}, nil
}

func (id ReservationID) String() string { return id.value.String() }

// ReservationStatus is the reservation lifecycle state.
type ReservationStatus string

const (
	StatusHeld      ReservationStatus = "HELD"
	StatusCommitted ReservationStatus = "COMMITTED"
	StatusReleased  ReservationStatus = "RELEASED"
)
