// Package contracts defines the integration event schemas this service consumes
// and produces. Because Go modules cannot share internal packages, the consumed
// order schema is redeclared here as a minimal view of the fields this service
// needs, kept in sync with the order service by the shared wire format.
package contracts

import (
	"encoding/json"
	"fmt"
	"time"
)

// Kafka header keys and the consumed order stream constants.
const (
	HeaderEventType = "event-type"

	TopicOrderEvents   = "orders.events"
	TypeOrderConfirmed = "order.confirmed.v1"

	// timeLayout is the millisecond-precision RFC 3339 layout used on the wire.
	timeLayout = "2006-01-02T15:04:05.000Z07:00"
)

// Produced fulfillment stream constants.
const (
	TopicFulfillmentEvents = "fulfillment.events"
	TypeShipmentCreated    = "shipment.created.v1"
	TypeShipmentDispatched = "shipment.dispatched.v1"
	TypeShipmentDelivered  = "shipment.delivered.v1"
	TypeShipmentFailed     = "shipment.failed.v1"
)

// FormatTime renders a timestamp in the canonical wire layout.
func FormatTime(t time.Time) string { return t.UTC().Format(timeLayout) }

// OrderConfirmedV1 is the consumed order.confirmed.v1 event (minimal view).
type OrderConfirmedV1 struct {
	OrderID     string `json:"orderId"`
	ConfirmedAt string `json:"confirmedAt"`
}

// UnmarshalOrderConfirmed deserializes an order.confirmed.v1 body.
func UnmarshalOrderConfirmed(raw []byte) (OrderConfirmedV1, error) {
	var e OrderConfirmedV1
	if err := json.Unmarshal(raw, &e); err != nil {
		return OrderConfirmedV1{}, fmt.Errorf("unmarshal order.confirmed.v1: %w", err)
	}
	return e, nil
}

// ShipmentCreatedV1 is emitted when a shipment is opened for a confirmed order.
type ShipmentCreatedV1 struct {
	ShipmentID string `json:"shipmentId"`
	OrderID    string `json:"orderId"`
	Status     string `json:"status"`
	CreatedAt  string `json:"createdAt"`
}

// ShipmentDispatchedV1 is emitted when a carrier takes the shipment.
type ShipmentDispatchedV1 struct {
	ShipmentID   string `json:"shipmentId"`
	OrderID      string `json:"orderId"`
	Carrier      string `json:"carrier"`
	TrackingCode string `json:"trackingCode"`
	DispatchedAt string `json:"dispatchedAt"`
}

// ShipmentDeliveredV1 is emitted when the shipment is delivered.
type ShipmentDeliveredV1 struct {
	ShipmentID  string `json:"shipmentId"`
	OrderID     string `json:"orderId"`
	DeliveredAt string `json:"deliveredAt"`
}

// ShipmentFailedV1 is emitted when a shipment fails.
type ShipmentFailedV1 struct {
	ShipmentID string `json:"shipmentId"`
	OrderID    string `json:"orderId"`
	Reason     string `json:"reason"`
	FailedAt   string `json:"failedAt"`
}

// Marshal helpers return the JSON body for each produced event.
func (e ShipmentCreatedV1) Marshal() ([]byte, error)    { return marshal("shipment.created.v1", e) }
func (e ShipmentDispatchedV1) Marshal() ([]byte, error) { return marshal("shipment.dispatched.v1", e) }
func (e ShipmentDeliveredV1) Marshal() ([]byte, error)  { return marshal("shipment.delivered.v1", e) }
func (e ShipmentFailedV1) Marshal() ([]byte, error)     { return marshal("shipment.failed.v1", e) }

func marshal(name string, e any) ([]byte, error) {
	raw, err := json.Marshal(e)
	if err != nil {
		return nil, fmt.Errorf("marshal %s: %w", name, err)
	}
	return raw, nil
}
