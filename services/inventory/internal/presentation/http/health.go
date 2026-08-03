package http

import (
	"context"
	"net/http"
	"time"
)

// Pinger reports datastore readiness.
type Pinger interface {
	Ping(ctx context.Context) error
}

// HealthHandlers expose liveness and readiness probes.
type HealthHandlers struct{ db Pinger }

func NewHealthHandlers(db Pinger) *HealthHandlers { return &HealthHandlers{db: db} }

// Live reports process liveness and never touches dependencies.
func (h *HealthHandlers) Live(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// Ready reports readiness to serve traffic, gated on database connectivity.
func (h *HealthHandlers) Ready(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if err := h.db.Ping(ctx); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "unavailable", "reason": "database"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}
