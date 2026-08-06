package http

import (
	"log/slog"
	"net/http"

	"github.com/orderfulfillment/inventory/internal/infra/config"
	"github.com/orderfulfillment/inventory/internal/infra/metrics"
)

// NewRouter wires routes and the middleware chain. Business routes require
// authentication (when enabled); health probes are always public.
func NewRouter(h *Handlers, health *HealthHandlers, cfg config.AuthConfig, corsOrigins []string, logger *slog.Logger) http.Handler {
	mux := http.NewServeMux()

	auth := authMiddleware(cfg, logger)
	mux.Handle("POST /v1/reservations", auth(http.HandlerFunc(h.Reserve)))
	mux.Handle("GET /v1/reservations/{orderId}", auth(http.HandlerFunc(h.GetReservation)))
	mux.Handle("POST /v1/reservations/{orderId}/release", auth(http.HandlerFunc(h.Release)))
	mux.Handle("POST /v1/reservations/{orderId}/commit", auth(http.HandlerFunc(h.Commit)))
	mux.Handle("GET /v1/stock/{sku}", auth(http.HandlerFunc(h.GetStock)))
	mux.Handle("PUT /v1/stock/{sku}", auth(http.HandlerFunc(h.SetStock)))

	mux.HandleFunc("GET /livez", health.Live)
	mux.HandleFunc("GET /readyz", health.Ready)
	mux.HandleFunc("GET /healthz", health.Live)

	business := chain(mux,
		corsMiddleware(corsOrigins),
		traceExtractMiddleware,
		correlationMiddleware,
		recoveryMiddleware(logger),
		loggingMiddleware(logger),
		metricsMiddleware,
	)

	root := http.NewServeMux()
	root.Handle("GET /metrics", metrics.Handler())
	root.Handle("/", business)
	return root
}
