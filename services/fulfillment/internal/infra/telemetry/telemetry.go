package telemetry

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
)

// Provider wraps the tracer provider and exposes a clean shutdown hook.
type Provider struct {
	tracer   trace.Tracer
	shutdown func(context.Context) error
}

func (p *Provider) Tracer() trace.Tracer               { return p.tracer }
func (p *Provider) Shutdown(ctx context.Context) error { return p.shutdown(ctx) }

// Setup configures OpenTelemetry tracing. When endpoint is empty tracing is a
// no-op, which keeps local development and unit tests free of collector deps.
func Setup(ctx context.Context, serviceName, environment, endpoint string, sampleRatio float64) (*Provider, error) {
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{}, propagation.Baggage{},
	))

	if endpoint == "" {
		return &Provider{
			tracer:   noop.NewTracerProvider().Tracer(serviceName),
			shutdown: func(context.Context) error { return nil },
		}, nil
	}

	exporter, err := otlptracehttp.New(ctx, otlptracehttp.WithEndpointURL(endpoint))
	if err != nil {
		return nil, fmt.Errorf("create otlp exporter: %w", err)
	}

	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(serviceName),
			attribute.String("deployment.environment", environment),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("build resource: %w", err)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.TraceIDRatioBased(sampleRatio))),
	)
	otel.SetTracerProvider(tp)

	return &Provider{
		tracer:   tp.Tracer(serviceName),
		shutdown: tp.Shutdown,
	}, nil
}
