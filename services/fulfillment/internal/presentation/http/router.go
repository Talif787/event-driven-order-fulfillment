package http

import (
	"log/slog"
	"net/http"

	"github.com/orderfulfillment/fulfillment/internal/infra/config"
	"github.com/orderfulfillment/fulfillment/internal/infra/metrics"
)

// NewRouter wires routes and the middleware chain. Business routes require
// authentication when enabled; health probes are always public.
func NewRouter(h *Handlers, health *HealthHandlers, cfg config.AuthConfig, logger *slog.Logger) http.Handler {
	mux := http.NewServeMux()

	auth := authMiddleware(cfg, logger)
	mux.Handle("GET /v1/shipments/{orderId}", auth(http.HandlerFunc(h.GetShipment)))
	mux.Handle("POST /v1/shipments/{orderId}/dispatch", auth(http.HandlerFunc(h.Dispatch)))
	mux.Handle("POST /v1/shipments/{orderId}/deliver", auth(http.HandlerFunc(h.Deliver)))

	mux.HandleFunc("GET /livez", health.Live)
	mux.HandleFunc("GET /readyz", health.Ready)
	mux.HandleFunc("GET /healthz", health.Live)

	business := chain(mux,
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
