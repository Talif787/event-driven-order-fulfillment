// Package contracts defines the versioned integration event schemas exchanged
// over Kafka. Producer (outbox) and consumers (projections, sagas) share these
// types so the wire format has a single source of truth. When the schema
// registry lands, these types become the registered subjects.
package contracts

import (
	"encoding/json"
	"fmt"
	"time"
)

// Topic and event type constants for the order stream.
const (
	TopicOrderEvents = "orders.events"
	TypeOrderPlaced  = "order.placed.v1"

	// HeaderEventType and HeaderContentType are the Kafka header keys carried
	// on every record so consumers can route without decoding the body.
	HeaderEventType   = "event-type"
	HeaderContentType = "content-type"
	HeaderEventID     = "event-id"

	// timeLayout is the millisecond-precision RFC 3339 layout used on the wire.
	timeLayout = "2006-01-02T15:04:05.000Z07:00"
)

// OrderPlacedV1 is the integration event emitted when an order is accepted.
type OrderPlacedV1 struct {
	OrderID    string      `json:"orderId"`
	CustomerID string      `json:"customerId"`
	Items      []LineItem  `json:"items"`
	ShipTo     Address     `json:"shipTo"`
	TotalMinor int64       `json:"totalMinor"`
	Currency   string      `json:"currency"`
	PlacedAt   string      `json:"placedAt"`
}

// LineItem is one ordered line on the wire.
type LineItem struct {
	SKU            string `json:"sku"`
	Quantity       int32  `json:"quantity"`
	UnitPriceMinor int64  `json:"unitPriceMinor"`
}

// Address is the shipping address on the wire.
type Address struct {
	Line1      string `json:"line1"`
	Line2      string `json:"line2"`
	City       string `json:"city"`
	Region     string `json:"region"`
	PostalCode string `json:"postalCode"`
	Country    string `json:"country"`
}

// FormatTime renders a timestamp in the canonical wire layout.
func FormatTime(t time.Time) string { return t.UTC().Format(timeLayout) }

// ParsePlacedAt parses the PlacedAt field back into a time.Time.
func (e OrderPlacedV1) ParsePlacedAt() (time.Time, error) {
	t, err := time.Parse(timeLayout, e.PlacedAt)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse placedAt %q: %w", e.PlacedAt, err)
	}
	return t, nil
}

// Marshal serializes the event body.
func (e OrderPlacedV1) Marshal() ([]byte, error) {
	raw, err := json.Marshal(e)
	if err != nil {
		return nil, fmt.Errorf("marshal order.placed.v1: %w", err)
	}
	return raw, nil
}

// UnmarshalOrderPlaced deserializes an order.placed.v1 body.
func UnmarshalOrderPlaced(raw []byte) (OrderPlacedV1, error) {
	var e OrderPlacedV1
	if err := json.Unmarshal(raw, &e); err != nil {
		return OrderPlacedV1{}, fmt.Errorf("unmarshal order.placed.v1: %w", err)
	}
	return e, nil
}
