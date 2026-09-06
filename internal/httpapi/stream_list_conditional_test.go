package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/streamclips"
)

func TestListStreamJobsNotModifiedWhenUnchanged(t *testing.T) {
	repo := newFakeStreamRepo()
	jobID := uuid.New()
	now := time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)
	repo.jobs[jobID] = streamclips.Job{
		ID:        jobID,
		Status:    streamclips.StatusRendered,
		CreatedAt: now,
		UpdatedAt: now,
	}
	h := NewHandlers(newFakeRepo(), newFakeStorage(), &fakeQueue{}, WithStreamRepository(repo))

	first := httptest.NewRecorder()
	Routes(h).ServeHTTP(first, httptest.NewRequest(http.MethodGet, "/api/stream-jobs?limit=10", nil))
	if first.Code != http.StatusOK {
		t.Fatalf("first status = %d, want 200; body=%s", first.Code, first.Body.String())
	}
	etag := first.Header().Get("ETag")
	if etag == "" {
		t.Fatal("first response missing ETag")
	}

	second := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/stream-jobs?limit=10", nil)
	req.Header.Set("If-None-Match", etag)
	Routes(h).ServeHTTP(second, req)
	if second.Code != http.StatusNotModified {
		t.Fatalf("second status = %d, want 304; body=%s", second.Code, second.Body.String())
	}
	if second.Body.Len() != 0 {
		t.Fatalf("304 body = %q, want empty", second.Body.String())
	}
}
