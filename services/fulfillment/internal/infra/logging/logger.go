package logging

import (
	"context"
	"log/slog"
	"os"
)

type contextKey string

const correlationIDKey contextKey = "correlation_id"

// New builds a JSON structured logger bound to the service and environment.
func New(serviceName, environment, level string) *slog.Logger {
	var lvl slog.Level
	if err := lvl.UnmarshalText([]byte(level)); err != nil {
		lvl = slog.LevelInfo
	}
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: lvl})
	return slog.New(&correlationHandler{inner: handler}).With(
		slog.String("service", serviceName),
		slog.String("env", environment),
	)
}

// WithCorrelationID stores a correlation id in the context for log enrichment.
func WithCorrelationID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, correlationIDKey, id)
}

// CorrelationID extracts the correlation id if present.
func CorrelationID(ctx context.Context) (string, bool) {
	v, ok := ctx.Value(correlationIDKey).(string)
	return v, ok
}

type correlationHandler struct{ inner slog.Handler }

func (h *correlationHandler) Enabled(ctx context.Context, l slog.Level) bool {
	return h.inner.Enabled(ctx, l)
}

func (h *correlationHandler) Handle(ctx context.Context, r slog.Record) error {
	if id, ok := CorrelationID(ctx); ok {
		r.AddAttrs(slog.String("correlation_id", id))
	}
	return h.inner.Handle(ctx, r)
}

func (h *correlationHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &correlationHandler{inner: h.inner.WithAttrs(attrs)}
}

func (h *correlationHandler) WithGroup(name string) slog.Handler {
	return &correlationHandler{inner: h.inner.WithGroup(name)}
}
