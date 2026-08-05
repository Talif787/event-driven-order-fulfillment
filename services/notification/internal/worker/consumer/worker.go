// Package consumer hosts the notification worker: it fans in the order,
// payment, and fulfillment streams on one consumer group and hands each record
// to the dispatcher. Offsets commit only after the handler returns nil, giving
// at-least-once processing; the dispatcher dedupes, so redelivery is harmless.
// A returned error stops the worker so the supervisor restarts it and
// reprocessing resumes from the last commit.
package consumer

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/segmentio/kafka-go"
	"go.opentelemetry.io/otel/trace"

	"github.com/orderfulfillment/notification/internal/app"
	"github.com/orderfulfillment/notification/internal/contracts"
	"github.com/orderfulfillment/notification/internal/infra/deadletter"
	"github.com/orderfulfillment/notification/internal/infra/tracing"
)

type Worker struct {
	reader     *kafka.Reader
	dispatcher *app.Dispatcher
	logger     *slog.Logger
	tracer     trace.Tracer
	dlq        *deadletter.Publisher
}

func NewWorker(brokers, topics []string, groupID string, dispatcher *app.Dispatcher, logger *slog.Logger, tracer trace.Tracer, dlq *deadletter.Publisher) *Worker {
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:     brokers,
		GroupID:     groupID,
		GroupTopics: topics,
		MinBytes:    1,
		MaxBytes:    10 << 20,
	})
	return &Worker{reader: reader, dispatcher: dispatcher, logger: logger, tracer: tracer, dlq: dlq}
}

func (w *Worker) Run(ctx context.Context) error {
	w.logger.Info("notification consumer started")
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
		msgCtx := tracing.ExtractFromKafka(ctx, msg.Headers)
		msgCtx, span := w.tracer.Start(msgCtx, "notification.dispatch")
		err = w.dispatcher.Handle(msgCtx, eventType, msg.Value)
		span.End()
		if err != nil {
			if !errors.Is(err, deadletter.ErrPermanent) {
				return fmt.Errorf("handle %q at offset %d on %s: %w", eventType, msg.Offset, msg.Topic, err)
			}
			if dlqErr := w.dlq.Publish(ctx, msg, err); dlqErr != nil {
				return fmt.Errorf("dead-letter publish at offset %d: %w", msg.Offset, dlqErr)
			}
			w.logger.Warn("routed poison message to dead-letter",
				slog.Int64("offset", msg.Offset), slog.String("error", err.Error()))
		}

		if err := w.reader.CommitMessages(ctx, msg); err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return fmt.Errorf("commit offset: %w", err)
		}
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
