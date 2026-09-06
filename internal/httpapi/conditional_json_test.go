package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/job"
)

func TestListJobsNotModifiedWhenUnchanged(t *testing.T) {
	repo := newFakeRepo()
	id := uuid.New()
	now := time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)
	repo.jobs[id] = job.Job{
		ID:        id,
		Status:    job.StatusParsed,
		CreatedAt: now,
		UpdatedAt: now,
	}
	h := NewHandlers(repo, newFakeStorage(), &fakeQueue{})
	router := chi.NewRouter()
	router.Get("/api/jobs", h.ListJobs)

	first := httptest.NewRecorder()
	router.ServeHTTP(first, httptest.NewRequest(http.MethodGet, "/api/jobs?limit=10", nil))
	if first.Code != http.StatusOK {
		t.Fatalf("first status = %d, want 200; body=%s", first.Code, first.Body.String())
	}
	etag := first.Header().Get("ETag")
	if etag == "" {
		t.Fatal("first response missing ETag")
	}
	if first.Header().Get("Cache-Control") != "private, no-cache" {
		t.Fatalf("Cache-Control = %q", first.Header().Get("Cache-Control"))
	}
	if !strings.Contains(first.Body.String(), id.String()) {
		t.Fatalf("first body missing job id: %s", first.Body.String())
	}

	second := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/jobs?limit=10", nil)
	req.Header.Set("If-None-Match", etag)
	router.ServeHTTP(second, req)
	if second.Code != http.StatusNotModified {
		t.Fatalf("second status = %d, want 304; body=%s", second.Code, second.Body.String())
	}
	if second.Body.Len() != 0 {
		t.Fatalf("304 body = %q, want empty", second.Body.String())
	}
	if second.Header().Get("ETag") != etag {
		t.Fatalf("304 ETag = %q, want %q", second.Header().Get("ETag"), etag)
	}
}

func TestListJobsWritesBodyWhenStatusChanges(t *testing.T) {
	repo := newFakeRepo()
	id := uuid.New()
	now := time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)
	repo.jobs[id] = job.Job{ID: id, Status: job.StatusScanning, CreatedAt: now, UpdatedAt: now}
	h := NewHandlers(repo, newFakeStorage(), &fakeQueue{})
	router := chi.NewRouter()
	router.Get("/api/jobs", h.ListJobs)

	first := httptest.NewRecorder()
	router.ServeHTTP(first, httptest.NewRequest(http.MethodGet, "/api/jobs?limit=10", nil))
	etag := first.Header().Get("ETag")

	repo.jobs[id] = job.Job{ID: id, Status: job.StatusScanned, CreatedAt: now, UpdatedAt: now.Add(time.Second)}

	second := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/jobs?limit=10", nil)
	req.Header.Set("If-None-Match", etag)
	router.ServeHTTP(second, req)
	if second.Code != http.StatusOK {
		t.Fatalf("changed status = %d, want 200; body=%s", second.Code, second.Body.String())
	}
	if second.Header().Get("ETag") == etag {
		t.Fatal("changed list reused the previous ETag")
	}
	var resp struct {
		Jobs []struct {
			Status job.Status `json:"status"`
		} `json:"jobs"`
	}
	if err := json.Unmarshal(second.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Jobs) != 1 || resp.Jobs[0].Status != job.StatusScanned {
		t.Fatalf("listed %+v, want scanned", resp.Jobs)
	}
}

func TestNoneMatchAcceptsWeakAndStrongTokens(t *testing.T) {
	etag := `W/"abc123"`
	cases := []struct {
		header string
		want   bool
	}{
		{"", false},
		{etag, true},
		{`"abc123"`, true},
		{`W/"abc123", W/"other"`, true},
		{`W/"other"`, false},
		{"*", true},
	}
	for _, tc := range cases {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		if tc.header != "" {
			req.Header.Set("If-None-Match", tc.header)
		}
		if got := noneMatch(req, etag); got != tc.want {
			t.Fatalf("If-None-Match %q = %v, want %v", tc.header, got, tc.want)
		}
	}
}

func TestJobListFingerprintChangesWithSummary(t *testing.T) {
	id := uuid.New()
	item := jobListItem{Job: job.Job{ID: id, Status: job.StatusScanned}}
	without := jobListFingerprintForTest([]jobListItem{item})
	item.Summary = &jobSummary{Match: rosterMatch{Map: "de_mirage"}}
	with := jobListFingerprintForTest([]jobListItem{item})
	if without == with {
		t.Fatal("fingerprint ignored an inlined roster summary")
	}
}
