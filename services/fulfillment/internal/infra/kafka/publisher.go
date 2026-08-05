package kafka

import (
	"context"
	"fmt"

	"github.com/segmentio/kafka-go"

	"github.com/orderfulfillment/fulfillment/internal/app"
	"github.com/orderfulfillment/fulfillment/internal/contracts"
	"github.com/orderfulfillment/fulfillment/internal/infra/tracing"
)

// Publisher writes fulfillment events to Kafka. It requires acknowledgement
// from all in-sync replicas and partitions by key (order id) so a given order's
// events keep their order. It implements app.EventPublisher.
type Publisher struct {
	writer *kafka.Writer
	topic  string
}

func NewPublisher(brokers []string, topic string) *Publisher {
	return &Publisher{
		writer: &kafka.Writer{
			Addr:                   kafka.TCP(brokers...),
			Balancer:               &kafka.Hash{},
			RequiredAcks:           kafka.RequireAll,
			AllowAutoTopicCreation: true,
		},
		topic: topic,
	}
}

func (p *Publisher) Publish(ctx context.Context, events ...app.Event) error {
	if len(events) == 0 {
		return nil
	}
	messages := make([]kafka.Message, 0, len(events))
	for _, e := range events {
		headers := []kafka.Header{{Key: contracts.HeaderEventType, Value: []byte(e.Type)}}
		tracing.InjectToKafkaHeaders(ctx, &headers)
		messages = append(messages, kafka.Message{
			Topic:   p.topic,
			Key:     []byte(e.Key),
			Value:   e.Payload,
			Headers: headers,
		})
	}
	if err := p.writer.WriteMessages(ctx, messages...); err != nil {
		return fmt.Errorf("publish %d fulfillment events: %w", len(messages), err)
	}
	return nil
}

func (p *Publisher) Close() error { return p.writer.Close() }
