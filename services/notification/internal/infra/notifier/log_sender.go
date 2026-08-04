// Package notifier holds notification delivery adapters. LogSender is a
// simulated channel: it logs the rendered notification instead of calling an
// external provider, which is enough to demonstrate the fan-in end to end.
package notifier

import (
	"context"
	"log/slog"

	"github.com/orderfulfillment/notification/internal/domain/notification"
)

type LogSender struct{ logger *slog.Logger }

func NewLogSender(logger *slog.Logger) *LogSender { return &LogSender{logger: logger} }

func (s *LogSender) Send(ctx context.Context, n *notification.Notification) error {
	s.logger.InfoContext(ctx, "notification sent",
		slog.String("channel", string(n.Channel())),
		slog.String("recipient", n.Recipient()),
		slog.String("event_type", n.EventType()),
		slog.String("order_id", n.OrderID().String()),
		slog.String("subject", n.Subject()),
		slog.String("body", n.Body()),
	)
	return nil
}
