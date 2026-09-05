package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/storage"
	"github.com/rechedev9/cliphub/internal/streamclips"
)

func TestStreamRenderedOutputsComeFromPublishedRevision(t *testing.T) {
	repo := newFakeStreamRepo()
	store := newFakeStorage()
	jobID := uuid.New()
	plan := streamclips.DefaultEditPlan()
	plan.Clips = []streamclips.ClipRange{{ID: "draft-clip", StartSeconds: 0, EndSeconds: 3, Title: "Published title"}}
	planJSON, err := json.Marshal(plan)
	if err != nil {
		t.Fatal(err)
	}
	repo.jobs[jobID] = streamclips.Job{ID: jobID, Status: streamclips.StatusRendered, EditPlan: planJSON}

	revision := uuid.New()
	videoKey, _ := streamclips.RenderRevisionVideoKey(jobID, plan.Variant, revision, "draft-clip")
	resultKey, _ := streamclips.RenderRevisionResultKey(jobID, plan.Variant, revision)
	galleryKey, _ := streamclips.RenderRevisionGalleryKey(jobID, plan.Variant, revision)
	prefix, _ := streamclips.RenderRevisionPrefix(jobID, plan.Variant, revision)
	planKey, _ := streamclips.RenderRevisionDeliveryKey(jobID, plan.Variant, revision, "edit-plan.json")
	coverKey, _ := streamclips.RenderRevisionDeliveryKey(jobID, plan.Variant, revision, "cover.jpg")
	result, err := streamclips.NewRenderResult(jobID, plan.Variant, []streamclips.VideoEntry{{
		ClipID: "draft-clip", Title: "Published title", Key: videoKey, DurationSeconds: 3,
	}}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	result.Warnings = []string{"freeze at 00:01"}
	putStreamOutputJSON(t, store, resultKey, result)
	putStreamOutputJSON(t, store, planKey, plan)
	_ = store.Put(videoKey, strings.NewReader("revision-one-video"))
	_ = store.Put(coverKey, strings.NewReader("cover"))
	state, err := streamclips.NewRenderState(jobID, plan.Variant, streamclips.StatusRendered, nil, "", result.Clips)
	if err != nil {
		t.Fatal(err)
	}
	state.ResultKey, state.GalleryKey, state.ArtifactDir = resultKey, galleryKey, prefix
	state.Delivery = []streamclips.DeliveryEntry{
		{Name: "edit-plan.json", Kind: "plan", Key: planKey},
		{Name: "cover.jpg", Kind: "cover", Key: coverKey},
	}
	h := NewHandlers(newFakeRepo(), store, &fakeQueue{}, WithStreamRepository(repo))
	if err := h.writeStreamRenderState(state); err != nil {
		t.Fatal(err)
	}

	got := getStreamListOutput(t, h, jobID)
	if got.ArtifactRevision != revision.String() || got.ClipID != "draft-clip" || got.Title != "Published title" || got.DurationSeconds != 3 || got.AspectRatio != "9:16" {
		t.Fatalf("published output = %+v", got)
	}
	if got.Stale || !got.ReviewRequired || len(got.Warnings) != 1 {
		t.Fatalf("published review projection = %+v", got)
	}
	if !strings.Contains(got.VideoURL, "/revisions/"+revision.String()+"/videos/draft-clip") || !strings.Contains(got.CoverURL, "/revisions/"+revision.String()+"/delivery/cover.jpg") {
		t.Fatalf("revision URLs = video %q cover %q", got.VideoURL, got.CoverURL)
	}
	detail := httptest.NewRecorder()
	Routes(h).ServeHTTP(detail, httptest.NewRequest(http.MethodGet, "/api/stream-jobs/"+jobID.String(), nil))
	if detail.Code != http.StatusOK || !strings.Contains(detail.Body.String(), `"rendered_outputs":[`) || !strings.Contains(detail.Body.String(), `"edit_plan":`) {
		t.Fatalf("detail response = %d %s", detail.Code, detail.Body.String())
	}

	// A changed draft marks the committed output stale but cannot invent a
	// library item for a clip which has never been rendered.
	changed := plan
	changed.Clips = []streamclips.ClipRange{{ID: "new-draft-only", StartSeconds: 4, EndSeconds: 8}}
	changedJSON, _ := json.Marshal(changed)
	j := repo.jobs[jobID]
	j.EditPlan = changedJSON
	repo.jobs[jobID] = j
	for _, attemptStatus := range []streamclips.Status{streamclips.StatusRendering, streamclips.StatusFailed} {
		state.Status = attemptStatus
		state.Published = true
		if err := h.writeStreamRenderState(state); err != nil {
			t.Fatal(err)
		}
		got = getStreamListOutput(t, h, jobID)
		if !got.Stale || !got.ReviewRequired || got.RenderStatus != streamclips.StatusRendered || got.ClipID != "draft-clip" || len(got.Warnings) != 1 {
			t.Fatalf("changed draft with %s attempt output = %+v", attemptStatus, got)
		}
	}

	// The list performs an existence check without opening media bytes and
	// omits a deleted artifact instead of advertising a phantom output.
	delete(store.puts, videoKey)
	rr := httptest.NewRecorder()
	Routes(h).ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/stream-jobs?limit=10", nil))
	if rr.Code != http.StatusOK || strings.Contains(rr.Body.String(), `"clip_id":"draft-clip"`) {
		t.Fatalf("list with missing video = %d %s", rr.Code, rr.Body.String())
	}
}

func TestStreamRevisionURLDoesNotFollowReplacement(t *testing.T) {
	repo := newFakeStreamRepo()
	store := newFakeStorage()
	jobID := uuid.New()
	variant := streamclips.DefaultVariant().Name
	repo.jobs[jobID] = streamclips.Job{ID: jobID, Status: streamclips.StatusRendered}
	h := NewHandlers(newFakeRepo(), store, &fakeQueue{}, WithStreamRepository(repo))

	firstRevision := uuid.New()
	firstKey, _ := streamclips.RenderRevisionVideoKey(jobID, variant, firstRevision, "same-clip")
	_ = store.Put(firstKey, strings.NewReader("first-revision"))
	firstURL := "/api/stream-jobs/" + jobID.String() + "/renders/" + variant + "/revisions/" + firstRevision.String() + "/videos/same-clip"

	secondRevision := uuid.New()
	secondKey, _ := streamclips.RenderRevisionVideoKey(jobID, variant, secondRevision, "same-clip")
	_ = store.Put(secondKey, strings.NewReader("second-revision"))
	state, _ := streamclips.NewRenderState(jobID, variant, streamclips.StatusRendered, nil, "", []streamclips.VideoEntry{{ClipID: "same-clip", Key: secondKey}})
	state.ArtifactDir, _ = streamclips.RenderRevisionPrefix(jobID, variant, secondRevision)
	state.ResultKey, _ = streamclips.RenderRevisionResultKey(jobID, variant, secondRevision)
	state.GalleryKey, _ = streamclips.RenderRevisionGalleryKey(jobID, variant, secondRevision)
	if err := h.writeStreamRenderState(state); err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	Routes(h).ServeHTTP(rr, httptest.NewRequest(http.MethodGet, firstURL, nil))
	if rr.Code != http.StatusOK || rr.Body.String() != "first-revision" {
		t.Fatalf("old revision response = %d %q", rr.Code, rr.Body.String())
	}
	delete(store.puts, firstKey)
	rr = httptest.NewRecorder()
	Routes(h).ServeHTTP(rr, httptest.NewRequest(http.MethodGet, firstURL, nil))
	if rr.Code != http.StatusNotFound || !strings.Contains(rr.Body.String(), `"code":"not_found"`) {
		t.Fatalf("missing old revision response = %d %s", rr.Code, rr.Body.String())
	}
}

func TestLegacyStreamRevisionURLExpiresWhenPointerMoves(t *testing.T) {
	repo := newFakeStreamRepo()
	store := newFakeStorage()
	jobID := uuid.New()
	variant := streamclips.DefaultVariant().Name
	repo.jobs[jobID] = streamclips.Job{ID: jobID, Status: streamclips.StatusRendered}
	h := NewHandlers(newFakeRepo(), store, &fakeQueue{}, WithStreamRepository(repo))

	legacyKey, _ := streamclips.RenderVideoKey(jobID, variant, "clip-one")
	_ = store.Put(legacyKey, strings.NewReader("legacy-video"))
	legacy, _ := streamclips.NewRenderState(jobID, variant, streamclips.StatusRendered, nil, "", []streamclips.VideoEntry{{ClipID: "clip-one", Key: legacyKey}})
	if err := h.writeStreamRenderState(legacy); err != nil {
		t.Fatal(err)
	}
	legacyRevision, err := streamArtifactRevision(legacy)
	if err != nil {
		t.Fatal(err)
	}
	legacyURL := "/api/stream-jobs/" + jobID.String() + "/renders/" + variant + "/revisions/" + legacyRevision + "/videos/clip-one"
	router := Routes(h)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, legacyURL, nil))
	if rr.Code != http.StatusOK || rr.Body.String() != "legacy-video" {
		t.Fatalf("legacy revision response = %d %q", rr.Code, rr.Body.String())
	}

	newRevision := uuid.New()
	newKey, _ := streamclips.RenderRevisionVideoKey(jobID, variant, newRevision, "clip-one")
	modern, _ := streamclips.NewRenderState(jobID, variant, streamclips.StatusRendered, nil, "", []streamclips.VideoEntry{{ClipID: "clip-one", Key: newKey}})
	modern.ArtifactDir, _ = streamclips.RenderRevisionPrefix(jobID, variant, newRevision)
	modern.ResultKey, _ = streamclips.RenderRevisionResultKey(jobID, variant, newRevision)
	modern.GalleryKey, _ = streamclips.RenderRevisionGalleryKey(jobID, variant, newRevision)
	if err := h.writeStreamRenderState(modern); err != nil {
		t.Fatal(err)
	}
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, legacyURL, nil))
	if rr.Code != http.StatusNotFound {
		t.Fatalf("expired legacy revision response = %d %s", rr.Code, rr.Body.String())
	}
}

