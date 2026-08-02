package order

import "errors"

var (
	// ErrValidation signals an invariant or input validation failure.
	ErrValidation = errors.New("validation error")
	// ErrInvalidIdentifier signals a malformed identifier.
	ErrInvalidIdentifier = errors.New("invalid identifier")
	// ErrNotFound signals a missing aggregate.
	ErrNotFound = errors.New("order not found")
	// ErrConcurrency signals an optimistic concurrency conflict on append.
	ErrConcurrency = errors.New("concurrency conflict")
	// ErrInvalidTransition signals an illegal lifecycle transition.
	ErrInvalidTransition = errors.New("invalid state transition")
)
