package httpapi

import (
	"bytes"
	"fmt"
	"net/http"
	"runtime/debug"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
	"github.com/rechedev9/cliphub/internal/obs"
)

// traceHTTP records mutations and failed requests. Successful polling, media
// bodies, credentials, raw URLs, query strings and request bodies stay out.
func traceHTTP(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		requestID := uuid.NewString()
		w.Header().Set("X-ClipHub-Request-ID", requestID)
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		capture := &errorResponseCapture{writer: ww}
		ww.Tee(capture)
		defer func() {
			panicked := recover()
			status := ww.Status()
			if status == 0 && panicked == nil {
				status = http.StatusOK
			}
			if panicked == nil && status < 400 && (r.Method == http.MethodGet || r.Method == http.MethodHead || r.Method == http.MethodOptions) {
				return
			}
			pattern := "unmatched"
			if route := chi.RouteContext(r.Context()); route != nil && route.RoutePattern() != "" {
				pattern = route.RoutePattern()
			}
			trace := obs.TraceContext{AttemptID: requestID, Operation: "http.request"}
			if strings.Contains(pattern, "/jobs/{id}") || strings.Contains(pattern, "/stream-jobs/{id}") || strings.Contains(pattern, "/editor/projects/{id}") {
				if id, err := uuid.Parse(chi.URLParam(r, "id")); err == nil {
					trace.JobID = id.String()
				}
			}
			level, outcome := "info", "ok"
			if status >= 400 {
				level, outcome = "error", "error"
			}
			message := fmt.Sprintf("%s %s status=%d", r.Method, pattern, status)
			if capture.body.Len() > 0 {
				message += "\n" + capture.body.String()
			}
			event := "http.completed"
			if panicked != nil {
				event, level, outcome = "http.panicked", "error", "error"
				message += fmt.Sprintf("\npanic: %v\n%s", panicked, debug.Stack())
			}
			obs.EmitTrace(obs.WithTrace(r.Context(), trace), obs.TraceEntry{Event: event, Level: level, Message: message, Outcome: outcome, DurationMS: time.Since(started).Milliseconds()})
			if panicked != nil {
				panic(panicked)
			}
		}()
		next.ServeHTTP(ww, r)
	})
}

type errorResponseCapture struct {
	writer middleware.WrapResponseWriter
	body   bytes.Buffer
}

func (c *errorResponseCapture) Write(p []byte) (int, error) {
	if c.writer.Status() >= 400 && strings.HasPrefix(c.writer.Header().Get("Content-Type"), "application/json") && c.body.Len() < 16<<10 {
		n := min(len(p), (16<<10)-c.body.Len())
		_, _ = c.body.Write(p[:n])
	}
	return len(p), nil
}
