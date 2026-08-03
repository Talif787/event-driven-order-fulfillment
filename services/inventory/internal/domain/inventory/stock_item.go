package inventory

import "fmt"

// StockItem is the stock ledger for one SKU. Available is sellable stock;
// Reserved is stock held for orders that are in flight but not yet shipped.
// Version drives optimistic concurrency: the persistence layer increments it on
// every write and rejects a write made against a stale version.
type StockItem struct {
	sku       SKU
	available int64
	reserved  int64
	version   int64
}

// NewStockItem creates a fresh ledger entry with the given available quantity.
func NewStockItem(sku SKU, available int64) (*StockItem, error) {
	if available < 0 {
		return nil, fmt.Errorf("%w: available must be non-negative", ErrValidation)
	}
	return &StockItem{sku: sku, available: available}, nil
}

// RehydrateStockItem rebuilds a StockItem from persisted state.
func RehydrateStockItem(sku SKU, available, reserved, version int64) *StockItem {
	return &StockItem{sku: sku, available: available, reserved: reserved, version: version}
}

func (s *StockItem) SKU() SKU         { return s.sku }
func (s *StockItem) Available() int64 { return s.available }
func (s *StockItem) Reserved() int64  { return s.reserved }
func (s *StockItem) Version() int64   { return s.version }

// Reserve holds qty units: available decreases, reserved increases. It fails
// when there is not enough available stock.
func (s *StockItem) Reserve(qty int64) error {
	if qty <= 0 {
		return fmt.Errorf("%w: reserve quantity must be positive", ErrValidation)
	}
	if qty > s.available {
		return fmt.Errorf("%w: sku %s wants %d, has %d available", ErrInsufficientStock, s.sku, qty, s.available)
	}
	s.available -= qty
	s.reserved += qty
	return nil
}

// Release returns qty held units to available. This is the compensation for a
// reservation that is being cancelled.
func (s *StockItem) Release(qty int64) error {
	if qty <= 0 {
		return fmt.Errorf("%w: release quantity must be positive", ErrValidation)
	}
	if qty > s.reserved {
		return fmt.Errorf("%w: release %d exceeds reserved %d", ErrValidation, qty, s.reserved)
	}
	s.reserved -= qty
	s.available += qty
	return nil
}

// Commit finalizes qty held units: the stock leaves the warehouse, so reserved
// decreases and available is unchanged.
func (s *StockItem) Commit(qty int64) error {
	if qty <= 0 {
		return fmt.Errorf("%w: commit quantity must be positive", ErrValidation)
	}
	if qty > s.reserved {
		return fmt.Errorf("%w: commit %d exceeds reserved %d", ErrValidation, qty, s.reserved)
	}
	s.reserved -= qty
	return nil
}
