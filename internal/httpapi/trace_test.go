package httpapi

import (
	"bytes"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
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
