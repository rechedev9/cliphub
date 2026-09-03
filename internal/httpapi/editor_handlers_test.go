package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/artifacts"
	"github.com/rechedev9/cliphub/internal/mediaassets"
	"github.com/rechedev9/cliphub/internal/streamclips"
	"github.com/rechedev9/cliphub/internal/tasks"
	"github.com/rechedev9/cliphub/internal/timelineplan"
)

type fakeEditorAssets struct {
	mu     sync.Mutex
	assets map[uuid.UUID]mediaassets.Asset
}

func newFakeEditorAssets() *fakeEditorAssets {
	return &fakeEditorAssets{assets: map[uuid.UUID]mediaassets.Asset{}}
}

func (r *fakeEditorAssets) Create(_ context.Context, a *mediaassets.Asset) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if a.ID == uuid.Nil {
		a.ID = uuid.New()
	}
	r.assets[a.ID] = *a
	return nil
}

func (r *fakeEditorAssets) Get(_ context.Context, id uuid.UUID) (mediaassets.Asset, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	a, ok := r.assets[id]
	if !ok {
		return mediaassets.Asset{}, mediaassets.ErrNotFound
	}
	return a, nil
}

func (r *fakeEditorAssets) GetBySHA256(_ context.Context, digest string) (mediaassets.Asset, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, a := range r.assets {
		if a.SHA256 == digest {
			return a, nil
		}
	}
	return mediaassets.Asset{}, mediaassets.ErrNotFound
}

func (r *fakeEditorAssets) List(_ context.Context, _ int) ([]mediaassets.Asset, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]mediaassets.Asset, 0, len(r.assets))
	for _, a := range r.assets {
		out = append(out, a)
	}
	return out, nil
}

func (r *fakeEditorAssets) Delete(_ context.Context, id uuid.UUID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.assets, id)
	return nil
}

type fakeEditorProjects struct {
	mu       sync.Mutex
	projects map[uuid.UUID]timelineplan.Project
}

func newFakeEditorProjects() *fakeEditorProjects {
	return &fakeEditorProjects{projects: map[uuid.UUID]timelineplan.Project{}}
}

func (r *fakeEditorProjects) Create(_ context.Context, p *timelineplan.Project) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}
	r.projects[p.ID] = *p
	return nil
}

func (r *fakeEditorProjects) Get(_ context.Context, id uuid.UUID) (timelineplan.Project, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	p, ok := r.projects[id]
	if !ok {
		return timelineplan.Project{}, timelineplan.ErrNotFound
	}
	return p, nil
}

func (r *fakeEditorProjects) List(_ context.Context, _ int) ([]timelineplan.Project, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]timelineplan.Project, 0, len(r.projects))
	for _, p := range r.projects {
		out = append(out, p)
	}
	return out, nil
}

func (r *fakeEditorProjects) Delete(_ context.Context, id uuid.UUID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.projects, id)
	return nil
}

func (r *fakeEditorProjects) UpdateStatus(_ context.Context, id uuid.UUID, s timelineplan.Status, reason string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	p, ok := r.projects[id]
	if !ok {
		return timelineplan.ErrNotFound
	}
	p.Status = s
	p.FailureReason = reason
	r.projects[id] = p
	return nil
}

func (r *fakeEditorProjects) SetPlan(_ context.Context, id uuid.UUID, plan timelineplan.Document) error {
	raw, err := json.Marshal(plan)
	if err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	p, ok := r.projects[id]
	if !ok {
		return timelineplan.ErrNotFound
	}
	p.Plan = raw
	r.projects[id] = p
	return nil
}

