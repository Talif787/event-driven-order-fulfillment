// Package tracing bridges OpenTelemetry trace context across the Kafka boundary.
// Notification is a leaf consumer, so it only extracts: it reads the trace
// context off consumed Kafka headers so each notification continues the trace of
// the event that produced it.
package tracing

import (
	"context"

	"github.com/segmentio/kafka-go"
	"go.opentelemetry.io/otel"
)

// ExtractFromKafka returns a context carrying the trace context found in the
// Kafka message headers, so the consumer continues the producer's trace.
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
