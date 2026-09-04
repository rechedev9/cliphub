package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
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

// batchStatusRepo is a fakeRepo that also offers the bulk status projection, so
// one batch poll costs one query. It counts both entry points and can fail the
// bulk read, which is what a locked database looks like to this handler.
type batchStatusRepo struct {
	*fakeRepo
	statusCalls   atomic.Int64
	statusesCalls atomic.Int64
	statusesErr   error
}

func newBatchStatusRepo() *batchStatusRepo { return &batchStatusRepo{fakeRepo: newFakeRepo()} }

func (r *batchStatusRepo) GetStatus(ctx context.Context, id uuid.UUID) (job.Status, string, int, error) {
	r.statusCalls.Add(1)
	return r.fakeRepo.GetStatus(ctx, id)
}

func (r *batchStatusRepo) GetStatuses(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]job.StatusRow, error) {
	r.statusesCalls.Add(1)
	if r.statusesErr != nil {
		return nil, r.statusesErr
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	out := make(map[uuid.UUID]job.StatusRow, len(ids))
	for _, id := range ids {
		status, reason, segments, err := r.fakeRepo.GetStatus(ctx, id)
		if err != nil {
			continue
		}
		out[id] = job.StatusRow{Status: status, FailureReason: reason, SegmentCount: segments}
	}
	return out, nil
}

// batchStatusBody is the wire shape the Studio client reads: `job`/`render` nil
// for "gone" and "no render yet", and `error` for a row the server could not
// read at all.
type batchStatusBody struct {
	Items []struct {
		JobID   uuid.UUID `json:"job_id"`
		Variant string    `json:"variant"`
		Job     *struct {
			Status string `json:"status"`
		} `json:"job"`
		Render *struct {
			Status string `json:"status"`
		} `json:"render"`
		Error *struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	} `json:"items"`
}

func putBatchRenderState(t *testing.T, store *fakeStorage, id uuid.UUID, variant, status string) {
	t.Helper()
	state := renderplan.NewRenderVariantState(renderplan.NewRenderVariantStateOptions{
		JobID:   id,
		Variant: variant,
		Status:  status,
	})
	key, err := renderplan.RenderVariantStateKey(id, variant)
	if err != nil {
		t.Fatal(err)
	}
	body, err := json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Put(key, bytes.NewReader(body)); err != nil {
		t.Fatal(err)
	}
}

func batchStatusRequest(t *testing.T, h *Handlers, items string) *httptest.ResponseRecorder {
	t.Helper()
	r := chi.NewRouter()
	r.Get("/api/jobs/batch-status", h.BatchStatus)
	req := httptest.NewRequest(http.MethodGet, "/api/jobs/batch-status?items="+items, nil)
	rw := httptest.NewRecorder()
	r.ServeHTTP(rw, req)
	return rw
}

func decodeBatchStatus(t *testing.T, rw *httptest.ResponseRecorder) batchStatusBody {
	t.Helper()
	var body batchStatusBody
	if err := json.Unmarshal(rw.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v; body=%s", err, rw.Body.String())
	}
	return body
}

func mustRenderStateKey(t *testing.T, id uuid.UUID, variant string) string {
	t.Helper()
	key, err := renderplan.RenderVariantStateKey(id, variant)
	if err != nil {
		t.Fatal(err)
	}
	return key
}

// One reel the server cannot read must not collapse the batch: the client falls
// back to per-reel polling for the whole tick, which is the 2N-request
// regression this endpoint removed. The bad row degrades on its own, and it
// degrades through an explicit error rather than a nil half - a nil job latches
// the reel as gone and a nil render re-derives `record`, firing an unrequested
// capture.
func TestBatchStatusDegradesAnUnreadableRenderRowOnItsOwn(t *testing.T) {
	cases := []struct {
		name    string
		corrupt func(t *testing.T, store *fakeStorage, id uuid.UUID, variant string)
	}{
		{
			name: "render result undecodable",
			corrupt: func(t *testing.T, store *fakeStorage, id uuid.UUID, variant string) {
				t.Helper()
				// A ready state whose complete-render result cannot be decoded:
				// the settled probe fails, and the locked path re-reads the same
				// unreadable result and returns the error.
				putBatchRenderState(t, store, id, variant, renderplan.RenderVariantStatusReady)
				ref, err := renderplan.NewRenderVariantArtifactRef(id, variant, renderplan.RenderVariantArtifactResult, "")
				if err != nil {
					t.Fatal(err)
				}
				if err := store.Put(ref.Key, strings.NewReader(`{"shorts":`)); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "render state document malformed",
			corrupt: func(t *testing.T, store *fakeStorage, id uuid.UUID, variant string) {
				t.Helper()
				key := mustRenderStateKey(t, id, variant)
				if err := store.Put(key, strings.NewReader(`{"artifact_prefix":"`)); err != nil {
					t.Fatal(err)
				}
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := newBatchStatusRepo()
			store := newFakeStorage()
			healthy := uuid.New()
			repo.jobs[healthy] = job.Job{ID: healthy, Status: job.StatusParsed}
			broken := uuid.New()
			repo.jobs[broken] = job.Job{ID: broken, Status: job.StatusComposed}
			tc.corrupt(t, store, broken, "viral-60-clean")
			queued := uuid.New()
			repo.jobs[queued] = job.Job{ID: queued, Status: job.StatusComposed}
			putBatchRenderState(t, store, queued, "viral-60-clean", renderplan.RenderVariantStatusQueued)

			h := NewHandlers(repo, store, &fakeQueue{})
			rw := batchStatusRequest(t, h,
				healthy.String()+":viral-60-clean,"+broken.String()+":viral-60-clean,"+queued.String()+":viral-60-clean")
			if rw.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200; body=%s", rw.Code, rw.Body.String())
			}
			body := decodeBatchStatus(t, rw)
			if len(body.Items) != 3 {
				t.Fatalf("items = %d, want 3; body=%s", len(body.Items), rw.Body.String())
			}
			if body.Items[0].Job == nil || body.Items[0].Job.Status != "parsed" || body.Items[0].Error != nil {
				t.Fatalf("healthy item = %+v, want a parsed job and no error", body.Items[0])
			}
			bad := body.Items[1]
			if bad.Error == nil || bad.Error.Code != batchStatusCodeRenderUnreadable {
				t.Fatalf("broken item = %+v, want error code %q", bad, batchStatusCodeRenderUnreadable)
			}
			if bad.Job != nil || bad.Render != nil {
				t.Fatalf("broken item = %+v, want both halves null beside the error", bad)
			}
			if bad.Error.Message == "" || strings.Contains(rw.Body.String(), "artifact_prefix") {
				t.Fatalf("broken item error = %+v, want a generic message with no storage detail", bad.Error)
			}
			if body.Items[2].Render == nil || body.Items[2].Render.Status != "queued" || body.Items[2].Error != nil {
				t.Fatalf("trailing item = %+v, want the queued render the aborted walk used to drop", body.Items[2])
			}
		})
	}
}

// A failed status query is one failure per row, not a 500: every row says so
// explicitly and no render state is read for a job whose status is unknown.
func TestBatchStatusDegradesEveryRowWhenTheStatusQueryFails(t *testing.T) {
	repo := newBatchStatusRepo()
	repo.statusesErr = errors.New("database is locked")
	store := newFakeStorage()
	id := uuid.New()
	repo.jobs[id] = job.Job{ID: id, Status: job.StatusComposed}
	putBatchRenderState(t, store, id, "viral-60-clean", renderplan.RenderVariantStatusQueued)
	stateKey := mustRenderStateKey(t, id, "viral-60-clean")
	other := uuid.New()
	repo.jobs[other] = job.Job{ID: other, Status: job.StatusParsed}
	store.resetOpenCounts()

	h := NewHandlers(repo, store, &fakeQueue{})
	rw := batchStatusRequest(t, h, id.String()+":viral-60-clean,"+other.String()+":viral-60-clean")
	if rw.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rw.Code, rw.Body.String())
	}
	body := decodeBatchStatus(t, rw)
	if len(body.Items) != 2 {
		t.Fatalf("items = %d, want 2; body=%s", len(body.Items), rw.Body.String())
	}
	for i, item := range body.Items {
		if item.Error == nil || item.Error.Code != batchStatusCodeJobUnreadable || item.Job != nil || item.Render != nil {
			t.Fatalf("item %d = %+v, want %q with both halves null", i, item, batchStatusCodeJobUnreadable)
		}
	}
	if strings.Contains(rw.Body.String(), "database is locked") {
		t.Fatalf("response exposed the driver error: %s", rw.Body.String())
	}
	if got := store.openCount(stateKey); got != 0 {
		t.Fatalf("render state opened %d times for a job whose status is unknown, want 0", got)
	}
}

