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

	"github.com/orderfulfillment/notification/internal/app"
	"github.com/orderfulfillment/notification/internal/contracts"
)

type Worker struct {
	reader     *kafka.Reader
	dispatcher *app.Dispatcher
	logger     *slog.Logger
}

func NewWorker(brokers, topics []string, groupID string, dispatcher *app.Dispatcher, logger *slog.Logger) *Worker {
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:     brokers,
		GroupID:     groupID,
		GroupTopics: topics,
		MinBytes:    1,
		MaxBytes:    10 << 20,
	})
	return &Worker{reader: reader, dispatcher: dispatcher, logger: logger}
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
		if err := w.dispatcher.Handle(ctx, eventType, msg.Value); err != nil {
			return fmt.Errorf("handle %q at offset %d on %s: %w", eventType, msg.Offset, msg.Topic, err)
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
