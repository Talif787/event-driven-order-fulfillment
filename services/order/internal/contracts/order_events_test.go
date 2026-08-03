package contracts

import (
	"testing"
	"time"
)

func TestOrderPlacedV1_RoundTrip(t *testing.T) {
	now := time.Date(2026, 8, 2, 12, 30, 15, 500_000_000, time.UTC)
	original := OrderPlacedV1{
		OrderID:    "11111111-1111-1111-1111-111111111111",
		CustomerID: "22222222-2222-2222-2222-222222222222",
		Items:      []LineItem{{SKU: "SKU-1", Quantity: 2, UnitPriceMinor: 1500}},
		ShipTo:     Address{Line1: "1 Main St", City: "Boston", Region: "MA", PostalCode: "02118", Country: "US"},
		TotalMinor: 3000,
		Currency:   "USD",
		PlacedAt:   FormatTime(now),
	}

	raw, err := original.Marshal()
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	decoded, err := UnmarshalOrderPlaced(raw)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decoded.OrderID != original.OrderID || decoded.TotalMinor != 3000 || decoded.Currency != "USD" {
		t.Fatalf("scalar round trip mismatch: %+v", decoded)
	}
	if len(decoded.Items) != 1 || decoded.Items[0].SKU != "SKU-1" || decoded.Items[0].Quantity != 2 {
		t.Fatalf("items round trip mismatch: %+v", decoded.Items)
	}
	if decoded.ShipTo.City != "Boston" || decoded.ShipTo.Country != "US" {
		t.Fatalf("address round trip mismatch: %+v", decoded.ShipTo)
	}

	parsed, err := decoded.ParsePlacedAt()
	if err != nil {
		t.Fatalf("parse placedAt: %v", err)
	}
	if !parsed.Equal(now) {
		t.Fatalf("placedAt round trip mismatch: got %s want %s", parsed, now)
	}
}
