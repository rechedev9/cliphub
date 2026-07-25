package httpapi

import (
	"encoding/json"
	"net/http"

	"github.com/rechedev9/fragforge/internal/obs"
)

// Health is a cheap liveness probe that never touches the database. It is
// registered outside the auth middleware, so it answers unauthenticated callers
// and must never derive anything from a credential: the desktop discovery
// handshake that once had it sign an HMAC proof is gone.
func (h *Handlers) Health(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{"service": "fragforge", "status": "ok"})
}

// Metrics serves the local pipeline counters in the Prometheus text exposition
// format so a Prometheus server can scrape them. The counters live in the
// shared obs directory written by the CLI, batch runs, and workers.
func (h *Handlers) Metrics(w http.ResponseWriter, r *http.Request) {
	rec, err := obs.New(obs.DefaultDir())
	if err != nil {
		http.Error(w, "metrics unavailable", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	obs.WritePrometheus(w, rec.Snapshot())
}
