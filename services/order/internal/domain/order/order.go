package order

import (
	"fmt"
	"time"
)

// Order is the event-sourced order aggregate. State is derived by applying
// events; new behaviour appends events rather than mutating fields directly.
type Order struct {
	id         OrderID
	customerID CustomerID
	items      []LineItem
	shipTo     Address
	total      Money
	status     Status
	version    int64
	changes    []DomainEvent
}

// PlaceOrder is the factory that creates a new order and records OrderPlaced.
// It enforces creation invariants; downstream reservation and payment are
// coordinated by the saga in later phases.
func PlaceOrder(id OrderID, customerID CustomerID, items []LineItem, shipTo Address, now time.Time) (*Order, error) {
	if id.IsZero() {
		return nil, fmt.Errorf("%w: order id is required", ErrValidation)
	}
	if len(items) == 0 {
		return nil, fmt.Errorf("%w: at least one line item is required", ErrValidation)
	}

	total := Money{Currency: items[0].UnitPrice.Currency}
	seen := make(map[string]struct{}, len(items))
	for _, li := range items {
		if _, dup := seen[li.SKU.String()]; dup {
			return nil, fmt.Errorf("%w: duplicate sku %q", ErrValidation, li.SKU.String())
		}
		seen[li.SKU.String()] = struct{}{}
		sum, err := total.Add(li.LineTotal())
		if err != nil {
			return nil, err
		}
		total = sum
	}

	o := &Order{}
	o.raise(OrderPlaced{
		OrderID:    id,
		CustomerID: customerID,
		Items:      items,
		ShipTo:     shipTo,
		Total:      total,
		PlacedAt:   now.UTC(),
	})
	return o, nil
}

// Rehydrate rebuilds an aggregate from its persisted event history.
func Rehydrate(history []DomainEvent) (*Order, error) {
	if len(history) == 0 {
		return nil, ErrNotFound
	}
	o := &Order{}
	for _, e := range history {
		o.apply(e)
		o.version++
	}
	return o, nil
}

// raise applies an event to state and stages it for persistence.
func (o *Order) raise(e DomainEvent) {
	o.apply(e)
	o.version++
	o.changes = append(o.changes, e)
}

// apply mutates state from an event. It must be exhaustive and side-effect free.
func (o *Order) apply(e DomainEvent) {
	switch ev := e.(type) {
	case OrderPlaced:
		o.id = ev.OrderID
		o.customerID = ev.CustomerID
		o.items = ev.Items
		o.shipTo = ev.ShipTo
		o.total = ev.Total
		o.status = StatusPending
	}
}

// UncommittedChanges returns events staged since load or creation.
func (o *Order) UncommittedChanges() []DomainEvent { return o.changes }

// MarkChangesCommitted clears staged events after a successful persist.
func (o *Order) MarkChangesCommitted() { o.changes = nil }

func (o *Order) ID() OrderID          { return o.id }
func (o *Order) CustomerID() CustomerID { return o.customerID }
func (o *Order) Items() []LineItem    { return o.items }
func (o *Order) ShipTo() Address      { return o.shipTo }
func (o *Order) Total() Money         { return o.total }
func (o *Order) Status() Status       { return o.status }
func (o *Order) Version() int64       { return o.version }
