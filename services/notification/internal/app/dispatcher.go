package app

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/google/uuid"

	"github.com/orderfulfillment/notification/internal/contracts"
	"github.com/orderfulfillment/notification/internal/domain/notification"
	"github.com/orderfulfillment/notification/internal/infra/deadletter"
	"github.com/orderfulfillment/notification/internal/infra/metrics"
)

// Dispatcher turns an integration event into a notification and delivers it. It
// is idempotent: the store dedupes on (event type, order id), so a redelivered
// event is recorded once and never resent. Event types this service does not
// notify on are ignored.
type Dispatcher struct {
	store  NotificationStore
	sender Sender
	clock  Clock
	logger *slog.Logger
}

func NewDispatcher(store NotificationStore, sender Sender, clock Clock, logger *slog.Logger) *Dispatcher {
	return &Dispatcher{store: store, sender: sender, clock: clock, logger: logger}
}

// message is the rendered content for one event.
type message struct {
	orderID uuid.UUID
	subject string
	body    string
}

// Handle renders and delivers a notification for a supported event, skipping
// unrecognized types and duplicates.
func (d *Dispatcher) Handle(ctx context.Context, eventType string, payload []byte) error {
	msg, ok, err := render(eventType, payload)
	if err != nil {
		return deadletter.Permanent(err)
	}
	if !ok {
		return nil
	}

	n := notification.New(
		uuid.New(),
		eventType+":"+msg.orderID.String(),
		eventType,
		msg.orderID,
		notification.ChannelEmail,
		recipientFor(msg.orderID),
		msg.subject,
		msg.body,
		d.clock.Now(),
	)

	isNew, err := d.store.RecordIfNew(ctx, n)
	if err != nil {
		return fmt.Errorf("record notification: %w", err)
	}
	if !isNew {
		d.logger.InfoContext(ctx, "duplicate notification skipped",
			slog.String("event_type", eventType), slog.String("order_id", msg.orderID.String()))
		return nil
	}
	if err := d.sender.Send(ctx, n); err != nil {
		return err
	}
	metrics.NotificationsSent.WithLabelValues(eventType).Inc()
	return nil
}

// recipientFor stubs recipient resolution: a real system would look up the
// customer's contact from a profile keyed on the order.
func recipientFor(orderID uuid.UUID) string {
	return "notify+" + orderID.String() + "@example.com"
}

func render(eventType string, payload []byte) (message, bool, error) {
	switch eventType {
	case contracts.TypeOrderPlaced:
		var e contracts.OrderPlaced
		if err := contracts.Unmarshal(payload, &e); err != nil {
			return message{}, false, err
		}
		id, err := parseOrder(e.OrderID)
		if err != nil {
			return message{}, false, err
		}
		return message{id, "Order received",
			fmt.Sprintf("We received your order %s. Total %s.", id, money(e.TotalMinor, e.Currency))}, true, nil

	case contracts.TypeOrderConfirmed:
		id, err := decodeOrderID(payload)
		if err != nil {
			return message{}, false, err
		}
		return message{id, "Order confirmed",
			fmt.Sprintf("Your order %s is confirmed and being prepared.", id)}, true, nil

	case contracts.TypeOrderCancelled:
		var e contracts.OrderCancelled
		if err := contracts.Unmarshal(payload, &e); err != nil {
			return message{}, false, err
		}
		id, err := parseOrder(e.OrderID)
		if err != nil {
			return message{}, false, err
		}
		return message{id, "Order cancelled",
			fmt.Sprintf("Your order %s was cancelled.%s", id, reasonSuffix(e.Reason))}, true, nil

	case contracts.TypePaymentCaptured, contracts.TypePaymentDeclined, contracts.TypePaymentSettled,
		contracts.TypePaymentRefunded, contracts.TypePaymentFailed:
		var e contracts.PaymentEvent
		if err := contracts.Unmarshal(payload, &e); err != nil {
			return message{}, false, err
		}
		id, err := parseOrder(e.OrderID)
		if err != nil {
			return message{}, false, err
		}
		subject, body := paymentMessage(eventType, id, e)
		return message{id, subject, body}, true, nil

	case contracts.TypeShipmentCreated:
		id, err := decodeOrderID(payload)
		if err != nil {
			return message{}, false, err
		}
		return message{id, "Preparing your shipment",
			fmt.Sprintf("A shipment was created for order %s.", id)}, true, nil

	case contracts.TypeShipmentDispatched:
		var e contracts.ShipmentDispatched
		if err := contracts.Unmarshal(payload, &e); err != nil {
			return message{}, false, err
		}
		id, err := parseOrder(e.OrderID)
		if err != nil {
			return message{}, false, err
		}
		return message{id, "Your order shipped",
			fmt.Sprintf("Order %s shipped via %s (tracking %s).", id, e.Carrier, e.TrackingCode)}, true, nil

	case contracts.TypeShipmentDelivered:
		id, err := decodeOrderID(payload)
		if err != nil {
			return message{}, false, err
		}
		return message{id, "Your order was delivered",
			fmt.Sprintf("Order %s has been delivered.", id)}, true, nil

	case contracts.TypeShipmentFailed:
		var e contracts.ShipmentFailed
		if err := contracts.Unmarshal(payload, &e); err != nil {
			return message{}, false, err
		}
		id, err := parseOrder(e.OrderID)
		if err != nil {
			return message{}, false, err
		}
		return message{id, "There was a problem with your shipment",
			fmt.Sprintf("The shipment for order %s failed.%s", id, reasonSuffix(e.Reason))}, true, nil

	default:
		return message{}, false, nil
	}
}

func paymentMessage(eventType string, id uuid.UUID, e contracts.PaymentEvent) (string, string) {
	amount := money(e.AmountMinor, e.Currency)
	switch eventType {
	case contracts.TypePaymentCaptured:
		return "Payment received", fmt.Sprintf("We captured %s for order %s.", amount, id)
	case contracts.TypePaymentDeclined:
		return "Payment declined", fmt.Sprintf("Payment for order %s was declined.", id)
	case contracts.TypePaymentSettled:
		return "Payment settled", fmt.Sprintf("Payment of %s for order %s has settled.", amount, id)
	case contracts.TypePaymentRefunded:
		return "Payment refunded", fmt.Sprintf("We refunded %s for order %s.", amount, id)
	default: // TypePaymentFailed
		return "Payment failed", fmt.Sprintf("Payment for order %s failed.", id)
	}
}

// decodeOrderID handles events where only the order id is needed.
func decodeOrderID(payload []byte) (uuid.UUID, error) {
	var e contracts.OrderRef
	if err := contracts.Unmarshal(payload, &e); err != nil {
		return uuid.Nil, err
	}
	return parseOrder(e.OrderID)
}

func parseOrder(raw string) (uuid.UUID, error) {
	id, err := uuid.Parse(raw)
	if err != nil {
		return uuid.Nil, fmt.Errorf("parse orderId %q: %w", raw, err)
	}
	return id, nil
}

// money renders minor units as major units with the currency, for example
// 1299 USD as "12.99 USD".
func money(minor int64, currency string) string {
	whole := minor / 100
	frac := minor % 100
	if frac < 0 {
		frac = -frac
	}
	out := fmt.Sprintf("%d.%02d", whole, frac)
	if c := strings.TrimSpace(currency); c != "" {
		out += " " + c
	}
	return out
}

func reasonSuffix(reason string) string {
	if r := strings.TrimSpace(reason); r != "" {
		return " Reason: " + r + "."
	}
	return ""
}