func TestStreamRevisionRoutesRejectTraversal(t *testing.T) {
	repo := newFakeStreamRepo()
	jobID := uuid.New()
	repo.jobs[jobID] = streamclips.Job{ID: jobID, Status: streamclips.StatusRendered}
	h := NewHandlers(newFakeRepo(), newFakeStorage(), &fakeQueue{}, WithStreamRepository(repo))
	base := "/api/stream-jobs/" + jobID.String() + "/renders/" + streamclips.DefaultVariant().Name + "/revisions/"
	for _, requestPath := range []string{
		base + "not-a-revision/videos/clip-one",
		base + uuid.NewString() + "/videos/%2e%2e%2fsecret",
		base + uuid.NewString() + "/delivery/%2e%2e%2fedit-plan.json",
	} {
		t.Run(requestPath, func(t *testing.T) {
			rr := httptest.NewRecorder()
			Routes(h).ServeHTTP(rr, httptest.NewRequest(http.MethodGet, requestPath, nil))
			if rr.Code == http.StatusOK {
				t.Fatalf("traversal request unexpectedly succeeded: %s", rr.Body.String())
			}
		})
	}
}

func TestStreamRevisionVideoHonorsRangeRequests(t *testing.T) {
	repo := newFakeStreamRepo()
	jobID := uuid.New()
	variant := streamclips.DefaultVariant().Name
	revision := uuid.New()
	repo.jobs[jobID] = streamclips.Job{ID: jobID, Status: streamclips.StatusRendered}
	store, err := storage.NewLocal(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	key, _ := streamclips.RenderRevisionVideoKey(jobID, variant, revision, "clip-one")
	if err := store.Put(key, strings.NewReader("0123456789")); err != nil {
		t.Fatal(err)
	}
	h := NewHandlers(newFakeRepo(), store, &fakeQueue{}, WithStreamRepository(repo))
	requestPath := "/api/stream-jobs/" + jobID.String() + "/renders/" + variant + "/revisions/" + revision.String() + "/videos/clip-one"
	req := httptest.NewRequest(http.MethodGet, requestPath, nil)
	req.Header.Set("Range", "bytes=2-5")
	rr := httptest.NewRecorder()
	Routes(h).ServeHTTP(rr, req)
	if rr.Code != http.StatusPartialContent || rr.Body.String() != "2345" || rr.Header().Get("Content-Range") != "bytes 2-5/10" {
		t.Fatalf("range response = %d %q Content-Range=%q", rr.Code, rr.Body.String(), rr.Header().Get("Content-Range"))
	}
	if got := rr.Header().Get("Cache-Control"); got != artifactCacheImmutable {
		t.Fatalf("Cache-Control = %q, want %q", got, artifactCacheImmutable)
	}
	if got := rr.Header().Get("Last-Modified"); got == "" {
		t.Fatal("Last-Modified is empty for local immutable stream artifact")
	}
}

func TestStreamRenderedOutputsOmitOnlyMissingItem(t *testing.T) {
	repo := newFakeStreamRepo()
	store := newFakeStorage()
	jobID := uuid.New()
	variant := streamclips.VariantStreamerLandscape16x9
	plan := streamclips.DefaultEditPlan()
	plan.Variant = variant
	plan.Clips = []streamclips.ClipRange{
		{ID: "present", StartSeconds: 0, EndSeconds: 1},
		{ID: "missing", StartSeconds: 1, EndSeconds: 2},
	}
	planJSON, _ := json.Marshal(plan)
	repo.jobs[jobID] = streamclips.Job{ID: jobID, Status: streamclips.StatusRendered, EditPlan: planJSON}
	revision := uuid.New()
	prefix, _ := streamclips.RenderRevisionPrefix(jobID, variant, revision)
	resultKey, _ := streamclips.RenderRevisionResultKey(jobID, variant, revision)
	galleryKey, _ := streamclips.RenderRevisionGalleryKey(jobID, variant, revision)
	planKey, _ := streamclips.RenderRevisionDeliveryKey(jobID, variant, revision, "edit-plan.json")
	presentKey, _ := streamclips.RenderRevisionVideoKey(jobID, variant, revision, "present")
	missingKey, _ := streamclips.RenderRevisionVideoKey(jobID, variant, revision, "missing")
	videos := []streamclips.VideoEntry{{ClipID: "present", Key: presentKey}, {ClipID: "missing", Key: missingKey}}
	result, _ := streamclips.NewRenderResult(jobID, variant, videos, time.Now())
	putStreamOutputJSON(t, store, resultKey, result)
	putStreamOutputJSON(t, store, planKey, plan)
	_ = store.Put(presentKey, strings.NewReader("present-video"))
	state, _ := streamclips.NewRenderState(jobID, variant, streamclips.StatusRendered, nil, "", videos)
	state.ResultKey, state.GalleryKey, state.ArtifactDir = resultKey, galleryKey, prefix
	state.Delivery = []streamclips.DeliveryEntry{{Name: "edit-plan.json", Kind: "plan", Key: planKey}}
	h := NewHandlers(newFakeRepo(), store, &fakeQueue{}, WithStreamRepository(repo))
	if err := h.writeStreamRenderState(state); err != nil {
		t.Fatal(err)
	}
	output := getStreamListOutput(t, h, jobID)
	if output.ClipID != "present" || output.AspectRatio != "16:9" {
		t.Fatalf("remaining output = %+v", output)
	}
}

func TestStreamListIsolatesCorruptRenderedOutputsToOneJob(t *testing.T) {
	repo := newFakeStreamRepo()
	store := newFakeStorage()
	variant := streamclips.DefaultVariant().Name
	validID := uuid.New()
	corruptID := uuid.New()
	for _, id := range []uuid.UUID{validID, corruptID} {
		repo.jobs[id] = streamclips.Job{ID: id, Status: streamclips.StatusRendered, CreatedAt: time.Now()}
	}

	revision := uuid.New()
	videoKey, _ := streamclips.RenderRevisionVideoKey(validID, variant, revision, "valid-clip")
	resultKey, _ := streamclips.RenderRevisionResultKey(validID, variant, revision)
	galleryKey, _ := streamclips.RenderRevisionGalleryKey(validID, variant, revision)
	prefix, _ := streamclips.RenderRevisionPrefix(validID, variant, revision)
	videos := []streamclips.VideoEntry{{ClipID: "valid-clip", Key: videoKey}}
	result, _ := streamclips.NewRenderResult(validID, variant, videos, time.Now())
	putStreamOutputJSON(t, store, resultKey, result)
	_ = store.Put(videoKey, strings.NewReader("valid-video"))
	state, _ := streamclips.NewRenderState(validID, variant, streamclips.StatusRendered, nil, "", videos)
	state.ResultKey, state.GalleryKey, state.ArtifactDir = resultKey, galleryKey, prefix
	h := NewHandlers(newFakeRepo(), store, &fakeQueue{}, WithStreamRepository(repo))
	if err := h.writeStreamRenderState(state); err != nil {
		t.Fatal(err)
	}
	corruptStateKey, _ := streamclips.RenderStateKey(corruptID, variant)
	_ = store.Put(corruptStateKey, strings.NewReader(`{"not":"valid render state"}`))

	rr := httptest.NewRecorder()
	Routes(h).ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/stream-jobs?limit=10", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("list response = %d %s", rr.Code, rr.Body.String())
	}
	var response struct {
		Jobs []struct {
			ID                         uuid.UUID              `json:"id"`
			RenderedOutputs            []streamRenderedOutput `json:"rendered_outputs"`
			RenderedOutputsUnavailable bool                   `json:"rendered_outputs_unavailable"`
		} `json:"jobs"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if len(response.Jobs) != 2 {
		t.Fatalf("jobs = %d, want 2: %s", len(response.Jobs), rr.Body.String())
	}
	for _, job := range response.Jobs {
		switch job.ID {
		case validID:
			if job.RenderedOutputsUnavailable || len(job.RenderedOutputs) != 1 || job.RenderedOutputs[0].ClipID != "valid-clip" {
				t.Fatalf("valid sibling = %+v", job)
			}
		case corruptID:
			if !job.RenderedOutputsUnavailable || len(job.RenderedOutputs) != 0 {
				t.Fatalf("corrupt sibling = %+v", job)
			}
		default:
			t.Fatalf("unexpected job %s", job.ID)
		}
	}
}

func putStreamOutputJSON(t *testing.T, store *fakeStorage, key string, value any) {
	t.Helper()
	body, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Put(key, bytes.NewReader(body)); err != nil {
		t.Fatal(err)
	}
}

func getStreamListOutput(t *testing.T, h *Handlers, jobID uuid.UUID) streamRenderedOutput {
	t.Helper()
	rr := httptest.NewRecorder()
	Routes(h).ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/stream-jobs?limit=10", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("list response = %d %s", rr.Code, rr.Body.String())
	}
	var body struct {
		Jobs []struct {
			ID              uuid.UUID              `json:"id"`
			RenderedOutputs []streamRenderedOutput `json:"rendered_outputs"`
		} `json:"jobs"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	for _, job := range body.Jobs {
		if job.ID == jobID && len(job.RenderedOutputs) == 1 {
			return job.RenderedOutputs[0]
		}
	}
	t.Fatalf("job %s did not have one rendered output: %s", jobID, rr.Body.String())
	return streamRenderedOutput{}
}
