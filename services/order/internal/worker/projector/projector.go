package projector

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/segmentio/kafka-go"

	"github.com/orderfulfillment/order/internal/app/projection"
	"github.com/orderfulfillment/order/internal/contracts"
	"github.com/orderfulfillment/order/internal/infra/metrics"
)

// Worker consumes the order event stream and applies events to the read model.
// Offsets are committed only after a successful projection write, giving
// at-least-once processing. The projection upsert is idempotent, so redelivery
// is harmless. A failed apply stops the worker so the supervisor restarts it and
// reprocesses from the last commit; poison-message handling (a dead-letter
// topic) is the next hardening step.
type Worker struct {
	reader  *kafka.Reader
	service *projection.Service
	logger  *slog.Logger
}

func NewWorker(brokers, topics []string, groupID string, service *projection.Service, logger *slog.Logger) *Worker {
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:     brokers,
		GroupID:     groupID,
		GroupTopics: topics,
		MinBytes:    1,
		MaxBytes:    10 << 20,
	})
	return &Worker{reader: reader, service: service, logger: logger}
}

func (w *Worker) Run(ctx context.Context) error {
	w.logger.Info("order projector started")
	for {
		msg, err := w.reader.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil || errors.Is(err, context.Canceled) {
				return nil
			}
			w.logger.Warn("fetch failed, retrying", slog.String("error", err.Error()))
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(time.Second):
			}
			continue
		}

		eventType := headerValue(msg.Headers, contracts.HeaderEventType)
		if err := w.service.Apply(ctx, eventType, msg.Value); err != nil {
			return fmt.Errorf("apply event %q at offset %d: %w", eventType, msg.Offset, err)
		}

		if err := w.reader.CommitMessages(ctx, msg); err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return fmt.Errorf("commit offset: %w", err)
		}

		metrics.EventsConsumed.WithLabelValues(metrics.Service, msg.Topic).Inc()
	}
}

func (w *Worker) Close() error { return w.reader.Close() }

func headerValue(headers []kafka.Header, key string) string {
	for _, h := range headers {
		if h.Key == key {
			return string(h.Value)
		}
	}
	return ""
}
