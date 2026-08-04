package fulfillment

import "errors"

var (
	ErrValidation        = errors.New("validation error")
	ErrInvalidIdentifier = errors.New("invalid identifier")
	ErrShipmentNotFound  = errors.New("shipment not found")
	ErrShipmentExists    = errors.New("shipment already exists")
	ErrConcurrency       = errors.New("concurrent modification")
	ErrInvalidTransition = errors.New("invalid shipment transition")
)
