// Package payment provides a stub payment gateway for the saga. It stands in for
// the real Payment service (Phase 4) so the saga runs and demonstrates both the
// success and the compensation paths. The outcome is configurable so the
// decline path can be exercised on demand.
package payment

import (
	"context"
	"strings"

	"github.com/google/uuid"

	"github.com/orderfulfillment/order/internal/app/saga"
)

// StubGateway approves every authorization, or declines every one when
// configured with outcome "decline".
type StubGateway struct{ decline bool }

// NewStubGateway builds the stub. Any outcome other than "decline" approves.
func NewStubGateway(outcome string) *StubGateway {
	return &StubGateway{decline: strings.EqualFold(strings.TrimSpace(outcome), "decline")}
}

// Authorize returns an approval with a synthetic reference, or a clean decline.
// It never returns a transport error, so the saga treats its result as final.
func (g *StubGateway) Authorize(_ context.Context, _ saga.PaymentRequest) (saga.PaymentResult, error) {
	if g.decline {
		return saga.PaymentResult{Approved: false, Decline: "stub gateway configured to decline"}, nil
	}
	return saga.PaymentResult{Approved: true, Reference: "stub-" + uuid.NewString()}, nil
}
