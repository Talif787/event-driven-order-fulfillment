// Package deadletter routes poison messages off the main consumer path. A
// poison message is one that fails deterministically (a malformed or
// unparseable payload): retrying it can never succeed, so it would crash-loop
// the consumer and block every message behind it. Permanent marks such an error;
// the worker routes those to the dead-letter topic and commits past them, while
// transient errors (a broker or database briefly down) still stop the worker so
// it retries from the last commit.
package deadletter

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/segmentio/kafka-go"
)

// ErrPermanent marks an error as non-retryable.
var ErrPermanent = errors.New("permanent")

// Permanent wraps err so the worker routes the message to the dead-letter topic
// instead of retrying it.
func Permanent(err error) error {
	return fmt.Errorf("%w: %w", ErrPermanent, err)
}

// Publisher writes poison messages to the dead-letter topic, preserving the
// original payload and stamping metadata about where and why it failed.
type Publisher struct {
	writer *kafka.Writer
}

func NewPublisher(brokers []string, topic string) *Publisher {
	return &Publisher{writer: &kafka.Writer{
		Addr:                   kafka.TCP(brokers...),
		Topic:                  topic,
		Balancer:               &kafka.Hash{},
		RequiredAcks:           kafka.RequireAll,
		AllowAutoTopicCreation: true,
	}}
}

// Publish sends the original message to the dead-letter topic, carrying its
// headers plus the source topic, offset, and failure cause.
func (p *Publisher) Publish(ctx context.Context, msg kafka.Message, cause error) error {
	headers := append([]kafka.Header{}, msg.Headers...)
	headers = append(headers,
		kafka.Header{Key: "dlq-source-topic", Value: []byte(msg.Topic)},
		kafka.Header{Key: "dlq-source-offset", Value: []byte(strconv.FormatInt(msg.Offset, 10))},
		kafka.Header{Key: "dlq-error", Value: []byte(cause.Error())},
	)
	return p.writer.WriteMessages(ctx, kafka.Message{
		Key:     msg.Key,
		Value:   msg.Value,
		Headers: headers,
	})
}

func (p *Publisher) Close() error { return p.writer.Close() }
