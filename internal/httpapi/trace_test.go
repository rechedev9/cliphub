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
	"github.com/rechedev9/cliphub/internal/job"
	"github.com/rechedev9/cliphub/internal/rules"
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

// Studio polls the roster of a job until it exists; each 409 on the way is
// expected and must not read as an error in studio.log.
func TestHTTPTraceLogsNotReadyPollBelowErrorLevel(t *testing.T) {
	var logs bytes.Buffer
	prior := log.Writer()
	log.SetOutput(&logs)
	defer log.SetOutput(prior)
	r := chi.NewRouter()
	r.Use(traceHTTP)
	r.Get("/api/jobs/{id}/roster", func(w http.ResponseWriter, r *http.Request) {
		writeError(w, http.StatusConflict, "roster not ready")
	})
	r.Post("/api/jobs/{id}/parse", func(w http.ResponseWriter, r *http.Request) {
		writeError(w, http.StatusConflict, "job is not ready to parse")
	})

	r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/jobs/"+uuid.NewString()+"/roster", nil))
	if text := logs.String(); !strings.Contains(text, `"level":"info"`) || strings.Contains(text, `"outcome":"error"`) {
		t.Fatalf("roster poll trace = %s, want info level without an error outcome", text)
	}
	logs.Reset()
	r.Get(teamLogoRoute, func(w http.ResponseWriter, r *http.Request) {
		writeError(w, http.StatusNotFound, "no logo")
	})
	r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, teamLogoRoute+"?name=Equipo", nil))
	if text := logs.String(); strings.Contains(text, `"outcome":"error"`) {
		t.Fatalf("logo miss trace = %s, want no error outcome", text)
	}
	logs.Reset()
	r.Get(mapShotRoute, func(w http.ResponseWriter, r *http.Request) {
		writeError(w, http.StatusNotFound, "no picture")
	})
	r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/maps/de_cache/shot", nil))
	if text := logs.String(); strings.Contains(text, `"outcome":"error"`) {
		t.Fatalf("map picture miss trace = %s, want no error outcome", text)
	}
	logs.Reset()
	r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/api/jobs/"+uuid.NewString()+"/parse", nil))
	if text := logs.String(); !strings.Contains(text, `"level":"error"`) {
		t.Fatalf("refused mutation trace = %s, want error level", text)
	}
}

// wait_for_job polls a render variant before its state exists; the real
// handler's 404 must keep its body but log at info, while a missing job stays an error.
func TestHTTPTraceLogsRenderVariantNotReadyPollBelowErrorLevel(t *testing.T) {
	var logs bytes.Buffer
	prior := log.Writer()
	log.SetOutput(&logs)
	defer log.SetOutput(prior)
	repo := newFakeRepo()
	j := job.Job{ID: uuid.New(), Status: job.StatusRecorded, Rules: rules.Default()}
	repo.jobs[j.ID] = j
	h := NewHandlers(repo, newFakeStorage(), &fakeQueue{})
	r := chi.NewRouter()
	r.Use(traceHTTP)
	r.Get(renderVariantRoute, h.GetRenderVariant)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/jobs/"+j.ID.String()+"/renders/viral-60-clean", nil))
	if w.Code != http.StatusNotFound || !strings.Contains(w.Body.String(), "render variant not found") {
		t.Fatalf("response changed: %d %s", w.Code, w.Body.String())
	}
	if text := logs.String(); !strings.Contains(text, `"level":"info"`) || strings.Contains(text, `"level":"error"`) || strings.Contains(text, `"outcome":"error"`) {
		t.Fatalf("not-ready render poll trace = %s, want info level without an error outcome", text)
	}
	logs.Reset()
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/jobs/"+uuid.NewString()+"/renders/viral-60-clean", nil))
	if w.Code != http.StatusNotFound || !strings.Contains(w.Body.String(), "job not found") {
		t.Fatalf("missing job response = %d %s, want 404 job not found", w.Code, w.Body.String())
	}
	if text := logs.String(); !strings.Contains(text, `"level":"error"`) || !strings.Contains(text, `"outcome":"error"`) {
		t.Fatalf("missing job trace = %s, want error level", text)
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
