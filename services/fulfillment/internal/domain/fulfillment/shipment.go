package fulfillment

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Shipment is the fulfillment aggregate for one order. State changes go through
// guarded transitions rather than setters: each returns true on a real change,
// false on an idempotent no-op, and an error on an illegal move. Optimistic
// concurrency is carried in Version.
type Shipment struct {
	id            uuid.UUID
	orderID       uuid.UUID
	status        ShipmentStatus
	carrier       string
	trackingCode  string
	failureReason string
	version       int64
	createdAt     time.Time
	updatedAt     time.Time
}

// Create opens a new shipment for a confirmed order in the CREATED state.
func Create(orderID uuid.UUID, now time.Time) (*Shipment, error) {
	if orderID == uuid.Nil {
		return nil, fmt.Errorf("%w: orderId is required", ErrValidation)
	}
	return &Shipment{
		id:        uuid.New(),
		orderID:   orderID,
		status:    StatusCreated,
		version:   0,
		createdAt: now,
		updatedAt: now,
	}, nil
}

// Rehydrate rebuilds a shipment from persisted state.
func Rehydrate(id, orderID uuid.UUID, status ShipmentStatus, carrier, trackingCode, failureReason string,
	version int64, createdAt, updatedAt time.Time) *Shipment {
	return &Shipment{
		id: id, orderID: orderID, status: status, carrier: carrier, trackingCode: trackingCode,
		failureReason: failureReason, version: version, createdAt: createdAt, updatedAt: updatedAt,
	}
}

// Dispatch moves CREATED to DISPATCHED, recording the carrier and tracking code.
// Re-dispatching an already dispatched shipment is an idempotent no-op.
func (s *Shipment) Dispatch(carrier, trackingCode string, now time.Time) (bool, error) {
	if s.status == StatusDispatched {
		return false, nil
	}
	if s.status != StatusCreated {
		return false, fmt.Errorf("%w: cannot dispatch a %s shipment", ErrInvalidTransition, s.status)
	}
	if carrier == "" {
		return false, fmt.Errorf("%w: carrier is required", ErrValidation)
	}
	s.status = StatusDispatched
	s.carrier = carrier
	s.trackingCode = trackingCode
	s.updatedAt = now
	return true, nil
}

// Deliver moves DISPATCHED to DELIVERED. Delivering twice is a no-op.
func (s *Shipment) Deliver(now time.Time) (bool, error) {
	if s.status == StatusDelivered {
		return false, nil
	}
	if s.status != StatusDispatched {
		return false, fmt.Errorf("%w: cannot deliver a %s shipment", ErrInvalidTransition, s.status)
	}
	s.status = StatusDelivered
	s.updatedAt = now
	return true, nil
}

// Fail moves an open shipment to FAILED. Failing twice is a no-op.
func (s *Shipment) Fail(reason string, now time.Time) (bool, error) {
	if s.status == StatusFailed {
		return false, nil
	}
	if s.status == StatusDelivered {
		return false, fmt.Errorf("%w: cannot fail a delivered shipment", ErrInvalidTransition)
	}
	s.status = StatusFailed
	s.failureReason = reason
	s.updatedAt = now
	return true, nil
}

func (s *Shipment) ID() uuid.UUID          { return s.id }
func (s *Shipment) OrderID() uuid.UUID     { return s.orderID }
func (s *Shipment) Status() ShipmentStatus { return s.status }
func (s *Shipment) Carrier() string        { return s.carrier }
func (s *Shipment) TrackingCode() string   { return s.trackingCode }
func (s *Shipment) FailureReason() string  { return s.failureReason }
func (s *Shipment) Version() int64         { return s.version }
func (s *Shipment) CreatedAt() time.Time   { return s.createdAt }
func (s *Shipment) UpdatedAt() time.Time   { return s.updatedAt }
