package saga

// State is the saga's position in the reserve, pay, commit sequence. The state
// is persisted after every step so a resumed saga skips completed work.
type State string

const (
	StateStarted   State = "STARTED"
	StateReserved  State = "RESERVED"
	StatePaid      State = "PAID"
	StateCompleted State = "COMPLETED"
	StateCancelled State = "CANCELLED"
)

// Instance is the durable saga record for one order.
type Instance struct {
	OrderID         string
	State           State
	ReservationHeld bool
	PaymentRef      string
	Reason          string
}

// NewInstance starts a saga in the STARTED state.
func NewInstance(orderID string) Instance {
	return Instance{OrderID: orderID, State: StateStarted}
}

// Terminal reports whether the saga has reached an end state.
func (i Instance) Terminal() bool {
	return i.State == StateCompleted || i.State == StateCancelled
}
