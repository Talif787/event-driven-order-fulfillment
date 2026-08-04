// Package metrics holds the Prometheus instrumentation for the inventory
// service: the metric definitions and the scrape handler. Inventory is a single
// HTTP service with no worker processes, so there is no standalone metrics
// server here.
package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Service is the value of the "service" label on this module's metrics.
const Service = "inventory"

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

	// Reservations counts reservation lifecycle outcomes: held on a successful
	// hold, rejected when stock is insufficient, released on compensation, and
	// committed when an order is fulfilled.
	Reservations = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "reservations_total",
		Help: "Reservation outcomes by result.",
	}, []string{"outcome"})
)

// Handler returns the Prometheus scrape handler. The default registry also
// carries the Go runtime and process collectors.
func Handler() http.Handler { return promhttp.Handler() }
