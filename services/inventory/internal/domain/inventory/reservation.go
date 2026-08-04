package inventory

import "fmt"

// ReservationLine is one held SKU and quantity within a reservation.
type ReservationLine struct {
	SKU      SKU
	Quantity Quantity
}

// Reservation is the aggregate that ties an order to the stock held for it. Its
// status drives the lifecycle: HELD on creation, then either COMMITTED (the
// order shipped) or RELEASED (the order was cancelled or failed downstream).
type Reservation struct {
	id      ReservationID
	orderID OrderID
	status  ReservationStatus
	lines   []ReservationLine
}

// NewReservation creates a HELD reservation, rejecting empty or duplicate lines.
func NewReservation(id ReservationID, orderID OrderID, lines []ReservationLine) (*Reservation, error) {
	if len(lines) == 0 {
		return nil, fmt.Errorf("%w: reservation needs at least one line", ErrValidation)
	}
	seen := make(map[string]bool, len(lines))
	for _, l := range lines {
		if seen[l.SKU.String()] {
			return nil, fmt.Errorf("%w: duplicate sku %s", ErrValidation, l.SKU)
		}
		seen[l.SKU.String()] = true
	}
	return &Reservation{id: id, orderID: orderID, status: StatusHeld, lines: lines}, nil
}

// RehydrateReservation rebuilds a reservation from persisted state.
func RehydrateReservation(id ReservationID, orderID OrderID, status ReservationStatus, lines []ReservationLine) *Reservation {
	return &Reservation{id: id, orderID: orderID, status: status, lines: lines}
}

func (r *Reservation) ID() ReservationID         { return r.id }
func (r *Reservation) OrderID() OrderID          { return r.orderID }
func (r *Reservation) Status() ReservationStatus { return r.status }
func (r *Reservation) Lines() []ReservationLine  { return r.lines }

// Release transitions HELD to RELEASED. It returns changed=false when the
// reservation is already RELEASED, which makes release idempotent, and an error
// when the reservation is in a terminal state that cannot be released.
func (r *Reservation) Release() (changed bool, err error) {
	switch r.status {
	case StatusReleased:
		return false, nil
	case StatusHeld:
		r.status = StatusReleased
		return true, nil
	default:
		return false, fmt.Errorf("%w: cannot release a %s reservation", ErrInvalidTransition, r.status)
	}
}

// Commit transitions HELD to COMMITTED. It returns changed=false when already
// COMMITTED (idempotent) and an error for an illegal transition.
func (r *Reservation) Commit() (changed bool, err error) {
	switch r.status {
	case StatusCommitted:
		return false, nil
	case StatusHeld:
		r.status = StatusCommitted
		return true, nil
	default:
		return false, fmt.Errorf("%w: cannot commit a %s reservation", ErrInvalidTransition, r.status)
	}
}
