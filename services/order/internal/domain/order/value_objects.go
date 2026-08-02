package order

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// OrderID identifies an order aggregate.
type OrderID struct{ value uuid.UUID }

func NewOrderID() OrderID { return OrderID{value: uuid.New()} }

func ParseOrderID(s string) (OrderID, error) {
	v, err := uuid.Parse(s)
	if err != nil {
		return OrderID{}, fmt.Errorf("%w: order id %q", ErrInvalidIdentifier, s)
	}
	return OrderID{value: v}, nil
}

func (id OrderID) String() string { return id.value.String() }
func (id OrderID) IsZero() bool   { return id.value == uuid.Nil }

// CustomerID identifies the customer placing the order.
type CustomerID struct{ value uuid.UUID }

func ParseCustomerID(s string) (CustomerID, error) {
	v, err := uuid.Parse(s)
	if err != nil {
		return CustomerID{}, fmt.Errorf("%w: customer id %q", ErrInvalidIdentifier, s)
	}
	return CustomerID{value: v}, nil
}

func (id CustomerID) String() string { return id.value.String() }

// SKU is a validated stock keeping unit.
type SKU struct{ value string }

func NewSKU(s string) (SKU, error) {
	s = strings.TrimSpace(s)
	if s == "" || len(s) > 64 {
		return SKU{}, fmt.Errorf("%w: sku must be 1..64 chars", ErrValidation)
	}
	return SKU{value: s}, nil
}

func (s SKU) String() string { return s.value }

// Quantity is a positive order line quantity.
type Quantity struct{ value int32 }

func NewQuantity(v int32) (Quantity, error) {
	if v <= 0 || v > 10_000 {
		return Quantity{}, fmt.Errorf("%w: quantity must be 1..10000", ErrValidation)
	}
	return Quantity{value: v}, nil
}

func (q Quantity) Value() int32 { return q.value }

// Money holds a currency and an integer amount in minor units to avoid float error.
type Money struct {
	Currency   string
	MinorUnits int64
}

func NewMoney(currency string, minorUnits int64) (Money, error) {
	currency = strings.ToUpper(strings.TrimSpace(currency))
	if len(currency) != 3 {
		return Money{}, fmt.Errorf("%w: currency must be ISO 4217 alpha-3", ErrValidation)
	}
	if minorUnits < 0 {
		return Money{}, fmt.Errorf("%w: amount must be non-negative", ErrValidation)
	}
	return Money{Currency: currency, MinorUnits: minorUnits}, nil
}

func (m Money) Add(other Money) (Money, error) {
	if m.Currency != other.Currency {
		return Money{}, fmt.Errorf("%w: currency mismatch %s vs %s", ErrValidation, m.Currency, other.Currency)
	}
	return Money{Currency: m.Currency, MinorUnits: m.MinorUnits + other.MinorUnits}, nil
}

// LineItem is one line of an order.
type LineItem struct {
	SKU       SKU
	Quantity  Quantity
	UnitPrice Money
}

func (li LineItem) LineTotal() Money {
	return Money{Currency: li.UnitPrice.Currency, MinorUnits: li.UnitPrice.MinorUnits * int64(li.Quantity.Value())}
}

// Address is a validated shipping address.
type Address struct {
	Line1      string
	Line2      string
	City       string
	Region     string
	PostalCode string
	Country    string
}

func NewAddress(line1, line2, city, region, postalCode, country string) (Address, error) {
	required := map[string]string{"line1": line1, "city": city, "postalCode": postalCode, "country": country}
	for field, v := range required {
		if strings.TrimSpace(v) == "" {
			return Address{}, fmt.Errorf("%w: address %s is required", ErrValidation, field)
		}
	}
	country = strings.ToUpper(strings.TrimSpace(country))
	if len(country) != 2 {
		return Address{}, fmt.Errorf("%w: country must be ISO 3166-1 alpha-2", ErrValidation)
	}
	return Address{
		Line1: strings.TrimSpace(line1), Line2: strings.TrimSpace(line2),
		City: strings.TrimSpace(city), Region: strings.TrimSpace(region),
		PostalCode: strings.TrimSpace(postalCode), Country: country,
	}, nil
}

// Status is the order lifecycle state.
type Status string

const (
	StatusPending   Status = "PENDING"
	StatusConfirmed Status = "CONFIRMED"
	StatusCancelled Status = "CANCELLED"
)

// IdempotencyKey deduplicates client submissions.
type IdempotencyKey struct{ value string }

func NewIdempotencyKey(s string) (IdempotencyKey, error) {
	s = strings.TrimSpace(s)
	if len(s) < 8 || len(s) > 128 {
		return IdempotencyKey{}, fmt.Errorf("%w: idempotency key must be 8..128 chars", ErrValidation)
	}
	return IdempotencyKey{value: s}, nil
}

func (k IdempotencyKey) String() string { return k.value }

// Clock abstracts time for deterministic tests.
type Clock interface{ Now() time.Time }
