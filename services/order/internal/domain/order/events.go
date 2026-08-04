package order

import "time"

// EventType enumerates persisted domain event types.
type EventType string

const (
	OrderPlacedType    EventType = "order.placed.v1"
	OrderConfirmedType EventType = "order.confirmed.v1"
	OrderCancelledType EventType = "order.cancelled.v1"
)

// DomainEvent is the interface every persisted event satisfies.
type DomainEvent interface {
	EventType() EventType
	OccurredAt() time.Time
	AggregateID() OrderID
}

// OrderPlaced is emitted when an order is accepted.
type OrderPlaced struct {
	OrderID    OrderID
	CustomerID CustomerID
	Items      []LineItem
	ShipTo     Address
	Total      Money
	PlacedAt   time.Time
}

func (e OrderPlaced) EventType() EventType  { return OrderPlacedType }
func (e OrderPlaced) OccurredAt() time.Time { return e.PlacedAt }
func (e OrderPlaced) AggregateID() OrderID  { return e.OrderID }

// OrderConfirmed is emitted when the saga completes successfully: stock is
// committed and payment is captured.
type OrderConfirmed struct {
	OrderID     OrderID
	ConfirmedAt time.Time
}

func (e OrderConfirmed) EventType() EventType  { return OrderConfirmedType }
func (e OrderConfirmed) OccurredAt() time.Time { return e.ConfirmedAt }
func (e OrderConfirmed) AggregateID() OrderID  { return e.OrderID }

// OrderCancelled is emitted when the saga compensates: a downstream step failed
// and any held stock has been released. Reason records why.
type OrderCancelled struct {
	OrderID     OrderID
	Reason      string
	CancelledAt time.Time
}

func (e OrderCancelled) EventType() EventType  { return OrderCancelledType }
func (e OrderCancelled) OccurredAt() time.Time { return e.CancelledAt }
func (e OrderCancelled) AggregateID() OrderID  { return e.OrderID }
