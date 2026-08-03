package postgres

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/orderfulfillment/order/internal/domain/order"
)

// storedOrderPlaced is the persisted representation of the OrderPlaced event.
type storedOrderPlaced struct {
	OrderID    string            `json:"orderId"`
	CustomerID string            `json:"customerId"`
	Items      []storedLineItem  `json:"items"`
	ShipTo     storedAddress     `json:"shipTo"`
	TotalMinor int64             `json:"totalMinor"`
	Currency   string            `json:"currency"`
	PlacedAt   time.Time         `json:"placedAt"`
}

type storedLineItem struct {
	SKU            string `json:"sku"`
	Quantity       int32  `json:"quantity"`
	UnitPriceMinor int64  `json:"unitPriceMinor"`
	Currency       string `json:"currency"`
}

type storedAddress struct {
	Line1, Line2, City, Region, PostalCode, Country string
}

type storedOrderConfirmed struct {
	OrderID     string    `json:"orderId"`
	ConfirmedAt time.Time `json:"confirmedAt"`
}

type storedOrderCancelled struct {
	OrderID     string    `json:"orderId"`
	Reason      string    `json:"reason"`
	CancelledAt time.Time `json:"cancelledAt"`
}

func encodeEvent(e order.DomainEvent) (string, []byte, error) {
	switch ev := e.(type) {
	case order.OrderPlaced:
		items := make([]storedLineItem, 0, len(ev.Items))
		for _, it := range ev.Items {
			items = append(items, storedLineItem{
				SKU: it.SKU.String(), Quantity: it.Quantity.Value(),
				UnitPriceMinor: it.UnitPrice.MinorUnits, Currency: it.UnitPrice.Currency,
			})
		}
		payload := storedOrderPlaced{
			OrderID: ev.OrderID.String(), CustomerID: ev.CustomerID.String(), Items: items,
			ShipTo: storedAddress{
				Line1: ev.ShipTo.Line1, Line2: ev.ShipTo.Line2, City: ev.ShipTo.City,
				Region: ev.ShipTo.Region, PostalCode: ev.ShipTo.PostalCode, Country: ev.ShipTo.Country,
			},
			TotalMinor: ev.Total.MinorUnits, Currency: ev.Total.Currency, PlacedAt: ev.PlacedAt,
		}
		raw, err := json.Marshal(payload)
		if err != nil {
			return "", nil, fmt.Errorf("marshal order.placed: %w", err)
		}
		return string(order.OrderPlacedType), raw, nil
	case order.OrderConfirmed:
		raw, err := json.Marshal(storedOrderConfirmed{OrderID: ev.OrderID.String(), ConfirmedAt: ev.ConfirmedAt})
		if err != nil {
			return "", nil, fmt.Errorf("marshal order.confirmed: %w", err)
		}
		return string(order.OrderConfirmedType), raw, nil
	case order.OrderCancelled:
		raw, err := json.Marshal(storedOrderCancelled{OrderID: ev.OrderID.String(), Reason: ev.Reason, CancelledAt: ev.CancelledAt})
		if err != nil {
			return "", nil, fmt.Errorf("marshal order.cancelled: %w", err)
		}
		return string(order.OrderCancelledType), raw, nil
	default:
		return "", nil, fmt.Errorf("unknown event type %T", e)
	}
}

func decodeEvent(eventType string, raw []byte) (order.DomainEvent, error) {
	switch order.EventType(eventType) {
	case order.OrderPlacedType:
		var p storedOrderPlaced
		if err := json.Unmarshal(raw, &p); err != nil {
			return nil, fmt.Errorf("unmarshal order.placed: %w", err)
		}
		orderID, err := order.ParseOrderID(p.OrderID)
		if err != nil {
			return nil, err
		}
		customerID, err := order.ParseCustomerID(p.CustomerID)
		if err != nil {
			return nil, err
		}
		items := make([]order.LineItem, 0, len(p.Items))
		for _, it := range p.Items {
			sku, err := order.NewSKU(it.SKU)
			if err != nil {
				return nil, err
			}
			qty, err := order.NewQuantity(it.Quantity)
			if err != nil {
				return nil, err
			}
			price, err := order.NewMoney(it.Currency, it.UnitPriceMinor)
			if err != nil {
				return nil, err
			}
			items = append(items, order.LineItem{SKU: sku, Quantity: qty, UnitPrice: price})
		}
		addr, err := order.NewAddress(p.ShipTo.Line1, p.ShipTo.Line2, p.ShipTo.City, p.ShipTo.Region, p.ShipTo.PostalCode, p.ShipTo.Country)
		if err != nil {
			return nil, err
		}
		total, err := order.NewMoney(p.Currency, p.TotalMinor)
		if err != nil {
			return nil, err
		}
		return order.OrderPlaced{
			OrderID: orderID, CustomerID: customerID, Items: items,
			ShipTo: addr, Total: total, PlacedAt: p.PlacedAt,
		}, nil
	case order.OrderConfirmedType:
		var p storedOrderConfirmed
		if err := json.Unmarshal(raw, &p); err != nil {
			return nil, fmt.Errorf("unmarshal order.confirmed: %w", err)
		}
		orderID, err := order.ParseOrderID(p.OrderID)
		if err != nil {
			return nil, err
		}
		return order.OrderConfirmed{OrderID: orderID, ConfirmedAt: p.ConfirmedAt}, nil
	case order.OrderCancelledType:
		var p storedOrderCancelled
		if err := json.Unmarshal(raw, &p); err != nil {
			return nil, fmt.Errorf("unmarshal order.cancelled: %w", err)
		}
		orderID, err := order.ParseOrderID(p.OrderID)
		if err != nil {
			return nil, err
		}
		return order.OrderCancelled{OrderID: orderID, Reason: p.Reason, CancelledAt: p.CancelledAt}, nil
	default:
		return nil, fmt.Errorf("unknown stored event type %q", eventType)
	}
}
