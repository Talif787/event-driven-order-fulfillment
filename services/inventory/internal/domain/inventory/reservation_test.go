package inventory

import (
	"errors"
	"testing"

	"github.com/google/uuid"
)

func lines(t *testing.T) []ReservationLine {
	t.Helper()
	qty, err := NewQuantity(2)
	if err != nil {
		t.Fatalf("NewQuantity: %v", err)
	}
	return []ReservationLine{{SKU: mustSKU(t, "SKU-1"), Quantity: qty}}
}

func newHeld(t *testing.T) *Reservation {
	t.Helper()
	oid, err := ParseOrderID(uuid.NewString())
	if err != nil {
		t.Fatalf("ParseOrderID: %v", err)
	}
	res, err := NewReservation(NewReservationID(), oid, lines(t))
	if err != nil {
		t.Fatalf("NewReservation: %v", err)
	}
	return res
}

func TestNewReservationRejectsEmpty(t *testing.T) {
	oid, _ := ParseOrderID(uuid.NewString())
	if _, err := NewReservation(NewReservationID(), oid, nil); !errors.Is(err, ErrValidation) {
		t.Fatalf("want ErrValidation, got %v", err)
	}
}

func TestNewReservationRejectsDuplicateSKU(t *testing.T) {
	oid, _ := ParseOrderID(uuid.NewString())
	qty, _ := NewQuantity(1)
	dup := []ReservationLine{
		{SKU: mustSKU(t, "SKU-1"), Quantity: qty},
		{SKU: mustSKU(t, "SKU-1"), Quantity: qty},
	}
	if _, err := NewReservation(NewReservationID(), oid, dup); !errors.Is(err, ErrValidation) {
		t.Fatalf("want ErrValidation, got %v", err)
	}
}

func TestNewReservationStartsHeld(t *testing.T) {
	if got := newHeld(t).Status(); got != StatusHeld {
		t.Fatalf("want HELD, got %s", got)
	}
}

func TestReleaseTransitionAndIdempotency(t *testing.T) {
	res := newHeld(t)
	changed, err := res.Release()
	if err != nil || !changed || res.Status() != StatusReleased {
		t.Fatalf("first release: changed=%v err=%v status=%s", changed, err, res.Status())
	}
	changed, err = res.Release()
	if err != nil || changed {
		t.Fatalf("second release should be idempotent no-op: changed=%v err=%v", changed, err)
	}
}

func TestCommitTransitionAndIdempotency(t *testing.T) {
	res := newHeld(t)
	changed, err := res.Commit()
	if err != nil || !changed || res.Status() != StatusCommitted {
		t.Fatalf("first commit: changed=%v err=%v status=%s", changed, err, res.Status())
	}
	changed, err = res.Commit()
	if err != nil || changed {
		t.Fatalf("second commit should be idempotent no-op: changed=%v err=%v", changed, err)
	}
}

func TestReleaseAfterCommitIsInvalid(t *testing.T) {
	res := newHeld(t)
	if _, err := res.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}
	if _, err := res.Release(); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("want ErrInvalidTransition, got %v", err)
	}
}

func TestCommitAfterReleaseIsInvalid(t *testing.T) {
	res := newHeld(t)
	if _, err := res.Release(); err != nil {
		t.Fatalf("release: %v", err)
	}
	if _, err := res.Commit(); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("want ErrInvalidTransition, got %v", err)
	}
}
