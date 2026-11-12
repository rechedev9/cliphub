package clipmind

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// limiter is a fixed-window counter per key. It guards the account routes
// against password guessing and signup floods; the per-account lockout in the
// catalog covers a slow attack spread over many addresses.
type limiter struct {
	mu      sync.Mutex
	now     func() time.Time
	windows map[string]*window
}

type window struct {
	start time.Time
	count int
}

func newLimiter(now func() time.Time) *limiter {
	return &limiter{now: now, windows: map[string]*window{}}
}

func (l *limiter) allow(key string, limit int, span time.Duration) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	if len(l.windows) > 10000 {
		for k, w := range l.windows {
			if now.Sub(w.start) > time.Hour {
				delete(l.windows, k)
			}
		}
	}
	w, ok := l.windows[key]
	if !ok || now.Sub(w.start) >= span {
		l.windows[key] = &window{start: now, count: 1}
		return true
	}
	w.count++
	return w.count <= limit
}

// clientIP is the caller's address; behind a trusted proxy it is the first
// X-Forwarded-For entry.
func clientIP(r *http.Request, trustProxy bool) string {
	if trustProxy {
		if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
			first, _, _ := strings.Cut(fwd, ",")
			return strings.TrimSpace(first)
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
