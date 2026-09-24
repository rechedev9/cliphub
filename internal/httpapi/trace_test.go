package httpapi

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/obs"
)

func TestHTTPTraceCapturesFailureWithoutChangingResponseOrLoggingRequestSecrets(t *testing.T) {
	var logs bytes.Buffer
	prior := log.Writer()
	log.SetOutput(&logs)
	defer log.SetOutput(prior)
	r := chi.NewRouter()
	r.Use(traceHTTP)
	r.Post("/api/jobs/{id}/render", func(w http.ResponseWriter, r *http.Request) {
		writeError(w, 422, "decoder cannot open requested stream")
	})
	id := uuid.NewString()
	req := httptest.NewRequest(http.MethodPost, "/api/jobs/"+id+"/render?token=query-secret", strings.NewReader(`{"password":"body-secret"}`))
	req.Header.Set("Authorization", "Bearer header-secret")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 422 || !strings.Contains(w.Body.String(), "decoder cannot open requested stream") {
		t.Fatalf("response changed: %d %s", w.Code, w.Body.String())
	}
	if _, err := uuid.Parse(w.Header().Get("X-ClipHub-Request-ID")); err != nil {
		t.Fatalf("request correlation missing: %v", err)
	}
	text := logs.String()
	if !strings.Contains(text, id) || !strings.Contains(text, "decoder cannot open requested stream") || !strings.Contains(text, "/api/jobs/{id}/render") {
		t.Fatalf("missing HTTP evidence: %s", text)
	}
	if strings.Contains(text, "query-secret") || strings.Contains(text, "body-secret") || strings.Contains(text, "header-secret") {
		t.Fatalf("request data leaked: %s", text)
	}
}

func TestHTTPTraceLevelSeparatesClientAndServerErrors(t *testing.T) {
	for _, tc := range []struct {
		status         int
		level, outcome string
	}{
		{http.StatusAccepted, "info", "ok"},
		{http.StatusBadRequest, "warn", "error"},
		{http.StatusNotFound, "warn", "error"},
		{http.StatusTooManyRequests, "warn", "error"},
		{http.StatusInternalServerError, "error", "error"},
		{http.StatusServiceUnavailable, "error", "error"},
	} {
		t.Run(http.StatusText(tc.status), func(t *testing.T) {
			var logs bytes.Buffer
			prior := log.Writer()
			log.SetOutput(&logs)
			defer log.SetOutput(prior)
			r := chi.NewRouter()
			r.Use(traceHTTP)
			r.Post("/api/jobs/{id}/render", func(w http.ResponseWriter, r *http.Request) {
				if tc.status >= 400 {
					writeError(w, tc.status, "canary")
					return
				}
				w.WriteHeader(tc.status)
			})
			r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/api/jobs/"+uuid.NewString()+"/render", nil))
			entry := singleHTTPTrace(t, logs.String())
			if entry.Event != "http.completed" || entry.Level != tc.level || entry.Outcome != tc.outcome {
				t.Fatalf("status %d traced as %+v, want level=%s outcome=%s", tc.status, entry, tc.level, tc.outcome)
			}
		})
	}
}

func TestHTTPTracePanicIsAnError(t *testing.T) {
	var logs bytes.Buffer
	prior := log.Writer()
	log.SetOutput(&logs)
	defer log.SetOutput(prior)
	r := chi.NewRouter()
	r.Use(traceHTTP)
	r.Post("/api/jobs/{id}/render", func(http.ResponseWriter, *http.Request) { panic("level canary") })
	func() {
		defer func() { _ = recover() }()
		r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/api/jobs/"+uuid.NewString()+"/render", nil))
	}()
	if entry := singleHTTPTrace(t, logs.String()); entry.Event != "http.panicked" || entry.Level != "error" {
		t.Fatalf("panic traced as %+v", entry)
	}
}

func singleHTTPTrace(t *testing.T, logs string) obs.TraceEntry {
	t.Helper()
	var entries []obs.TraceEntry
	for _, line := range strings.Split(logs, "\n") {
		_, body, ok := strings.Cut(line, obs.TracePrefix)
		if !ok {
			continue
		}
		var entry obs.TraceEntry
		if err := json.Unmarshal([]byte(body), &entry); err != nil {
			t.Fatal(err)
		}
		entries = append(entries, entry)
	}
	if len(entries) != 1 {
		t.Fatalf("want one HTTP trace, got %d: %s", len(entries), logs)
	}
	return entries[0]
}

func TestHTTPTraceRetainsPanicWithoutTurningItIntoSuccessfulPolling(t *testing.T) {
	var logs bytes.Buffer
	prior := log.Writer()
	log.SetOutput(&logs)
	defer log.SetOutput(prior)
	r := chi.NewRouter()
	r.Use(traceHTTP)
	r.Get("/api/stream-jobs/{id}", func(http.ResponseWriter, *http.Request) { panic("decoder panic canary") })
	id := uuid.NewString()
	defer func() {
		if recover() != "decoder panic canary" {
			t.Error("HTTP panic behavior changed")
		}
		for _, expected := range []string{"http.panicked", id, "decoder panic canary", "goroutine", `"outcome":"error"`} {
			if !strings.Contains(logs.String(), expected) {
				t.Errorf("missing %s: %s", expected, logs.String())
			}
		}
	}()
	r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/stream-jobs/"+id, nil))
}
