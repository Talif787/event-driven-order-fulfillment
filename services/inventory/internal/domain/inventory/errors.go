package inventory

import "errors"

var (
	// ErrValidation signals an invariant or input validation failure.
	ErrValidation = errors.New("validation error")
	// ErrInvalidIdentifier signals a malformed identifier.
	ErrInvalidIdentifier = errors.New("invalid identifier")
	// ErrSKUNotFound signals an unknown stock keeping unit.
	ErrSKUNotFound = errors.New("sku not found")
	// ErrInsufficientStock signals a reservation that exceeds available stock.
	ErrInsufficientStock = errors.New("insufficient stock")
	// ErrReservationNotFound signals a missing reservation.
	ErrReservationNotFound = errors.New("reservation not found")
	// ErrConcurrency signals an optimistic concurrency conflict.
	ErrConcurrency = errors.New("concurrency conflict")
	// ErrInvalidTransition signals an illegal reservation lifecycle transition.
	ErrInvalidTransition = errors.New("invalid state transition")
)