func TestEditorProjectPlanAndRenderAdmission(t *testing.T) {
	t.Parallel()
	assets := newFakeEditorAssets()
	projects := newFakeEditorProjects()
	queue := &fakeQueue{}
	h := NewHandlers(newFakeRepo(), newFakeStorage(), queue, WithEditorRepositories(assets, projects))
	srv := httptest.NewServer(Routes(h))
	t.Cleanup(srv.Close)

	create, err := http.NewRequest(http.MethodPost, srv.URL+"/api/editor/projects", strings.NewReader(`{"title":"Ace reel"}`))
	if err != nil {
		t.Fatal(err)
	}
	create.Header.Set("Content-Type", "application/json")
	resp, err := srv.Client().Do(create)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create status = %d", resp.StatusCode)
	}
	var created struct {
		ID   uuid.UUID             `json:"id"`
		Plan timelineplan.Document `json:"plan"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&created); err != nil {
		t.Fatal(err)
	}
	if created.Plan.Canvas.Width != 1080 {
		t.Fatalf("default canvas = %+v", created.Plan.Canvas)
	}
	if len(created.Plan.Tracks) == 0 || created.Plan.Tracks[0].Items == nil {
		t.Fatalf("default plan items must be an empty array, got %+v", created.Plan.Tracks)
	}

	assetID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	plan := timelineplan.DefaultDocument()
	plan.Tracks[0].Items = []timelineplan.Item{{
		ID: "clip-1", AssetID: assetID.String(), SourceIn: 0, SourceOut: 1.5,
	}}
	raw, err := json.Marshal(plan)
	if err != nil {
		t.Fatal(err)
	}
	put, err := http.NewRequest(http.MethodPut, srv.URL+"/api/editor/projects/"+created.ID.String()+"/plan", bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	put.Header.Set("Content-Type", "application/json")
	putResp, err := srv.Client().Do(put)
	if err != nil {
		t.Fatal(err)
	}
	defer putResp.Body.Close()
	if putResp.StatusCode != http.StatusOK {
		t.Fatalf("put plan status = %d", putResp.StatusCode)
	}

	previewBody := strings.NewReader(`{"time":0.2}`)
	preview, err := http.NewRequest(http.MethodPost, srv.URL+"/api/editor/projects/"+created.ID.String()+"/preview", previewBody)
	if err != nil {
		t.Fatal(err)
	}
	preview.Header.Set("Content-Type", "application/json")
	prevResp, err := srv.Client().Do(preview)
	if err != nil {
		t.Fatal(err)
	}
	defer prevResp.Body.Close()
	if prevResp.StatusCode != http.StatusOK {
		t.Fatalf("preview status = %d", prevResp.StatusCode)
	}
	var sample timelineplan.Sample
	if err := json.NewDecoder(prevResp.Body).Decode(&sample); err != nil {
		t.Fatal(err)
	}
	if len(sample.Layers) != 1 || sample.Layers[0].ItemID != "clip-1" {
		t.Fatalf("preview layers = %+v", sample.Layers)
	}

	render, err := http.NewRequest(http.MethodPost, srv.URL+"/api/editor/projects/"+created.ID.String()+"/render", nil)
	if err != nil {
		t.Fatal(err)
	}
	rendResp, err := srv.Client().Do(render)
	if err != nil {
		t.Fatal(err)
	}
	defer rendResp.Body.Close()
	if rendResp.StatusCode != http.StatusAccepted {
		t.Fatalf("render status = %d", rendResp.StatusCode)
	}
	if len(queue.enqueued) != 1 || queue.enqueued[0].Type() != tasks.TypeRenderTimeline {
		t.Fatalf("queue = %#v", queue.enqueued)
	}
}

func TestCreateEditorAssetRejectsMissingFile(t *testing.T) {
	t.Parallel()
	h := NewHandlers(newFakeRepo(), newFakeStorage(), &fakeQueue{}, WithEditorRepositories(newFakeEditorAssets(), newFakeEditorProjects()))
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	_ = w.WriteField("config", `{"file_name":"x.mp4"}`)
	_ = w.Close()
	req := httptest.NewRequest(http.MethodPost, "/api/editor/assets", &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	rw := httptest.NewRecorder()
	Routes(h).ServeHTTP(rw, req)
	if rw.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body=%s", rw.Code, rw.Body.String())
	}
}

func TestStartEditorRenderDoesNotMarkRenderingWhenEnqueueFails(t *testing.T) {
	t.Parallel()
	assets := newFakeEditorAssets()
	projects := newFakeEditorProjects()
	queue := &fakeQueue{err: errors.New("queue is full")}
	h := NewHandlers(newFakeRepo(), newFakeStorage(), queue, WithEditorRepositories(assets, projects))
	srv := httptest.NewServer(Routes(h))
	t.Cleanup(srv.Close)

	create, err := http.NewRequest(http.MethodPost, srv.URL+"/api/editor/projects", strings.NewReader(`{"title":"Ace reel"}`))
	if err != nil {
		t.Fatal(err)
	}
	create.Header.Set("Content-Type", "application/json")
	resp, err := srv.Client().Do(create)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create status = %d", resp.StatusCode)
	}

	list, err := projects.List(context.Background(), 1)
	if err != nil || len(list) != 1 {
		t.Fatalf("list projects: %v %#v", err, list)
	}
	plan := timelineplan.DefaultDocument()
	plan.Tracks[0].Items = []timelineplan.Item{{
		ID: "clip-1", AssetID: "11111111-1111-1111-1111-111111111111", SourceIn: 0, SourceOut: 1,
	}}
	if err := projects.SetPlan(context.Background(), list[0].ID, plan); err != nil {
		t.Fatal(err)
	}

	render, err := http.NewRequest(http.MethodPost, srv.URL+"/api/editor/projects/"+list[0].ID.String()+"/render", nil)
	if err != nil {
		t.Fatal(err)
	}
	rendResp, err := srv.Client().Do(render)
	if err != nil {
		t.Fatal(err)
	}
	rendResp.Body.Close()
	if rendResp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("render status = %d, want 500", rendResp.StatusCode)
	}
	got, err := projects.Get(context.Background(), list[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status == timelineplan.StatusRendering {
		t.Fatalf("status = %s after enqueue failure, want draft", got.Status)
	}
}

// Accepted editor renders live only in the process-local queue; a shutdown
// discard must move the project off `rendering` or the next POST is a
// permanent 409.
func TestStartEditorRenderDiscardFailsAdmittedProject(t *testing.T) {
	t.Parallel()
	assets := newFakeEditorAssets()
	projects := newFakeEditorProjects()
	queue := &fakeQueue{}
	store := newFakeStorage()
	h := NewHandlers(newFakeRepo(), store, queue, WithEditorRepositories(assets, projects))
	srv := httptest.NewServer(Routes(h))
	t.Cleanup(srv.Close)

	create, err := http.NewRequest(http.MethodPost, srv.URL+"/api/editor/projects", strings.NewReader(`{"title":"Ace reel"}`))
	if err != nil {
		t.Fatal(err)
	}
	create.Header.Set("Content-Type", "application/json")
	resp, err := srv.Client().Do(create)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	list, err := projects.List(context.Background(), 1)
	if err != nil || len(list) != 1 {
		t.Fatalf("list projects: %v %#v", err, list)
	}
	plan := timelineplan.DefaultDocument()
	plan.Tracks[0].Items = []timelineplan.Item{{
		ID: "clip-1", AssetID: "11111111-1111-1111-1111-111111111111", SourceIn: 0, SourceOut: 1,
	}}
	if err := projects.SetPlan(context.Background(), list[0].ID, plan); err != nil {
		t.Fatal(err)
	}

	render, err := http.NewRequest(http.MethodPost, srv.URL+"/api/editor/projects/"+list[0].ID.String()+"/render", nil)
	if err != nil {
		t.Fatal(err)
	}
	rendResp, err := srv.Client().Do(render)
	if err != nil {
		t.Fatal(err)
	}
	rendResp.Body.Close()
	if rendResp.StatusCode != http.StatusAccepted {
		t.Fatalf("render status = %d, want 202", rendResp.StatusCode)
	}
	admitted, err := projects.Get(context.Background(), list[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if admitted.Status != timelineplan.StatusRendering {
		t.Fatalf("status after admission = %s, want rendering", admitted.Status)
	}
	if len(queue.transitions) != 1 {
		t.Fatalf("queue transitions = %d, want 1", len(queue.transitions))
	}

	if err := queue.transitions[0](errors.New("inline queue task discarded during shutdown")); err != nil {
		t.Fatalf("discard transition error = %v", err)
	}
	got, err := projects.Get(context.Background(), list[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != timelineplan.StatusFailed || !strings.Contains(got.FailureReason, "discarded during shutdown") {
		t.Fatalf("project after discard = status %s, reason %q; want failed discard reason", got.Status, got.FailureReason)
	}
	state, ok, err := h.readEditorRenderState(list[0].ID)
	if err != nil || !ok {
		t.Fatalf("read editor render state: ok=%v err=%v", ok, err)
	}
	if state.Status != timelineplan.StatusFailed || !strings.Contains(state.Error, "discarded during shutdown") {
		t.Fatalf("render state after discard = status %q, error %q; want failed discard reason", state.Status, state.Error)
	}
}

func TestGetEditorProjectCoercesNullItems(t *testing.T) {
	t.Parallel()
	projects := newFakeEditorProjects()
	h := NewHandlers(newFakeRepo(), newFakeStorage(), &fakeQueue{}, WithEditorRepositories(newFakeEditorAssets(), projects))
	srv := httptest.NewServer(Routes(h))
	t.Cleanup(srv.Close)

	p := &timelineplan.Project{
		Title:  "legacy",
		Status: timelineplan.StatusDraft,
		Plan:   []byte(`{"schema_version":"1.0","canvas":{"width":1080,"height":1920,"fps":60},"tracks":[{"id":"v1","kind":"video","items":null}]}`),
	}
	if err := projects.Create(context.Background(), p); err != nil {
		t.Fatal(err)
	}

	resp, err := srv.Client().Get(srv.URL + "/api/editor/projects/" + p.ID.String())
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	var body bytes.Buffer
	if _, err := body.ReadFrom(resp.Body); err != nil {
		t.Fatal(err)
	}
	got := body.String()
	if strings.Contains(got, `"items":null`) {
		t.Fatalf("GET still serializes null items: %s", got)
	}
	if !strings.Contains(got, `"items":[]`) {
		t.Fatalf("GET missing empty items array: %s", got)
	}
}

func TestEditorNotConfigured(t *testing.T) {
	t.Parallel()
	h := NewHandlers(newFakeRepo(), newFakeStorage(), &fakeQueue{})
	req := httptest.NewRequest(http.MethodGet, "/api/editor/projects", nil)
	rw := httptest.NewRecorder()
	Routes(h).ServeHTTP(rw, req)
	if rw.Code != http.StatusNotImplemented {
		t.Fatalf("status = %d, want 501", rw.Code)
	}
}

func TestImportEditorAsset(t *testing.T) {
	t.Parallel()
	jobID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	demoKey, err := artifacts.RenderVariantVideoKey(jobID, "viral-60-clean", "ace")
	if err != nil {
		t.Fatal(err)
	}
	streamKey, err := streamclips.RenderVideoKey(jobID, streamclips.VariantStreamer4060, "clip-1")
	if err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name       string
		configure  bool
		seedKey    string
		body       string
		wantStatus int
		wantSubstr string
		wantOrigin mediaassets.Origin
	}{
		{
			name:       "not configured",
			body:       `{"source":"demo","job_id":"` + jobID.String() + `"}`,
			wantStatus: http.StatusNotImplemented,
		},
		{
			name:       "invalid json",
			configure:  true,
			body:       `{`,
			wantStatus: http.StatusBadRequest,
			wantSubstr: "invalid import JSON",
		},
		{
			name:       "invalid job id",
			configure:  true,
			body:       `{"source":"demo","job_id":"nope"}`,
			wantStatus: http.StatusBadRequest,
			wantSubstr: "invalid job id",
		},
		{
			name:       "invalid source",
			configure:  true,
			body:       `{"source":"upload","job_id":"` + jobID.String() + `"}`,
			wantStatus: http.StatusBadRequest,
			wantSubstr: "source must be demo or stream",
		},
		{
			name:       "missing demo video",
			configure:  true,
			body:       `{"source":"demo","job_id":"` + jobID.String() + `","variant":"viral-60-clean","name":"ace"}`,
			wantStatus: http.StatusNotFound,
			wantSubstr: "source video not found",
		},
		{
			name:       "imports demo render",
			configure:  true,
			seedKey:    demoKey,
			body:       `{"source":"demo","job_id":"` + jobID.String() + `","variant":"viral-60-clean","name":"ace"}`,
			wantStatus: http.StatusCreated,
			wantOrigin: mediaassets.OriginDemoRender,
		},
		{
			name:       "imports stream render",
			configure:  true,
			seedKey:    streamKey,
			body:       `{"source":"stream","job_id":"` + jobID.String() + `","variant":"` + streamclips.VariantStreamer4060 + `","name":"clip-1"}`,
			wantStatus: http.StatusCreated,
			wantOrigin: mediaassets.OriginStreamRender,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			store := newFakeStorage()
			if tc.seedKey != "" {
				if err := store.Put(tc.seedKey, strings.NewReader("fake-mp4")); err != nil {
					t.Fatal(err)
				}
			}
			var opts []Option
			if tc.configure {
				opts = append(opts, WithEditorRepositories(newFakeEditorAssets(), newFakeEditorProjects()))
			}
			h := NewHandlers(newFakeRepo(), store, &fakeQueue{}, opts...)
			req := httptest.NewRequest(http.MethodPost, "/api/editor/assets/import", strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			rw := httptest.NewRecorder()
			Routes(h).ServeHTTP(rw, req)
			if rw.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d; body=%s", rw.Code, tc.wantStatus, rw.Body.String())
			}
			if tc.wantSubstr != "" && !strings.Contains(rw.Body.String(), tc.wantSubstr) {
				t.Fatalf("body = %s, want substring %q", rw.Body.String(), tc.wantSubstr)
			}
			if tc.wantStatus != http.StatusCreated {
				return
			}
			var asset mediaassets.Asset
			if err := json.Unmarshal(rw.Body.Bytes(), &asset); err != nil {
				t.Fatal(err)
			}
			if asset.Origin != tc.wantOrigin {
				t.Fatalf("origin = %s, want %s", asset.Origin, tc.wantOrigin)
			}
		})
	}
}
