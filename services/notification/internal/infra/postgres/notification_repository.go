package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/orderfulfillment/notification/internal/domain/notification"
)

// NotificationRepository is the pgx-backed notification store.
type NotificationRepository struct{ pool *pgxpool.Pool }

func NewNotificationRepository(pool *pgxpool.Pool) *NotificationRepository {
	return &NotificationRepository{pool: pool}
}

// RecordIfNew inserts the notification, skipping the write when its dedupe key
// already exists. A returned true means it was newly inserted (and should be
// sent); false means a duplicate that must not be resent.
func (r *NotificationRepository) RecordIfNew(ctx context.Context, n *notification.Notification) (bool, error) {
	const q = `INSERT INTO notifications
	           (id, dedupe_key, event_type, order_id, channel, recipient, subject, body, created_at)
	           VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	           ON CONFLICT (dedupe_key) DO NOTHING`
	tag, err := r.pool.Exec(ctx, q,
		n.ID(), n.DedupeKey(), n.EventType(), n.OrderID(), string(n.Channel()),
		n.Recipient(), n.Subject(), n.Body(), n.CreatedAt())
	if err != nil {
		return false, fmt.Errorf("insert notification: %w", err)
	}
	return tag.RowsAffected() == 1, nil
}
