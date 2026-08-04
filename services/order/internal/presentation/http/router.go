package http

import (
	"log/slog"
	"net/http"

	"github.com/orderfulfillment/order/internal/infra/config"
	"github.com/orderfulfillment/order/internal/infra/metrics"
)

// NewRouter wires routes and the middleware chain. Business routes require
// authentication (when enabled); health probes are always public. The /metrics
// scrape endpoint is served off the chain so it is neither logged nor counted.
func NewRouter(h *Handlers, health *HealthHandlers, cfg config.AuthConfig, logger *slog.Logger) http.Handler {
	mux := http.NewServeMux()

	auth := authMiddleware(cfg, logger)
	mux.Handle("POST /v1/orders", auth(http.HandlerFunc(h.PlaceOrder)))
	mux.Handle("GET /v1/orders/{id}", auth(http.HandlerFunc(h.GetOrder)))

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
