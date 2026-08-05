// Package orchestrator hosts the saga worker: it consumes the order event
// stream and drives the fulfillment saga for each placed order. It mirrors the
// projector's at-least-once consume loop: offsets are committed only after the
// saga step returns nil, and a returned error stops the worker so the
// supervisor restarts it and the saga resumes from its persisted state.
package orchestrator

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/segmentio/kafka-go"

	"github.com/orderfulfillment/order/internal/app/saga"
	"github.com/orderfulfillment/order/internal/contracts"
	"github.com/orderfulfillment/order/internal/infra/tracing"
)

// Worker consumes order events and runs the saga on order.placed.
type Worker struct {
	reader       *kafka.Reader
	orchestrator *saga.Orchestrator
	logger       *slog.Logger
}

func NewWorker(brokers, topics []string, groupID string, orch *saga.Orchestrator, logger *slog.Logger) *Worker {
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:     brokers,
		GroupID:     groupID,
		GroupTopics: topics,
		MinBytes:    1,
		MaxBytes:    10 << 20,
	})
	return &Worker{reader: reader, orchestrator: orch, logger: logger}
}

func (w *Worker) Run(ctx context.Context) error {
	w.logger.Info("order saga orchestrator started")
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

		// The saga only reacts to placements. The confirmed and cancelled events
		// it emits itself flow through this same topic and are ignored here.
		if headerValue(msg.Headers, contracts.HeaderEventType) == contracts.TypeOrderPlaced {
			event, err := contracts.UnmarshalOrderPlaced(msg.Value)
			if err != nil {
				return fmt.Errorf("unmarshal order.placed at offset %d: %w", msg.Offset, err)
			}
			msgCtx := tracing.ExtractFromKafka(ctx, msg.Headers)
			if err := w.orchestrator.Handle(msgCtx, event); err != nil {
				return fmt.Errorf("run saga for order %s at offset %d: %w", event.OrderID, msg.Offset, err)
			}
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
