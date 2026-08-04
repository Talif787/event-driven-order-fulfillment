// Package notification holds the notification record: one rendered, deliverable
// message produced from an integration event.
package notification

import (
	"time"

	"github.com/google/uuid"
)

// Channel is the delivery medium. Only email is modelled here; adding SMS or
// push is a matter of another constant and another sender.
type Channel string

const ChannelEmail Channel = "EMAIL"

// Notification is a rendered message ready for (simulated) delivery. DedupeKey
// is the natural idempotency key, so the same logical event never notifies
// twice.
type Notification struct {
	id        uuid.UUID
	dedupeKey string
	eventType string
	orderID   uuid.UUID
	channel   Channel
	recipient string
	subject   string
	body      string
	createdAt time.Time
}

func New(id uuid.UUID, dedupeKey, eventType string, orderID uuid.UUID,
	channel Channel, recipient, subject, body string, createdAt time.Time) *Notification {
	return &Notification{
		id: id, dedupeKey: dedupeKey, eventType: eventType, orderID: orderID,
		channel: channel, recipient: recipient, subject: subject, body: body, createdAt: createdAt,
	}
}

func (n *Notification) ID() uuid.UUID        { return n.id }
func (n *Notification) DedupeKey() string    { return n.dedupeKey }
func (n *Notification) EventType() string    { return n.eventType }
func (n *Notification) OrderID() uuid.UUID   { return n.orderID }
func (n *Notification) Channel() Channel     { return n.channel }
func (n *Notification) Recipient() string    { return n.recipient }
func (n *Notification) Subject() string      { return n.subject }
func (n *Notification) Body() string         { return n.body }
func (n *Notification) CreatedAt() time.Time { return n.createdAt }
