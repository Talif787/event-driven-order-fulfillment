package fulfillment

// ShipmentStatus is the lifecycle state of a shipment.
//
// A shipment is CREATED when its order is confirmed, DISPATCHED once a carrier
// takes it, and DELIVERED on arrival. FAILED is a terminal failure (for example
// a carrier rejection). DELIVERED and FAILED are terminal.
type ShipmentStatus string

const (
	StatusCreated    ShipmentStatus = "CREATED"
	StatusDispatched ShipmentStatus = "DISPATCHED"
	StatusDelivered  ShipmentStatus = "DELIVERED"
	StatusFailed     ShipmentStatus = "FAILED"
)