// The batch folded N round trips into one; it must fold the N status queries
// too. A repository without the bulk read still answers, byte for byte.
func TestBatchStatusReadsEveryJobStatusInOneQuery(t *testing.T) {
	first := uuid.MustParse("11111111-1111-4111-8111-111111111111")
	second := uuid.MustParse("22222222-2222-4222-8222-222222222222")
	seed := func(t *testing.T, repo *fakeRepo, store *fakeStorage) {
		t.Helper()
		repo.jobs[first] = job.Job{ID: first, Status: job.StatusComposed}
		repo.jobs[second] = job.Job{ID: second, Status: job.StatusParsed}
		putBatchRenderState(t, store, first, "viral-60-clean", renderplan.RenderVariantStatusQueued)
	}
	// The same job under two variants is two rows and one id to read.
	items := first.String() + ":viral-60-clean," + first.String() + ":viral-aggressive-60," + second.String() + ":viral-60-clean"

	bulkRepo := newBatchStatusRepo()
	bulkStore := newFakeStorage()
	seed(t, bulkRepo.fakeRepo, bulkStore)
	bulk := batchStatusRequest(t, NewHandlers(bulkRepo, bulkStore, &fakeQueue{}), items)
	if bulk.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", bulk.Code, bulk.Body.String())
	}
	if got := bulkRepo.statusesCalls.Load(); got != 1 {
		t.Fatalf("GetStatuses calls = %d, want 1 for a 3-item batch", got)
	}
	if got := bulkRepo.statusCalls.Load(); got != 0 {
		t.Fatalf("GetStatus calls = %d, want 0 once the bulk read answered", got)
	}

	plainRepo := newFakeRepo()
	plainStore := newFakeStorage()
	seed(t, plainRepo, plainStore)
	plain := batchStatusRequest(t, NewHandlers(plainRepo, plainStore, &fakeQueue{}), items)
	if plain.Code != http.StatusOK {
		t.Fatalf("fallback status = %d, want 200; body=%s", plain.Code, plain.Body.String())
	}
	if plain.Body.String() != bulk.Body.String() {
		t.Fatalf("fallback body differs:\n one query: %s\n per item:  %s", bulk.Body.String(), plain.Body.String())
	}
}

// A client that navigated away stops the walk: the remaining render-state reads
// would only fill a response nothing will read.
func TestBatchStatusStopsOnACancelledContext(t *testing.T) {
	repo := newBatchStatusRepo()
	store := newFakeStorage()
	id := uuid.New()
	repo.jobs[id] = job.Job{ID: id, Status: job.StatusComposed}
	putBatchRenderState(t, store, id, "viral-60-clean", renderplan.RenderVariantStatusQueued)
	stateKey := mustRenderStateKey(t, id, "viral-60-clean")
	store.resetOpenCounts()

	h := NewHandlers(repo, store, &fakeQueue{})
	r := chi.NewRouter()
	r.Get("/api/jobs/batch-status", h.BatchStatus)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	req := httptest.NewRequest(http.MethodGet, "/api/jobs/batch-status?items="+id.String()+":viral-60-clean", nil).WithContext(ctx)
	rw := httptest.NewRecorder()
	r.ServeHTTP(rw, req)

	if rw.Body.Len() != 0 {
		t.Fatalf("body = %q, want nothing written for a cancelled request", rw.Body.String())
	}
	if got := store.openCount(stateKey); got != 0 {
		t.Fatalf("render state opened %d times after cancellation, want 0", got)
	}
}
