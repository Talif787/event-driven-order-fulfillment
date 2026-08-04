// Package metrics holds the Prometheus instrumentation for the fulfillment
// service: the metric definitions, the scrape handler, and a standalone metrics
// and health server for the consumer process, which has no HTTP server.
package metrics

import (
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Service is the value of the "service" label on this module's metrics.
const Service = "fulfillment"

var (
	// HTTPRequests counts HTTP requests by method, matched route, and status.
	HTTPRequests = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "http_requests_total",
		Help: "Total HTTP requests by method, route, and status.",
	}, []string{"service", "method", "path", "status"})

	// HTTPDuration observes HTTP request latency by method and matched route.
	HTTPDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "http_request_duration_seconds",
		Help:    "HTTP request duration in seconds by method and route.",
		Buckets: prometheus.DefBuckets,
	}, []string{"service", "method", "path"})

	// Shipments counts shipment transitions: created when a shipment is opened,
	// dispatched and delivered as it advances.
	Shipments = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "shipments_total",
		Help: "Shipment transitions by type.",
	}, []string{"transition"})
)

// Handler returns the Prometheus scrape handler. The default registry also
// carries the Go runtime and process collectors.
func Handler() http.Handler { return promhttp.Handler() }

// StartServer runs a minimal metrics and health server in the background for the
// consumer process, which has no HTTP server. It serves /metrics for scraping
// and /livez and /readyz for the Kubernetes probes. The caller shuts down the
// returned server on exit.
func StartServer(addr string, logger *slog.Logger) *http.Server {
	mux := http.NewServeMux()
	mux.Handle("/metrics", Handler())
	ok := func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }
	mux.HandleFunc("/livez", ok)
	mux.HandleFunc("/readyz", ok)

	srv := &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("metrics server failed", slog.String("error", err.Error()))
		}
	}()
	logger.Info("metrics server started", slog.String("addr", addr))
	return srv
}
