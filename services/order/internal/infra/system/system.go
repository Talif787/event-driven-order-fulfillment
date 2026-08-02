package system

import (
	"time"

	"github.com/orderfulfillment/order/internal/domain/order"
)

// Clock is the production wall-clock implementation.
type Clock struct{}

func (Clock) Now() time.Time { return time.Now().UTC() }

// IDGenerator produces order identifiers backed by random UUIDs.
type IDGenerator struct{}

func (IDGenerator) NewOrderID() order.OrderID { return order.NewOrderID() }
