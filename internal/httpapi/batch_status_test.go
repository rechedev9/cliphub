package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/job"
	"github.com/rechedev9/cliphub/internal/renderplan"
)

func TestBatchStatusFoldsJobAndRenderPerItem(t *testing.T) {
	repo := newFakeRepo()
	store := newFakeStorage()
	parsed := uuid.New()
	repo.jobs[parsed] = job.Job{ID: parsed, Status: job.StatusParsed}
	rendered := uuid.New()
	repo.jobs[rendered] = job.Job{ID: rendered, Status: job.StatusComposed}
	state := renderplan.NewRenderVariantState(renderplan.NewRenderVariantStateOptions{
		JobID:   rendered,
		Variant: "viral-60-clean",
		Status:  renderplan.RenderVariantStatusQueued,
	})
	key, err := renderplan.RenderVariantStateKey(rendered, "viral-60-clean")
	if err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Put(key, bytes.NewReader(b)); err != nil {
		t.Fatal(err)
	}
	gone := uuid.New()
	h := NewHandlers(repo, store, &fakeQueue{})
	r := chi.NewRouter()
	r.Get("/api/jobs/batch-status", h.BatchStatus)

	items := parsed.String() + ":viral-60-clean," + rendered.String() + ":viral-60-clean," + gone.String() + ":viral-60-clean"
	req := httptest.NewRequest(http.MethodGet, "/api/jobs/batch-status?items="+items, nil)
	rw := httptest.NewRecorder()
	r.ServeHTTP(rw, req)
	if rw.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rw.Code, rw.Body.String())
	}
	var resp struct {
		Items []struct {
			JobID uuid.UUID `json:"job_id"`
			Job   *struct {
				Status string `json:"status"`
			} `json:"job"`
			Render *struct {
				Status string   `json:"status"`
				Videos []string `json:"videos"`
			} `json:"render"`
		} `json:"items"`
	}
	if err := json.Unmarshal(rw.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v; body=%s", err, rw.Body.String())
	}
	if len(resp.Items) != 3 {
		t.Fatalf("items = %d, want 3", len(resp.Items))
	}
	if resp.Items[0].JobID != parsed || resp.Items[0].Job == nil || resp.Items[0].Job.Status != "parsed" || resp.Items[0].Render != nil {
		t.Fatalf("parsed item = %+v, want job parsed and no render", resp.Items[0])
	}
	if resp.Items[1].JobID != rendered || resp.Items[1].Job == nil || resp.Items[1].Render == nil || resp.Items[1].Render.Status != "queued" {
		t.Fatalf("rendered item = %+v, want job and a queued render", resp.Items[1])
	}
	if resp.Items[2].JobID != gone || resp.Items[2].Job != nil || resp.Items[2].Render != nil {
		t.Fatalf("gone item = %+v, want null job and render", resp.Items[2])
	}
}

func TestBatchStatusOmitsLeftoverRenderBeforeRecorded(t *testing.T) {
	repo := newFakeRepo()
	store := newFakeStorage()
	recording := uuid.New()
	repo.jobs[recording] = job.Job{ID: recording, Status: job.StatusRecording}
	state := renderplan.NewRenderVariantState(renderplan.NewRenderVariantStateOptions{
		JobID:   recording,
		Variant: "viral-60-clean",
		Status:  renderplan.RenderVariantStatusReady,
	})
	key, err := renderplan.RenderVariantStateKey(recording, "viral-60-clean")
	if err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Put(key, bytes.NewReader(b)); err != nil {
		t.Fatal(err)
	}
	h := NewHandlers(repo, store, &fakeQueue{})
	r := chi.NewRouter()
	r.Get("/api/jobs/batch-status", h.BatchStatus)

	req := httptest.NewRequest(http.MethodGet, "/api/jobs/batch-status?items="+recording.String()+":viral-60-clean", nil)
	rw := httptest.NewRecorder()
	r.ServeHTTP(rw, req)
	if rw.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rw.Code, rw.Body.String())
	}
	var resp struct {
		Items []struct {
			Job *struct {
				Status string `json:"status"`
			} `json:"job"`
			Render *struct {
				Status string `json:"status"`
			} `json:"render"`
		} `json:"items"`
	}
	if err := json.Unmarshal(rw.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v; body=%s", err, rw.Body.String())
	}
	if len(resp.Items) != 1 {
		t.Fatalf("items = %d, want 1", len(resp.Items))
	}
	if resp.Items[0].Job == nil || resp.Items[0].Job.Status != "recording" || resp.Items[0].Render != nil {
		t.Fatalf("item = %+v, want recording job and no leftover render", resp.Items[0])
	}
}

func TestBatchStatusRejectsMalformedItems(t *testing.T) {
	h := NewHandlers(newFakeRepo(), newFakeStorage(), &fakeQueue{})
	r := chi.NewRouter()
	r.Get("/api/jobs/batch-status", h.BatchStatus)
	for _, items := range []string{"not-a-uuid:viral-60-clean", uuid.New().String(), uuid.New().String() + ":bad/variant"} {
		req := httptest.NewRequest(http.MethodGet, "/api/jobs/batch-status?items="+items, nil)
		rw := httptest.NewRecorder()
		r.ServeHTTP(rw, req)
		if rw.Code != http.StatusBadRequest {
			t.Fatalf("items %q: status = %d, want 400; body=%s", items, rw.Code, rw.Body.String())
		}
	}
	req := httptest.NewRequest(http.MethodGet, "/api/jobs/batch-status", nil)
	rw := httptest.NewRecorder()
	r.ServeHTTP(rw, req)
	if rw.Code != http.StatusOK || rw.Body.String() != "{\"items\":[]}\n" {
		t.Fatalf("empty items: status = %d body=%q", rw.Code, rw.Body.String())
	}
}
