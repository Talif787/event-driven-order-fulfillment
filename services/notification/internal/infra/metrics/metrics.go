// Package metrics holds the Prometheus instrumentation for the notification
// service. Notification is a pure consumer with no business HTTP server, so this
// package provides the notifications counter, the scrape handler, and a
// standalone metrics and health server for the consumer process.
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
const Service = "notification"

// NotificationsSent counts notifications actually sent, by source event type. A
// deduplicated (skipped) event is not counted, so the metric means "sent".
var NotificationsSent = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "notifications_sent_total",
	Help: "Notifications sent, by source event type.",
}, []string{"event_type"})

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
