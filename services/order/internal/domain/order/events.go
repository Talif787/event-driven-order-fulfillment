package order

import "time"

// EventType enumerates persisted domain event types.
type EventType string

const OrderPlacedType EventType = "order.placed.v1"

// DomainEvent is the interface every persisted event satisfies.
type DomainEvent interface {
	EventType() EventType
	OccurredAt() time.Time
	AggregateID() OrderID
}

// OrderPlaced is emitted when an order is accepted.
type OrderPlaced struct {
	OrderID     OrderID
	CustomerID  CustomerID
	Items       []LineItem
	ShipTo      Address
	Total       Money
	PlacedAt    time.Time
}

func (e OrderPlaced) EventType() EventType   { return OrderPlacedType }
func (e OrderPlaced) OccurredAt() time.Time  { return e.PlacedAt }
func (e OrderPlaced) AggregateID() OrderID   { return e.OrderID }
