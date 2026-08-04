package app

import (
	"context"
	"time"

	"github.com/orderfulfillment/notification/internal/domain/notification"
)

// NotificationStore persists notifications idempotently.
type NotificationStore interface {
	// RecordIfNew stores the notification, returning true if it was newly
	// inserted and false if its dedupe key already existed.
	RecordIfNew(ctx context.Context, n *notification.Notification) (bool, error)
}

// Sender delivers a notification. The local implementation logs it (a simulated
// send); a real one would call an email or SMS provider.
type Sender interface {
	Send(ctx context.Context, n *notification.Notification) error
}

// Clock supplies the current time, injected so tests are deterministic.
type Clock interface {
	Now() time.Time
}

// SystemClock is the production clock.
type SystemClock struct{}

func (SystemClock) Now() time.Time { return time.Now().UTC() }
