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

	"github.com/rechedev9/cliphub/internal/mediaassets"
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
