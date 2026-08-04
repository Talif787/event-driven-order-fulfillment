// Package contracts defines minimal views of the integration events this
// service consumes across the order, payment, and fulfillment streams. Each
// struct captures only the fields the notifier needs. Because Go modules cannot
// share internal packages, these views are kept in sync with the producing
// services by the shared wire format.
package contracts

import (
	"encoding/json"
	"fmt"
)

// HeaderEventType is the Kafka header key that carries the event type.
const HeaderEventType = "event-type"

// Consumed topics.
const (
	TopicOrderEvents       = "orders.events"
	TopicPaymentEvents     = "payments.events"
	TopicFulfillmentEvents = "fulfillment.events"
)

// Event type strings across all three streams.
const (
	TypeOrderPlaced    = "order.placed.v1"
	TypeOrderConfirmed = "order.confirmed.v1"
	TypeOrderCancelled = "order.cancelled.v1"

	TypePaymentCaptured = "payment.captured.v1"
	TypePaymentDeclined = "payment.declined.v1"
	TypePaymentSettled  = "payment.settled.v1"
	TypePaymentRefunded = "payment.refunded.v1"
	TypePaymentFailed   = "payment.failed.v1"

	TypeShipmentCreated    = "shipment.created.v1"
	TypeShipmentDispatched = "shipment.dispatched.v1"
	TypeShipmentDelivered  = "shipment.delivered.v1"
	TypeShipmentFailed     = "shipment.failed.v1"
)

// OrderPlaced is the minimal view of order.placed.v1.
type OrderPlaced struct {
	OrderID    string `json:"orderId"`
	CustomerID string `json:"customerId"`
	TotalMinor int64  `json:"totalMinor"`
	Currency   string `json:"currency"`
}

// OrderConfirmed is the minimal view of order.confirmed.v1.
type OrderConfirmed struct {
	OrderID string `json:"orderId"`
}

// OrderCancelled is the minimal view of order.cancelled.v1.
type OrderCancelled struct {
	OrderID string `json:"orderId"`
	Reason  string `json:"reason"`
}

// PaymentEvent is the shared shape of every payments.events record; the event
// type header distinguishes captured, declined, settled, refunded, and failed.
type PaymentEvent struct {
	OrderID     string `json:"orderId"`
	AmountMinor int64  `json:"amountMinor"`
	Currency    string `json:"currency"`
	Status      string `json:"status"`
}

// ShipmentDispatched is the minimal view of shipment.dispatched.v1.
type ShipmentDispatched struct {
	OrderID      string `json:"orderId"`
	Carrier      string `json:"carrier"`
	TrackingCode string `json:"trackingCode"`
}

// ShipmentFailed is the minimal view of shipment.failed.v1.
type ShipmentFailed struct {
	OrderID string `json:"orderId"`
	Reason  string `json:"reason"`
}

// OrderRef is the minimal view for events where only the order id is needed
// (shipment.created and shipment.delivered).
type OrderRef struct {
	OrderID string `json:"orderId"`
}

// Unmarshal decodes an event body into the provided view.
func Unmarshal(raw []byte, v any) error {
	if err := json.Unmarshal(raw, v); err != nil {
		return fmt.Errorf("unmarshal event: %w", err)
	}
	return nil
}
