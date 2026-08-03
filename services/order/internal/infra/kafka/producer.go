package kafka

import (
	"context"
	"fmt"

	"github.com/segmentio/kafka-go"

	"github.com/orderfulfillment/order/internal/app"
	"github.com/orderfulfillment/order/internal/contracts"
)

// Publisher writes outbox records to Kafka. It requires acknowledgement from
// all in-sync replicas so the relay only marks a row published after the event
// is durably stored on the broker. Partitioning is by key (aggregate id) so
// events for one aggregate keep their order.
type Publisher struct{ writer *kafka.Writer }

func NewPublisher(brokers []string) *Publisher {
	return &Publisher{writer: &kafka.Writer{
		Addr:                   kafka.TCP(brokers...),
		Balancer:               &kafka.Hash{},
		RequiredAcks:           kafka.RequireAll,
		AllowAutoTopicCreation: true,
	}}
}

// Publish writes all records synchronously. If any write fails the whole call
// returns an error and no record should be treated as published.
func (p *Publisher) Publish(ctx context.Context, records []app.OutboxRecord) error {
	if len(records) == 0 {
		return nil
	}
	messages := make([]kafka.Message, 0, len(records))
	for _, r := range records {
		headers := make([]kafka.Header, 0, len(r.Headers)+2)
		headers = append(headers,
			kafka.Header{Key: contracts.HeaderEventType, Value: []byte(r.EventType)},
			kafka.Header{Key: contracts.HeaderEventID, Value: []byte(r.EventID)},
		)
		for k, v := range r.Headers {
			headers = append(headers, kafka.Header{Key: k, Value: []byte(v)})
		}
		messages = append(messages, kafka.Message{
			Topic:   r.Topic,
			Key:     []byte(r.AggregateID),
			Value:   r.Payload,
			Headers: headers,
		})
	}
	if err := p.writer.WriteMessages(ctx, messages...); err != nil {
		return fmt.Errorf("publish %d records: %w", len(messages), err)
	}
	return nil
}

// Close flushes and releases the writer.
func (p *Publisher) Close() error { return p.writer.Close() }
