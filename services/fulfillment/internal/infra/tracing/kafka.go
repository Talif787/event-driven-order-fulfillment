// Package tracing bridges OpenTelemetry trace context across the Kafka boundary.
// The global propagator (W3C tracecontext) is configured in the telemetry
// setup; these helpers read it off consumed Kafka headers and write it onto
// produced ones, so a trace continues through the event backbone.
package tracing

import (
	"context"

	"github.com/segmentio/kafka-go"
	"go.opentelemetry.io/otel"
)

// ExtractFromKafka returns a context carrying the trace context found in the
// Kafka message headers, so a consumer continues the producer's trace.
func ExtractFromKafka(ctx context.Context, headers []kafka.Header) context.Context {
	local := headers
	return otel.GetTextMapPropagator().Extract(ctx, &kafkaHeaderCarrier{headers: &local})
}

// InjectToKafkaHeaders writes the current trace context onto the Kafka headers,
// so a produced event carries the trace forward to downstream consumers.
func InjectToKafkaHeaders(ctx context.Context, headers *[]kafka.Header) {
	otel.GetTextMapPropagator().Inject(ctx, &kafkaHeaderCarrier{headers: headers})
}

// kafkaHeaderCarrier adapts Kafka headers to the propagator's TextMapCarrier.
type kafkaHeaderCarrier struct{ headers *[]kafka.Header }

func (c *kafkaHeaderCarrier) Get(key string) string {
	for _, h := range *c.headers {
		if h.Key == key {
			return string(h.Value)
		}
	}
	return ""
}

func (c *kafkaHeaderCarrier) Set(key, value string) {
	for i := range *c.headers {
		if (*c.headers)[i].Key == key {
			(*c.headers)[i].Value = []byte(value)
			return
		}
	}
	*c.headers = append(*c.headers, kafka.Header{Key: key, Value: []byte(value)})
}

func (c *kafkaHeaderCarrier) Keys() []string {
	keys := make([]string, 0, len(*c.headers))
	for _, h := range *c.headers {
		keys = append(keys, h.Key)
	}
	return keys
}
