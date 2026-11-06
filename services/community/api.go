package main

import (
	"encoding/json"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

const (
	// writesPerHour bounds submit and delete calls from one client IP.
	writesPerHour = 30
	maxTrackedIPs = 50000
)

type api struct {
	store   *store
	limiter *ipLimiter
}

func newAPI(s *store) *api {
	return &api{store: s, limiter: &ipLimiter{limit: writesPerHour, window: time.Hour, seen: map[string]*ipWindow{}}}
}

func (a *api) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/pro-reference", a.handleProReference)
	mux.HandleFunc("GET /v1/rankings", a.handleRankings)
	mux.HandleFunc("POST /v1/rankings/submit", a.handleSubmit)
	mux.HandleFunc("DELETE /v1/rankings/installs/{id}", a.handleDelete)
	mux.HandleFunc("GET /v1/health", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	return mux
}

func (a *api) internal(w http.ResponseWriter, op string, err error) {
	log.Printf("community stage=%s class=internal error=%v", op, err)
	writeError(w, http.StatusInternalServerError, "internal error")
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// clientIP trusts X-Forwarded-For only from the loopback reverse proxy.
func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	ip := net.ParseIP(host)
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" && ip != nil && ip.IsLoopback() {
		parts := strings.Split(fwd, ",")
		return strings.TrimSpace(parts[len(parts)-1])
	}
	return host
}

type ipWindow struct {
	start time.Time
	count int
}

// ipLimiter is a fixed window per IP, enough to stop one client flooding writes.
type ipLimiter struct {
	mu     sync.Mutex
	limit  int
	window time.Duration
	seen   map[string]*ipWindow
}

func (l *ipLimiter) allow(ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	t := time.Now()
	if len(l.seen) >= maxTrackedIPs {
		for k, v := range l.seen {
			if t.Sub(v.start) >= l.window {
				delete(l.seen, k)
			}
		}
		if len(l.seen) >= maxTrackedIPs {
			return false
		}
	}
	w := l.seen[ip]
	if w == nil || t.Sub(w.start) >= l.window {
		l.seen[ip] = &ipWindow{start: t, count: 1}
		return true
	}
	w.count++
	return w.count <= l.limit
}
