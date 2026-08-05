// Package tracing bridges OpenTelemetry trace context across the Kafka boundary.
// The global propagator (W3C tracecontext) is configured in the telemetry
// setup; these helpers carry it onto outbox headers at produce time and read it
// back off Kafka headers at consume time, so a trace spans the event backbone.
package tracing

import (
	"context"

	"github.com/segmentio/kafka-go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
)

// InjectToMap writes the current trace context into a string-map carrier. The
// order service stamps traceparent onto outbox headers at enqueue time, so the
// trace begins at the API request and rides the persisted event to Kafka.
func InjectToMap(ctx context.Context, carrier map[string]string) {
	otel.GetTextMapPropagator().Inject(ctx, propagation.MapCarrier(carrier))
}

// ExtractFromKafka returns a context carrying the trace context found in the
// Kafka message headers, so a consumer continues the producer's trace.
func ExtractFromKafka(ctx context.Context, headers []kafka.Header) context.Context {
	return otel.GetTextMapPropagator().Extract(ctx, kafkaHeaderCarrier(headers))
}

// kafkaHeaderCarrier adapts read access to Kafka headers for the propagator.
type kafkaHeaderCarrier []kafka.Header

func (c kafkaHeaderCarrier) Get(key string) string {
	for _, h := range c {
		if h.Key == key {
			return string(h.Value)
		}
	}
	return ""
}

// Set is unused on the extract path but required by the TextMapCarrier interface.
func (c kafkaHeaderCarrier) Set(string, string) {}

func (c kafkaHeaderCarrier) Keys() []string {
	keys := make([]string, 0, len(c))
	for _, h := range c {
		keys = append(keys, h.Key)
	}
	return keys
}
