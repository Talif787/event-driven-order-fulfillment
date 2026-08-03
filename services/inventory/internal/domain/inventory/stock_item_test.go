package inventory

import (
	"errors"
	"testing"
)

func mustSKU(t *testing.T, s string) SKU {
	t.Helper()
	sku, err := NewSKU(s)
	if err != nil {
		t.Fatalf("NewSKU(%q): %v", s, err)
	}
	return sku
}

func TestNewStockItemRejectsNegative(t *testing.T) {
	if _, err := NewStockItem(mustSKU(t, "SKU-1"), -1); !errors.Is(err, ErrValidation) {
		t.Fatalf("want ErrValidation, got %v", err)
	}
}

func TestReserveMovesAvailableToReserved(t *testing.T) {
	item, _ := NewStockItem(mustSKU(t, "SKU-1"), 10)
	if err := item.Reserve(4); err != nil {
		t.Fatalf("Reserve: %v", err)
	}
	if item.Available() != 6 || item.Reserved() != 4 {
		t.Fatalf("want available=6 reserved=4, got available=%d reserved=%d", item.Available(), item.Reserved())
	}
}

func TestReserveInsufficientLeavesStateUnchanged(t *testing.T) {
	item, _ := NewStockItem(mustSKU(t, "SKU-1"), 3)
	err := item.Reserve(5)
	if !errors.Is(err, ErrInsufficientStock) {
		t.Fatalf("want ErrInsufficientStock, got %v", err)
	}
	if item.Available() != 3 || item.Reserved() != 0 {
		t.Fatalf("state changed on failed reserve: available=%d reserved=%d", item.Available(), item.Reserved())
	}
}

func TestReserveRejectsNonPositive(t *testing.T) {
	item, _ := NewStockItem(mustSKU(t, "SKU-1"), 3)
	if err := item.Reserve(0); !errors.Is(err, ErrValidation) {
		t.Fatalf("want ErrValidation, got %v", err)
	}
}

func TestReleaseReturnsReservedToAvailable(t *testing.T) {
	item := RehydrateStockItem(mustSKU(t, "SKU-1"), 6, 4, 2)
	if err := item.Release(3); err != nil {
		t.Fatalf("Release: %v", err)
	}
	if item.Available() != 9 || item.Reserved() != 1 {
		t.Fatalf("want available=9 reserved=1, got available=%d reserved=%d", item.Available(), item.Reserved())
	}
}

func TestReleaseOverReservedFails(t *testing.T) {
	item := RehydrateStockItem(mustSKU(t, "SKU-1"), 6, 4, 2)
	if err := item.Release(5); !errors.Is(err, ErrValidation) {
		t.Fatalf("want ErrValidation, got %v", err)
	}
}

func TestCommitReducesReservedOnly(t *testing.T) {
	item := RehydrateStockItem(mustSKU(t, "SKU-1"), 6, 4, 2)
	if err := item.Commit(4); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if item.Available() != 6 || item.Reserved() != 0 {
		t.Fatalf("want available=6 reserved=0, got available=%d reserved=%d", item.Available(), item.Reserved())
	}
}

func TestCommitOverReservedFails(t *testing.T) {
	item := RehydrateStockItem(mustSKU(t, "SKU-1"), 6, 4, 2)
	if err := item.Commit(5); !errors.Is(err, ErrValidation) {
		t.Fatalf("want ErrValidation, got %v", err)
	}
}
