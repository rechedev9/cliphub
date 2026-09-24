package httpapi

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/job"
	"github.com/rechedev9/cliphub/internal/obs"
)

func captureUserReports(t *testing.T) *bytes.Buffer {
	t.Helper()
	var logs bytes.Buffer
	prior := log.Writer()
	log.SetOutput(&logs)
	t.Cleanup(func() { log.SetOutput(prior) })
	return &logs
}

func userReportTraces(t *testing.T, logs string) []obs.TraceEntry {
	t.Helper()
	var reports []obs.TraceEntry
	for _, line := range strings.Split(logs, "\n") {
		_, body, ok := strings.Cut(line, obs.TracePrefix)
		if !ok {
			continue
		}
		var entry obs.TraceEntry
		if err := json.Unmarshal([]byte(body), &entry); err != nil {
			t.Fatal(err)
		}
		if entry.Event == "user.report" {
			reports = append(reports, entry)
		}
	}
	return reports
}

func postUserReport(h *Handlers, id, body string) *httptest.ResponseRecorder {
	rw := httptest.NewRecorder()
	Routes(h).ServeHTTP(rw, httptest.NewRequest(http.MethodPost, "/api/jobs/"+id+"/report", strings.NewReader(body)))
	return rw
}

func TestUserReportEmitsJobTraceAndRateLimitsRepeats(t *testing.T) {
	logs := captureUserReports(t)
	repo := newFakeRepo()
	id := uuid.New()
	repo.jobs[id] = job.Job{ID: id, Status: job.StatusDone}
	h := NewHandlers(repo, newFakeStorage(), &fakeQueue{})

	rw := postUserReport(h, id.String(), `{"category":"black_video"}`)
	if rw.Code != http.StatusAccepted || strings.TrimSpace(rw.Body.String()) != `{"status":"reported"}` {
		t.Fatalf("report = %d %s", rw.Code, rw.Body.String())
	}
	reports := userReportTraces(t, logs.String())
	if len(reports) != 1 {
		t.Fatalf("want one user.report trace, got %d: %s", len(reports), logs.String())
	}
	if got := reports[0]; got.JobID != id.String() || got.Level != "warn" || got.Message != "category=black_video" {
		t.Fatalf("user.report trace = %+v", got)
	}

	rw = postUserReport(h, id.String(), `{"category":"audio"}`)
	if rw.Code != http.StatusTooManyRequests {
		t.Fatalf("second report within 60 s = %d %s, want 429", rw.Code, rw.Body.String())
	}
	if len(userReportTraces(t, logs.String())) != 1 {
		t.Fatal("a rate-limited report still emitted a trace")
	}
	other := uuid.New()
	repo.jobs[other] = job.Job{ID: other, Status: job.StatusFailed}
	if rw := postUserReport(h, other.String(), `{"category":"cuts"}`); rw.Code != http.StatusAccepted {
		t.Fatalf("the limit is per job: %d %s", rw.Code, rw.Body.String())
	}
}

func TestUserReportRejectsInvalidInput(t *testing.T) {
	logs := captureUserReports(t)
	repo := newFakeRepo()
	id := uuid.New()
	repo.jobs[id] = job.Job{ID: id, Status: job.StatusDone}
	h := NewHandlers(repo, newFakeStorage(), &fakeQueue{})
	for _, tc := range []struct {
		name, id, body string
		want           int
	}{
		{"unknown category", id.String(), `{"category":"slow"}`, http.StatusBadRequest},
		{"free text", id.String(), `{"category":"other","text":"my name is"}`, http.StatusBadRequest},
		{"missing category", id.String(), `{}`, http.StatusBadRequest},
		{"malformed body", id.String(), `{"category":`, http.StatusBadRequest},
		{"invalid job id", "not-a-uuid", `{"category":"other"}`, http.StatusBadRequest},
		{"unknown job", uuid.NewString(), `{"category":"other"}`, http.StatusNotFound},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if rw := postUserReport(h, tc.id, tc.body); rw.Code != tc.want {
				t.Fatalf("status = %d, want %d; body=%s", rw.Code, tc.want, rw.Body.String())
			}
		})
	}
	if reports := userReportTraces(t, logs.String()); len(reports) != 0 {
		t.Fatalf("rejected reports emitted traces: %+v", reports)
	}
	// Rejected requests must not consume the job's report window.
	if rw := postUserReport(h, id.String(), `{"category":"wrong_overlay"}`); rw.Code != http.StatusAccepted {
		t.Fatalf("valid report after rejections = %d %s", rw.Code, rw.Body.String())
	}
}

func TestUserReportLimiterReopensAfterInterval(t *testing.T) {
	now := time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC)
	limiter := userReportLimiter{now: func() time.Time { return now }}
	id := uuid.New()
	if !limiter.allow(id) {
		t.Fatal("first report refused")
	}
	now = now.Add(userReportInterval - time.Second)
	if limiter.allow(id) {
		t.Fatal("report inside the interval accepted")
	}
	now = now.Add(time.Second)
	if !limiter.allow(id) {
		t.Fatal("report after the interval refused")
	}
	if len(limiter.last) != 1 {
		t.Fatalf("expired entries are not pruned: %d", len(limiter.last))
	}
}
