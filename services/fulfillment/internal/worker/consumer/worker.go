// Package consumer hosts the fulfillment worker: it consumes the order event
// stream and opens a shipment for each confirmed order. It mirrors the order
// projector's at-least-once loop: offsets commit only after the handler returns
// nil, and a returned error stops the worker so the supervisor restarts it and
// reprocessing resumes from the last commit. Shipment creation is idempotent on
// order id, so redelivery is harmless.
package consumer

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/segmentio/kafka-go"
	"go.opentelemetry.io/otel/trace"

	"github.com/orderfulfillment/fulfillment/internal/app"
	"github.com/orderfulfillment/fulfillment/internal/contracts"
	"github.com/orderfulfillment/fulfillment/internal/infra/tracing"
)

type Worker struct {
	reader  *kafka.Reader
	service *app.Service
	logger  *slog.Logger
	tracer  trace.Tracer
}

func NewWorker(brokers, topics []string, groupID string, service *app.Service, logger *slog.Logger, tracer trace.Tracer) *Worker {
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:     brokers,
		GroupID:     groupID,
		GroupTopics: topics,
		MinBytes:    1,
		MaxBytes:    10 << 20,
	})
	return &Worker{reader: reader, service: service, logger: logger, tracer: tracer}
}

func (w *Worker) Run(ctx context.Context) error {
	w.logger.Info("fulfillment consumer started")
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

		// The service only reacts to confirmed orders. Cancelled orders never
		// reach fulfillment, and other event types are ignored.
		if headerValue(msg.Headers, contracts.HeaderEventType) == contracts.TypeOrderConfirmed {
			if err := w.handleConfirmed(ctx, msg); err != nil {
				return err
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

// handleConfirmed continues the producer's trace: it extracts the trace context
// from the message, opens a consume span, and creates the shipment under it, so
// the shipment.created it publishes carries this span forward.
func (w *Worker) handleConfirmed(ctx context.Context, msg kafka.Message) error {
	ctx = tracing.ExtractFromKafka(ctx, msg.Headers)
	ctx, span := w.tracer.Start(ctx, "fulfillment.consume")
	defer span.End()

	event, err := contracts.UnmarshalOrderConfirmed(msg.Value)
	if err != nil {
		return fmt.Errorf("unmarshal order.confirmed at offset %d: %w", msg.Offset, err)
	}
	orderID, err := uuid.Parse(event.OrderID)
	if err != nil {
		return fmt.Errorf("parse orderId %q at offset %d: %w", event.OrderID, msg.Offset, err)
	}
	if _, err := w.service.CreateForOrder(ctx, orderID); err != nil {
		return fmt.Errorf("create shipment for order %s at offset %d: %w", event.OrderID, msg.Offset, err)
	}
	return nil
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
