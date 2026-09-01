package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/klauspost/compress/zstd"

	"github.com/rechedev9/cliphub/internal/artifacts"
	"github.com/rechedev9/cliphub/internal/composition"
	"github.com/rechedev9/cliphub/internal/editor"
	"github.com/rechedev9/cliphub/internal/job"
	"github.com/rechedev9/cliphub/internal/killplan"
	"github.com/rechedev9/cliphub/internal/moments"
	"github.com/rechedev9/cliphub/internal/obs"
	"github.com/rechedev9/cliphub/internal/recapplan"
	"github.com/rechedev9/cliphub/internal/recording"
	"github.com/rechedev9/cliphub/internal/renderplan"
	"github.com/rechedev9/cliphub/internal/rules"
	"github.com/rechedev9/cliphub/internal/storage"
	"github.com/rechedev9/cliphub/internal/streamclips"
	"github.com/rechedev9/cliphub/internal/tasks"
)

func renderVariantTestKey(kind renderplan.RenderVariantArtifactKind) func(uuid.UUID, string, string) (string, error) {
	return func(id uuid.UUID, variant, name string) (string, error) {
		ref, err := renderplan.NewRenderVariantArtifactRef(id, variant, kind, name)
		return ref.Key, err
	}
}

// fakeRepo implements JobRepository for tests.
type fakeRepo struct {
	jobs            map[uuid.UUID]job.Job
	getErr          error
	deleteErr       error
	updateHonorsCtx bool
}

type fakeStreamRepo struct {
	jobs map[uuid.UUID]streamclips.Job
}

type blockingSetStreamRepo struct {
	*fakeStreamRepo
	entered chan struct{}
	release chan struct{}
}

func (r *blockingSetStreamRepo) SetEditPlan(ctx context.Context, id uuid.UUID, plan streamclips.EditPlan) error {
	close(r.entered)
	select {
	case <-r.release:
	case <-ctx.Done():
		return ctx.Err()
	}
	return r.fakeStreamRepo.SetEditPlan(ctx, id, plan)
}

func newFakeStreamRepo() *fakeStreamRepo {
	return &fakeStreamRepo{jobs: map[uuid.UUID]streamclips.Job{}}
}

func reviewedDefaultEditPlanJSON(t *testing.T) json.RawMessage {
	t.Helper()
	plan := streamclips.DefaultEditPlan()
	plan.FaceCropReviewed = true
	plan.Clips = []streamclips.ClipRange{{ID: "clip-001", StartSeconds: 0, EndSeconds: 1}}
	raw, err := json.Marshal(plan)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func (f *fakeStreamRepo) Create(_ context.Context, j *streamclips.Job) error {
	f.jobs[j.ID] = *j
	return nil
}

func (f *fakeStreamRepo) Get(_ context.Context, id uuid.UUID) (streamclips.Job, error) {
	j, ok := f.jobs[id]
	if !ok {
		return streamclips.Job{}, streamclips.ErrNotFound
	}
	return j, nil
}

func (f *fakeStreamRepo) List(_ context.Context, limit int) ([]streamclips.Job, error) {
	jobs := make([]streamclips.Job, 0, len(f.jobs))
	for _, j := range f.jobs {
		jobs = append(jobs, j)
		if len(jobs) == limit {
			break
		}
	}
	return jobs, nil
}

func (f *fakeStreamRepo) UpdateStatus(_ context.Context, id uuid.UUID, s streamclips.Status, reason string) error {
	j, ok := f.jobs[id]
	if !ok {
		return streamclips.ErrNotFound
	}
	j.Status = s
	j.FailureReason = reason
	j.FailureCode = jobFailureCode(reason, "")
	if s == streamclips.StatusFailed {
		j.SourceURL = ""
	}
	f.jobs[id] = j
	return nil
}

func (f *fakeStreamRepo) SetEditPlan(_ context.Context, id uuid.UUID, plan streamclips.EditPlan) error {
	j, ok := f.jobs[id]
	if !ok {
		return streamclips.ErrNotFound
	}
	b, err := json.Marshal(plan)
	if err != nil {
		return err
	}
	j.EditPlan = b
	j.Status = streamclips.StatusReady
	f.jobs[id] = j
	return nil
}

func (f *fakeStreamRepo) SetAcquired(_ context.Context, id uuid.UUID, probe streamclips.SourceProbe, sha256, discoveredTitle string) error {
	j, ok := f.jobs[id]
	if !ok {
		return streamclips.ErrNotFound
	}
	j.Probe = probe
	j.SourceSHA256 = sha256
	if j.Title == "" {
		j.Title = discoveredTitle
	}
	j.Status = streamclips.StatusReady
	j.FailureReason = ""
	j.SourceURL = ""
	f.jobs[id] = j
	return nil
}

func newFakeRepo() *fakeRepo { return &fakeRepo{jobs: map[uuid.UUID]job.Job{}} }
func (f *fakeRepo) Create(_ context.Context, j *job.Job) error {
	if j.ID == uuid.Nil {
		j.ID = uuid.New()
	}
	f.jobs[j.ID] = *j
	return nil
}
func (f *fakeRepo) Get(_ context.Context, id uuid.UUID) (job.Job, error) {
	if f.getErr != nil {
		return job.Job{}, f.getErr
	}
	j, ok := f.jobs[id]
	if !ok {
		return job.Job{}, job.ErrNotFound
	}
	return j, nil
}

func (f *fakeRepo) GetMeta(ctx context.Context, id uuid.UUID) (job.Job, error) {
	j, err := f.Get(ctx, id)
	if err != nil {
		return job.Job{}, err
	}
	j.KillPlan = nil
	return j, nil
}

func (f *fakeRepo) GetStatus(ctx context.Context, id uuid.UUID) (job.Status, string, int, error) {
	j, err := f.Get(ctx, id)
	if err != nil {
		return 0, "", 0, err
	}
	segmentCount := 0
	if j.Status == job.StatusRecording && j.KillPlan != nil {
		segmentCount = len(j.KillPlan.Segments)
	}
	return j.Status, j.FailureReason, segmentCount, nil
}
func (f *fakeRepo) List(_ context.Context, limit int) ([]job.Job, error) {
	jobs := make([]job.Job, 0, len(f.jobs))
	for _, j := range f.jobs {
		j.KillPlan = nil
		jobs = append(jobs, j)
		if len(jobs) == limit {
			break
		}
	}
	return jobs, nil
}
func (f *fakeRepo) ListBySeries(_ context.Context, seriesID string) ([]job.Job, error) {
	jobs := make([]job.Job, 0, len(f.jobs))
	for _, j := range f.jobs {
		if j.SeriesID == seriesID {
			j.KillPlan = nil
			jobs = append(jobs, j)
		}
	}
	sort.Slice(jobs, func(i, k int) bool {
		if jobs[i].CreatedAt.Equal(jobs[k].CreatedAt) {
			return jobs[i].ID.String() < jobs[k].ID.String()
		}
		return jobs[i].CreatedAt.Before(jobs[k].CreatedAt)
	})
	return jobs, nil
}
func (f *fakeRepo) UpdateStatus(ctx context.Context, id uuid.UUID, s job.Status, reason string) error {
	if f.updateHonorsCtx {
		if err := ctx.Err(); err != nil {
			return err
		}
	}
	j, ok := f.jobs[id]
	if !ok {
		return job.ErrNotFound
	}
	j.Status = s
	j.FailureReason = reason
	j.FailureCode = obs.ClassOf(reason)
	f.jobs[id] = j
	return nil
}
func (f *fakeRepo) Delete(_ context.Context, id uuid.UUID) error {
	if f.deleteErr != nil {
		return f.deleteErr
	}
	delete(f.jobs, id)
	return nil
}
func (f *fakeRepo) SetParseInputs(_ context.Context, id uuid.UUID, steamID string, r rules.Rules) error {
	j, ok := f.jobs[id]
	if !ok {
		return job.ErrNotFound
	}
	if j.Status != job.StatusScanned && j.Status != job.StatusParsed {
		return job.ErrConflict
	}
	j.TargetSteamID = steamID
	j.Rules = r
	j.Status = job.StatusParsing
	f.jobs[id] = j
	return nil
}

// fakeStorage records every Put call.
type fakeStorage struct {
	puts     map[string][]byte
	deleted  []string
	onDelete func(string)
}

func newFakeStorage() *fakeStorage { return &fakeStorage{puts: map[string][]byte{}} }
func (f *fakeStorage) Put(key string, r io.Reader) error {
	b, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	f.puts[key] = b
	return nil
}
func (f *fakeStorage) Open(key string) (io.ReadCloser, error) {
	b, ok := f.puts[key]
	if !ok {
		return nil, os.ErrNotExist
	}
	return io.NopCloser(bytes.NewReader(b)), nil
}
func (f *fakeStorage) Exists(key string) (bool, error) {
	_, ok := f.puts[key]
	return ok, nil
}
func (f *fakeStorage) Delete(key string) error {
	delete(f.puts, key)
	f.deleted = append(f.deleted, key)
	if f.onDelete != nil {
		f.onDelete(key)
	}
	return nil
}

// DeleteTree removes every stored key under the given prefix, mirroring the
// recursive delete the local filesystem backend provides.
func (f *fakeStorage) DeleteTree(key string) error {
	prefix := key + "/"
	for k := range f.puts {
		if k == key || strings.HasPrefix(k, prefix) {
			delete(f.puts, k)
		}
	}
	return nil
}

// fakeQueue captures enqueued tasks.
type fakeQueue struct {
	enqueued    []*asynq.Task
	options     [][]asynq.Option
	transitions []func(error) error
	err         error
}

func (q *fakeQueue) Enqueue(t *asynq.Task, opts ...asynq.Option) (*asynq.TaskInfo, error) {
	return q.enqueue(t, nil, opts...)
}

func (q *fakeQueue) EnqueueWithTransition(t *asynq.Task, transition func(error) error, opts ...asynq.Option) (*asynq.TaskInfo, error) {
	return q.enqueue(t, transition, opts...)
}

func (q *fakeQueue) enqueue(t *asynq.Task, transition func(error) error, opts ...asynq.Option) (*asynq.TaskInfo, error) {
	if transition != nil {
		if err := transition(q.err); err != nil {
			return nil, err
		}
	}
	if q.err != nil {
		return nil, q.err
	}
	q.enqueued = append(q.enqueued, t)
	q.options = append(q.options, opts)
	if transition != nil {
		q.transitions = append(q.transitions, transition)
	}
	return &asynq.TaskInfo{ID: "x"}, nil
}

// demoMagic is the CS2 (Source 2) demo header CreateJob validates against.
var demoMagic = []byte("PBDEMS2\x00")

func TestDemoUploadSizeLimits(t *testing.T) {
	if got, want := maxDemoBytes, 700<<20; got != want {
		t.Fatalf("maxDemoBytes = %d, want %d", got, want)
	}
	if got, want := maxMultipartBytes, maxDemoBytes+1<<20; got != want {
		t.Fatalf("maxMultipartBytes = %d, want %d", got, want)
	}
}

// multipartBody builds a CreateJob upload whose demo bytes start with a valid
// CS2 demo header, so it exercises the happy path. Tests that need an invalid
// header build their own body.
func multipartBody(t *testing.T, demoBytes []byte, configJSON string) (*bytes.Buffer, string) {
	t.Helper()
	body := &bytes.Buffer{}
	mw := multipart.NewWriter(body)
	demoPart, _ := mw.CreateFormFile("demo", "test.dem")
	demoPart.Write(demoMagic)
	demoPart.Write(demoBytes)
	mw.WriteField("config", configJSON)
	mw.Close()
	return body, mw.FormDataContentType()
}

// multipartBodyRaw builds a CreateJob upload with exactly the given demo bytes,
// for tests that assert on the magic-byte validation itself.
func multipartBodyRaw(t *testing.T, demoBytes []byte, configJSON string) (*bytes.Buffer, string) {
	t.Helper()
	body := &bytes.Buffer{}
	mw := multipart.NewWriter(body)
	demoPart, _ := mw.CreateFormFile("demo", "test.dem")
	demoPart.Write(demoBytes)
	mw.WriteField("config", configJSON)
	mw.Close()
	return body, mw.FormDataContentType()
}

// multipartBodyFields builds a CreateJob upload with a valid demo header, the
// given demo file name, and arbitrary extra form fields (e.g. config,
// series_id). It is used by the series/file-name tests.
func multipartBodyFields(t *testing.T, filename string, demoBytes []byte, fields map[string]string) (*bytes.Buffer, string) {
	t.Helper()
	body := &bytes.Buffer{}
	mw := multipart.NewWriter(body)
	demoPart, _ := mw.CreateFormFile("demo", filename)
	demoPart.Write(demoMagic)
	demoPart.Write(demoBytes)
	for k, v := range fields {
		mw.WriteField(k, v)
	}
	mw.Close()
	return body, mw.FormDataContentType()
}

func TestPostJobsCreatesJobAndEnqueues(t *testing.T) {
	repo := newFakeRepo()
	store := newFakeStorage()
	queue := &fakeQueue{}
	h := NewHandlers(repo, store, queue)

	body, ct := multipartBody(t, []byte("dem-bytes"), `{"target_steamid":"76561198000000000"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/jobs", body)
	req.Header.Set("Content-Type", ct)
	rw := httptest.NewRecorder()

	h.CreateJob(rw, req)

	if rw.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body=%s", rw.Code, rw.Body.String())
	}
	var resp struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	}
	_ = json.Unmarshal(rw.Body.Bytes(), &resp)
	if resp.Status != "queued" {
		t.Errorf("status = %q, want queued", resp.Status)
	}
	if len(repo.jobs) != 1 {
		t.Errorf("repo has %d jobs, want 1", len(repo.jobs))
	}
	if len(store.puts) != 1 {
		t.Errorf("storage has %d puts, want 1", len(store.puts))
	}
	if len(queue.enqueued) != 1 {
		t.Errorf("queue has %d tasks, want 1", len(queue.enqueued))
	}
}

func TestPostJobsRemovesMultipartTempFiles(t *testing.T) {
	withIsolatedTempDir(t)
	repo := newFakeRepo()
	store := newFakeStorage()
	queue := &fakeQueue{}
	h := NewHandlers(repo, store, queue)

	body, ct := multipartBody(t, bytes.Repeat([]byte("d"), multipartMemBudget+1), `{"target_steamid":"76561198000000000"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/jobs", body)
	req.Header.Set("Content-Type", ct)
	rw := httptest.NewRecorder()

	h.CreateJob(rw, req)

	if rw.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body=%s", rw.Code, rw.Body.String())
	}
	assertMultipartTempDirEmpty(t)
}

func TestListJobsReturnsRecentJobsWithoutKillPlan(t *testing.T) {
	plan := killplan.NewPlan()
	plan.Segments = []killplan.Segment{{ID: "seg-001"}}
	cases := []struct {
		name       string
		stored     job.Job
		wantStatus job.Status
		wantReason string
	}{
		{
			name:       "parsed",
			stored:     job.Job{Status: job.StatusParsed, DemoFileName: "a.dem", Rules: rules.Default(), KillPlan: &plan},
			wantStatus: job.StatusParsed,
		},
		{
			name:       "failed",
			stored:     job.Job{Status: job.StatusFailed, FailureReason: "capture failed", DemoFileName: "b.dem", KillPlan: &plan},
			wantStatus: job.StatusFailed,
			wantReason: "capture failed",
		},
		{
			name:       "recording",
			stored:     job.Job{Status: job.StatusRecording, DemoFileName: "c.dem", KillPlan: &plan},
			wantStatus: job.StatusRecording,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := newFakeRepo()
			id := uuid.New()
			tc.stored.ID = id
			repo.jobs[id] = tc.stored
			h := NewHandlers(repo, newFakeStorage(), &fakeQueue{})

			r := chi.NewRouter()
			r.Get("/api/jobs", h.ListJobs)
			req := httptest.NewRequest(http.MethodGet, "/api/jobs?limit=10", nil)
			rw := httptest.NewRecorder()
			r.ServeHTTP(rw, req)

			if rw.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200; body=%s", rw.Code, rw.Body.String())
			}
			if strings.Contains(rw.Body.String(), "kill_plan") || strings.Contains(rw.Body.String(), "seg-001") {
				t.Fatalf("list response should not include kill_plan: %s", rw.Body.String())
			}
			var resp struct {
				Jobs []job.Job `json:"jobs"`
			}
			if err := json.Unmarshal(rw.Body.Bytes(), &resp); err != nil {
				t.Fatalf("decode list: %v", err)
			}
			if len(resp.Jobs) != 1 {
				t.Fatalf("listed %d jobs, want 1", len(resp.Jobs))
			}
			got := resp.Jobs[0]
			if got.ID != id || got.Status != tc.wantStatus || got.FailureReason != tc.wantReason || got.DemoFileName != tc.stored.DemoFileName {
				t.Fatalf("listed %+v, want id=%s status=%s reason=%q file=%q", got, id, tc.wantStatus, tc.wantReason, tc.stored.DemoFileName)
			}
			if got.KillPlan != nil {
				t.Fatal("listed kill plan")
			}
		})
	}
}

func TestSanitizeDemoFileName(t *testing.T) {
	longName := strings.Repeat("a", 200) + ".dem"
	// U+FEFF BOM, U+200B zero-width space, U+202E RTL override: Cf format
	// characters that must be dropped, not just Cc controls. Built from rune
	// values so no invisible characters hide in the source.
	formatCharsName := string(rune(0xFEFF)) + "med" + string(rune(0x200B)) + "io" + string(rune(0x202E)) + "med.dem"
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"plain", "match.dem", "match.dem"},
		{"windows_path", `C:\replays\game one.dem`, "game one.dem"},
		{"url_path", "uploads/2026/final.dem", "final.dem"},
		{"mixed_separators", `dir/sub\match.dem`, "match.dem"},
		{"control_chars", "clip\t\n\x00.dem", "clip.dem"},
		{"format_chars", formatCharsName, "mediomed.dem"},
		{"over_long", longName, strings.Repeat("a", 128)},
		{"empty", "", ""},
		{"only_separators", `a/b\`, ""},
		{"only_control", "\x00\x01\x02", ""},
		{"whitespace_only", "   ", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := sanitizeDemoFileName(tc.in)
			if got != tc.want {
				t.Fatalf("sanitizeDemoFileName(%q) = %q, want %q", tc.in, got, tc.want)
			}
			if len([]rune(got)) > maxDemoFileNameRunes {
				t.Fatalf("sanitizeDemoFileName(%q) = %q, exceeds %d runes", tc.in, got, maxDemoFileNameRunes)
			}
		})
	}
}

func TestCreateJobStoresSeriesIDAndFileName(t *testing.T) {
	repo := newFakeRepo()
	h := NewHandlers(repo, newFakeStorage(), &fakeQueue{})

	// Upper-case series id and a Windows path exercise canonicalization and
	// base-name sanitization in one happy-path request.
	series := strings.ToUpper(uuid.NewString())
	fields := map[string]string{
		"config":    `{"target_steamid":"76561198000000000"}`,
		"series_id": series,
	}
	body, ct := multipartBodyFields(t, `C:\replays\game one.dem`, []byte("dem-bytes"), fields)
	req := httptest.NewRequest(http.MethodPost, "/api/jobs", body)
	req.Header.Set("Content-Type", ct)
	rw := httptest.NewRecorder()

	h.CreateJob(rw, req)

	if rw.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body=%s", rw.Code, rw.Body.String())
	}
	var resp struct {
		ID uuid.UUID `json:"id"`
	}
	if err := json.Unmarshal(rw.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	stored, ok := repo.jobs[resp.ID]
	if !ok {
		t.Fatalf("job %s not stored", resp.ID)
	}
	if got, want := stored.SeriesID, strings.ToLower(series); got != want {
		t.Fatalf("SeriesID = %q, want canonical %q", got, want)
	}
	if got, want := stored.DemoFileName, "game one.dem"; got != want {
		t.Fatalf("DemoFileName = %q, want %q", got, want)
	}
}

func TestCreateJobRejectsInvalidSeriesID(t *testing.T) {
	repo := newFakeRepo()
	h := NewHandlers(repo, newFakeStorage(), &fakeQueue{})

	fields := map[string]string{
		"config":    `{"target_steamid":"76561198000000000"}`,
		"series_id": "not-a-uuid",
	}
	body, ct := multipartBodyFields(t, "match.dem", []byte("dem-bytes"), fields)
	req := httptest.NewRequest(http.MethodPost, "/api/jobs", body)
	req.Header.Set("Content-Type", ct)
	rw := httptest.NewRecorder()

	h.CreateJob(rw, req)

	if rw.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body=%s", rw.Code, rw.Body.String())
	}
	if len(repo.jobs) != 0 {
		t.Fatalf("repo stored %d jobs, want 0 on invalid series_id", len(repo.jobs))
	}
}

func TestCreateJobWithoutSeriesIDLeavesFieldEmpty(t *testing.T) {
	repo := newFakeRepo()
	h := NewHandlers(repo, newFakeStorage(), &fakeQueue{})

	body, ct := multipartBody(t, []byte("dem-bytes"), `{"target_steamid":"76561198000000000"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/jobs", body)
	req.Header.Set("Content-Type", ct)
	rw := httptest.NewRecorder()

	h.CreateJob(rw, req)

	if rw.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body=%s", rw.Code, rw.Body.String())
	}
	for _, j := range repo.jobs {
		if j.SeriesID != "" {
			t.Fatalf("SeriesID = %q, want empty when series_id absent", j.SeriesID)
		}
		// multipartBody uploads as "test.dem", so the name is captured.
		if j.DemoFileName != "test.dem" {
			t.Fatalf("DemoFileName = %q, want test.dem", j.DemoFileName)
		}
	}
}

func TestListJobsBySeries(t *testing.T) {
	repo := newFakeRepo()
	series := uuid.New()
	other := uuid.New()
	base := time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC)

	// Three jobs in the target series, inserted out of creation order so the
	// handler must sort them; one job in another series; one with no series.
	for _, offset := range []int{2, 0, 1} {
		id := uuid.New()
		repo.jobs[id] = job.Job{
			ID:        id,
			Status:    job.StatusQueued,
			SeriesID:  series.String(),
			CreatedAt: base.Add(time.Duration(offset) * time.Minute),
		}
	}
	otherID := uuid.New()
	repo.jobs[otherID] = job.Job{ID: otherID, Status: job.StatusQueued, SeriesID: other.String(), CreatedAt: base}
	loneID := uuid.New()
	repo.jobs[loneID] = job.Job{ID: loneID, Status: job.StatusQueued, CreatedAt: base}

	// Expected upload order is by CreatedAt ascending.
	var want []uuid.UUID
	for _, j := range sortedByCreatedAt(repo.jobs, series.String()) {
		want = append(want, j.ID)
	}

	h := NewHandlers(repo, newFakeStorage(), &fakeQueue{})
	r := chi.NewRouter()
	r.Get("/api/jobs", h.ListJobs)

	req := httptest.NewRequest(http.MethodGet, "/api/jobs?series_id="+series.String(), nil)
	rw := httptest.NewRecorder()
	r.ServeHTTP(rw, req)

	if rw.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rw.Code, rw.Body.String())
	}
	var resp struct {
		Jobs []job.Job `json:"jobs"`
	}
	if err := json.Unmarshal(rw.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(resp.Jobs) != len(want) {
		t.Fatalf("got %d jobs, want %d: %+v", len(resp.Jobs), len(want), resp.Jobs)
	}
	for i, j := range resp.Jobs {
		if j.ID != want[i] {
			t.Fatalf("jobs[%d].ID = %s, want %s (order)", i, j.ID, want[i])
		}
		if j.SeriesID != series.String() {
			t.Fatalf("jobs[%d].SeriesID = %q, want %q", i, j.SeriesID, series.String())
		}
	}

	// Invalid series_id is a 400.
	bad := httptest.NewRequest(http.MethodGet, "/api/jobs?series_id=not-a-uuid", nil)
	badRW := httptest.NewRecorder()
	r.ServeHTTP(badRW, bad)
	if badRW.Code != http.StatusBadRequest {
		t.Fatalf("invalid series_id status = %d, want 400; body=%s", badRW.Code, badRW.Body.String())
	}
}

// sortedByCreatedAt returns the target series' jobs ordered by CreatedAt, the
// same order ListBySeries must produce.
func sortedByCreatedAt(jobs map[uuid.UUID]job.Job, seriesID string) []job.Job {
	out := []job.Job{}
	for _, j := range jobs {
		if j.SeriesID == seriesID {
			out = append(out, j)
		}
	}
	sort.Slice(out, func(i, k int) bool {
		if out[i].CreatedAt.Equal(out[k].CreatedAt) {
			return out[i].ID.String() < out[k].ID.String()
		}
		return out[i].CreatedAt.Before(out[k].CreatedAt)
	})
	return out
}

func TestListLoadoutsReturnsCatalog(t *testing.T) {
	h := NewHandlers(newFakeRepo(), newFakeStorage(), &fakeQueue{})

	r := chi.NewRouter()
	r.Get("/api/loadouts", h.ListLoadouts)
	req := httptest.NewRequest(http.MethodGet, "/api/loadouts", nil)
	rw := httptest.NewRecorder()
	r.ServeHTTP(rw, req)

	if rw.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rw.Code, rw.Body.String())
	}
	if !strings.Contains(rw.Body.String(), editor.PresetViral60Clean) {
		t.Fatalf("body missing loadout: %s", rw.Body.String())
	}
}

func TestListPresetsReturnsRegistry(t *testing.T) {
	h := NewHandlers(newFakeRepo(), newFakeStorage(), &fakeQueue{})

	r := chi.NewRouter()
	r.Get("/api/presets", h.ListPresets)
	req := httptest.NewRequest(http.MethodGet, "/api/presets", nil)
	rw := httptest.NewRecorder()
	r.ServeHTTP(rw, req)

	if rw.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rw.Code, rw.Body.String())
	}
	var resp struct {
		Default string `json:"default"`
		Presets []struct {
			Name        string `json:"name"`
			Description string `json:"description"`
			Default     bool   `json:"default"`
			FPS         int    `json:"fps"`
			Width       int    `json:"width"`
			Height      int    `json:"height"`
		} `json:"presets"`
	}
	if err := json.Unmarshal(rw.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Default != editor.PresetViral60Clean {
		t.Fatalf("default = %q, want %q", resp.Default, editor.PresetViral60Clean)
	}
	if got, want := len(resp.Presets), len(editor.PresetNames()); got != want {
		t.Fatalf("presets = %d, want %d", got, want)
	}
	first := resp.Presets[0]
	if first.Name != editor.PresetViral60Clean || !first.Default || first.Description == "" {
		t.Fatalf("first preset = %#v, want default %s", first, editor.PresetViral60Clean)
	}
	if first.FPS != 60 || first.Width != 1080 || first.Height != 1920 {
		t.Fatalf("first preset geometry = %#v, want 1080x1920@60", first)
	}
}

func TestWorkbenchServesLocalApp(t *testing.T) {
	h := NewHandlers(newFakeRepo(), newFakeStorage(), &fakeQueue{})

	r := chi.NewRouter()
	r.Get("/", h.Workbench)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rw := httptest.NewRecorder()
	r.ServeHTTP(rw, req)

	if rw.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rw.Code)
	}
	body := rw.Body.String()
	for _, want := range []string{"ClipHub Workbench", "Mutation token", "workbench-shell", "HTMX", `hx-post="/ui/jobs"`, `hx-get="/ui/jobs"`, `hx-get="/ui/workspace"`} {
		if !strings.Contains(body, want) {
			t.Fatalf("workbench missing %q", want)
		}
	}
	if !strings.Contains(body, `"X-ClipHub-Token"`) {
		t.Fatalf("workbench missing mutation token header")
	}
	if strings.Contains(body, "X-ZackVideo-Token") {
		t.Fatalf("workbench uses stale mutation token header")
	}
	if strings.Contains(body, "WORKBENCH_HTMX") || strings.Contains(body, "WORKBENCH_CSS") {
		t.Fatalf("workbench contains unreplaced template markers")
	}
	if strings.Contains(body, "type JobStatus") || strings.Contains(body, "interface AppState") {
		t.Fatalf("workbench still embeds the old TypeScript app")
	}
}

func TestWorkbenchWorkspaceOnboardsAndDeepLinksSelectedJob(t *testing.T) {
	repo := newFakeRepo()
	h := NewHandlers(repo, newFakeStorage(), &fakeQueue{})
	r := Routes(h)

	req := httptest.NewRequest(http.MethodGet, "/ui/workspace", nil)
	rw := httptest.NewRecorder()
	r.ServeHTTP(rw, req)
	if rw.Code != http.StatusOK {
		t.Fatalf("onboarding status = %d, want 200; body=%s", rw.Code, rw.Body.String())
	}
	for _, want := range []string{"Start here", "Ready for local run", "No Node server required", "shortslistosparasubir"} {
		if !strings.Contains(rw.Body.String(), want) {
			t.Fatalf("onboarding missing %q: %s", want, rw.Body.String())
		}
	}

	j := job.Job{ID: uuid.New(), Status: job.StatusScanned, DemoPath: "demos/deep.dem", Rules: rules.Default()}
	repo.jobs[j.ID] = j
	req = httptest.NewRequest(http.MethodGet, "/ui/workspace", nil)
	req.Header.Set("HX-Current-URL", "http://127.0.0.1:8080/?job="+j.ID.String())
	rw = httptest.NewRecorder()
	r.ServeHTTP(rw, req)
	if rw.Code != http.StatusOK {
		t.Fatalf("deep-link status = %d, want 200; body=%s", rw.Code, rw.Body.String())
	}
	for _, want := range []string{j.ID.String(), `hx-swap-oob="true"`, "Choose the POV to clip", "deep.dem"} {
		if !strings.Contains(rw.Body.String(), want) {
			t.Fatalf("deep-link workspace missing %q: %s", want, rw.Body.String())
		}
	}
}

func TestWorkbenchHTMXFragmentsExposeLocalFlow(t *testing.T) {
	repo := newFakeRepo()
	store := newFakeStorage()
	plan := killplan.NewPlan()
	plan.Demo.Map = "de_mirage"
	plan.Demo.Tickrate = 64
	plan.Target.NameInDemo = "MartinezSa"
	plan.Segments = []killplan.Segment{{
		ID:        "seg-001",
		Round:     4,
		TickStart: 640,
		TickEnd:   1280,
		Kills: []killplan.Kill{{
			Weapon:   "ak47",
			Headshot: true,
			Victim:   killplan.Player{NameInDemo: "alex"},
		}},
	}}
	j := job.Job{
		ID:            uuid.New(),
		Status:        job.StatusRecorded,
		DemoPath:      "demos/local.dem",
		TargetSteamID: "76561198000000000",
		Rules:         rules.Default(),
		KillPlan:      &plan,
	}
	repo.jobs[j.ID] = j
	h := NewHandlers(repo, store, &fakeQueue{})
	r := Routes(h)

	req := httptest.NewRequest(http.MethodGet, "/ui/jobs?selected="+j.ID.String(), nil)
	rw := httptest.NewRecorder()
	r.ServeHTTP(rw, req)
	if rw.Code != http.StatusOK {
		t.Fatalf("jobs status = %d, want 200; body=%s", rw.Code, rw.Body.String())
	}
	for _, want := range []string{j.ID.String(), `hx-get="/ui/jobs/` + j.ID.String(), `aria-selected="true"`} {
		if !strings.Contains(rw.Body.String(), want) {
			t.Fatalf("jobs fragment missing %q: %s", want, rw.Body.String())
		}
	}

	req = httptest.NewRequest(http.MethodGet, "/ui/jobs/"+j.ID.String(), nil)
	rw = httptest.NewRecorder()
	r.ServeHTTP(rw, req)
	if rw.Code != http.StatusOK {
		t.Fatalf("job status = %d, want 200; body=%s", rw.Code, rw.Body.String())
	}
	for _, want := range []string{
		"Generate short",
		"Choose your short",
		`hx-post="/ui/jobs/` + j.ID.String() + `/generate"`,
		"Kill Feed", "Clean POV", "Full HUD",
		"short-9x16", "landscape-16x9", "Punch-in",
		"de_mirage", "MartinezSa",
	} {
		if !strings.Contains(rw.Body.String(), want) {
			t.Fatalf("job fragment missing %q: %s", want, rw.Body.String())
		}
	}
}

func TestWorkbenchCreateJobWithTargetEnqueuesParse(t *testing.T) {
	repo := newFakeRepo()
	queue := &fakeQueue{}
	h := NewHandlers(repo, newFakeStorage(), queue)
	r := Routes(h)

	body := &bytes.Buffer{}
	mw := multipart.NewWriter(body)
	demoPart, _ := mw.CreateFormFile("demo", "target.dem")
	demoPart.Write(demoMagic)
	demoPart.Write([]byte("dem-bytes"))
	mw.WriteField("target_steamid", "76561198000000000")
	mw.Close()

	req := httptest.NewRequest(http.MethodPost, "/ui/jobs", body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	rw := httptest.NewRecorder()
	r.ServeHTTP(rw, req)

	if rw.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204; body=%s", rw.Code, rw.Body.String())
	}
	if got := rw.Header().Get("HX-Redirect"); !strings.HasPrefix(got, "/?job=") {
		t.Fatalf("HX-Redirect = %q, want job redirect", got)
	}
	if len(queue.enqueued) != 1 || queue.enqueued[0].Type() != tasks.TypeParseDemo {
		t.Fatalf("queue = %#v, want parse task", queue.enqueued)
	}
	for _, j := range repo.jobs {
		if j.TargetSteamID != "76561198000000000" {
			t.Fatalf("TargetSteamID = %q, want submitted target", j.TargetSteamID)
		}
	}
}

func TestWorkbenchRenderFormEnqueuesEditOptions(t *testing.T) {
	repo := newFakeRepo()
	queue := &fakeQueue{}
	plan := killplan.NewPlan()
	plan.Segments = []killplan.Segment{{ID: "seg-001", TickStart: 1, TickEnd: 2}}
	j := job.Job{ID: uuid.New(), Status: job.StatusRecorded, Rules: rules.Default(), KillPlan: &plan}
	repo.jobs[j.ID] = j
	h := NewHandlers(repo, newFakeStorage(), queue)
	r := Routes(h)

	body := strings.NewReader("variant=viral-60-clean&music=synth-one&format=landscape-16x9&kill_effect=velocity&transition=whip&intro=on&outro=on")
	req := httptest.NewRequest(http.MethodPost, "/ui/jobs/"+j.ID.String()+"/render", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rw := httptest.NewRecorder()
	r.ServeHTTP(rw, req)

	if rw.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rw.Code, rw.Body.String())
	}
	if len(queue.enqueued) != 1 || queue.enqueued[0].Type() != tasks.TypeRenderVariant {
		t.Fatalf("queue = %#v, want one render task", queue.enqueued)
	}
	var payload tasks.RenderVariantPayload
	if err := json.Unmarshal(queue.enqueued[0].Payload(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Variant != editor.PresetViral60Clean || payload.MusicKey != "synth-one" {
		t.Fatalf("payload variant/music = %q/%q", payload.Variant, payload.MusicKey)
	}
	wantEdit := renderplan.EditRequest{
		Format:        renderplan.FormatLandscape16x9,
		KillEffect:    renderplan.KillEffectVelocity,
		Transition:    renderplan.TransitionWhip,
		Intro:         true,
		Outro:         true,
		CoverStrategy: renderplan.CoverStrategyGenerated,
	}
	if payload.Edit != wantEdit {
		t.Fatalf("edit = %#v, want %#v", payload.Edit, wantEdit)
	}
	if !strings.Contains(rw.Body.String(), `Queued for render`) {
		t.Fatalf("fragment missing queued render state: %s", rw.Body.String())
	}
}

func TestPostJobsMarksJobFailedWhenEnqueueFails(t *testing.T) {
	repo := newFakeRepo()
	store := newFakeStorage()
	queue := &fakeQueue{err: errors.New("redis down")}
	h := NewHandlers(repo, store, queue)

	body, ct := multipartBody(t, []byte("dem-bytes"), `{"target_steamid":"76561198000000000"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/jobs", body)
	req.Header.Set("Content-Type", ct)
	rw := httptest.NewRecorder()

	h.CreateJob(rw, req)

	if rw.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500; body=%s", rw.Code, rw.Body.String())
	}
	if len(repo.jobs) != 1 {
		t.Fatalf("repo jobs = %d, want 1", len(repo.jobs))
	}
	for _, j := range repo.jobs {
		if j.Status != job.StatusFailed {
			t.Fatalf("job status = %s, want failed (must not be stranded in queued with no task)", j.Status)
		}
	}
}

func TestPostJobsMarksAcceptedPendingJobFailedWhenQueueDiscardsIt(t *testing.T) {
	repo := newFakeRepo()
	queue := &fakeQueue{}
	h := NewHandlers(repo, newFakeStorage(), queue)

	body, contentType := multipartBody(t, []byte("dem-bytes"), `{"target_steamid":"76561198000000000"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/jobs", body)
	req.Header.Set("Content-Type", contentType)
	rw := httptest.NewRecorder()
	h.CreateJob(rw, req)

	if rw.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body=%s", rw.Code, rw.Body.String())
	}
	if len(queue.transitions) != 1 {
		t.Fatalf("queue transitions = %d, want 1", len(queue.transitions))
	}
	if err := queue.transitions[0](errors.New("inline queue task discarded during shutdown")); err != nil {
		t.Fatalf("discard transition error = %v", err)
	}
	for _, j := range repo.jobs {
		if j.Status != job.StatusFailed || !strings.Contains(j.FailureReason, "discarded during shutdown") {
			t.Fatalf("job after discard = status %s, reason %q; want failed discard reason", j.Status, j.FailureReason)
		}
	}
}

func TestPostJobsFailedWriteSurvivesCancelledRequestContext(t *testing.T) {
	repo := newFakeRepo()
	repo.updateHonorsCtx = true // mimic pgxpool: refuse a cancelled context
	store := newFakeStorage()
	queue := &fakeQueue{err: errors.New("redis down")}
	h := NewHandlers(repo, store, queue)

	body, ct := multipartBody(t, []byte("dem-bytes"), `{"target_steamid":"76561198000000000"}`)
	ctx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequest(http.MethodPost, "/api/jobs", body).WithContext(ctx)
	req.Header.Set("Content-Type", ct)
	cancel() // client disconnect / proxy deadline before the handler finishes
	rw := httptest.NewRecorder()

	h.CreateJob(rw, req)

	if len(repo.jobs) != 1 {
		t.Fatalf("repo jobs = %d, want 1", len(repo.jobs))
	}
	for _, j := range repo.jobs {
		if j.Status != job.StatusFailed {
			t.Fatalf("job status = %s, want failed (compensating write must survive a cancelled request context)", j.Status)
		}
	}
}

func TestGetJobHidesInternalErrorDetails(t *testing.T) {
	repo := newFakeRepo()
	repo.getErr = errors.New(`pq: relation "jobs" does not exist [secret-schema]`)
	h := NewHandlers(repo, newFakeStorage(), &fakeQueue{})

	r := chi.NewRouter()
	r.Get("/api/jobs/{id}", h.GetJob)
	req := httptest.NewRequest(http.MethodGet, "/api/jobs/"+uuid.New().String(), nil)
	rw := httptest.NewRecorder()
	r.ServeHTTP(rw, req)

	if rw.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rw.Code)
	}
	if strings.Contains(rw.Body.String(), "secret-schema") {
		t.Fatalf("response leaked internal error detail: %s", rw.Body.String())
	}
}

func TestPostJobsRejectsMissingDemo(t *testing.T) {
	h := NewHandlers(newFakeRepo(), newFakeStorage(), &fakeQueue{})

	body := &bytes.Buffer{}
	mw := multipart.NewWriter(body)
	mw.WriteField("config", `{"target_steamid":"76561198000000000"}`)
	mw.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/jobs", body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	rw := httptest.NewRecorder()

	h.CreateJob(rw, req)
	if rw.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rw.Code)
	}
}

func TestPostJobsRejectsInvalidSteamID(t *testing.T) {
	h := NewHandlers(newFakeRepo(), newFakeStorage(), &fakeQueue{})

	body, ct := multipartBody(t, []byte("x"), `{"target_steamid":"not-a-number"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/jobs", body)
	req.Header.Set("Content-Type", ct)
	rw := httptest.NewRecorder()

	h.CreateJob(rw, req)
	if rw.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rw.Code)
	}
}

func TestPostJobsValidatesDemoMagicBytes(t *testing.T) {
	cases := []struct {
		name       string
		demo       []byte
		wantStatus int
	}{
		{name: "cs2 source2 demo", demo: []byte("PBDEMS2\x00rest-of-demo"), wantStatus: http.StatusCreated},
		{name: "legacy gotv demo", demo: []byte("HL2DEMO\x00rest-of-demo"), wantStatus: http.StatusCreated},
		{name: "not a demo", demo: []byte("just some bytes"), wantStatus: http.StatusBadRequest},
		{name: "short non-demo body", demo: []byte("PB2"), wantStatus: http.StatusBadRequest},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := newFakeRepo()
			store := newFakeStorage()
			queue := &fakeQueue{}
			h := NewHandlers(repo, store, queue)

			body, ct := multipartBodyRaw(t, tc.demo, `{"target_steamid":"76561198000000000"}`)
			req := httptest.NewRequest(http.MethodPost, "/api/jobs", body)
			req.Header.Set("Content-Type", ct)
			rw := httptest.NewRecorder()

			h.CreateJob(rw, req)

			if rw.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d; body=%s", rw.Code, tc.wantStatus, rw.Body.String())
			}
			if tc.wantStatus == http.StatusBadRequest {
				if !strings.Contains(rw.Body.String(), "not a CS2 demo") {
					t.Fatalf("body = %s, want not-a-demo error", rw.Body.String())
				}
				if len(store.puts) != 0 {
					t.Fatalf("storage puts = %d, want 0 (must reject before Put)", len(store.puts))
				}
				return
			}
			// The full demo bytes (header included) must reach storage intact.
			for _, stored := range store.puts {
				if !bytes.Equal(stored, tc.demo) {
					t.Fatalf("stored demo = %q, want full bytes %q", stored, tc.demo)
				}
			}
		})
	}
}

func TestPostJobsAcceptsZstdDemos(t *testing.T) {
	plain := []byte("PBDEMS2\x00rest-of-demo")
	tests := []struct {
		name       string
		filename   string
		payload    []byte
		wantStatus int
		wantStored []byte
		wantName   string
	}{
		{
			name:       "faceit zst",
			filename:   "1-b5604ae7-c676-454b-901a-0b02014abd94-1-2.dem.zst",
			payload:    zstdDemoBytes(t, plain),
			wantStatus: http.StatusCreated,
			wantStored: plain,
			wantName:   "1-b5604ae7-c676-454b-901a-0b02014abd94-1-2.dem",
		},
		{
			name:       "zst wrapping garbage",
			filename:   "junk.dem.zst",
			payload:    zstdDemoBytes(t, []byte("just some bytes")),
			wantStatus: http.StatusBadRequest,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repo := newFakeRepo()
			store := newFakeStorage()
			h := NewHandlers(repo, store, &fakeQueue{})
			body, ct := multipartBodyNamed(t, test.filename, test.payload, `{"target_steamid":"76561198000000000"}`)
			req := httptest.NewRequest(http.MethodPost, "/api/jobs", body)
			req.Header.Set("Content-Type", ct)
			rw := httptest.NewRecorder()
			h.CreateJob(rw, req)
			if rw.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d body=%s", rw.Code, test.wantStatus, rw.Body.String())
			}
			if test.wantStatus != http.StatusCreated {
				if len(store.puts) != 0 {
					t.Fatalf("storage puts = %d, want 0", len(store.puts))
				}
				return
			}
			var stored []byte
			for _, value := range store.puts {
				stored = value
			}
			if !bytes.Equal(stored, test.wantStored) {
				t.Fatalf("stored = %q, want decompressed demo", stored)
			}
			for _, job := range repo.jobs {
				if job.DemoFileName != test.wantName {
					t.Fatalf("DemoFileName = %q, want %q", job.DemoFileName, test.wantName)
				}
			}
		})
	}
}

func multipartBodyNamed(t *testing.T, filename string, demoBytes []byte, configJSON string) (*bytes.Buffer, string) {
	t.Helper()
	body := &bytes.Buffer{}
	mw := multipart.NewWriter(body)
	demoPart, err := mw.CreateFormFile("demo", filename)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := demoPart.Write(demoBytes); err != nil {
		t.Fatal(err)
	}
	if err := mw.WriteField("config", configJSON); err != nil {
		t.Fatal(err)
	}
	if err := mw.Close(); err != nil {
		t.Fatal(err)
	}
	return body, mw.FormDataContentType()
}

func zstdDemoBytes(t *testing.T, plain []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	enc, err := zstd.NewWriter(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := enc.Write(plain); err != nil {
		t.Fatal(err)
	}
	if err := enc.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestPostJobsWithTargetEnqueuesParse(t *testing.T) {
	repo := newFakeRepo()
	queue := &fakeQueue{}
	h := NewHandlers(repo, newFakeStorage(), queue)

	body, ct := multipartBody(t, []byte("dem-bytes"), `{"target_steamid":"76561198000000000"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/jobs", body)
	req.Header.Set("Content-Type", ct)
	rw := httptest.NewRecorder()

	h.CreateJob(rw, req)

	if rw.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body=%s", rw.Code, rw.Body.String())
	}
	if len(queue.enqueued) != 1 || queue.enqueued[0].Type() != tasks.TypeParseDemo {
		t.Fatalf("queue = %#v, want one parse task", queue.enqueued)
	}
}

func TestPostJobsWithoutTargetEnqueuesScan(t *testing.T) {
	repo := newFakeRepo()
	queue := &fakeQueue{}
	h := NewHandlers(repo, newFakeStorage(), queue)

	body, ct := multipartBody(t, []byte("dem-bytes"), ``)
	req := httptest.NewRequest(http.MethodPost, "/api/jobs", body)
	req.Header.Set("Content-Type", ct)
	rw := httptest.NewRecorder()

	h.CreateJob(rw, req)

	if rw.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body=%s", rw.Code, rw.Body.String())
	}
	if len(queue.enqueued) != 1 || queue.enqueued[0].Type() != tasks.TypeScanRoster {
		t.Fatalf("queue = %#v, want one scan task", queue.enqueued)
	}
	for _, j := range repo.jobs {
		if j.TargetSteamID != "" {
			t.Fatalf("TargetSteamID = %q, want empty for scan-first job", j.TargetSteamID)
		}
	}
}

func TestGetRosterReturns409BeforeScan(t *testing.T) {
	repo := newFakeRepo()
	j := job.Job{ID: uuid.New(), Status: job.StatusScanning, Rules: rules.Default()}
	repo.jobs[j.ID] = j
	h := NewHandlers(repo, newFakeStorage(), &fakeQueue{})

	r := chi.NewRouter()
	r.Get("/api/jobs/{id}/roster", h.GetRoster)
	req := httptest.NewRequest(http.MethodGet, "/api/jobs/"+j.ID.String()+"/roster", nil)
	rw := httptest.NewRecorder()
	r.ServeHTTP(rw, req)

	if rw.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409; body=%s", rw.Code, rw.Body.String())
	}
	if !strings.Contains(rw.Body.String(), "roster not ready") {
		t.Fatalf("body = %s, want roster-not-ready", rw.Body.String())
	}
}

func TestGetRosterReturnsPlayersAfterScan(t *testing.T) {
	repo := newFakeRepo()
	store := newFakeStorage()
	j := job.Job{ID: uuid.New(), Status: job.StatusScanned, Rules: rules.Default()}
	repo.jobs[j.ID] = j
	_ = store.Put(artifacts.RosterKey(j.ID), bytes.NewReader([]byte(`{"players":[{"steamid64":"765","name":"kekO","team":"CT","kills":24,"deaths":14,"assists":5}]}`)))
	h := NewHandlers(repo, store, &fakeQueue{})

	r := chi.NewRouter()
	r.Get("/api/jobs/{id}/roster", h.GetRoster)
	req := httptest.NewRequest(http.MethodGet, "/api/jobs/"+j.ID.String()+"/roster", nil)
	rw := httptest.NewRecorder()
	r.ServeHTTP(rw, req)

	if rw.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rw.Code, rw.Body.String())
	}
	if got := rw.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json", got)
	}
	for _, want := range []string{`"players"`, `"kekO"`, `"kills":24`} {
		if !strings.Contains(rw.Body.String(), want) {
			t.Fatalf("body missing %s: %s", want, rw.Body.String())
		}
	}
}

func TestStartParseAcceptsScannedJob(t *testing.T) {
	repo := newFakeRepo()
	queue := &fakeQueue{}
	j := job.Job{ID: uuid.New(), Status: job.StatusScanned, Rules: rules.Default()}
	repo.jobs[j.ID] = j
	h := NewHandlers(repo, newFakeStorage(), queue)

	r := chi.NewRouter()
	r.Post("/api/jobs/{id}/parse", h.StartParse)
	req := httptest.NewRequest(http.MethodPost, "/api/jobs/"+j.ID.String()+"/parse", strings.NewReader(`{"target_steamid":"76561198000000000"}`))
	req.Header.Set("Content-Type", "application/json")
	rw := httptest.NewRecorder()
	r.ServeHTTP(rw, req)

	if rw.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202; body=%s", rw.Code, rw.Body.String())
	}
	if !strings.Contains(rw.Body.String(), `"status":"parsing"`) {
		t.Fatalf("body missing parsing status: %s", rw.Body.String())
	}
	if len(queue.enqueued) != 1 || queue.enqueued[0].Type() != tasks.TypeParseDemo {
		t.Fatalf("queue = %#v, want one parse task", queue.enqueued)
	}
	if got := repo.jobs[j.ID].TargetSteamID; got != "76561198000000000" {
		t.Fatalf("TargetSteamID = %q, want persisted target", got)
	}
}

func TestStartParseMarksAcceptedPendingJobFailedWhenQueueDiscardsIt(t *testing.T) {
	repo := newFakeRepo()
	queue := &fakeQueue{}
	j := job.Job{ID: uuid.New(), Status: job.StatusScanned, Rules: rules.Default()}
	repo.jobs[j.ID] = j
	h := NewHandlers(repo, newFakeStorage(), queue)

	r := chi.NewRouter()
	r.Post("/api/jobs/{id}/parse", h.StartParse)
	req := httptest.NewRequest(http.MethodPost, "/api/jobs/"+j.ID.String()+"/parse", strings.NewReader(`{"target_steamid":"76561198000000000"}`))
	rw := httptest.NewRecorder()
	r.ServeHTTP(rw, req)

	if rw.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202; body=%s", rw.Code, rw.Body.String())
	}
	if len(queue.transitions) != 1 {
		t.Fatalf("queue transitions = %d, want 1", len(queue.transitions))
	}
	if err := queue.transitions[0](errors.New("inline queue task discarded during shutdown")); err != nil {
		t.Fatalf("discard transition error = %v", err)
	}
	got := repo.jobs[j.ID]
	if got.Status != job.StatusFailed || !strings.Contains(got.FailureReason, "discarded during shutdown") {
		t.Fatalf("job after discard = status %s, reason %q; want failed discard reason", got.Status, got.FailureReason)
	}
}

func TestStartParseRejectsNonUintTarget(t *testing.T) {
	repo := newFakeRepo()
	queue := &fakeQueue{}
	j := job.Job{ID: uuid.New(), Status: job.StatusScanned, Rules: rules.Default()}
	repo.jobs[j.ID] = j
	h := NewHandlers(repo, newFakeStorage(), queue)

	r := chi.NewRouter()
	r.Post("/api/jobs/{id}/parse", h.StartParse)
	req := httptest.NewRequest(http.MethodPost, "/api/jobs/"+j.ID.String()+"/parse", strings.NewReader(`{"target_steamid":"not-a-number"}`))
	req.Header.Set("Content-Type", "application/json")
	rw := httptest.NewRecorder()
	r.ServeHTTP(rw, req)

	if rw.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body=%s", rw.Code, rw.Body.String())
	}
	if len(queue.enqueued) != 0 {
		t.Fatalf("enqueued = %d, want 0", len(queue.enqueued))
	}
}

func TestStartParseRejectsWrongState(t *testing.T) {
	repo := newFakeRepo()
	queue := &fakeQueue{}
	j := job.Job{ID: uuid.New(), Status: job.StatusQueued, Rules: rules.Default()}
	repo.jobs[j.ID] = j
	h := NewHandlers(repo, newFakeStorage(), queue)

	r := chi.NewRouter()
	r.Post("/api/jobs/{id}/parse", h.StartParse)
	req := httptest.NewRequest(http.MethodPost, "/api/jobs/"+j.ID.String()+"/parse", strings.NewReader(`{"target_steamid":"76561198000000000"}`))
	req.Header.Set("Content-Type", "application/json")
	rw := httptest.NewRecorder()
	r.ServeHTTP(rw, req)

	if rw.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409; body=%s", rw.Code, rw.Body.String())
	}
	if len(queue.enqueued) != 0 {
		t.Fatalf("enqueued = %d, want 0", len(queue.enqueued))
	}
}

func TestGetJobReturnsJob(t *testing.T) {
	repo := newFakeRepo()
	j := job.Job{
		ID:            uuid.New(),
		Status:        job.StatusQueued,
		DemoPath:      "demos/x.dem",
		DemoSHA256:    "abc",
		TargetSteamID: "76561198000000000",
		Rules:         rules.Default(),
	}
	repo.jobs[j.ID] = j

	h := NewHandlers(repo, newFakeStorage(), &fakeQueue{})

	r := chi.NewRouter()
	r.Get("/api/jobs/{id}", h.GetJob)

	req := httptest.NewRequest(http.MethodGet, "/api/jobs/"+j.ID.String(), nil)
	rw := httptest.NewRecorder()
	r.ServeHTTP(rw, req)

	if rw.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rw.Code)
	}
	var got struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	}
	_ = json.Unmarshal(rw.Body.Bytes(), &got)
	if got.ID != j.ID.String() {
		t.Errorf("id = %q, want %q", got.ID, j.ID.String())
	}
	if got.Status != "queued" {
		t.Errorf("status = %q, want queued", got.Status)
	}
}

func TestGetJobReturns404WhenMissing(t *testing.T) {
	h := NewHandlers(newFakeRepo(), newFakeStorage(), &fakeQueue{})
	r := chi.NewRouter()
	r.Get("/api/jobs/{id}", h.GetJob)

	req := httptest.NewRequest(http.MethodGet, "/api/jobs/"+uuid.New().String(), nil)
	rw := httptest.NewRecorder()
	r.ServeHTTP(rw, req)

	if rw.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rw.Code)
	}
}

func TestGetJobReturns400OnInvalidUUID(t *testing.T) {
	h := NewHandlers(newFakeRepo(), newFakeStorage(), &fakeQueue{})
	r := chi.NewRouter()
	r.Get("/api/jobs/{id}", h.GetJob)

	req := httptest.NewRequest(http.MethodGet, "/api/jobs/not-a-uuid", nil)
	rw := httptest.NewRecorder()
	r.ServeHTTP(rw, req)

	if rw.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rw.Code)
	}
}

func TestGetPlanReturns409WhenJobNotParsed(t *testing.T) {
	repo := newFakeRepo()
	j := job.Job{ID: uuid.New(), Status: job.StatusQueued, Rules: rules.Default()}
	repo.jobs[j.ID] = j
	h := NewHandlers(repo, newFakeStorage(), &fakeQueue{})

	r := chi.NewRouter()
	r.Get("/api/jobs/{id}/plan", h.GetPlan)

	req := httptest.NewRequest(http.MethodGet, "/api/jobs/"+j.ID.String()+"/plan", nil)
	rw := httptest.NewRecorder()
	r.ServeHTTP(rw, req)

	if rw.Code != http.StatusConflict {
		t.Errorf("status = %d, want 409 (not yet ready)", rw.Code)
	}
}

func TestGetPlanReturnsPlanWhenReady(t *testing.T) {
	repo := newFakeRepo()
	plan := killplan.NewPlan()
	plan.Demo.Map = "de_inferno"
	j := job.Job{ID: uuid.New(), Status: job.StatusParsed, Rules: rules.Default(), KillPlan: &plan}
	repo.jobs[j.ID] = j
	h := NewHandlers(repo, newFakeStorage(), &fakeQueue{})

	r := chi.NewRouter()
	r.Get("/api/jobs/{id}/plan", h.GetPlan)

	req := httptest.NewRequest(http.MethodGet, "/api/jobs/"+j.ID.String()+"/plan", nil)
	rw := httptest.NewRecorder()
	r.ServeHTTP(rw, req)

	if rw.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rw.Code)
	}
	if !strings.Contains(rw.Body.String(), "de_inferno") {
		t.Errorf("body does not include map: %s", rw.Body.String())
	}
}

func TestGetMomentsReturnsDerivedMomentDocument(t *testing.T) {
	repo := newFakeRepo()
	plan := killplan.NewPlan()
	plan.Demo.Tickrate = 64
	plan.Segments = []killplan.Segment{{
		ID:        "seg-001",
		Round:     5,
		TickStart: 64,
		TickEnd:   128,
		Kills: []killplan.Kill{{
			Tick:     80,
			Weapon:   "weapon_awp",
			Headshot: true,
		}},
	}}
	j := job.Job{ID: uuid.New(), Status: job.StatusParsed, Rules: rules.Default(), KillPlan: &plan}
	repo.jobs[j.ID] = j
	h := NewHandlers(repo, newFakeStorage(), &fakeQueue{})

	r := chi.NewRouter()
	r.Get("/api/jobs/{id}/moments", h.GetMoments)
	req := httptest.NewRequest(http.MethodGet, "/api/jobs/"+j.ID.String()+"/moments", nil)
	rw := httptest.NewRecorder()
	r.ServeHTTP(rw, req)

	if rw.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rw.Code, rw.Body.String())
	}
	for _, want := range []string{`"schema_version":"1.0"`, `"segment_id":"seg-001"`, `"awp"`, `"headshot"`} {
		if !strings.Contains(rw.Body.String(), want) {
			t.Fatalf("body missing %s: %s", want, rw.Body.String())
		}
	}
}

func TestGetMomentsReturns409WhenJobNotParsed(t *testing.T) {
	repo := newFakeRepo()
	j := job.Job{ID: uuid.New(), Status: job.StatusQueued, Rules: rules.Default()}
	repo.jobs[j.ID] = j
	h := NewHandlers(repo, newFakeStorage(), &fakeQueue{})

	r := chi.NewRouter()
	r.Get("/api/jobs/{id}/moments", h.GetMoments)
	req := httptest.NewRequest(http.MethodGet, "/api/jobs/"+j.ID.String()+"/moments", nil)
	rw := httptest.NewRecorder()
	r.ServeHTTP(rw, req)

	if rw.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409", rw.Code)
	}
}

func TestStartRecordingEnqueuesRecordTaskWhenParsed(t *testing.T) {
	repo := newFakeRepo()
	queue := &fakeQueue{}
	plan := killplan.NewPlan()
	j := job.Job{ID: uuid.New(), Status: job.StatusParsed, Rules: rules.Default(), KillPlan: &plan}
	repo.jobs[j.ID] = j
	h := NewHandlers(repo, newFakeStorage(), queue, WithCapabilities(Capabilities{RecordEnabled: true}))

	r := chi.NewRouter()
	r.Post("/api/jobs/{id}/record", h.StartRecording)
	req := httptest.NewRequest(http.MethodPost, "/api/jobs/"+j.ID.String()+"/record", nil)
	rw := httptest.NewRecorder()
	r.ServeHTTP(rw, req)

	if rw.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202; body=%s", rw.Code, rw.Body.String())
	}
	if len(queue.enqueued) != 1 {
		t.Fatalf("enqueued = %d, want 1", len(queue.enqueued))
	}
	if queue.enqueued[0].Type() != tasks.TypeRecordDemo {
		t.Fatalf("task type = %q, want %q", queue.enqueued[0].Type(), tasks.TypeRecordDemo)
	}
	if len(queue.options) != 1 || !hasAsynqOption(queue.options[0], "Unique(") {
		t.Fatalf("enqueue options = %#v, want Unique option for dedup", queue.options)
	}
	if !hasAsynqOption(queue.options[0], "MaxRetry(0)") {
		t.Fatalf("enqueue options = %#v, want MaxRetry(0) so capture never auto-retries", queue.options)
	}
}

func TestStartRecordingAppliesPresetCaptureHUD(t *testing.T) {
	tests := []struct {
		name                     string
		preset                   string
		format                   string
		wantHUD                  string
		wantPortraitSafeKillfeed bool
	}{
		{name: "kill feed vertical", preset: editor.PresetViral60Clean, format: renderplan.FormatShort9x16, wantHUD: "deathnotices", wantPortraitSafeKillfeed: true},
		{name: "kill feed landscape", preset: editor.PresetViral60Clean, format: renderplan.FormatLandscape16x9, wantHUD: "deathnotices"},
		{name: "clean POV", preset: editor.PresetCleanPOV60, format: renderplan.FormatShort9x16, wantHUD: "clean"},
		{name: "full HUD vertical", preset: editor.PresetFullHUD60, format: renderplan.FormatShort9x16, wantHUD: "gameplay", wantPortraitSafeKillfeed: true},
		{name: "full HUD landscape", preset: editor.PresetFullHUD60, format: renderplan.FormatLandscape16x9, wantHUD: "gameplay"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := newFakeRepo()
			queue := &fakeQueue{}
			plan := killplan.NewPlan()
			j := job.Job{ID: uuid.New(), Status: job.StatusParsed, Rules: rules.Default(), KillPlan: &plan}
			repo.jobs[j.ID] = j
			h := NewHandlers(repo, newFakeStorage(), queue, WithCapabilities(Capabilities{RecordEnabled: true}))

			r := chi.NewRouter()
			r.Post("/api/jobs/{id}/record", h.StartRecording)
			body := fmt.Sprintf(`{"preset":%q,"edit":{"format":%q}}`, tc.preset, tc.format)
			req := httptest.NewRequest(http.MethodPost, "/api/jobs/"+j.ID.String()+"/record", strings.NewReader(body))
			rw := httptest.NewRecorder()
			r.ServeHTTP(rw, req)

			if rw.Code != http.StatusAccepted {
				t.Fatalf("status = %d, want 202; body=%s", rw.Code, rw.Body.String())
			}
			if len(queue.enqueued) != 1 {
				t.Fatalf("enqueued = %d, want 1", len(queue.enqueued))
			}
			var payload tasks.RecordDemoPayload
			if err := json.Unmarshal(queue.enqueued[0].Payload(), &payload); err != nil {
				t.Fatalf("unmarshal record payload: %v", err)
			}
			if payload.HUDMode != tc.wantHUD {
				t.Fatalf("HUDMode = %q, want %q", payload.HUDMode, tc.wantHUD)
			}
			if payload.PortraitSafeKillfeed != tc.wantPortraitSafeKillfeed {
				t.Fatalf("PortraitSafeKillfeed = %t, want %t", payload.PortraitSafeKillfeed, tc.wantPortraitSafeKillfeed)
			}
		})
	}
}

func TestStartRecordingNativeHUDAndRecap(t *testing.T) {
	// Locked Full Demo wire: landscape recap + native HUD + comms on viral-60-clean.
	const fullDemoEdit = `{"format":"landscape-16x9","killEffect":"clean","transition":"cut","intro":false,"outro":false,"hook_text":false,"kill_counter":false,"match_recap":true,"voice_comms":true,"voice_volume":0.85,"native_hud":true,"cover_strategy":"generated-gameplay"}`
	tests := []struct {
		name       string
		body       string
		storeRecap bool
		emptyRecap bool
		wantHUD    string
		wantRecap  bool
		wantCode   int
	}{
		{
			name:     "native HUD overrides killfeed preset",
			body:     `{"preset":"viral-60-clean","edit":{"format":"landscape-16x9","native_hud":true}}`,
			wantHUD:  "gameplay",
			wantCode: http.StatusAccepted,
		},
		{
			name:       "locked full demo recap ignores kill-burst ids",
			body:       `{"preset":"viral-60-clean","segment_ids":["seg-001"],"edit":` + fullDemoEdit + `}`,
			storeRecap: true,
			wantHUD:    "gameplay",
			wantRecap:  true,
			wantCode:   http.StatusAccepted,
		},
		{
			name:     "match recap without sidecar is conflict",
			body:     `{"preset":"viral-60-clean","segment_ids":["seg-001"],"edit":` + fullDemoEdit + `}`,
			wantCode: http.StatusConflict,
		},
		{
			name:       "match recap with empty sidecar is conflict",
			body:       `{"preset":"viral-60-clean","edit":` + fullDemoEdit + `}`,
			storeRecap: true,
			emptyRecap: true,
			wantCode:   http.StatusConflict,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := newFakeRepo()
			queue := &fakeQueue{}
			store := newFakeStorage()
			plan := killplan.NewPlan()
			plan.Segments = []killplan.Segment{{ID: "seg-001", TickStart: 100, TickEnd: 200}}
			j := job.Job{ID: uuid.New(), Status: job.StatusParsed, Rules: rules.Default(), KillPlan: &plan}
			repo.jobs[j.ID] = j
			if tc.storeRecap {
				recap := killplan.NewPlan()
				if !tc.emptyRecap {
					recap.Segments = []killplan.Segment{{ID: "recap-001", Round: 1, TickStart: 1, TickEnd: 9000}}
				}
				if err := recapplan.Store(store, j.ID, recap); err != nil {
					t.Fatal(err)
				}
			}
			h := NewHandlers(repo, store, queue, WithCapabilities(Capabilities{RecordEnabled: true}))

			r := chi.NewRouter()
			r.Post("/api/jobs/{id}/record", h.StartRecording)
			req := httptest.NewRequest(http.MethodPost, "/api/jobs/"+j.ID.String()+"/record", strings.NewReader(tc.body))
			rw := httptest.NewRecorder()
			r.ServeHTTP(rw, req)

			if rw.Code != tc.wantCode {
				t.Fatalf("status = %d, want %d; body=%s", rw.Code, tc.wantCode, rw.Body.String())
			}
			if tc.wantCode != http.StatusAccepted {
				if len(queue.enqueued) != 0 {
					t.Fatalf("enqueued = %d, want 0", len(queue.enqueued))
				}
				return
			}
			var payload tasks.RecordDemoPayload
			if err := json.Unmarshal(queue.enqueued[0].Payload(), &payload); err != nil {
				t.Fatalf("unmarshal record payload: %v", err)
			}
			if payload.HUDMode != tc.wantHUD {
				t.Fatalf("HUDMode = %q, want %q", payload.HUDMode, tc.wantHUD)
			}
			if payload.UseRecapPlan != tc.wantRecap {
				t.Fatalf("UseRecapPlan = %t, want %t", payload.UseRecapPlan, tc.wantRecap)
			}
			if tc.wantRecap && len(payload.SegmentIDs) != 0 {
				t.Fatalf("SegmentIDs = %v, want empty so recap records every round", payload.SegmentIDs)
			}
		})
	}
}

func TestStartRecordingAdmissionByStatus(t *testing.T) {
	const fullDemoEdit = `{"format":"landscape-16x9","killEffect":"clean","transition":"cut","intro":false,"outro":false,"hook_text":false,"kill_counter":false,"match_recap":true,"voice_comms":true,"voice_volume":0.85,"native_hud":true,"cover_strategy":"generated-gameplay"}`
	plan := killplan.NewPlan()
	plan.Segments = []killplan.Segment{{ID: "seg-001", TickStart: 100, TickEnd: 200}}

	tests := []struct {
		name        string
		status      job.Status
		body        string
		storeRecap  bool
		wantCode    int
		wantEnqueue int
		wantHUD     string
		wantRecap   bool
	}{
		{
			name:        "recording with kill plan is in-progress, no second enqueue",
			status:      job.StatusRecording,
			wantCode:    http.StatusAccepted,
			wantEnqueue: 0,
		},
		{
			name:        "parsed recap sidecar still admits use_recap_plan and gameplay HUD",
			status:      job.StatusParsed,
			body:        `{"preset":"viral-60-clean","segment_ids":["seg-001"],"edit":` + fullDemoEdit + `}`,
			storeRecap:  true,
			wantCode:    http.StatusAccepted,
			wantEnqueue: 1,
			wantHUD:     "gameplay",
			wantRecap:   true,
		},
		{
			name:        "scanning is still rejected",
			status:      job.StatusScanning,
			wantCode:    http.StatusConflict,
			wantEnqueue: 0,
		},
		{
			name:        "composing is still rejected",
			status:      job.StatusComposing,
			wantCode:    http.StatusConflict,
			wantEnqueue: 0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := newFakeRepo()
			queue := &fakeQueue{}
			store := newFakeStorage()
			j := job.Job{ID: uuid.New(), Status: tc.status, Rules: rules.Default(), KillPlan: &plan}
			repo.jobs[j.ID] = j
			if tc.storeRecap {
				recap := killplan.NewPlan()
				recap.Segments = []killplan.Segment{{ID: "recap-001", Round: 1, TickStart: 1, TickEnd: 9000}}
				if err := recapplan.Store(store, j.ID, recap); err != nil {
					t.Fatal(err)
				}
			}
			h := NewHandlers(repo, store, queue, WithCapabilities(Capabilities{RecordEnabled: true}))

			r := chi.NewRouter()
			r.Post("/api/jobs/{id}/record", h.StartRecording)
			var body io.Reader
			if tc.body != "" {
				body = strings.NewReader(tc.body)
			}
			req := httptest.NewRequest(http.MethodPost, "/api/jobs/"+j.ID.String()+"/record", body)
			rw := httptest.NewRecorder()
			r.ServeHTTP(rw, req)

			if rw.Code != tc.wantCode {
				t.Fatalf("status = %d, want %d; body=%s", rw.Code, tc.wantCode, rw.Body.String())
			}
			if len(queue.enqueued) != tc.wantEnqueue {
				t.Fatalf("enqueued = %d, want %d", len(queue.enqueued), tc.wantEnqueue)
			}
			if tc.wantEnqueue == 0 {
				return
			}
			var payload tasks.RecordDemoPayload
			if err := json.Unmarshal(queue.enqueued[0].Payload(), &payload); err != nil {
				t.Fatalf("unmarshal record payload: %v", err)
			}
			if payload.HUDMode != tc.wantHUD {
				t.Fatalf("HUDMode = %q, want %q", payload.HUDMode, tc.wantHUD)
			}
			if payload.UseRecapPlan != tc.wantRecap {
				t.Fatalf("UseRecapPlan = %t, want %t", payload.UseRecapPlan, tc.wantRecap)
			}
			if tc.wantRecap && len(payload.SegmentIDs) != 0 {
				t.Fatalf("SegmentIDs = %v, want empty so recap records every round", payload.SegmentIDs)
			}
		})
	}
}

func TestGetRecapPlanReturnsStoredRounds(t *testing.T) {
	repo := newFakeRepo()
	store := newFakeStorage()
	plan := killplan.NewPlan()
	j := job.Job{ID: uuid.New(), Status: job.StatusParsed, Rules: rules.Default(), KillPlan: &plan}
	repo.jobs[j.ID] = j
	recap := killplan.NewPlan()
	recap.Segments = []killplan.Segment{{ID: "recap-001", Round: 4, TickStart: 10, TickEnd: 4000}}
	if err := recapplan.Store(store, j.ID, recap); err != nil {
		t.Fatal(err)
	}
	h := NewHandlers(repo, store, &fakeQueue{})

	r := chi.NewRouter()
	r.Get("/api/jobs/{id}/recap-plan", h.GetRecapPlan)
	req := httptest.NewRequest(http.MethodGet, "/api/jobs/"+j.ID.String()+"/recap-plan", nil)
	rw := httptest.NewRecorder()
	r.ServeHTTP(rw, req)
	if rw.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rw.Code, rw.Body.String())
	}
	var got killplan.Plan
	if err := json.Unmarshal(rw.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Segments) != 1 || got.Segments[0].Round != 4 || got.Segments[0].TickStart != 10 {
		t.Fatalf("recap = %#v, want stored full-round window", got.Segments)
	}
}

func TestStartRecordingRejectsUnknownPreset(t *testing.T) {
	repo := newFakeRepo()
	queue := &fakeQueue{}
	plan := killplan.NewPlan()
	j := job.Job{ID: uuid.New(), Status: job.StatusParsed, Rules: rules.Default(), KillPlan: &plan}
	repo.jobs[j.ID] = j
	h := NewHandlers(repo, newFakeStorage(), queue, WithCapabilities(Capabilities{RecordEnabled: true}))

	r := chi.NewRouter()
	r.Post("/api/jobs/{id}/record", h.StartRecording)
	req := httptest.NewRequest(http.MethodPost, "/api/jobs/"+j.ID.String()+"/record", strings.NewReader(`{"preset":"no-such-preset"}`))
	rw := httptest.NewRecorder()
	r.ServeHTTP(rw, req)

	if rw.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 for unknown preset; body=%s", rw.Code, rw.Body.String())
	}
	if len(queue.enqueued) != 0 {
		t.Fatalf("enqueued = %d, want 0", len(queue.enqueued))
	}
}

func TestStartRecordingPassesSelectedSegmentIDs(t *testing.T) {
	repo := newFakeRepo()
	queue := &fakeQueue{}
	plan := killplan.NewPlan()
	plan.Segments = []killplan.Segment{{ID: "seg-001"}, {ID: "seg-002"}}
	j := job.Job{ID: uuid.New(), Status: job.StatusParsed, Rules: rules.Default(), KillPlan: &plan}
	repo.jobs[j.ID] = j
	h := NewHandlers(repo, newFakeStorage(), queue, WithCapabilities(Capabilities{RecordEnabled: true}))

	r := chi.NewRouter()
	r.Post("/api/jobs/{id}/record", h.StartRecording)
	req := httptest.NewRequest(http.MethodPost, "/api/jobs/"+j.ID.String()+"/record", strings.NewReader(`{"segment_ids":["seg-002"]}`))
	rw := httptest.NewRecorder()
	r.ServeHTTP(rw, req)

	if rw.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202; body=%s", rw.Code, rw.Body.String())
	}
	var payload tasks.RecordDemoPayload
	if err := json.Unmarshal(queue.enqueued[0].Payload(), &payload); err != nil {
		t.Fatalf("unmarshal record payload: %v", err)
	}
	if len(payload.SegmentIDs) != 1 || payload.SegmentIDs[0] != "seg-002" {
		t.Fatalf("SegmentIDs = %v, want [seg-002]", payload.SegmentIDs)
	}
}

func TestStartRecordingRejectsUnknownSegmentID(t *testing.T) {
	repo := newFakeRepo()
	queue := &fakeQueue{}
	plan := killplan.NewPlan()
	plan.Segments = []killplan.Segment{{ID: "seg-001"}}
	j := job.Job{ID: uuid.New(), Status: job.StatusParsed, Rules: rules.Default(), KillPlan: &plan}
	repo.jobs[j.ID] = j
	h := NewHandlers(repo, newFakeStorage(), queue, WithCapabilities(Capabilities{RecordEnabled: true}))

	r := chi.NewRouter()
	r.Post("/api/jobs/{id}/record", h.StartRecording)
	req := httptest.NewRequest(http.MethodPost, "/api/jobs/"+j.ID.String()+"/record", strings.NewReader(`{"segment_ids":["seg-001","seg-999"]}`))
	rw := httptest.NewRecorder()
	r.ServeHTTP(rw, req)

	if rw.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 for unknown segment id; body=%s", rw.Code, rw.Body.String())
	}
	if len(queue.enqueued) != 0 {
		t.Fatalf("enqueued = %d, want 0", len(queue.enqueued))
	}
}

func TestStartRecordingRejectsJobWithoutPlan(t *testing.T) {
	repo := newFakeRepo()
	queue := &fakeQueue{}
	j := job.Job{ID: uuid.New(), Status: job.StatusParsed, Rules: rules.Default()}
	repo.jobs[j.ID] = j
	h := NewHandlers(repo, newFakeStorage(), queue, WithCapabilities(Capabilities{RecordEnabled: true}))

	r := chi.NewRouter()
	r.Post("/api/jobs/{id}/record", h.StartRecording)
	req := httptest.NewRequest(http.MethodPost, "/api/jobs/"+j.ID.String()+"/record", nil)
	rw := httptest.NewRecorder()
	r.ServeHTTP(rw, req)

	if rw.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409", rw.Code)
	}
	if len(queue.enqueued) != 0 {
		t.Fatalf("enqueued = %d, want 0", len(queue.enqueued))
	}
}

func TestStartRecordingAllowsIdempotentRetryWhenRecorded(t *testing.T) {
	repo := newFakeRepo()
	queue := &fakeQueue{}
	plan := killplan.NewPlan()
	j := job.Job{ID: uuid.New(), Status: job.StatusRecorded, Rules: rules.Default(), KillPlan: &plan}
	repo.jobs[j.ID] = j
	h := NewHandlers(repo, newFakeStorage(), queue, WithCapabilities(Capabilities{RecordEnabled: true}))

	r := chi.NewRouter()
	r.Post("/api/jobs/{id}/record", h.StartRecording)
	req := httptest.NewRequest(http.MethodPost, "/api/jobs/"+j.ID.String()+"/record", nil)
	rw := httptest.NewRecorder()
	r.ServeHTTP(rw, req)

	if rw.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202; body=%s", rw.Code, rw.Body.String())
	}
	if len(queue.enqueued) != 1 {
		t.Fatalf("enqueued = %d, want 1", len(queue.enqueued))
	}
}

func TestStartRecordingAllowsRetryWhenFailed(t *testing.T) {
	repo := newFakeRepo()
	queue := &fakeQueue{}
	plan := killplan.NewPlan()
	// A capture that failed (CS2 crash) keeps its kill plan; the user retries.
	j := job.Job{ID: uuid.New(), Status: job.StatusFailed, Rules: rules.Default(), KillPlan: &plan}
	repo.jobs[j.ID] = j
	h := NewHandlers(repo, newFakeStorage(), queue, WithCapabilities(Capabilities{RecordEnabled: true}))

	r := chi.NewRouter()
	r.Post("/api/jobs/{id}/record", h.StartRecording)
	req := httptest.NewRequest(http.MethodPost, "/api/jobs/"+j.ID.String()+"/record", nil)
	rw := httptest.NewRecorder()
	r.ServeHTTP(rw, req)

	if rw.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202; body=%s", rw.Code, rw.Body.String())
	}
	if len(queue.enqueued) != 1 {
		t.Fatalf("enqueued = %d, want 1", len(queue.enqueued))
	}
}

func TestStartRecordingRejectsFailedJobWithoutPlan(t *testing.T) {
	repo := newFakeRepo()
	queue := &fakeQueue{}
	// Failed before it was ever parsed: no kill plan, so re-record stays rejected.
	j := job.Job{ID: uuid.New(), Status: job.StatusFailed, Rules: rules.Default()}
	repo.jobs[j.ID] = j
	h := NewHandlers(repo, newFakeStorage(), queue, WithCapabilities(Capabilities{RecordEnabled: true}))

	r := chi.NewRouter()
	r.Post("/api/jobs/{id}/record", h.StartRecording)
	req := httptest.NewRequest(http.MethodPost, "/api/jobs/"+j.ID.String()+"/record", nil)
	rw := httptest.NewRecorder()
	r.ServeHTTP(rw, req)

	if rw.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409", rw.Code)
	}
	if len(queue.enqueued) != 0 {
		t.Fatalf("enqueued = %d, want 0", len(queue.enqueued))
	}
}

func TestStartCompositionEnqueuesComposeTaskWhenRecorded(t *testing.T) {
	repo := newFakeRepo()
	queue := &fakeQueue{}
	j := job.Job{ID: uuid.New(), Status: job.StatusRecorded, Rules: rules.Default()}
	repo.jobs[j.ID] = j
	h := NewHandlers(repo, newFakeStorage(), queue)

	r := chi.NewRouter()
	r.Post("/api/jobs/{id}/compose", h.StartComposition)
	req := httptest.NewRequest(http.MethodPost, "/api/jobs/"+j.ID.String()+"/compose", nil)
	rw := httptest.NewRecorder()
	r.ServeHTTP(rw, req)

	if rw.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202; body=%s", rw.Code, rw.Body.String())
	}
	if len(queue.enqueued) != 1 {
		t.Fatalf("enqueued = %d, want 1", len(queue.enqueued))
	}
	if queue.enqueued[0].Type() != tasks.TypeComposeFinal {
		t.Fatalf("task type = %q, want %q", queue.enqueued[0].Type(), tasks.TypeComposeFinal)
	}
}

func TestStartCompositionAllowsIdempotentRetryWhenComposed(t *testing.T) {
	repo := newFakeRepo()
	queue := &fakeQueue{}
	j := job.Job{ID: uuid.New(), Status: job.StatusComposed, Rules: rules.Default()}
	repo.jobs[j.ID] = j
	h := NewHandlers(repo, newFakeStorage(), queue)

	r := chi.NewRouter()
	r.Post("/api/jobs/{id}/compose", h.StartComposition)
	req := httptest.NewRequest(http.MethodPost, "/api/jobs/"+j.ID.String()+"/compose", nil)
	rw := httptest.NewRecorder()
	r.ServeHTTP(rw, req)

	if rw.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202; body=%s", rw.Code, rw.Body.String())
	}
	if len(queue.enqueued) != 1 {
		t.Fatalf("enqueued = %d, want 1", len(queue.enqueued))
	}
}

func TestStartCompositionRejectsWrongStatus(t *testing.T) {
	repo := newFakeRepo()
	queue := &fakeQueue{}
	j := job.Job{ID: uuid.New(), Status: job.StatusParsed, Rules: rules.Default()}
	repo.jobs[j.ID] = j
	h := NewHandlers(repo, newFakeStorage(), queue)

	r := chi.NewRouter()
	r.Post("/api/jobs/{id}/compose", h.StartComposition)
	req := httptest.NewRequest(http.MethodPost, "/api/jobs/"+j.ID.String()+"/compose", nil)
	rw := httptest.NewRecorder()
	r.ServeHTTP(rw, req)

	if rw.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409", rw.Code)
	}
	if len(queue.enqueued) != 0 {
		t.Fatalf("enqueued = %d, want 0", len(queue.enqueued))
	}
}

func TestStartRenderVariantEnqueuesRenderTaskWhenRecorded(t *testing.T) {
	repo := newFakeRepo()
	queue := &fakeQueue{}
	j := job.Job{ID: uuid.New(), Status: job.StatusRecorded, Rules: rules.Default()}
	repo.jobs[j.ID] = j
	h := NewHandlers(repo, newFakeStorage(), queue)

	r := chi.NewRouter()
	r.Post("/api/jobs/{id}/renders/{variant}", h.StartRenderVariant)
	req := httptest.NewRequest(http.MethodPost, "/api/jobs/"+j.ID.String()+"/renders/viral-60-clean", strings.NewReader(`{"music":"track01","edit":{"format":"landscape-16x9","killEffect":"velocity","transition":"whip","intro":true,"outro":true,"hook_text":true,"kill_counter":true,"cover_strategy":"no-cover","intro_text":"Watch this ace","outro_text":"follow for more"}}`))
	req.Header.Set("Content-Type", "application/json")
	rw := httptest.NewRecorder()
	r.ServeHTTP(rw, req)

	if rw.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202; body=%s", rw.Code, rw.Body.String())
	}
	if len(queue.enqueued) != 1 {
		t.Fatalf("enqueued = %d, want 1", len(queue.enqueued))
	}
	if queue.enqueued[0].Type() != tasks.TypeRenderVariant {
		t.Fatalf("task type = %q, want %q", queue.enqueued[0].Type(), tasks.TypeRenderVariant)
	}
	var payload tasks.RenderVariantPayload
	if err := json.Unmarshal(queue.enqueued[0].Payload(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.JobID != j.ID || payload.Variant != editor.PresetViral60Clean {
		t.Fatalf("payload = %#v, want job %s variant %s", payload, j.ID, editor.PresetViral60Clean)
	}
	if payload.MusicKey != "track01" {
		t.Fatalf("music key = %q, want track01", payload.MusicKey)
	}
	if payload.Edit.Format != renderplan.FormatLandscape16x9 || payload.Edit.KillEffect != renderplan.KillEffectVelocity || payload.Edit.Transition != renderplan.TransitionWhip || !payload.Edit.Intro || !payload.Edit.Outro {
		t.Fatalf("edit payload = %#v", payload.Edit)
	}
	if payload.Edit.IntroText != "Watch this ace" || payload.Edit.OutroText != "follow for more" {
		t.Fatalf("edit bookend text = %q / %q, want round-tripped custom text", payload.Edit.IntroText, payload.Edit.OutroText)
	}
	if !payload.Edit.HookText || !payload.Edit.KillCounter {
		t.Fatalf("edit automatic text = hook %v / counter %v, want true / true", payload.Edit.HookText, payload.Edit.KillCounter)
	}
	if payload.Edit.CoverStrategy != renderplan.CoverStrategyNone {
		t.Fatalf("edit cover strategy = %q, want %q", payload.Edit.CoverStrategy, renderplan.CoverStrategyNone)
	}
	if len(queue.options) != 1 || !hasAsynqOption(queue.options[0], "Unique(") {
		t.Fatalf("enqueue options = %#v, want Unique option", queue.options)
	}
	if !hasAsynqOption(queue.options[0], "MaxRetry(0)") {
		t.Fatalf("enqueue options = %#v, want MaxRetry(0) so media render never auto-retries", queue.options)
	}
	statusKey, err := artifacts.RenderVariantStatusKey(j.ID, editor.PresetViral60Clean)
	if err != nil {
		t.Fatal(err)
	}
	var state renderplan.RenderVariantState
	if err := json.Unmarshal(storeBytes(t, h.storage, statusKey), &state); err != nil {
		t.Fatal(err)
	}
	if got, want := state.Status, renderplan.RenderVariantStatusQueued; got != want {
		t.Fatalf("state status = %q, want %q", got, want)
	}
}

func TestStartRenderVariantRejectsOutOfRangeMusicVolume(t *testing.T) {
	repo := newFakeRepo()
	queue := &fakeQueue{}
	j := job.Job{ID: uuid.New(), Status: job.StatusRecorded, Rules: rules.Default()}
	repo.jobs[j.ID] = j
	h := NewHandlers(repo, newFakeStorage(), queue)

	r := chi.NewRouter()
	r.Post("/api/jobs/{id}/renders/{variant}", h.StartRenderVariant)
	body := `{"music":{"key":"track01","volume":1.5}}`
	req := httptest.NewRequest(http.MethodPost, "/api/jobs/"+j.ID.String()+"/renders/viral-60-clean", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rw := httptest.NewRecorder()
	r.ServeHTTP(rw, req)

	if rw.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body=%s", rw.Code, rw.Body.String())
	}
	if len(queue.enqueued) != 0 {
		t.Fatalf("enqueued = %d, want 0 for rejected volume", len(queue.enqueued))
	}
}

func TestStartRenderVariantThreadsMusicVolume(t *testing.T) {
	repo := newFakeRepo()
	queue := &fakeQueue{}
	j := job.Job{ID: uuid.New(), Status: job.StatusRecorded, Rules: rules.Default()}
	repo.jobs[j.ID] = j
	h := NewHandlers(repo, newFakeStorage(), queue)

	r := chi.NewRouter()
	r.Post("/api/jobs/{id}/renders/{variant}", h.StartRenderVariant)
	body := `{"music":{"key":"track01","volume":0.35}}`
	req := httptest.NewRequest(http.MethodPost, "/api/jobs/"+j.ID.String()+"/renders/viral-60-clean", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rw := httptest.NewRecorder()
	r.ServeHTTP(rw, req)

	if rw.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202; body=%s", rw.Code, rw.Body.String())
	}
	if len(queue.enqueued) != 1 {
		t.Fatalf("enqueued = %d, want 1", len(queue.enqueued))
	}
	var payload tasks.RenderVariantPayload
	if err := json.Unmarshal(queue.enqueued[0].Payload(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.MusicKey != "track01" || payload.MusicVolume != 0.35 {
		t.Fatalf("music = %q/%v, want track01/0.35", payload.MusicKey, payload.MusicVolume)
	}
}

func TestStartRenderVariantThreadsGameAndVoiceVolume(t *testing.T) {
	repo := newFakeRepo()
	queue := &fakeQueue{}
	j := job.Job{ID: uuid.New(), Status: job.StatusRecorded, Rules: rules.Default()}
	repo.jobs[j.ID] = j
	h := NewHandlers(repo, newFakeStorage(), queue)

	r := chi.NewRouter()
	r.Post("/api/jobs/{id}/renders/{variant}", h.StartRenderVariant)
	body := `{"music":{"key":"track01","volume":0.8,"game_volume":0.2},"edit":{"voice_comms":true,"voice_volume":0.85}}`
	req := httptest.NewRequest(http.MethodPost, "/api/jobs/"+j.ID.String()+"/renders/viral-60-clean", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rw := httptest.NewRecorder()
	r.ServeHTTP(rw, req)

	if rw.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202; body=%s", rw.Code, rw.Body.String())
	}
	var payload tasks.RenderVariantPayload
	if err := json.Unmarshal(queue.enqueued[0].Payload(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.GameVolume == nil || *payload.GameVolume != 0.2 {
		t.Fatalf("game volume = %v, want 0.2", payload.GameVolume)
	}
	if !payload.Edit.VoiceComms || payload.Edit.VoiceVolume == nil || *payload.Edit.VoiceVolume != 0.85 {
		t.Fatalf("voice = comms=%v volume=%v, want true/0.85", payload.Edit.VoiceComms, payload.Edit.VoiceVolume)
	}
}

func TestStartRenderVariantRejectsOutOfRangeGameVolume(t *testing.T) {
	repo := newFakeRepo()
	queue := &fakeQueue{}
	j := job.Job{ID: uuid.New(), Status: job.StatusRecorded, Rules: rules.Default()}
	repo.jobs[j.ID] = j
	h := NewHandlers(repo, newFakeStorage(), queue)

	r := chi.NewRouter()
	r.Post("/api/jobs/{id}/renders/{variant}", h.StartRenderVariant)
	body := `{"music":{"key":"track01","game_volume":1.5}}`
	req := httptest.NewRequest(http.MethodPost, "/api/jobs/"+j.ID.String()+"/renders/viral-60-clean", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rw := httptest.NewRecorder()
	r.ServeHTTP(rw, req)

	if rw.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body=%s", rw.Code, rw.Body.String())
	}
	if len(queue.enqueued) != 0 {
		t.Fatalf("enqueued = %d, want 0 for rejected game volume", len(queue.enqueued))
	}
}

func TestStartRenderVariantRejectsWhileGuidedGenerateIsActive(t *testing.T) {
	repo := newFakeRepo()
	store := newFakeStorage()
	queue := &fakeQueue{}
	j := job.Job{ID: uuid.New(), Status: job.StatusRecorded, Rules: rules.Default()}
	repo.jobs[j.ID] = j
	h := NewHandlers(repo, store, queue)
	if err := h.generateIntents.Begin(j.ID, renderplan.GenerateIntent{
		Variant:     editor.PresetViral60Clean,
		Edit:        renderplan.DefaultEditRequest(),
		ActiveRunID: uuid.New(),
	}, nil); err != nil {
		t.Fatal(err)
	}

	r := chi.NewRouter()
	r.Post("/api/jobs/{id}/renders/{variant}", h.StartRenderVariant)
	req := httptest.NewRequest(http.MethodPost, "/api/jobs/"+j.ID.String()+"/renders/viral-60-clean", nil)
	rw := httptest.NewRecorder()
	r.ServeHTTP(rw, req)

	if rw.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409; body=%s", rw.Code, rw.Body.String())
	}
	if len(queue.enqueued) != 0 {
		t.Fatalf("enqueued = %d, want 0 while guided generate is active", len(queue.enqueued))
	}
	if _, ok := store.puts[mustRenderVariantStatusKey(j.ID, editor.PresetViral60Clean)]; ok {
		t.Fatal("manual render conflict published a queued render state")
	}
}

func TestStartRenderVariantPreservesReadyStateWhenTaskIsDuplicate(t *testing.T) {
	repo := newFakeRepo()
	store := newFakeStorage()
	queue := &fakeQueue{err: asynq.ErrDuplicateTask}
	j := job.Job{ID: uuid.New(), Status: job.StatusRecorded, Rules: rules.Default()}
	repo.jobs[j.ID] = j
	h := NewHandlers(repo, store, queue)
	loadout, err := renderplan.LoadoutForVariant(editor.PresetViral60Clean)
	if err != nil {
		t.Fatalf("LoadoutForVariant error = %v", err)
	}
	ready, err := renderplan.NewRenderVariantStateForLoadout(renderplan.NewRenderVariantStateForLoadoutOptions{
		JobID:   j.ID,
		Loadout: loadout,
		Status:  renderplan.RenderVariantStatusReady,
	})
	if err != nil {
		t.Fatalf("NewRenderVariantStateForLoadout error = %v", err)
	}
	if err := h.writeRenderVariantState(ready); err != nil {
		t.Fatalf("writeRenderVariantState error = %v", err)
	}
	putAssistantJSON(t, store, ready.RenderResultKey, editor.Result{
		Preset: editor.PresetViral60Clean,
	})

	r := chi.NewRouter()
	r.Post("/api/jobs/{id}/renders/{variant}", h.StartRenderVariant)
	req := httptest.NewRequest(http.MethodPost, "/api/jobs/"+j.ID.String()+"/renders/viral-60-clean", nil)
	rw := httptest.NewRecorder()
	r.ServeHTTP(rw, req)

	if rw.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202; body=%s", rw.Code, rw.Body.String())
	}
	if !strings.Contains(rw.Body.String(), `"status":"ready"`) {
		t.Fatalf("duplicate response did not preserve ready state: %s", rw.Body.String())
	}
	state, ok, err := h.readRenderVariantState(j.ID, editor.PresetViral60Clean)
	if err != nil || !ok {
		t.Fatalf("readRenderVariantState = (%v, %v, %v)", state, ok, err)
	}
	if state.Status != renderplan.RenderVariantStatusReady {
		t.Fatalf("state status = %q, want ready", state.Status)
	}
}

func TestStartRenderVariantReviewReplacementUsesExactRevisionCAS(t *testing.T) {
	tests := []struct {
		name       string
		queueErr   error
		prefix     string
		warnings   string
		wantStatus int
		wantQueued bool
	}{
		{
			name:       "accepted replacement retains committed artifact pointer",
			prefix:     "current",
			warnings:   `["freeze at 00:12"]`,
			wantStatus: http.StatusAccepted,
			wantQueued: true,
		},
		{
			name:       "stale revision is rejected before enqueue",
			prefix:     "stale",
			warnings:   `["freeze at 00:12"]`,
			wantStatus: http.StatusConflict,
		},
		{
			name:       "stale warnings are rejected before enqueue",
			prefix:     "current",
			warnings:   `["different warning"]`,
			wantStatus: http.StatusConflict,
		},
		{
			name:       "missing expectations cannot replace a review",
			warnings:   `null`,
			wantStatus: http.StatusConflict,
		},
		{
			name:       "duplicate task does not claim correction was accepted",
			queueErr:   asynq.ErrDuplicateTask,
			prefix:     "current",
			warnings:   `["freeze at 00:12"]`,
			wantStatus: http.StatusConflict,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := newFakeRepo()
			store := newFakeStorage()
			queue := &fakeQueue{err: tc.queueErr}
			j := job.Job{ID: uuid.New(), Status: job.StatusRecorded, Rules: rules.Default()}
			repo.jobs[j.ID] = j
			h := NewHandlers(repo, store, queue)
			loadout, err := renderplan.LoadoutForVariant(editor.PresetViral60Clean)
			if err != nil {
				t.Fatal(err)
			}
			review, err := renderplan.NewRenderVariantStateForLoadout(renderplan.NewRenderVariantStateForLoadoutOptions{
				JobID:      j.ID,
				Loadout:    loadout,
				Status:     renderplan.RenderVariantStatusReview,
				Warnings:   []string{"freeze at 00:12"},
				RevisionID: uuid.New(),
			})
			if err != nil {
				t.Fatal(err)
			}
			if err := h.writeRenderVariantState(review); err != nil {
				t.Fatal(err)
			}
			putAssistantJSON(t, store, review.EditDocumentKey, renderplan.EditDocument{
				SchemaVersion: renderplan.EditDocumentSchemaVersion,
				Edit:          renderplan.DefaultEditRequest(),
				Music:         &renderplan.MusicSnapshot{},
			})

			expectedPrefix := tc.prefix
			if expectedPrefix == "current" {
				expectedPrefix = review.ArtifactPrefix
			}
			body := fmt.Sprintf(
				`{"expected_artifact_prefix":%q,"expected_warnings":%s,"edit":{"transition":"whip"}}`,
				expectedPrefix,
				tc.warnings,
			)
			r := chi.NewRouter()
			r.Post("/api/jobs/{id}/renders/{variant}", h.StartRenderVariant)
			req := httptest.NewRequest(
				http.MethodPost,
				"/api/jobs/"+j.ID.String()+"/renders/viral-60-clean",
				strings.NewReader(body),
			)
			req.Header.Set("Content-Type", "application/json")
			rw := httptest.NewRecorder()
			r.ServeHTTP(rw, req)
			if rw.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d; body=%s", rw.Code, tc.wantStatus, rw.Body.String())
			}

			state, ok, err := h.readRenderVariantState(j.ID, editor.PresetViral60Clean)
			if err != nil || !ok {
				t.Fatalf("readRenderVariantState = (%v, %v, %v)", state, ok, err)
			}
			if tc.wantQueued {
				if state.Status != renderplan.RenderVariantStatusQueued {
					t.Fatalf("state status = %q, want queued", state.Status)
				}
				if state.ArtifactPrefix != review.ArtifactPrefix ||
					state.RenderResultKey != review.RenderResultKey {
					t.Fatalf("queued state lost committed revision pointer: %#v", state)
				}
				if len(queue.enqueued) != 1 {
					t.Fatalf("enqueued = %d, want 1", len(queue.enqueued))
				}
			} else {
				if state.Status != renderplan.RenderVariantStatusReview ||
					state.ArtifactPrefix != review.ArtifactPrefix {
					t.Fatalf("rejected replacement changed review state: %#v", state)
				}
				if len(queue.enqueued) != 0 {
					t.Fatalf("enqueued = %d, want 0", len(queue.enqueued))
				}
			}
		})
	}
}

func TestStartRenderVariantRejectsUnknownCorrectionFieldsWithoutReplacingReview(t *testing.T) {
	tests := []struct {
		name  string
		patch string
	}{
		{
			name:  "top-level field",
			patch: `"correccion":"whip"`,
		},
		{
			name:  "edit field",
			patch: `"edit":{"transiton":"whip"}`,
		},
		{
			name:  "music object field",
			patch: `"music":{"key":"phonk-01","volum":0.35}`,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := newFakeRepo()
			store := newFakeStorage()
			queue := &fakeQueue{}
			j := job.Job{ID: uuid.New(), Status: job.StatusRecorded, Rules: rules.Default()}
			repo.jobs[j.ID] = j
			h := NewHandlers(repo, store, queue)
			loadout, err := renderplan.LoadoutForVariant(editor.PresetViral60Clean)
			if err != nil {
				t.Fatal(err)
			}
			review, err := renderplan.NewRenderVariantStateForLoadout(renderplan.NewRenderVariantStateForLoadoutOptions{
				JobID:      j.ID,
				Loadout:    loadout,
				Status:     renderplan.RenderVariantStatusReview,
				Warnings:   []string{"freeze at 00:12"},
				RevisionID: uuid.New(),
			})
			if err != nil {
				t.Fatal(err)
			}
			if err := h.writeRenderVariantState(review); err != nil {
				t.Fatal(err)
			}

			body := fmt.Sprintf(
				`{"expected_artifact_prefix":%q,"expected_warnings":["freeze at 00:12"],%s}`,
				review.ArtifactPrefix,
				tc.patch,
			)
			r := chi.NewRouter()
			r.Post("/api/jobs/{id}/renders/{variant}", h.StartRenderVariant)
			req := httptest.NewRequest(
				http.MethodPost,
				"/api/jobs/"+j.ID.String()+"/renders/viral-60-clean",
				strings.NewReader(body),
			)
			req.Header.Set("Content-Type", "application/json")
			rw := httptest.NewRecorder()
			r.ServeHTTP(rw, req)

			if rw.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400; body=%s", rw.Code, rw.Body.String())
			}
			if len(queue.enqueued) != 0 {
				t.Fatalf("enqueued = %d, want no replacement for unknown field", len(queue.enqueued))
			}
			state, ok, err := h.readRenderVariantState(j.ID, editor.PresetViral60Clean)
			if err != nil || !ok {
				t.Fatalf("readRenderVariantState = (%v, %v, %v)", state, ok, err)
			}
			same, compareErr := sameRenderVariantState(state, &review)
			if compareErr != nil {
				t.Fatalf("compare render state: %v", compareErr)
			}
			if !same {
				t.Fatalf("unknown correction field changed review state:\ngot  %#v\nwant %#v", state, review)
			}
		})
	}
}

func TestStartRenderVariantPartialReviewCorrectionPreservesEffectiveEditAndMusic(t *testing.T) {
	repo := newFakeRepo()
	store := newFakeStorage()
	queue := &fakeQueue{}
	j := job.Job{ID: uuid.New(), Status: job.StatusRecorded, Rules: rules.Default()}
	repo.jobs[j.ID] = j
	h := NewHandlers(repo, store, queue)
	loadout, err := renderplan.LoadoutForVariant(editor.PresetViral60Clean)
	if err != nil {
		t.Fatal(err)
	}
	review, err := renderplan.NewRenderVariantStateForLoadout(renderplan.NewRenderVariantStateForLoadoutOptions{
		JobID:      j.ID,
		Loadout:    loadout,
		Status:     renderplan.RenderVariantStatusReview,
		Warnings:   []string{"freeze at 00:12"},
		RevisionID: uuid.New(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := h.writeRenderVariantState(review); err != nil {
		t.Fatal(err)
	}
	effectiveEdit := renderplan.EditRequest{
		Format:          renderplan.FormatLandscape16x9,
		KillEffect:      renderplan.KillEffectVelocity,
		Transition:      renderplan.TransitionFlash,
		Intro:           true,
		Outro:           true,
		HookText:        true,
		KillCounter:     true,
		CoverStrategy:   renderplan.CoverStrategyGenerated,
		CoverFirstFrame: true,
		IntroText:       "Approved intro",
		OutroText:       "Approved outro",
	}
	putAssistantJSON(t, store, review.EditDocumentKey, renderplan.EditDocument{
		SchemaVersion: renderplan.EditDocumentSchemaVersion,
		Edit:          effectiveEdit,
		Music:         &renderplan.MusicSnapshot{Key: "phonk-01", Volume: 0.35},
	})

	body := fmt.Sprintf(
		`{"expected_artifact_prefix":%q,"expected_warnings":["freeze at 00:12"],"edit":{"transition":"whip"}}`,
		review.ArtifactPrefix,
	)
	r := chi.NewRouter()
	r.Post("/api/jobs/{id}/renders/{variant}", h.StartRenderVariant)
	req := httptest.NewRequest(
		http.MethodPost,
		"/api/jobs/"+j.ID.String()+"/renders/viral-60-clean",
		strings.NewReader(body),
	)
	req.Header.Set("Content-Type", "application/json")
	rw := httptest.NewRecorder()
	r.ServeHTTP(rw, req)

	if rw.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202; body=%s", rw.Code, rw.Body.String())
	}
	if len(queue.enqueued) != 1 {
		t.Fatalf("enqueued = %d, want 1", len(queue.enqueued))
	}
	var payload tasks.RenderVariantPayload
	if err := json.Unmarshal(queue.enqueued[0].Payload(), &payload); err != nil {
		t.Fatal(err)
	}
	effectiveEdit.Transition = renderplan.TransitionWhip
	if payload.Edit != effectiveEdit {
		t.Fatalf("edit payload = %#v, want merged %#v", payload.Edit, effectiveEdit)
	}
	if payload.MusicKey != "phonk-01" || payload.MusicVolume != 0.35 {
		t.Fatalf("music payload = %q/%v, want preserved phonk-01/0.35", payload.MusicKey, payload.MusicVolume)
	}
}

func TestGetRenderVariantReturnsEffectiveEditForCurrentRevision(t *testing.T) {
	repo := newFakeRepo()
	store := newFakeStorage()
	j := job.Job{ID: uuid.New(), Status: job.StatusRecorded, Rules: rules.Default()}
	repo.jobs[j.ID] = j
	loadout, err := renderplan.LoadoutForVariant(editor.PresetViral60Clean)
	if err != nil {
		t.Fatal(err)
	}
	state, err := renderplan.NewRenderVariantStateForLoadout(renderplan.NewRenderVariantStateForLoadoutOptions{
		JobID:      j.ID,
		Loadout:    loadout,
		Status:     renderplan.RenderVariantStatusReady,
		RevisionID: uuid.New(),
	})
	if err != nil {
		t.Fatal(err)
	}
	h := NewHandlers(repo, store, &fakeQueue{})
	if err := h.writeRenderVariantState(state); err != nil {
		t.Fatal(err)
	}
	putAssistantJSON(t, store, state.RenderResultKey, editor.Result{Preset: editor.PresetViral60Clean})
	putAssistantJSON(t, store, state.EditDocumentKey, renderplan.EditDocument{
		SchemaVersion: renderplan.EditDocumentSchemaVersion,
		Edit: renderplan.EditRequest{
			Format:        renderplan.FormatLandscape16x9,
			KillEffect:    renderplan.KillEffectVelocity,
			Transition:    renderplan.TransitionWhip,
			Intro:         true,
			HookText:      true,
			CoverStrategy: renderplan.CoverStrategyNone,
		},
		Music: &renderplan.MusicSnapshot{Key: "phonk-01", Volume: 0.35},
	})

	r := chi.NewRouter()
	r.Get("/api/jobs/{id}/renders/{variant}", h.GetRenderVariant)
	req := httptest.NewRequest(
		http.MethodGet,
		"/api/jobs/"+j.ID.String()+"/renders/viral-60-clean",
		nil,
	)
	rw := httptest.NewRecorder()
	r.ServeHTTP(rw, req)
	if rw.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rw.Code, rw.Body.String())
	}
	if !strings.Contains(rw.Body.String(), `"edit":{"format":"landscape-16x9","killEffect":"velocity","transition":"whip"`) {
		t.Fatalf("response missing effective edit: %s", rw.Body.String())
	}
	if !strings.Contains(rw.Body.String(), `"music":{"key":"phonk-01","volume":0.35}`) {
		t.Fatalf("response missing effective music: %s", rw.Body.String())
	}
}

func TestRenderPublishBoardBlocksFailedReplacementState(t *testing.T) {
	repo := newFakeRepo()
	store := newFakeStorage()
	j := job.Job{ID: uuid.New(), Status: job.StatusRecorded, Rules: rules.Default()}
	repo.jobs[j.ID] = j
	loadout, err := renderplan.LoadoutForVariant(editor.PresetViral60Clean)
	if err != nil {
		t.Fatal(err)
	}
	state, err := renderplan.NewRenderVariantStateForLoadout(renderplan.NewRenderVariantStateForLoadoutOptions{
		JobID:      j.ID,
		Loadout:    loadout,
		Status:     renderplan.RenderVariantStatusFailed,
		Error:      "replacement render failed",
		RevisionID: uuid.New(),
	})
	if err != nil {
		t.Fatal(err)
	}
	h := NewHandlers(repo, store, &fakeQueue{})
	if err := h.writeRenderVariantState(state); err != nil {
		t.Fatal(err)
	}
	putAssistantJSON(t, store, state.RenderResultKey, editor.Result{
		Shorts: []editor.ShortResult{{SegmentID: "seg-001"}},
	})
	for _, kind := range []renderplan.RenderVariantArtifactKind{
		renderplan.RenderVariantArtifactVideo,
		renderplan.RenderVariantArtifactCaption,
	} {
		ref, err := renderplan.NewRenderVariantArtifactRefForState(state, kind, "seg-001")
		if err != nil {
			t.Fatal(err)
		}
		store.puts[ref.Key] = []byte("artifact")
	}

	r := chi.NewRouter()
	r.Get("/api/jobs/{id}/renders/{variant}/publish", h.GetRenderPublishBoard)
	req := httptest.NewRequest(
		http.MethodGet,
		"/api/jobs/"+j.ID.String()+"/renders/viral-60-clean/publish",
		nil,
	)
	rw := httptest.NewRecorder()
	r.ServeHTTP(rw, req)
	if rw.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rw.Code, rw.Body.String())
	}
	if !strings.Contains(rw.Body.String(), `"status":"failed"`) ||
		!strings.Contains(rw.Body.String(), `"render_ready":false`) ||
		!strings.Contains(rw.Body.String(), `"error":"replacement render failed"`) {
		t.Fatalf("publish board did not block failed replacement: %s", rw.Body.String())
	}
}

func TestStartRenderVariantMarksStateFailedWhenEnqueueFails(t *testing.T) {
	repo := newFakeRepo()
	store := newFakeStorage()
	queue := &fakeQueue{err: errors.New("inline queue is full")}
	j := job.Job{ID: uuid.New(), Status: job.StatusRecorded, Rules: rules.Default()}
	repo.jobs[j.ID] = j
	h := NewHandlers(repo, store, queue)

	r := chi.NewRouter()
	r.Post("/api/jobs/{id}/renders/{variant}", h.StartRenderVariant)
	req := httptest.NewRequest(http.MethodPost, "/api/jobs/"+j.ID.String()+"/renders/viral-60-clean", nil)
	rw := httptest.NewRecorder()
	r.ServeHTTP(rw, req)

	if rw.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500; body=%s", rw.Code, rw.Body.String())
	}
	state, ok, err := h.readRenderVariantState(j.ID, editor.PresetViral60Clean)
	if err != nil || !ok {
		t.Fatalf("readRenderVariantState = (%v, %v, %v)", state, ok, err)
	}
	if state.Status != renderplan.RenderVariantStatusFailed {
		t.Fatalf("state status = %q, want failed", state.Status)
	}
	if state.Error != "enqueue render task: inline queue is full" {
		t.Fatalf("state error = %q", state.Error)
	}
}

func TestStartRenderVariantMarksAcceptedPendingStateFailedWhenQueueDiscardsIt(t *testing.T) {
	repo := newFakeRepo()
	store := newFakeStorage()
	queue := &fakeQueue{}
	j := job.Job{ID: uuid.New(), Status: job.StatusRecorded, Rules: rules.Default()}
	repo.jobs[j.ID] = j
	h := NewHandlers(repo, store, queue)
	r := chi.NewRouter()
	r.Post("/api/jobs/{id}/renders/{variant}", h.StartRenderVariant)

	req := httptest.NewRequest(http.MethodPost, "/api/jobs/"+j.ID.String()+"/renders/viral-60-clean", nil)
	rw := httptest.NewRecorder()
	r.ServeHTTP(rw, req)
	if rw.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202; body=%s", rw.Code, rw.Body.String())
	}
	if len(queue.transitions) != 1 {
		t.Fatalf("queue transitions = %d, want 1", len(queue.transitions))
	}
	if err := queue.transitions[0](errors.New("inline queue task discarded during shutdown")); err != nil {
		t.Fatalf("discard transition error = %v", err)
	}
	state, ok, err := h.readRenderVariantState(j.ID, editor.PresetViral60Clean)
	if err != nil || !ok {
		t.Fatalf("readRenderVariantState = (%v, %v, %v)", state, ok, err)
	}
	if state.Status != renderplan.RenderVariantStatusFailed || !strings.Contains(state.Error, "discarded during shutdown") {
		t.Fatalf("state after discard = status %q, error %q; want failed discard reason", state.Status, state.Error)
	}
}

func TestStartRenderVariantRejectsOverlongBookendText(t *testing.T) {
	repo := newFakeRepo()
	queue := &fakeQueue{}
	j := job.Job{ID: uuid.New(), Status: job.StatusRecorded, Rules: rules.Default()}
	repo.jobs[j.ID] = j
	h := NewHandlers(repo, newFakeStorage(), queue)

	r := chi.NewRouter()
	r.Post("/api/jobs/{id}/renders/{variant}", h.StartRenderVariant)
	body := fmt.Sprintf(`{"edit":{"outro_text":"%s"}}`, strings.Repeat("a", 81))
	req := httptest.NewRequest(http.MethodPost, "/api/jobs/"+j.ID.String()+"/renders/viral-60-clean", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rw := httptest.NewRecorder()
	r.ServeHTTP(rw, req)

	if rw.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body=%s", rw.Code, rw.Body.String())
	}
	if len(queue.enqueued) != 0 {
		t.Fatalf("enqueued = %d, want 0", len(queue.enqueued))
	}
}

func TestGetRenderVariantReturnsQueuedState(t *testing.T) {
	repo := newFakeRepo()
	store := newFakeStorage()
	j := job.Job{ID: uuid.New(), Status: job.StatusRecorded, Rules: rules.Default()}
	repo.jobs[j.ID] = j
	loadout, err := renderplan.LoadoutForVariant(editor.PresetViral60Clean)
	if err != nil {
		t.Fatal(err)
	}
	state, err := renderplan.NewRenderVariantStateForLoadout(renderplan.NewRenderVariantStateForLoadoutOptions{
		JobID:   j.ID,
		Loadout: loadout,
		Status:  renderplan.RenderVariantStatusQueued,
	})
	if err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	statusKey, err := artifacts.RenderVariantStatusKey(j.ID, editor.PresetViral60Clean)
	if err != nil {
		t.Fatal(err)
	}
	_ = store.Put(statusKey, bytes.NewReader(b))
	h := NewHandlers(repo, store, &fakeQueue{})

	r := chi.NewRouter()
	r.Get("/api/jobs/{id}/renders/{variant}", h.GetRenderVariant)
	req := httptest.NewRequest(http.MethodGet, "/api/jobs/"+j.ID.String()+"/renders/viral-60-clean", nil)
	rw := httptest.NewRecorder()
	r.ServeHTTP(rw, req)

	if rw.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rw.Code, rw.Body.String())
	}
	if !strings.Contains(rw.Body.String(), `"status":"queued"`) {
		t.Fatalf("body missing queued state: %s", rw.Body.String())
	}
	if !strings.Contains(rw.Body.String(), "edit-document.json") {
		t.Fatalf("body missing artifact keys: %s", rw.Body.String())
	}
}

func TestLegacyWarningRenderCanBeResolvedOrRerenderedAfterGet(t *testing.T) {
	const warning = "freeze at 00:12.400"
	for _, action := range []string{"resolve", "rerender"} {
		t.Run(action, func(t *testing.T) {
			repo := newFakeRepo()
			store := newFakeStorage()
			queue := &fakeQueue{}
			j := job.Job{ID: uuid.New(), Status: job.StatusRecorded, Rules: rules.Default()}
			repo.jobs[j.ID] = j
			variant := editor.PresetViral60Clean
			resultKey, err := artifacts.RenderVariantResultKey(j.ID, variant)
			if err != nil {
				t.Fatal(err)
			}
			putAssistantJSON(t, store, resultKey, editor.Result{
				Preset:   variant,
				Warnings: []string{warning},
			})
			h := NewHandlers(repo, store, queue)
			r := chi.NewRouter()
			r.Get("/api/jobs/{id}/renders/{variant}", h.GetRenderVariant)
			r.Post("/api/jobs/{id}/renders/{variant}", h.StartRenderVariant)
			r.Post("/api/jobs/{id}/renders/{variant}/review", h.ResolveRenderReview)

			path := "/api/jobs/" + j.ID.String() + "/renders/" + variant
			req := httptest.NewRequest(http.MethodGet, path, nil)
			rw := httptest.NewRecorder()
			r.ServeHTTP(rw, req)
			if rw.Code != http.StatusOK {
				t.Fatalf("legacy GET status = %d, want 200; body=%s", rw.Code, rw.Body.String())
			}
			state, exists, err := h.readRenderVariantState(j.ID, variant)
			if err != nil || !exists {
				t.Fatalf("materialized state = (%#v, %v, %v), want durable review", state, exists, err)
			}
			if state.Status != renderplan.RenderVariantStatusReview ||
				!slices.Equal(state.Warnings, []string{warning}) {
				t.Fatalf("materialized state = %#v, want exact legacy review", state)
			}

			var body string
			if action == "resolve" {
				body = fmt.Sprintf(
					`{"note":"intentional beat hold","expected_artifact_prefix":%q,"expected_warnings":[%q]}`,
					state.ArtifactPrefix,
					warning,
				)
				path += "/review"
			} else {
				body = fmt.Sprintf(
					`{"expected_artifact_prefix":%q,"expected_warnings":[%q],"music":null,"edit":{"format":"short-9x16","killEffect":"punch-in","transition":"whip","intro":false,"outro":false,"hook_text":false,"kill_counter":false,"cover_strategy":"generated-gameplay","cover_first_frame":false,"intro_text":"","outro_text":""}}`,
					state.ArtifactPrefix,
					warning,
				)
			}
			req = httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			rw = httptest.NewRecorder()
			r.ServeHTTP(rw, req)

			wantStatus := http.StatusOK
			wantState := renderplan.RenderVariantStatusReady
			if action == "rerender" {
				wantStatus = http.StatusAccepted
				wantState = renderplan.RenderVariantStatusQueued
			}
			if rw.Code != wantStatus {
				t.Fatalf("%s status = %d, want %d; body=%s", action, rw.Code, wantStatus, rw.Body.String())
			}
			state, exists, err = h.readRenderVariantState(j.ID, variant)
			if err != nil || !exists || state.Status != wantState {
				t.Fatalf("%s state = (%#v, %v, %v), want %s", action, state, exists, err, wantState)
			}
			if action == "rerender" && len(queue.enqueued) != 1 {
				t.Fatalf("rerender enqueued = %d, want 1", len(queue.enqueued))
			}
		})
	}
}

func TestReadyRenderWarningStateMigratesAndRemainsActionable(t *testing.T) {
	const rendererWarning = "freeze at 00:12.400"
	expectedWarnings := []string{
		rendererWarning,
		"quality seg-001: unexpected_output_resolution",
	}
	for _, action := range []string{"resolve", "rerender"} {
		t.Run(action, func(t *testing.T) {
			repo := newFakeRepo()
			store := newFakeStorage()
			queue := &fakeQueue{}
			j := job.Job{ID: uuid.New(), Status: job.StatusRecorded, Rules: rules.Default()}
			repo.jobs[j.ID] = j
			variant := editor.PresetViral60Clean
			loadout, err := renderplan.LoadoutForVariant(variant)
			if err != nil {
				t.Fatal(err)
			}
			state, err := renderplan.NewRenderVariantStateForLoadout(renderplan.NewRenderVariantStateForLoadoutOptions{
				JobID:      j.ID,
				Loadout:    loadout,
				Status:     renderplan.RenderVariantStatusReady,
				Warnings:   []string{rendererWarning},
				RevisionID: uuid.New(),
			})
			if err != nil {
				t.Fatal(err)
			}
			h := NewHandlers(repo, store, queue)
			if err := h.writeRenderVariantState(state); err != nil {
				t.Fatal(err)
			}
			putAssistantJSON(t, store, state.RenderResultKey, editor.Result{
				Preset:   variant,
				Warnings: []string{rendererWarning},
				Shorts: []editor.ShortResult{{
					SegmentID:    "seg-001",
					OutputFormat: editor.OutputFormatShort9x16,
					PublishArtifact: recording.RecordingArtifact{
						Path:      "seg-001.mp4",
						SizeBytes: 10,
						Width:     720,
						Height:    1280,
					},
				}},
			})

			r := chi.NewRouter()
			r.Get("/api/jobs/{id}/renders/{variant}", h.GetRenderVariant)
			r.Post("/api/jobs/{id}/renders/{variant}", h.StartRenderVariant)
			r.Post("/api/jobs/{id}/renders/{variant}/review", h.ResolveRenderReview)
			renderPath := "/api/jobs/" + j.ID.String() + "/renders/" + variant
			req := httptest.NewRequest(http.MethodGet, renderPath, nil)
			rw := httptest.NewRecorder()
			r.ServeHTTP(rw, req)
			if rw.Code != http.StatusOK {
				t.Fatalf("ready GET status = %d, want 200; body=%s", rw.Code, rw.Body.String())
			}
			stateAfterGet, exists, err := h.readRenderVariantState(j.ID, variant)
			if err != nil || !exists {
				t.Fatalf("migrated state = (%#v, %v, %v)", stateAfterGet, exists, err)
			}
			if stateAfterGet.Status != renderplan.RenderVariantStatusReview ||
				!slices.Equal(stateAfterGet.Warnings, expectedWarnings) ||
				stateAfterGet.ReviewResolution != nil {
				t.Fatalf("migrated state = %#v, want exact unresolved warning set", stateAfterGet)
			}

			requestBody := map[string]any{
				"expected_artifact_prefix": stateAfterGet.ArtifactPrefix,
				"expected_warnings":        expectedWarnings,
			}
			actionPath := renderPath
			wantStatus := http.StatusAccepted
			wantState := renderplan.RenderVariantStatusQueued
			if action == "resolve" {
				requestBody["note"] = "intentional hold"
				actionPath += "/review"
				wantStatus = http.StatusOK
				wantState = renderplan.RenderVariantStatusReady
			} else {
				requestBody["music"] = nil
				requestBody["edit"] = map[string]any{
					"format":            renderplan.FormatShort9x16,
					"killEffect":        renderplan.KillEffectPunchIn,
					"transition":        renderplan.TransitionWhip,
					"intro":             false,
					"outro":             false,
					"hook_text":         false,
					"kill_counter":      false,
					"cover_strategy":    renderplan.CoverStrategyGenerated,
					"cover_first_frame": false,
					"intro_text":        "",
					"outro_text":        "",
				}
			}
			body, err := json.Marshal(requestBody)
			if err != nil {
				t.Fatal(err)
			}
			req = httptest.NewRequest(http.MethodPost, actionPath, bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			rw = httptest.NewRecorder()
			r.ServeHTTP(rw, req)
			if rw.Code != wantStatus {
				t.Fatalf("%s status = %d, want %d; body=%s", action, rw.Code, wantStatus, rw.Body.String())
			}
			finalState, exists, err := h.readRenderVariantState(j.ID, variant)
			if err != nil || !exists || finalState.Status != wantState {
				t.Fatalf("%s state = (%#v, %v, %v), want %s", action, finalState, exists, err, wantState)
			}
			if action == "rerender" && len(queue.enqueued) != 1 {
				t.Fatalf("rerender enqueued = %d, want 1", len(queue.enqueued))
			}
		})
	}
}

func TestReadyRenderWithoutStoredWarningsMigratesNestedArtifactWarningAndResolves(t *testing.T) {
	const warning = "quality seg-001: unexpected_output_resolution"
	repo := newFakeRepo()
	store := newFakeStorage()
	j := job.Job{ID: uuid.New(), Status: job.StatusRecorded, Rules: rules.Default()}
	repo.jobs[j.ID] = j
	variant := editor.PresetViral60Clean
	loadout, err := renderplan.LoadoutForVariant(variant)
	if err != nil {
		t.Fatal(err)
	}
	state, err := renderplan.NewRenderVariantStateForLoadout(renderplan.NewRenderVariantStateForLoadoutOptions{
		JobID:      j.ID,
		Loadout:    loadout,
		Status:     renderplan.RenderVariantStatusReady,
		RevisionID: uuid.New(),
	})
	if err != nil {
		t.Fatal(err)
	}
	h := NewHandlers(repo, store, &fakeQueue{})
	if err := h.writeRenderVariantState(state); err != nil {
		t.Fatal(err)
	}
	putAssistantJSON(t, store, state.RenderResultKey, editor.Result{
		Preset: variant,
		Shorts: []editor.ShortResult{{
			SegmentID:    "seg-001",
			OutputFormat: editor.OutputFormatShort9x16,
			PublishArtifact: recording.RecordingArtifact{
				Path:      "seg-001.mp4",
				SizeBytes: 10,
				Width:     720,
				Height:    1280,
			},
		}},
	})

	r := chi.NewRouter()
	r.Get("/api/jobs/{id}/renders/{variant}", h.GetRenderVariant)
	r.Post("/api/jobs/{id}/renders/{variant}/review", h.ResolveRenderReview)
	renderPath := "/api/jobs/" + j.ID.String() + "/renders/" + variant
	req := httptest.NewRequest(http.MethodGet, renderPath, nil)
	rw := httptest.NewRecorder()
	r.ServeHTTP(rw, req)
	if rw.Code != http.StatusOK {
		t.Fatalf("ready GET status = %d, want 200; body=%s", rw.Code, rw.Body.String())
	}
	migrated, exists, err := h.readRenderVariantState(j.ID, variant)
	if err != nil || !exists {
		t.Fatalf("migrated state = (%#v, %v, %v)", migrated, exists, err)
	}
	if migrated.Status != renderplan.RenderVariantStatusReview ||
		!slices.Equal(migrated.Warnings, []string{warning}) ||
		migrated.ReviewResolution != nil {
		t.Fatalf("migrated state = %#v, want unresolved nested artifact warning", migrated)
	}

	body, err := json.Marshal(map[string]any{
		"note":                     "reviewed artifact dimensions",
		"expected_artifact_prefix": migrated.ArtifactPrefix,
		"expected_warnings":        []string{warning},
	})
	if err != nil {
		t.Fatal(err)
	}
	req = httptest.NewRequest(http.MethodPost, renderPath+"/review", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rw = httptest.NewRecorder()
	r.ServeHTTP(rw, req)
	if rw.Code != http.StatusOK {
		t.Fatalf("review status = %d, want 200; body=%s", rw.Code, rw.Body.String())
	}
	resolved, exists, err := h.readRenderVariantState(j.ID, variant)
	if err != nil || !exists || resolved.Status != renderplan.RenderVariantStatusReady ||
		!resolved.ReviewResolvedFor([]string{warning}) {
		t.Fatalf("resolved state = (%#v, %v, %v), want ready with exact resolution", resolved, exists, err)
	}
}

func TestReadyRenderWithResolvedWarningsStaysReady(t *testing.T) {
	const warning = "freeze at 00:12.400"
	repo := newFakeRepo()
	store := newFakeStorage()
	j := job.Job{ID: uuid.New(), Status: job.StatusRecorded, Rules: rules.Default()}
	repo.jobs[j.ID] = j
	variant := editor.PresetViral60Clean
	loadout, err := renderplan.LoadoutForVariant(variant)
	if err != nil {
		t.Fatal(err)
	}
	updatedAt := time.Date(2026, time.July, 29, 10, 0, 0, 0, time.UTC)
	state, err := renderplan.NewRenderVariantStateForLoadout(renderplan.NewRenderVariantStateForLoadoutOptions{
		JobID:      j.ID,
		Loadout:    loadout,
		Status:     renderplan.RenderVariantStatusReady,
		Warnings:   []string{warning},
		Now:        updatedAt,
		RevisionID: uuid.New(),
	})
	if err != nil {
		t.Fatal(err)
	}
	state.ReviewResolution = &renderplan.RenderReviewResolution{
		ArtifactPrefix: state.ArtifactPrefix,
		Warnings:       []string{warning},
		Note:           "intentional hold",
		ReviewedAt:     updatedAt,
	}
	h := NewHandlers(repo, store, &fakeQueue{})
	if err := h.writeRenderVariantState(state); err != nil {
		t.Fatal(err)
	}
	putAssistantJSON(t, store, state.RenderResultKey, editor.Result{
		Preset:   variant,
		Warnings: []string{warning},
	})

	r := chi.NewRouter()
	r.Get("/api/jobs/{id}/renders/{variant}", h.GetRenderVariant)
	req := httptest.NewRequest(http.MethodGet, "/api/jobs/"+j.ID.String()+"/renders/"+variant, nil)
	rw := httptest.NewRecorder()
	r.ServeHTTP(rw, req)
	if rw.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rw.Code, rw.Body.String())
	}
	stored, exists, err := h.readRenderVariantState(j.ID, variant)
	if err != nil || !exists {
		t.Fatalf("stored state = (%#v, %v, %v)", stored, exists, err)
	}
	if stored.Status != renderplan.RenderVariantStatusReady ||
		!stored.ReviewResolvedFor([]string{warning}) ||
		stored.ReviewResolution.Note != "intentional hold" ||
		!stored.UpdatedAt.Equal(updatedAt) {
		t.Fatalf("resolved ready state was degraded: %#v", stored)
	}
}

func TestResolveRenderReviewPersistsExactDecisionAndUnblocksPublishBoard(t *testing.T) {
	repo := newFakeRepo()
	store := newFakeStorage()
	// The parent review state belongs to the composition worker. Resolving a
	// render-variant warning must not clear that independent review gate.
	j := job.Job{ID: uuid.New(), Status: job.StatusReviewRequired, Rules: rules.Default()}
	repo.jobs[j.ID] = j
	variant := editor.PresetViral60Clean
	loadout, err := renderplan.LoadoutForVariant(variant)
	if err != nil {
		t.Fatal(err)
	}
	warnings := []string{"freeze at 00:12.400"}
	state, err := renderplan.NewRenderVariantStateForLoadout(renderplan.NewRenderVariantStateForLoadoutOptions{
		JobID:    j.ID,
		Loadout:  loadout,
		Status:   renderplan.RenderVariantStatusReview,
		Warnings: warnings,
	})
	if err != nil {
		t.Fatal(err)
	}
	h := NewHandlers(repo, store, &fakeQueue{})
	if err := h.writeRenderVariantState(state); err != nil {
		t.Fatal(err)
	}
	result := editor.Result{
		Preset:        variant,
		Warnings:      warnings,
		CoversEnabled: true,
		Shorts:        []editor.ShortResult{{SegmentID: "seg-001"}},
	}
	resultBody, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	_ = store.Put(state.RenderResultKey, bytes.NewReader(resultBody))
	for _, key := range []string{
		state.PackManifestKey,
		state.GalleryKey,
		state.PublishSummaryKey,
	} {
		_ = store.Put(key, bytes.NewReader([]byte("artifact")))
	}
	for _, kind := range []renderplan.RenderVariantArtifactKind{
		renderplan.RenderVariantArtifactVideo,
		renderplan.RenderVariantArtifactCover,
		renderplan.RenderVariantArtifactCaption,
	} {
		ref, err := renderplan.NewRenderVariantArtifactRefForState(state, kind, "seg-001")
		if err != nil {
			t.Fatal(err)
		}
		_ = store.Put(ref.Key, bytes.NewReader([]byte("artifact")))
	}

	r := chi.NewRouter()
	r.Post("/api/jobs/{id}/renders/{variant}/review", h.ResolveRenderReview)
	r.Get("/api/jobs/{id}/renders/{variant}/publish", h.GetRenderPublishBoard)
	body := fmt.Sprintf(
		`{"note":"Freeze intentional para cerrar en el beat.","expected_artifact_prefix":%q,"expected_warnings":["freeze at 00:12.400"]}`,
		state.ArtifactPrefix,
	)
	req := httptest.NewRequest(http.MethodPost, "/api/jobs/"+j.ID.String()+"/renders/"+variant+"/review", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rw := httptest.NewRecorder()
	r.ServeHTTP(rw, req)
	if rw.Code != http.StatusOK {
		t.Fatalf("review status = %d, want 200; body=%s", rw.Code, rw.Body.String())
	}
	if !strings.Contains(rw.Body.String(), `"status":"ready"`) ||
		!strings.Contains(rw.Body.String(), `"note":"Freeze intentional para cerrar en el beat."`) {
		t.Fatalf("review response missing durable resolution: %s", rw.Body.String())
	}
	if got := repo.jobs[j.ID].Status; got != job.StatusReviewRequired {
		t.Fatalf("parent job status = %s, want independent composition review to remain required", got)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/jobs/"+j.ID.String()+"/renders/"+variant+"/publish", nil)
	rw = httptest.NewRecorder()
	r.ServeHTTP(rw, req)
	if rw.Code != http.StatusOK {
		t.Fatalf("publish status = %d, want 200; body=%s", rw.Code, rw.Body.String())
	}
	if !strings.Contains(rw.Body.String(), `"status":"ready"`) ||
		!strings.Contains(rw.Body.String(), `"render_ready":true`) {
		t.Fatalf("resolved publish board is not ready: %s", rw.Body.String())
	}
}

func TestResolveRenderReviewRejectsStaleRevision(t *testing.T) {
	repo := newFakeRepo()
	store := newFakeStorage()
	j := job.Job{ID: uuid.New(), Status: job.StatusRecorded, Rules: rules.Default()}
	repo.jobs[j.ID] = j
	loadout, err := renderplan.LoadoutForVariant(editor.PresetViral60Clean)
	if err != nil {
		t.Fatal(err)
	}
	state, err := renderplan.NewRenderVariantStateForLoadout(renderplan.NewRenderVariantStateForLoadoutOptions{
		JobID:    j.ID,
		Loadout:  loadout,
		Status:   renderplan.RenderVariantStatusReview,
		Warnings: []string{"current warning"},
	})
	if err != nil {
		t.Fatal(err)
	}
	h := NewHandlers(repo, store, &fakeQueue{})
	if err := h.writeRenderVariantState(state); err != nil {
		t.Fatal(err)
	}
	r := chi.NewRouter()
	r.Post("/api/jobs/{id}/renders/{variant}/review", h.ResolveRenderReview)
	req := httptest.NewRequest(
		http.MethodPost,
		"/api/jobs/"+j.ID.String()+"/renders/viral-60-clean/review",
		strings.NewReader(`{"note":"reviewed","expected_artifact_prefix":"stale","expected_warnings":["current warning"]}`),
	)
	req.Header.Set("Content-Type", "application/json")
	rw := httptest.NewRecorder()
	r.ServeHTTP(rw, req)
	if rw.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409; body=%s", rw.Code, rw.Body.String())
	}
	stored, _, err := h.readRenderVariantState(j.ID, editor.PresetViral60Clean)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Status != renderplan.RenderVariantStatusReview || stored.ReviewResolution != nil {
		t.Fatalf("stale review mutated state: %#v", stored)
	}
}

func TestStartRenderVariantRejectsUnsafeVariant(t *testing.T) {
	repo := newFakeRepo()
	queue := &fakeQueue{}
	j := job.Job{ID: uuid.New(), Status: job.StatusRecorded, Rules: rules.Default()}
	repo.jobs[j.ID] = j
	h := NewHandlers(repo, newFakeStorage(), queue)

	r := chi.NewRouter()
	r.Post("/api/jobs/{id}/renders/{variant}", h.StartRenderVariant)
	req := httptest.NewRequest(http.MethodPost, "/api/jobs/"+j.ID.String()+"/renders/bad.mp4", nil)
	rw := httptest.NewRecorder()
	r.ServeHTTP(rw, req)

	if rw.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body=%s", rw.Code, rw.Body.String())
	}
	if len(queue.enqueued) != 0 {
		t.Fatalf("enqueued = %d, want 0", len(queue.enqueued))
	}
}

func TestStartRenderVariantValidatesAgainstPresetRegistry(t *testing.T) {
	cases := []struct {
		name       string
		variant    string
		wantStatus int
	}{
		{name: "registered preset", variant: editor.PresetViral60Clean, wantStatus: http.StatusAccepted},
		{name: "unknown preset", variant: "made-up-preset", wantStatus: http.StatusBadRequest},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := newFakeRepo()
			queue := &fakeQueue{}
			j := job.Job{ID: uuid.New(), Status: job.StatusRecorded, Rules: rules.Default()}
			repo.jobs[j.ID] = j
			h := NewHandlers(repo, newFakeStorage(), queue)

			r := chi.NewRouter()
			r.Post("/api/jobs/{id}/renders/{variant}", h.StartRenderVariant)
			req := httptest.NewRequest(http.MethodPost, "/api/jobs/"+j.ID.String()+"/renders/"+tc.variant, nil)
			rw := httptest.NewRecorder()
			r.ServeHTTP(rw, req)

			if rw.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d; body=%s", rw.Code, tc.wantStatus, rw.Body.String())
			}
			if tc.wantStatus != http.StatusAccepted {
				if len(queue.enqueued) != 0 {
					t.Fatalf("enqueued = %d, want 0", len(queue.enqueued))
				}
				if !strings.Contains(rw.Body.String(), editor.PresetViral60Clean) {
					t.Fatalf("error body should list valid presets: %s", rw.Body.String())
				}
				return
			}
			if len(queue.enqueued) != 1 {
				t.Fatalf("enqueued = %d, want 1", len(queue.enqueued))
			}
		})
	}
}

func TestStartRenderVariantRejectsWrongStatus(t *testing.T) {
	repo := newFakeRepo()
	queue := &fakeQueue{}
	j := job.Job{ID: uuid.New(), Status: job.StatusParsed, Rules: rules.Default()}
	repo.jobs[j.ID] = j
	h := NewHandlers(repo, newFakeStorage(), queue)

	r := chi.NewRouter()
	r.Post("/api/jobs/{id}/renders/{variant}", h.StartRenderVariant)
	req := httptest.NewRequest(http.MethodPost, "/api/jobs/"+j.ID.String()+"/renders/viral-60-clean", nil)
	rw := httptest.NewRecorder()
	r.ServeHTTP(rw, req)

	if rw.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409; body=%s", rw.Code, rw.Body.String())
	}
	if len(queue.enqueued) != 0 {
		t.Fatalf("enqueued = %d, want 0", len(queue.enqueued))
	}
}

func TestGetRenderVariantReturnsReadyArtifactStatus(t *testing.T) {
	repo := newFakeRepo()
	store := newFakeStorage()
	j := job.Job{ID: uuid.New(), Status: job.StatusRecorded, Rules: rules.Default()}
	repo.jobs[j.ID] = j
	key, err := artifacts.RenderVariantResultKey(j.ID, editor.PresetViral60Clean)
	if err != nil {
		t.Fatal(err)
	}
	result := editor.Result{Preset: editor.PresetViral60Clean}
	b, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	_ = store.Put(key, bytes.NewReader(b))
	h := NewHandlers(repo, store, &fakeQueue{})

	r := chi.NewRouter()
	r.Get("/api/jobs/{id}/renders/{variant}", h.GetRenderVariant)
	req := httptest.NewRequest(http.MethodGet, "/api/jobs/"+j.ID.String()+"/renders/viral-60-clean", nil)
	rw := httptest.NewRecorder()
	r.ServeHTTP(rw, req)

	if rw.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rw.Code, rw.Body.String())
	}
	if !strings.Contains(rw.Body.String(), `"status":"ready"`) {
		t.Fatalf("body missing ready status: %s", rw.Body.String())
	}
	if !strings.Contains(rw.Body.String(), `"job_id"`) {
		t.Fatalf("body missing RenderVariantState job_id: %s", rw.Body.String())
	}
	if !strings.Contains(rw.Body.String(), "edit-document.json") {
		t.Fatalf("body missing state artifact keys: %s", rw.Body.String())
	}
}

func TestGetRenderVariantReportsArtifactNames(t *testing.T) {
	// Regression: the render-variant GET must report the reel's real on-disk
	// artifact names so the client stops guessing them from segment ids (which
	// 404'd because the editor writes a single "demo-compilation" compilation).
	// Uses real Local storage because the names come from listing the variant's
	// videos/ and covers/ dirs.
	cases := []struct {
		name       string
		writeFiles bool
		wantVideos string
		wantCovers string
	}{
		{
			name:       "video and cover present are listed",
			writeFiles: true,
			wantVideos: `"videos":["demo-compilation"]`,
			wantCovers: `"covers":["demo-compilation"]`,
		},
		{
			name:       "missing artifact dirs list as empty arrays",
			writeFiles: false,
			wantVideos: `"videos":[]`,
			wantCovers: `"covers":[]`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := newFakeRepo()
			store, err := storage.NewLocal(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			variant := editor.PresetViral60Clean
			j := job.Job{ID: uuid.New(), Status: job.StatusDone, Rules: rules.Default()}
			repo.jobs[j.ID] = j

			loadout, err := renderplan.LoadoutForVariant(variant)
			if err != nil {
				t.Fatal(err)
			}
			state, err := renderplan.NewRenderVariantStateForLoadout(renderplan.NewRenderVariantStateForLoadoutOptions{
				JobID:   j.ID,
				Loadout: loadout,
				Status:  renderplan.RenderVariantStatusReady,
			})
			if err != nil {
				t.Fatal(err)
			}
			b, err := json.Marshal(state)
			if err != nil {
				t.Fatal(err)
			}
			statusKey, err := artifacts.RenderVariantStatusKey(j.ID, variant)
			if err != nil {
				t.Fatal(err)
			}
			if err := store.Put(statusKey, bytes.NewReader(b)); err != nil {
				t.Fatal(err)
			}
			resultBody, err := json.Marshal(editor.Result{Preset: variant})
			if err != nil {
				t.Fatal(err)
			}
			if err := store.Put(state.RenderResultKey, bytes.NewReader(resultBody)); err != nil {
				t.Fatal(err)
			}
			if tc.writeFiles {
				videoKey, err := artifacts.RenderVariantVideoKey(j.ID, variant, "demo-compilation")
				if err != nil {
					t.Fatal(err)
				}
				coverKey, err := renderVariantTestKey(renderplan.RenderVariantArtifactCover)(j.ID, variant, "demo-compilation")
				if err != nil {
					t.Fatal(err)
				}
				for _, key := range []string{videoKey, coverKey} {
					if err := store.Put(key, bytes.NewReader([]byte("artifact"))); err != nil {
						t.Fatal(err)
					}
				}
			}
			h := NewHandlers(repo, store, &fakeQueue{})

			r := chi.NewRouter()
			r.Get("/api/jobs/{id}/renders/{variant}", h.GetRenderVariant)
			req := httptest.NewRequest(http.MethodGet, "/api/jobs/"+j.ID.String()+"/renders/"+variant, nil)
			rw := httptest.NewRecorder()
			r.ServeHTTP(rw, req)

			if rw.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200; body=%s", rw.Code, rw.Body.String())
			}
			body := rw.Body.String()
			if !strings.Contains(body, tc.wantVideos) {
				t.Errorf("body = %s\nwant videos %s", body, tc.wantVideos)
			}
			if !strings.Contains(body, tc.wantCovers) {
				t.Errorf("body = %s\nwant covers %s", body, tc.wantCovers)
			}
		})
	}
}

func TestGetMomentsPrefersStoredArtifact(t *testing.T) {
	repo := newFakeRepo()
	store := newFakeStorage()
	plan := killplan.NewPlan()
	j := job.Job{ID: uuid.New(), Status: job.StatusParsed, Rules: rules.Default(), KillPlan: &plan}
	repo.jobs[j.ID] = j
	_ = store.Put(moments.ArtifactKey(j.ID), bytes.NewReader([]byte(`{"schema_version":"stored"}`)))
	h := NewHandlers(repo, store, &fakeQueue{})

	r := chi.NewRouter()
	r.Get("/api/jobs/{id}/moments", h.GetMoments)
	req := httptest.NewRequest(http.MethodGet, "/api/jobs/"+j.ID.String()+"/moments", nil)
	rw := httptest.NewRecorder()
	r.ServeHTTP(rw, req)

	if rw.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rw.Code, rw.Body.String())
	}
	if !strings.Contains(rw.Body.String(), `"schema_version":"stored"`) {
		t.Fatalf("body = %s, want stored artifact", rw.Body.String())
	}
}

func TestRoutesRequireMutationTokenForPostsWhenConfigured(t *testing.T) {
	repo := newFakeRepo()
	h := NewHandlers(repo, newFakeStorage(), &fakeQueue{}, WithMutationToken("secret"))
	r := Routes(h)

	req := httptest.NewRequest(http.MethodPost, "/api/jobs", nil)
	rw := httptest.NewRecorder()
	r.ServeHTTP(rw, req)
	if rw.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rw.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/loadouts", nil)
	rw = httptest.NewRecorder()
	r.ServeHTTP(rw, req)
	if rw.Code != http.StatusOK {
		t.Fatalf("GET status = %d, want 200", rw.Code)
	}
}

func TestGetRenderPublishBoardReturnsReadyStatus(t *testing.T) {
	repo := newFakeRepo()
	store := newFakeStorage()
	j := job.Job{ID: uuid.New(), Status: job.StatusRecorded, Rules: rules.Default()}
	repo.jobs[j.ID] = j
	variant := editor.PresetViral60Clean
	resultKey, err := artifacts.RenderVariantResultKey(j.ID, variant)
	if err != nil {
		t.Fatal(err)
	}
	result := editor.Result{
		Preset: variant,
		Shorts: []editor.ShortResult{{
			SegmentID: "seg-001",
		}},
	}
	b, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	_ = store.Put(resultKey, bytes.NewReader(b))
	for _, keyFn := range []func(uuid.UUID, string) (string, error){
		artifacts.RenderVariantPackManifestKey,
		artifacts.RenderVariantGalleryKey,
		artifacts.RenderVariantPublishSummaryKey,
	} {
		key, err := keyFn(j.ID, variant)
		if err != nil {
			t.Fatal(err)
		}
		_ = store.Put(key, bytes.NewReader([]byte("artifact")))
	}
	videoKey, err := artifacts.RenderVariantVideoKey(j.ID, variant, "seg-001")
	if err != nil {
		t.Fatal(err)
	}
	coverKey, err := renderVariantTestKey(renderplan.RenderVariantArtifactCover)(j.ID, variant, "seg-001")
	if err != nil {
		t.Fatal(err)
	}
	captionKey, err := renderVariantTestKey(renderplan.RenderVariantArtifactCaption)(j.ID, variant, "seg-001")
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{videoKey, coverKey, captionKey} {
		_ = store.Put(key, bytes.NewReader([]byte("artifact")))
	}
	h := NewHandlers(repo, store, &fakeQueue{})

	r := chi.NewRouter()
	r.Get("/api/jobs/{id}/renders/{variant}/publish", h.GetRenderPublishBoard)
	req := httptest.NewRequest(http.MethodGet, "/api/jobs/"+j.ID.String()+"/renders/viral-60-clean/publish", nil)
	rw := httptest.NewRecorder()
	r.ServeHTTP(rw, req)

	if rw.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rw.Code, rw.Body.String())
	}
	for _, want := range []string{`"status":"ready"`, `"render_ready":true`, `"video_ready":true`, `"cover_ready":true`, `"caption_ready":true`} {
		if !strings.Contains(rw.Body.String(), want) {
			t.Fatalf("body missing %s: %s", want, rw.Body.String())
		}
	}
}

func TestGetRenderQualityReturnsReadyReport(t *testing.T) {
	repo := newFakeRepo()
	store := newFakeStorage()
	j := job.Job{ID: uuid.New(), Status: job.StatusRecorded, Rules: rules.Default()}
	repo.jobs[j.ID] = j
	variant := editor.PresetViral60Clean
	resultKey, err := artifacts.RenderVariantResultKey(j.ID, variant)
	if err != nil {
		t.Fatal(err)
	}
	result := editor.Result{
		Preset: variant,
		Shorts: []editor.ShortResult{{
			SegmentID: "seg-001",
			PublishArtifact: recording.RecordingArtifact{
				SizeBytes:       10,
				Width:           1080,
				Height:          1920,
				DurationSeconds: 30,
				Codec:           "h264",
			},
		}},
	}
	b, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	_ = store.Put(resultKey, bytes.NewReader(b))
	h := NewHandlers(repo, store, &fakeQueue{})

	r := chi.NewRouter()
	r.Get("/api/jobs/{id}/renders/{variant}/quality", h.GetRenderQuality)
	req := httptest.NewRequest(http.MethodGet, "/api/jobs/"+j.ID.String()+"/renders/viral-60-clean/quality", nil)
	rw := httptest.NewRecorder()
	r.ServeHTTP(rw, req)

	if rw.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rw.Code, rw.Body.String())
	}
	for _, want := range []string{`"status":"ready"`, `"video_width":1080`, `"video_height":1920`, `"video_codec":"h264"`} {
		if !strings.Contains(rw.Body.String(), want) {
			t.Fatalf("body missing %s: %s", want, rw.Body.String())
		}
	}
}

func TestRenderArtifactRoutesStreamKnownArtifacts(t *testing.T) {
	repo := newFakeRepo()
	store := newFakeStorage()
	j := job.Job{ID: uuid.New(), Status: job.StatusRecorded, Rules: rules.Default()}
	repo.jobs[j.ID] = j
	variant := editor.PresetViral60Clean
	videoKey, err := artifacts.RenderVariantVideoKey(j.ID, variant, "seg-001")
	if err != nil {
		t.Fatal(err)
	}
	_ = store.Put(videoKey, bytes.NewReader([]byte("mp4-bytes")))
	h := NewHandlers(repo, store, &fakeQueue{})

	r := chi.NewRouter()
	r.Get("/api/jobs/{id}/renders/{variant}/videos/{name}", h.GetRenderVideo)
	req := httptest.NewRequest(http.MethodGet, "/api/jobs/"+j.ID.String()+"/renders/viral-60-clean/videos/seg-001", nil)
	rw := httptest.NewRecorder()
	r.ServeHTTP(rw, req)

	if rw.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rw.Code, rw.Body.String())
	}
	if got := rw.Header().Get("Content-Type"); got != "video/mp4" {
		t.Fatalf("Content-Type = %q, want video/mp4", got)
	}
	if rw.Body.String() != "mp4-bytes" {
		t.Fatalf("body = %q, want mp4-bytes", rw.Body.String())
	}
}

func TestRenderVideoHonorsRangeRequests(t *testing.T) {
	// Regression: the browser <video> element needs Range support (206 +
	// Content-Range) to start playback; the handler used to always 200 with a
	// plain copy. Uses the real Local storage because ranges apply only to
	// seekable readers (*os.File).
	repo := newFakeRepo()
	store, err := storage.NewLocal(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	j := job.Job{ID: uuid.New(), Status: job.StatusRecorded, Rules: rules.Default()}
	repo.jobs[j.ID] = j
	videoKey, err := artifacts.RenderVariantVideoKey(j.ID, editor.PresetViral60Clean, "seg-001")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Put(videoKey, bytes.NewReader([]byte("mp4-bytes"))); err != nil {
		t.Fatal(err)
	}
	h := NewHandlers(repo, store, &fakeQueue{})

	r := chi.NewRouter()
	r.Get("/api/jobs/{id}/renders/{variant}/videos/{name}", h.GetRenderVideo)
	req := httptest.NewRequest(http.MethodGet, "/api/jobs/"+j.ID.String()+"/renders/viral-60-clean/videos/seg-001", nil)
	req.Header.Set("Range", "bytes=0-3")
	rw := httptest.NewRecorder()
	r.ServeHTTP(rw, req)

	if rw.Code != http.StatusPartialContent {
		t.Fatalf("status = %d, want 206; body=%s", rw.Code, rw.Body.String())
	}
	if got, want := rw.Body.String(), "mp4-"; got != want {
		t.Fatalf("body = %q, want %q", got, want)
	}
	if got, want := rw.Header().Get("Content-Range"), "bytes 0-3/9"; got != want {
		t.Fatalf("Content-Range = %q, want %q", got, want)
	}
	if got := rw.Header().Get("Content-Type"); got != "video/mp4" {
		t.Fatalf("Content-Type = %q, want video/mp4", got)
	}
}

func TestDeleteRenderVideoRemovesVideoCoverAndCaption(t *testing.T) {
	repo := newFakeRepo()
	store := newFakeStorage()
	j := job.Job{ID: uuid.New(), Status: job.StatusDone, Rules: rules.Default()}
	repo.jobs[j.ID] = j
	keys := make([]string, 0, 3)
	for _, derive := range []func(uuid.UUID, string, string) (string, error){
		artifacts.RenderVariantVideoKey,
		renderVariantTestKey(renderplan.RenderVariantArtifactCover),
		renderVariantTestKey(renderplan.RenderVariantArtifactCaption),
	} {
		key, err := derive(j.ID, editor.PresetViral60Clean, "seg-001_seg-002")
		if err != nil {
			t.Fatal(err)
		}
		_ = store.Put(key, bytes.NewReader([]byte("artifact-bytes")))
		keys = append(keys, key)
	}
	h := NewHandlers(repo, store, &fakeQueue{})

	r := chi.NewRouter()
	r.Delete("/api/jobs/{id}/renders/{variant}/videos/{name}", h.DeleteRenderVideo)
	req := httptest.NewRequest(http.MethodDelete, "/api/jobs/"+j.ID.String()+"/renders/viral-60-clean/videos/seg-001_seg-002", nil)
	rw := httptest.NewRecorder()
	r.ServeHTTP(rw, req)

	if rw.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204; body=%s", rw.Code, rw.Body.String())
	}
	for _, key := range keys {
		if _, ok := store.puts[key]; ok {
			t.Errorf("artifact %q still present after delete", key)
		}
	}

	// Deleting again is idempotent: a retry after a lost response must succeed.
	rw = httptest.NewRecorder()
	r.ServeHTTP(rw, httptest.NewRequest(http.MethodDelete, "/api/jobs/"+j.ID.String()+"/renders/viral-60-clean/videos/seg-001_seg-002", nil))
	if rw.Code != http.StatusNoContent {
		t.Fatalf("repeat delete status = %d, want 204; body=%s", rw.Code, rw.Body.String())
	}
}

func TestDeleteRenderVideoUsesOneRenderRevisionSnapshot(t *testing.T) {
	repo := newFakeRepo()
	store := newFakeStorage()
	j := job.Job{ID: uuid.New(), Status: job.StatusDone, Rules: rules.Default()}
	repo.jobs[j.ID] = j
	h := NewHandlers(repo, store, &fakeQueue{})
	loadout, err := renderplan.LoadoutForVariant(editor.PresetViral60Clean)
	if err != nil {
		t.Fatal(err)
	}
	stateA, err := renderplan.NewRenderVariantStateForLoadout(renderplan.NewRenderVariantStateForLoadoutOptions{
		JobID: j.ID, Loadout: loadout, Status: renderplan.RenderVariantStatusReady, RevisionID: uuid.New(),
	})
	if err != nil {
		t.Fatal(err)
	}
	stateB, err := renderplan.NewRenderVariantStateForLoadout(renderplan.NewRenderVariantStateForLoadoutOptions{
		JobID: j.ID, Loadout: loadout, Status: renderplan.RenderVariantStatusReady, RevisionID: uuid.New(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := h.writeRenderVariantState(stateA); err != nil {
		t.Fatal(err)
	}
	name := "seg-001"
	kinds := []renderplan.RenderVariantArtifactKind{
		renderplan.RenderVariantArtifactVideo,
		renderplan.RenderVariantArtifactCover,
		renderplan.RenderVariantArtifactCaption,
	}
	var refsA, refsB []renderplan.RenderVariantArtifactRef
	for _, kind := range kinds {
		refA, err := renderplan.NewRenderVariantArtifactRefForState(stateA, kind, name)
		if err != nil {
			t.Fatal(err)
		}
		refB, err := renderplan.NewRenderVariantArtifactRefForState(stateB, kind, name)
		if err != nil {
			t.Fatal(err)
		}
		refsA = append(refsA, refA)
		refsB = append(refsB, refB)
		_ = store.Put(refA.Key, bytes.NewReader([]byte("old revision")))
		_ = store.Put(refB.Key, bytes.NewReader([]byte("new revision")))
	}
	swapped := false
	store.onDelete = func(string) {
		if swapped {
			return
		}
		swapped = true
		if err := h.writeRenderVariantState(stateB); err != nil {
			t.Errorf("swap render state: %v", err)
		}
	}

	r := chi.NewRouter()
	r.Delete("/api/jobs/{id}/renders/{variant}/videos/{name}", h.DeleteRenderVideo)
	req := httptest.NewRequest(http.MethodDelete, "/api/jobs/"+j.ID.String()+"/renders/viral-60-clean/videos/"+name, nil)
	rw := httptest.NewRecorder()
	r.ServeHTTP(rw, req)

	if rw.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204; body=%s", rw.Code, rw.Body.String())
	}
	for i, ref := range refsA {
		if got := store.deleted[i]; got != ref.Key {
			t.Fatalf("deleted[%d] = %q, want snapshotted %q", i, got, ref.Key)
		}
	}
	for _, ref := range refsB {
		if _, ok := store.puts[ref.Key]; !ok {
			t.Errorf("new revision artifact %q was deleted", ref.Key)
		}
	}
}

func TestDeleteRenderVideoRejectsUnknownVariant(t *testing.T) {
	repo := newFakeRepo()
	j := job.Job{ID: uuid.New(), Status: job.StatusDone, Rules: rules.Default()}
	repo.jobs[j.ID] = j
	h := NewHandlers(repo, newFakeStorage(), &fakeQueue{})

	r := chi.NewRouter()
	r.Delete("/api/jobs/{id}/renders/{variant}/videos/{name}", h.DeleteRenderVideo)
	req := httptest.NewRequest(http.MethodDelete, "/api/jobs/"+j.ID.String()+"/renders/not-a-variant/videos/seg-001", nil)
	rw := httptest.NewRecorder()
	r.ServeHTTP(rw, req)

	if rw.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body=%s", rw.Code, rw.Body.String())
	}
}

func TestDeleteJobRemovesJobArtifactsAndDemo(t *testing.T) {
	repo := newFakeRepo()
	store, err := storage.NewLocal(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	j := job.Job{ID: uuid.New(), Status: job.StatusDone, Rules: rules.Default()}
	repo.jobs[j.ID] = j

	demoKey := "demos/" + j.ID.String() + ".dem"
	artifactKeys := []string{
		"jobs/" + j.ID.String() + "/recording/result.json",
		"jobs/" + j.ID.String() + "/renders/viral-60-clean/video.mp4",
	}
	if err := store.Put(demoKey, bytes.NewReader([]byte("PBDEMS2\x00"))); err != nil {
		t.Fatal(err)
	}
	for _, key := range artifactKeys {
		if err := store.Put(key, bytes.NewReader([]byte("artifact-bytes"))); err != nil {
			t.Fatal(err)
		}
	}
	h := NewHandlers(repo, store, &fakeQueue{})

	r := chi.NewRouter()
	r.Delete("/api/jobs/{id}", h.DeleteJob)
	req := httptest.NewRequest(http.MethodDelete, "/api/jobs/"+j.ID.String(), nil)
	rw := httptest.NewRecorder()
	r.ServeHTTP(rw, req)

	if rw.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204; body=%s", rw.Code, rw.Body.String())
	}
	if _, ok := repo.jobs[j.ID]; ok {
		t.Error("job still present in repo after delete")
	}
	for _, key := range append(artifactKeys, demoKey) {
		exists, err := store.Exists(key)
		if err != nil {
			t.Fatalf("Exists(%q) error = %v", key, err)
		}
		if exists {
			t.Errorf("artifact %q still present after delete", key)
		}
	}
	// The whole jobs/<id> tree must be gone, not just the seeded files.
	treeExists, err := store.Exists("jobs/" + j.ID.String())
	if err != nil {
		t.Fatalf("Exists(tree) error = %v", err)
	}
	if treeExists {
		t.Error("job artifact tree still present after delete")
	}

	// A repeat delete after success is a 404: the job is gone.
	rw = httptest.NewRecorder()
	r.ServeHTTP(rw, httptest.NewRequest(http.MethodDelete, "/api/jobs/"+j.ID.String(), nil))
	if rw.Code != http.StatusNotFound {
		t.Fatalf("repeat delete status = %d, want 404; body=%s", rw.Code, rw.Body.String())
	}
}

func TestDeleteJobRejectsInFlightJob(t *testing.T) {
	repo := newFakeRepo()
	store := newFakeStorage()
	j := job.Job{ID: uuid.New(), Status: job.StatusRecording, Rules: rules.Default()}
	repo.jobs[j.ID] = j
	demoKey := "demos/" + j.ID.String() + ".dem"
	_ = store.Put(demoKey, bytes.NewReader([]byte("PBDEMS2\x00")))
	h := NewHandlers(repo, store, &fakeQueue{})

	r := chi.NewRouter()
	r.Delete("/api/jobs/{id}", h.DeleteJob)
	req := httptest.NewRequest(http.MethodDelete, "/api/jobs/"+j.ID.String(), nil)
	rw := httptest.NewRecorder()
	r.ServeHTTP(rw, req)

	if rw.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409; body=%s", rw.Code, rw.Body.String())
	}
	if _, ok := repo.jobs[j.ID]; !ok {
		t.Error("job removed from repo despite 409")
	}
	if _, ok := store.puts[demoKey]; !ok {
		t.Error("demo removed from storage despite 409")
	}
}

func TestDeleteJobUnknownIDReturns404(t *testing.T) {
	h := NewHandlers(newFakeRepo(), newFakeStorage(), &fakeQueue{})

	r := chi.NewRouter()
	r.Delete("/api/jobs/{id}", h.DeleteJob)
	req := httptest.NewRequest(http.MethodDelete, "/api/jobs/"+uuid.New().String(), nil)
	rw := httptest.NewRecorder()
	r.ServeHTTP(rw, req)

	if rw.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404; body=%s", rw.Code, rw.Body.String())
	}
}

func TestRenderArtifactRoutesRejectUnsafeArtifactName(t *testing.T) {
	repo := newFakeRepo()
	j := job.Job{ID: uuid.New(), Status: job.StatusRecorded, Rules: rules.Default()}
	repo.jobs[j.ID] = j
	h := NewHandlers(repo, newFakeStorage(), &fakeQueue{})

	r := chi.NewRouter()
	r.Get("/api/jobs/{id}/renders/{variant}/videos/{name}", h.GetRenderVideo)
	req := httptest.NewRequest(http.MethodGet, "/api/jobs/"+j.ID.String()+"/renders/viral-60-clean/videos/seg-001.mp4", nil)
	rw := httptest.NewRecorder()
	r.ServeHTTP(rw, req)

	if rw.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body=%s", rw.Code, rw.Body.String())
	}
}

func TestRenderPackAndEditDocumentRoutesStreamJSON(t *testing.T) {
	repo := newFakeRepo()
	store := newFakeStorage()
	j := job.Job{ID: uuid.New(), Status: job.StatusRecorded, Rules: rules.Default()}
	repo.jobs[j.ID] = j
	variant := editor.PresetViral60Clean
	packKey, err := artifacts.RenderVariantPackManifestKey(j.ID, variant)
	if err != nil {
		t.Fatal(err)
	}
	editKey, err := artifacts.RenderVariantEditDocumentKey(j.ID, variant)
	if err != nil {
		t.Fatal(err)
	}
	_ = store.Put(packKey, bytes.NewReader([]byte(`{"items":[]}`)))
	_ = store.Put(editKey, bytes.NewReader([]byte(`{"schema_version":"1.0"}`)))
	h := NewHandlers(repo, store, &fakeQueue{})

	r := chi.NewRouter()
	r.Get("/api/jobs/{id}/renders/{variant}/pack", h.GetRenderPack)
	r.Get("/api/jobs/{id}/renders/{variant}/edit-document", h.GetRenderEditDocument)
	for _, path := range []string{
		"/api/jobs/" + j.ID.String() + "/renders/viral-60-clean/pack",
		"/api/jobs/" + j.ID.String() + "/renders/viral-60-clean/edit-document",
	} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rw := httptest.NewRecorder()
		r.ServeHTTP(rw, req)
		if rw.Code != http.StatusOK {
			t.Fatalf("%s status = %d, want 200; body=%s", path, rw.Code, rw.Body.String())
		}
		if got := rw.Header().Get("Content-Type"); got != "application/json" {
			t.Fatalf("%s Content-Type = %q, want application/json", path, got)
		}
	}
}

func TestGetFinalStreamsFinalArtifactWhenComposed(t *testing.T) {
	repo := newFakeRepo()
	store := newFakeStorage()
	j := job.Job{ID: uuid.New(), Status: job.StatusComposed, Rules: rules.Default()}
	repo.jobs[j.ID] = j
	_ = store.Put(composition.FinalArtifactKey(j.ID), bytes.NewReader([]byte("mp4-bytes")))
	h := NewHandlers(repo, store, &fakeQueue{})

	r := chi.NewRouter()
	r.Get("/api/jobs/{id}/final", h.GetFinal)
	req := httptest.NewRequest(http.MethodGet, "/api/jobs/"+j.ID.String()+"/final", nil)
	rw := httptest.NewRecorder()
	r.ServeHTTP(rw, req)

	if rw.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rw.Code, rw.Body.String())
	}
	if got := rw.Header().Get("Content-Type"); got != "video/mp4" {
		t.Fatalf("Content-Type = %q, want video/mp4", got)
	}
	if rw.Body.String() != "mp4-bytes" {
		t.Fatalf("body = %q, want mp4-bytes", rw.Body.String())
	}
}

func TestGetFinalReturns409BeforeComposed(t *testing.T) {
	repo := newFakeRepo()
	j := job.Job{ID: uuid.New(), Status: job.StatusRecorded, Rules: rules.Default()}
	repo.jobs[j.ID] = j
	h := NewHandlers(repo, newFakeStorage(), &fakeQueue{})

	r := chi.NewRouter()
	r.Get("/api/jobs/{id}/final", h.GetFinal)
	req := httptest.NewRequest(http.MethodGet, "/api/jobs/"+j.ID.String()+"/final", nil)
	rw := httptest.NewRecorder()
	r.ServeHTTP(rw, req)

	if rw.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409", rw.Code)
	}
}

func TestGetFinalReturns404WhenArtifactMissing(t *testing.T) {
	repo := newFakeRepo()
	j := job.Job{ID: uuid.New(), Status: job.StatusComposed, Rules: rules.Default()}
	repo.jobs[j.ID] = j
	h := NewHandlers(repo, newFakeStorage(), &fakeQueue{})

	r := chi.NewRouter()
	r.Get("/api/jobs/{id}/final", h.GetFinal)
	req := httptest.NewRequest(http.MethodGet, "/api/jobs/"+j.ID.String()+"/final", nil)
	rw := httptest.NewRecorder()
	r.ServeHTTP(rw, req)

	if rw.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rw.Code)
	}
}

func TestWorkbenchLocalProductFlowEndToEnd(t *testing.T) {
	repo := newFakeRepo()
	store := newFakeStorage()
	queue := &fakeQueue{}
	plan := killplan.NewPlan()
	plan.Demo.Map = "de_ancient"
	plan.Demo.Tickrate = 64
	plan.Target.NameInDemo = "MartinezSa"
	plan.Segments = []killplan.Segment{{
		ID:        "seg-001",
		Round:     2,
		TickStart: 640,
		TickEnd:   1280,
		Kills: []killplan.Kill{{
			Weapon:   "ak47",
			Headshot: true,
			Victim:   killplan.Player{NameInDemo: "alex"},
		}},
	}}
	j := job.Job{ID: uuid.New(), Status: job.StatusRecorded, DemoPath: "demos/demo.dem", TargetSteamID: "76561198000000000", Rules: rules.Default(), KillPlan: &plan}
	repo.jobs[j.ID] = j
	_ = store.Put(moments.ArtifactKey(j.ID), bytes.NewReader([]byte(`{"schema_version":"1.0","moments":[{"id":"mom-001","player":"MartinezSa"}]}`)))
	h := NewHandlers(repo, store, queue, WithMutationToken("secret"))
	r := Routes(h)

	get := func(path string) string {
		t.Helper()
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rw := httptest.NewRecorder()
		r.ServeHTTP(rw, req)
		if rw.Code != http.StatusOK {
			t.Fatalf("GET %s status = %d, want 200; body=%s", path, rw.Code, rw.Body.String())
		}
		return rw.Body.String()
	}
	post := func(path, body string, token bool, want int) string {
		t.Helper()
		req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
		if body != "" {
			req.Header.Set("Content-Type", "application/json")
		}
		if token {
			req.Header.Set("X-ClipHub-Token", "secret")
		}
		rw := httptest.NewRecorder()
		r.ServeHTTP(rw, req)
		if rw.Code != want {
			t.Fatalf("POST %s status = %d, want %d; body=%s", path, rw.Code, want, rw.Body.String())
		}
		return rw.Body.String()
	}

	for _, check := range []struct {
		path string
		want string
	}{
		{"/", "ClipHub Workbench"},
		{"/api/jobs", j.ID.String()},
		{"/api/loadouts", editor.PresetViral60Clean},
		{"/api/jobs/" + j.ID.String() + "/moments", "MartinezSa"},
	} {
		if body := get(check.path); !strings.Contains(body, check.want) {
			t.Fatalf("GET %s body missing %q: %s", check.path, check.want, body)
		}
	}

	renderPath := "/api/jobs/" + j.ID.String() + "/renders/viral-60-clean"
	post(renderPath, "", false, http.StatusUnauthorized)
	post(renderPath, "", true, http.StatusAccepted)
	if len(queue.enqueued) != 1 || queue.enqueued[0].Type() != tasks.TypeRenderVariant {
		t.Fatalf("queue after render = %#v", queue.enqueued)
	}
	if body := get(renderPath); !strings.Contains(body, `"status":"queued"`) {
		t.Fatalf("render state missing queued: %s", body)
	}

	resultKey, err := artifacts.RenderVariantResultKey(j.ID, editor.PresetViral60Clean)
	if err != nil {
		t.Fatal(err)
	}
	result := editor.Result{
		Preset: editor.PresetViral60Clean,
		Shorts: []editor.ShortResult{{
			SegmentID: "seg-001",
			PublishArtifact: recording.RecordingArtifact{
				SizeBytes:       123,
				Width:           1080,
				Height:          1920,
				DurationSeconds: 15,
				Codec:           "h264",
			},
		}},
	}
	b, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	_ = store.Put(resultKey, bytes.NewReader(b))
	for _, keyFn := range []func(uuid.UUID, string) (string, error){
		artifacts.RenderVariantPackManifestKey,
		artifacts.RenderVariantGalleryKey,
		artifacts.RenderVariantPublishSummaryKey,
	} {
		key, err := keyFn(j.ID, editor.PresetViral60Clean)
		if err != nil {
			t.Fatal(err)
		}
		_ = store.Put(key, bytes.NewReader([]byte("artifact")))
	}
	for _, keyFn := range []func(uuid.UUID, string, string) (string, error){
		artifacts.RenderVariantVideoKey,
		renderVariantTestKey(renderplan.RenderVariantArtifactCover),
		renderVariantTestKey(renderplan.RenderVariantArtifactCaption),
	} {
		key, err := keyFn(j.ID, editor.PresetViral60Clean, "seg-001")
		if err != nil {
			t.Fatal(err)
		}
		_ = store.Put(key, bytes.NewReader([]byte("artifact")))
	}
	publishPath := renderPath + "/publish"
	if body := get(publishPath); !strings.Contains(body, `"status":"ready"`) {
		t.Fatalf("publish board missing ready: %s", body)
	}
	if body := get(renderPath + "/quality"); !strings.Contains(body, `"status":"ready"`) || !strings.Contains(body, `"video_codec":"h264"`) {
		t.Fatalf("quality body missing ready codec: %s", body)
	}
}

// TestStreamJobEndpointsReturn501WhenRepositoryNotConfigured guards against a
// nil h.streamRepo (e.g. a deployment mode that never calls
// WithStreamRepository) crashing the handler instead of returning a clear
// error. See streamReady in stream_handlers.go.
func TestStreamJobEndpointsReturn501WhenRepositoryNotConfigured(t *testing.T) {
	h := NewHandlers(newFakeRepo(), newFakeStorage(), &fakeQueue{})
	r := Routes(h)

	req := httptest.NewRequest(http.MethodGet, "/api/stream-jobs", nil)
	rw := httptest.NewRecorder()
	r.ServeHTTP(rw, req)
	if rw.Code != http.StatusNotImplemented {
		t.Fatalf("list status = %d, want 501; body=%s", rw.Code, rw.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/api/stream-jobs/"+uuid.New().String(), nil)
	rw = httptest.NewRecorder()
	r.ServeHTTP(rw, req)
	if rw.Code != http.StatusNotImplemented {
		t.Fatalf("get status = %d, want 501; body=%s", rw.Code, rw.Body.String())
	}
}

func TestStreamJobFlowSavesPlanAndEnqueuesRender(t *testing.T) {
	streamRepo := newFakeStreamRepo()
	store := newFakeStorage()
	queue := &fakeQueue{}
	h := NewHandlers(newFakeRepo(), store, queue, WithStreamRepository(streamRepo))
	r := Routes(h)

	body := &bytes.Buffer{}
	mw := multipart.NewWriter(body)
	videoPart, _ := mw.CreateFormFile("video", "stream.mp4")
	_, _ = videoPart.Write([]byte("mp4-bytes"))
	_ = mw.WriteField("config", `{"title":"match stream"}`)
	_ = mw.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/stream-jobs", body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	rw := httptest.NewRecorder()
	r.ServeHTTP(rw, req)
	if rw.Code != http.StatusCreated {
		t.Fatalf("create status = %d, want 201; body=%s", rw.Code, rw.Body.String())
	}
	var created struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(rw.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	id, err := uuid.Parse(created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := store.puts[streamclips.SourceKey(id)]; !ok {
		t.Fatalf("storage missing stream source")
	}

	plan := streamclips.DefaultEditPlan()
	plan.FaceCropReviewed = true
	plan.Clips = []streamclips.ClipRange{{ID: "clip-001", StartSeconds: 1, EndSeconds: 3, Title: "one"}}
	planBody, err := json.Marshal(plan)
	if err != nil {
		t.Fatal(err)
	}
	req = httptest.NewRequest(http.MethodPut, "/api/stream-jobs/"+created.ID+"/edit-plan", bytes.NewReader(planBody))
	req.Header.Set("Content-Type", "application/json")
	rw = httptest.NewRecorder()
	r.ServeHTTP(rw, req)
	if rw.Code != http.StatusOK {
		t.Fatalf("plan status = %d, want 200; body=%s", rw.Code, rw.Body.String())
	}
	if streamRepo.jobs[id].Status != streamclips.StatusReady {
		t.Fatalf("stream status = %s, want ready", streamRepo.jobs[id].Status)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/stream-jobs/"+created.ID+"/renders/"+plan.Variant, nil)
	rw = httptest.NewRecorder()
	r.ServeHTTP(rw, req)
	if rw.Code != http.StatusAccepted {
		t.Fatalf("render status = %d, want 202; body=%s", rw.Code, rw.Body.String())
	}
	if len(queue.enqueued) != 1 || queue.enqueued[0].Type() != tasks.TypeRenderStreamClip {
		t.Fatalf("queue = %#v", queue.enqueued)
	}
}

func TestStreamJobRemovesMultipartTempFiles(t *testing.T) {
	withIsolatedTempDir(t)
	streamRepo := newFakeStreamRepo()
	store := newFakeStorage()
	queue := &fakeQueue{}
	h := NewHandlers(newFakeRepo(), store, queue, WithStreamRepository(streamRepo))

	body := &bytes.Buffer{}
	mw := multipart.NewWriter(body)
	videoPart, _ := mw.CreateFormFile("video", "stream.mp4")
	_, _ = videoPart.Write(bytes.Repeat([]byte("m"), multipartMemBudget+1))
	_ = mw.WriteField("config", `{"title":"match stream"}`)
	_ = mw.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/stream-jobs", body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	rw := httptest.NewRecorder()

	h.CreateStreamJob(rw, req)

	if rw.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body=%s", rw.Code, rw.Body.String())
	}
	assertMultipartTempDirEmpty(t)
}

func TestPutStreamEditPlanCompletesBeforeWorkerCanClaimSameJob(t *testing.T) {
	id := uuid.New()
	baseRepo := newFakeStreamRepo()
	planA := streamclips.DefaultEditPlan()
	planA.Clips = []streamclips.ClipRange{{ID: "clip-1", StartSeconds: 0, EndSeconds: 2, Title: "plan-a"}}
	planJSON, err := json.Marshal(planA)
	if err != nil {
		t.Fatal(err)
	}
	baseRepo.jobs[id] = streamclips.Job{
		ID: id, Status: streamclips.StatusReady, Probe: streamclips.SourceProbe{DurationSeconds: 10}, EditPlan: planJSON,
	}
	repo := &blockingSetStreamRepo{
		fakeStreamRepo: baseRepo,
		entered:        make(chan struct{}),
		release:        make(chan struct{}),
	}
	locks := streamclips.NewJobLocks()
	h := NewHandlers(newFakeRepo(), newFakeStorage(), &fakeQueue{},
		WithStreamRepository(repo), WithStreamJobLocks(locks),
	)
	r := Routes(h)
	planB := planA
	planB.Clips = append([]streamclips.ClipRange(nil), planA.Clips...)
	planB.Clips[0].Title = "plan-b"
	body, err := json.Marshal(planB)
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		req := httptest.NewRequest(http.MethodPut, "/api/stream-jobs/"+id.String()+"/edit-plan", bytes.NewReader(body))
		rw := httptest.NewRecorder()
		r.ServeHTTP(rw, req)
		done <- rw
	}()
	<-repo.entered

	claimed := make(chan streamclips.EditPlan, 1)
	go func() {
		release := locks.Lock(id)
		defer release()
		job, _ := repo.Get(context.Background(), id)
		var plan streamclips.EditPlan
		_ = json.Unmarshal(job.EditPlan, &plan)
		claimed <- plan
	}()
	select {
	case <-claimed:
		t.Fatal("worker claim passed an HTTP edit-plan persistence in progress")
	case <-time.After(20 * time.Millisecond):
	}
	close(repo.release)
	rw := <-done
	if rw.Code != http.StatusOK {
		t.Fatalf("PUT status = %d, want 200; body=%s", rw.Code, rw.Body.String())
	}
	select {
	case got := <-claimed:
		if got.Clips[0].Title != "plan-b" {
			t.Fatalf("claimed plan title = %q, want plan-b", got.Clips[0].Title)
		}
	case <-time.After(time.Second):
		t.Fatal("worker did not claim job after HTTP mutation committed")
	}
}

func TestPutStreamEditPlanRejectsLargeJSONBody(t *testing.T) {
	streamRepo := newFakeStreamRepo()
	id := uuid.New()
	streamRepo.jobs[id] = streamclips.Job{ID: id, Status: streamclips.StatusUploaded, SourcePath: streamclips.SourceKey(id)}
	h := NewHandlers(newFakeRepo(), newFakeStorage(), &fakeQueue{}, WithStreamRepository(streamRepo))

	r := chi.NewRouter()
	r.Put("/api/stream-jobs/{id}/edit-plan", h.PutStreamEditPlan)
	req := httptest.NewRequest(http.MethodPut, "/api/stream-jobs/"+id.String()+"/edit-plan", strings.NewReader(`{`+strings.Repeat(" ", maxJSONBodyBytes+1)))
	rw := httptest.NewRecorder()
	r.ServeHTTP(rw, req)

	if rw.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want 413; body=%s", rw.Code, rw.Body.String())
	}
}

func TestPutStreamEditPlanRejectsClipPastProbedSourceDuration(t *testing.T) {
	streamRepo := newFakeStreamRepo()
	id := uuid.New()
	streamRepo.jobs[id] = streamclips.Job{
		ID:         id,
		Status:     streamclips.StatusUploaded,
		SourcePath: streamclips.SourceKey(id),
		Probe:      streamclips.SourceProbe{DurationSeconds: 15.15},
	}
	h := NewHandlers(newFakeRepo(), newFakeStorage(), &fakeQueue{}, WithStreamRepository(streamRepo))
	plan := streamclips.DefaultEditPlan()
	plan.Clips = []streamclips.ClipRange{{ID: "clip-001", StartSeconds: 0, EndSeconds: 20}}
	body, err := json.Marshal(plan)
	if err != nil {
		t.Fatal(err)
	}

	r := chi.NewRouter()
	r.Put("/api/stream-jobs/{id}/edit-plan", h.PutStreamEditPlan)
	req := httptest.NewRequest(http.MethodPut, "/api/stream-jobs/"+id.String()+"/edit-plan", bytes.NewReader(body))
	rw := httptest.NewRecorder()
	r.ServeHTTP(rw, req)

	if rw.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body=%s", rw.Code, rw.Body.String())
	}
	if !strings.Contains(rw.Body.String(), "exceeds source duration 15.150") {
		t.Fatalf("body = %s, want source-duration error", rw.Body.String())
	}
	if streamRepo.jobs[id].Status != streamclips.StatusUploaded {
		t.Fatalf("job status = %s, want uploaded because invalid plan was not saved", streamRepo.jobs[id].Status)
	}
}

func TestPutStreamEditPlanRejectsUnknownFieldsAndFutureSchema(t *testing.T) {
	for _, tt := range []struct {
		name string
		body string
	}{
		{
			name: "unknown top-level field",
			body: `{"schema_version":"1.1","variant":"streamer-vertical-stack-40-60","clips":[],"efects":{"grade":true}}`,
		},
		{
			name: "unknown clip field",
			body: `{"schema_version":"1.1","variant":"streamer-vertical-stack-40-60","clips":[{"id":"clip-001","start_seconds":0,"end_seconds":1,"edti":{"speed":2}}]}`,
		},
		{
			name: "future schema",
			body: `{"schema_version":"999.0","variant":"streamer-vertical-stack-40-60","clips":[]}`,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			streamRepo := newFakeStreamRepo()
			id := uuid.New()
			streamRepo.jobs[id] = streamclips.Job{
				ID: id, Status: streamclips.StatusUploaded,
				SourcePath: streamclips.SourceKey(id),
				Probe:      streamclips.SourceProbe{DurationSeconds: 15},
			}
			h := NewHandlers(newFakeRepo(), newFakeStorage(), &fakeQueue{}, WithStreamRepository(streamRepo))
			r := chi.NewRouter()
			r.Put("/api/stream-jobs/{id}/edit-plan", h.PutStreamEditPlan)
			req := httptest.NewRequest(http.MethodPut, "/api/stream-jobs/"+id.String()+"/edit-plan", strings.NewReader(tt.body))
			rw := httptest.NewRecorder()
			r.ServeHTTP(rw, req)
			if rw.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400; body=%s", rw.Code, rw.Body.String())
			}
		})
	}
}

func TestStartStreamRenderAcceptsLegacyTwentySecondPlanWithoutPersistingMigration(t *testing.T) {
	streamRepo := newFakeStreamRepo()
	store := newFakeStorage()
	queue := &fakeQueue{}
	id := uuid.New()
	plan := streamclips.DefaultEditPlan()
	plan.FaceCropReviewed = true
	plan.Clips = []streamclips.ClipRange{{ID: "legacy", StartSeconds: 0, EndSeconds: 20}}
	planJSON, err := json.Marshal(plan)
	if err != nil {
		t.Fatal(err)
	}
	streamRepo.jobs[id] = streamclips.Job{
		ID:         id,
		Status:     streamclips.StatusReady,
		SourcePath: streamclips.SourceKey(id),
		Probe:      streamclips.SourceProbe{DurationSeconds: 15.15},
		EditPlan:   planJSON,
	}
	h := NewHandlers(newFakeRepo(), store, queue, WithStreamRepository(streamRepo))
	r := Routes(h)

	req := httptest.NewRequest(http.MethodPost, "/api/stream-jobs/"+id.String()+"/renders/"+plan.Variant, nil)
	rw := httptest.NewRecorder()
	r.ServeHTTP(rw, req)
	if rw.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202; body=%s", rw.Code, rw.Body.String())
	}
	var saved streamclips.EditPlan
	if err := json.Unmarshal(streamRepo.jobs[id].EditPlan, &saved); err != nil {
		t.Fatal(err)
	}
	if got, want := saved.Clips[0].EndSeconds, 20.0; got != want {
		t.Fatalf("saved legacy end_seconds = %.2f, want unchanged %.2f", got, want)
	}
	if _, ok := store.puts[streamclips.EditPlanKey(id)]; ok {
		t.Fatal("render start persisted an in-memory legacy migration")
	}
	if len(queue.enqueued) != 1 || queue.enqueued[0].Type() != tasks.TypeRenderStreamClip {
		t.Fatalf("queue = %#v, want one stream render", queue.enqueued)
	}
}

func TestStartStreamRenderRejectsApprovedPlanThatNeedsLegacyMigration(t *testing.T) {
	streamRepo := newFakeStreamRepo()
	queue := &fakeQueue{}
	id := uuid.New()
	plan := streamclips.DefaultEditPlan()
	plan.FaceCropReviewed = true
	plan.Clips = []streamclips.ClipRange{{ID: "legacy", StartSeconds: 0, EndSeconds: 20}}
	planJSON, err := json.Marshal(plan)
	if err != nil {
		t.Fatal(err)
	}
	streamRepo.jobs[id] = streamclips.Job{
		ID:         id,
		Status:     streamclips.StatusReady,
		SourcePath: streamclips.SourceKey(id),
		Probe:      streamclips.SourceProbe{DurationSeconds: 15.15},
		EditPlan:   planJSON,
	}
	h := NewHandlers(newFakeRepo(), newFakeStorage(), queue, WithStreamRepository(streamRepo))
	r := Routes(h)
	body := strings.NewReader(fmt.Sprintf(`{"expected_edit_plan_updated_at":%q}`, plan.UpdatedAt.Format(time.RFC3339Nano)))
	req := httptest.NewRequest(http.MethodPost, "/api/stream-jobs/"+id.String()+"/renders/"+plan.Variant, body)
	req.Header.Set("Content-Type", "application/json")
	rw := httptest.NewRecorder()
	r.ServeHTTP(rw, req)

	if rw.Code != http.StatusConflict || !strings.Contains(rw.Body.String(), "requires migration after approval") {
		t.Fatalf("response = %d %s, want actionable 409", rw.Code, rw.Body.String())
	}
	if len(queue.enqueued) != 0 {
		t.Fatalf("approved legacy plan queued %d render tasks, want zero", len(queue.enqueued))
	}
}

func TestStartStreamRenderRejectsBeforePersistingPartialLegacyMigration(t *testing.T) {
	streamRepo := newFakeStreamRepo()
	store := newFakeStorage()
	queue := &fakeQueue{}
	id := uuid.New()
	plan := streamclips.DefaultEditPlan()
	plan.Clips = []streamclips.ClipRange{
		{ID: "legacy", StartSeconds: 0, EndSeconds: 20},
		{ID: "custom-overrun", StartSeconds: 0, EndSeconds: 19},
	}
	planJSON, err := json.Marshal(plan)
	if err != nil {
		t.Fatal(err)
	}
	streamRepo.jobs[id] = streamclips.Job{
		ID:         id,
		Status:     streamclips.StatusReady,
		SourcePath: streamclips.SourceKey(id),
		Probe:      streamclips.SourceProbe{DurationSeconds: 15.15},
		EditPlan:   planJSON,
	}
	h := NewHandlers(newFakeRepo(), store, queue, WithStreamRepository(streamRepo))
	r := Routes(h)

	req := httptest.NewRequest(http.MethodPost, "/api/stream-jobs/"+id.String()+"/renders/"+plan.Variant, nil)
	rw := httptest.NewRecorder()
	r.ServeHTTP(rw, req)
	if rw.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body=%s", rw.Code, rw.Body.String())
	}
	var persisted streamclips.EditPlan
	if err := json.Unmarshal(streamRepo.jobs[id].EditPlan, &persisted); err != nil {
		t.Fatal(err)
	}
	if gotLegacy, gotCustom := persisted.Clips[0].EndSeconds, persisted.Clips[1].EndSeconds; gotLegacy != 20 || gotCustom != 19 {
		t.Fatalf("persisted clip ends = [%.0f %.0f], want unchanged [20 19]", gotLegacy, gotCustom)
	}
	if _, ok := store.puts[streamclips.EditPlanKey(id)]; ok {
		t.Fatal("invalid partially migrated edit-plan artifact was written")
	}
	if len(queue.enqueued) != 0 {
		t.Fatalf("invalid plan enqueued work: %#v", queue.enqueued)
	}
}

func TestStartStreamRenderRejectsLegacyPlanWhollyPastEOFWithoutPersisting(t *testing.T) {
	streamRepo := newFakeStreamRepo()
	store := newFakeStorage()
	queue := &fakeQueue{}
	id := uuid.New()
	plan := streamclips.DefaultEditPlan()
	plan.Clips = []streamclips.ClipRange{{ID: "legacy", StartSeconds: 16, EndSeconds: 20}}
	planJSON, err := json.Marshal(plan)
	if err != nil {
		t.Fatal(err)
	}
	streamRepo.jobs[id] = streamclips.Job{
		ID:         id,
		Status:     streamclips.StatusReady,
		SourcePath: streamclips.SourceKey(id),
		Probe:      streamclips.SourceProbe{DurationSeconds: 15.15},
		EditPlan:   planJSON,
	}
	h := NewHandlers(newFakeRepo(), store, queue, WithStreamRepository(streamRepo))
	r := Routes(h)
	req := httptest.NewRequest(http.MethodPost, "/api/stream-jobs/"+id.String()+"/renders/"+plan.Variant, nil)
	rw := httptest.NewRecorder()
	r.ServeHTTP(rw, req)
	if rw.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body=%s", rw.Code, rw.Body.String())
	}
	if !strings.Contains(rw.Body.String(), "no clips") {
		t.Fatalf("body = %s, want no-clips migration error", rw.Body.String())
	}
	var persisted streamclips.EditPlan
	if err := json.Unmarshal(streamRepo.jobs[id].EditPlan, &persisted); err != nil {
		t.Fatal(err)
	}
	if gotStart, gotEnd := persisted.Clips[0].StartSeconds, persisted.Clips[0].EndSeconds; gotStart != 16 || gotEnd != 20 {
		t.Fatalf("persisted legacy clip = %.2f-%.2f, want unchanged 16-20", gotStart, gotEnd)
	}
	if _, ok := store.puts[streamclips.EditPlanKey(id)]; ok {
		t.Fatal("empty migrated edit-plan artifact was written")
	}
	if len(queue.enqueued) != 0 {
		t.Fatalf("empty plan enqueued work: %#v", queue.enqueued)
	}
}

func TestStreamVideoRejectsUnsafeClipID(t *testing.T) {
	streamRepo := newFakeStreamRepo()
	id := uuid.New()
	streamRepo.jobs[id] = streamclips.Job{ID: id, Status: streamclips.StatusRendered, SourcePath: streamclips.SourceKey(id)}
	h := NewHandlers(newFakeRepo(), newFakeStorage(), &fakeQueue{}, WithStreamRepository(streamRepo))
	r := Routes(h)

	req := httptest.NewRequest(http.MethodGet, "/api/stream-jobs/"+id.String()+"/renders/streamer-vertical-stack/videos/bad.mp4", nil)
	rw := httptest.NewRecorder()
	r.ServeHTTP(rw, req)
	if rw.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body=%s", rw.Code, rw.Body.String())
	}
}

func TestStreamVideoServesNonConventionalKeyFromRenderResult(t *testing.T) {
	streamRepo := newFakeStreamRepo()
	id := uuid.New()
	const variant = "streamer-vertical-stack-40-60"
	streamRepo.jobs[id] = streamclips.Job{ID: id, Status: streamclips.StatusRendered, SourcePath: streamclips.SourceKey(id)}

	store := newFakeStorage()
	publishedKey, err := streamclips.RenderVideoKey(id, variant, "clip-1_v2")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Put(publishedKey, strings.NewReader("published-bytes")); err != nil {
		t.Fatal(err)
	}
	result, err := streamclips.NewRenderResult(id, variant, []streamclips.VideoEntry{{ClipID: "clip-1", Key: publishedKey}}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	resultKey, err := streamclips.RenderResultKey(id, variant)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Put(resultKey, bytes.NewReader(raw)); err != nil {
		t.Fatal(err)
	}

	h := NewHandlers(newFakeRepo(), store, &fakeQueue{}, WithStreamRepository(streamRepo))
	r := Routes(h)

	req := httptest.NewRequest(http.MethodGet, "/api/stream-jobs/"+id.String()+"/renders/"+variant+"/videos/clip-1", nil)
	rw := httptest.NewRecorder()
	r.ServeHTTP(rw, req)
	if rw.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rw.Code, rw.Body.String())
	}
	if got, want := rw.Body.String(), "published-bytes"; got != want {
		t.Fatalf("body = %q, want %q (published key from render result)", got, want)
	}
}

func TestStreamRenderArtifactsResolveAuthoritativeRevisionState(t *testing.T) {
	streamRepo := newFakeStreamRepo()
	id := uuid.New()
	revisionID := uuid.New()
	const variant = streamclips.VariantStreamer4060
	streamRepo.jobs[id] = streamclips.Job{ID: id, Status: streamclips.StatusRendered, SourcePath: streamclips.SourceKey(id)}

	store := newFakeStorage()
	videoKey, err := streamclips.RenderRevisionVideoKey(id, variant, revisionID, "clip-1_v2")
	if err != nil {
		t.Fatal(err)
	}
	galleryKey, err := streamclips.RenderRevisionGalleryKey(id, variant, revisionID)
	if err != nil {
		t.Fatal(err)
	}
	resultKey, err := streamclips.RenderRevisionResultKey(id, variant, revisionID)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Put(videoKey, strings.NewReader("revision-video")); err != nil {
		t.Fatal(err)
	}
	if err := store.Put(galleryKey, strings.NewReader("revision-gallery")); err != nil {
		t.Fatal(err)
	}
	h := NewHandlers(newFakeRepo(), store, &fakeQueue{}, WithStreamRepository(streamRepo))
	state, err := streamclips.NewRenderState(id, variant, streamclips.StatusRendered, nil, "", []streamclips.VideoEntry{{ClipID: "clip-1", Key: videoKey}})
	if err != nil {
		t.Fatal(err)
	}
	state.ResultKey = resultKey
	state.GalleryKey = galleryKey
	state.ArtifactDir, err = streamclips.RenderRevisionPrefix(id, variant, revisionID)
	if err != nil {
		t.Fatal(err)
	}
	if err := h.writeStreamRenderState(state); err != nil {
		t.Fatal(err)
	}
	r := Routes(h)

	for _, test := range []struct {
		path string
		want string
	}{
		{path: "/api/stream-jobs/" + id.String() + "/renders/" + variant + "/videos/clip-1", want: "revision-video"},
		{path: "/api/stream-jobs/" + id.String() + "/renders/" + variant + "/gallery", want: "revision-gallery"},
	} {
		req := httptest.NewRequest(http.MethodGet, test.path, nil)
		rw := httptest.NewRecorder()
		r.ServeHTTP(rw, req)
		if rw.Code != http.StatusOK || rw.Body.String() != test.want {
			t.Fatalf("GET %s = %d %q, want 200 %q", test.path, rw.Code, rw.Body.String(), test.want)
		}
	}
}

func TestStreamVideoFallsBackToPlainKeyWithoutRenderResult(t *testing.T) {
	streamRepo := newFakeStreamRepo()
	id := uuid.New()
	const variant = "streamer-vertical-stack-40-60"
	streamRepo.jobs[id] = streamclips.Job{ID: id, Status: streamclips.StatusRendered, SourcePath: streamclips.SourceKey(id)}

	store := newFakeStorage()
	plainKey, err := streamclips.RenderVideoKey(id, variant, "clip-1")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Put(plainKey, strings.NewReader("plain-bytes")); err != nil {
		t.Fatal(err)
	}

	h := NewHandlers(newFakeRepo(), store, &fakeQueue{}, WithStreamRepository(streamRepo))
	r := Routes(h)

	req := httptest.NewRequest(http.MethodGet, "/api/stream-jobs/"+id.String()+"/renders/"+variant+"/videos/clip-1", nil)
	rw := httptest.NewRecorder()
	r.ServeHTTP(rw, req)
	if rw.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rw.Code, rw.Body.String())
	}
	if got, want := rw.Body.String(), "plain-bytes"; got != want {
		t.Fatalf("body = %q, want %q (conventional key fallback)", got, want)
	}
}

func TestCreateStreamJobFromURLAcquiresAndEnqueues(t *testing.T) {
	streamRepo := newFakeStreamRepo()
	queue := &fakeQueue{}
	h := NewHandlers(newFakeRepo(), newFakeStorage(), queue,
		WithStreamRepository(streamRepo),
		WithCapabilities(Capabilities{YtdlpEnabled: true}),
	)
	r := Routes(h)

	req := httptest.NewRequest(http.MethodPost, "/api/stream-jobs", strings.NewReader(`{"source_url":"https://clips.twitch.tv/SomeSlug","title":"clutch"}`))
	req.Header.Set("Content-Type", "application/json")
	rw := httptest.NewRecorder()
	r.ServeHTTP(rw, req)

	if rw.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202; body=%s", rw.Code, rw.Body.String())
	}
	var created struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	}
	if err := json.Unmarshal(rw.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created.Status != string(streamclips.StatusAcquiring) {
		t.Fatalf("status = %q, want %q", created.Status, streamclips.StatusAcquiring)
	}
	id, err := uuid.Parse(created.ID)
	if err != nil {
		t.Fatal(err)
	}
	job, ok := streamRepo.jobs[id]
	if !ok {
		t.Fatal("stream job not created")
	}
	if job.SourceURL != "https://clips.twitch.tv/SomeSlug" {
		t.Fatalf("source url = %q", job.SourceURL)
	}
	if job.PublicSourceURL != "https://clips.twitch.tv/SomeSlug" {
		t.Fatalf("public source url = %q", job.PublicSourceURL)
	}
	if len(queue.enqueued) != 1 || queue.enqueued[0].Type() != tasks.TypeStreamAcquire {
		t.Fatalf("queue = %#v", queue.enqueued)
	}
}

func TestCreateStreamJobFromURLAcceptsKickClip(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		wantPub string
	}{
		{
			name:    "path",
			raw:     "https://kick.com/aimagia/clips/clip_01K8TRRRRPK5NL1N1FFFZ7C7",
			wantPub: "https://kick.com/aimagia/clips/clip_01K8TRRRRPK5NL1N1FFFZ7C7",
		},
		{
			name:    "query",
			raw:     "https://kick.com/aimagia?clip=clip_01K8TRRRRPK5NL1N1FFFZ7C7&utm_source=chat",
			wantPub: "https://kick.com/aimagia?clip=clip_01K8TRRRRPK5NL1N1FFFZ7C7",
		},
		{
			name:    "vod",
			raw:     "https://kick.com/xqc/videos/5c697a87-afce-4256-b01f-3c8fe71ef5cb",
			wantPub: "https://kick.com/xqc/videos/5c697a87-afce-4256-b01f-3c8fe71ef5cb",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			streamRepo := newFakeStreamRepo()
			queue := &fakeQueue{}
			h := NewHandlers(newFakeRepo(), newFakeStorage(), queue,
				WithStreamRepository(streamRepo),
				WithCapabilities(Capabilities{YtdlpEnabled: true}),
			)
			r := Routes(h)
			body, err := json.Marshal(map[string]string{"source_url": tt.raw})
			if err != nil {
				t.Fatal(err)
			}
			req := httptest.NewRequest(http.MethodPost, "/api/stream-jobs", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			rw := httptest.NewRecorder()
			r.ServeHTTP(rw, req)
			if rw.Code != http.StatusAccepted {
				t.Fatalf("status = %d, want 202; body=%s", rw.Code, rw.Body.String())
			}
			for _, job := range streamRepo.jobs {
				if job.SourceURL == "" {
					t.Fatal("source url not stored")
				}
				if job.PublicSourceURL != tt.wantPub {
					t.Fatalf("public source url = %q, want %q", job.PublicSourceURL, tt.wantPub)
				}
			}
		})
	}
}

func TestStreamJobAPINeverReturnsPrivateAcquisitionURL(t *testing.T) {
	streamRepo := newFakeStreamRepo()
	h := NewHandlers(newFakeRepo(), newFakeStorage(), &fakeQueue{},
		WithStreamRepository(streamRepo),
		WithCapabilities(Capabilities{YtdlpEnabled: true}),
	)
	r := Routes(h)
	const secret = "signed-private-value"
	req := httptest.NewRequest(
		http.MethodPost,
		"/api/stream-jobs",
		strings.NewReader(`{"source_url":"https://www.youtube.com/watch?v=abc123&utm_source=test&token=`+secret+`"}`),
	)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("create status = %d, want 202; body=%s", rec.Code, rec.Body.String())
	}
	var created struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}

	getReq := httptest.NewRequest(http.MethodGet, "/api/stream-jobs/"+created.ID, nil)
	getRec := httptest.NewRecorder()
	r.ServeHTTP(getRec, getReq)
	if getRec.Code != http.StatusOK {
		t.Fatalf("get status = %d, want 200; body=%s", getRec.Code, getRec.Body.String())
	}
	if strings.Contains(getRec.Body.String(), secret) || strings.Contains(getRec.Body.String(), "utm_source") {
		t.Fatalf("stream job response leaked private query material: %s", getRec.Body.String())
	}
	if !strings.Contains(getRec.Body.String(), `"source_url":"https://www.youtube.com/watch?v=abc123"`) {
		t.Fatalf("stream job response missing public source URL: %s", getRec.Body.String())
	}
}

func TestCreateStreamJobFromURLMarksAcceptedPendingJobFailedWhenQueueDiscardsIt(t *testing.T) {
	streamRepo := newFakeStreamRepo()
	queue := &fakeQueue{}
	h := NewHandlers(newFakeRepo(), newFakeStorage(), queue,
		WithStreamRepository(streamRepo),
		WithCapabilities(Capabilities{YtdlpEnabled: true}),
	)
	r := Routes(h)

	req := httptest.NewRequest(http.MethodPost, "/api/stream-jobs", strings.NewReader(`{"source_url":"https://clips.twitch.tv/SomeSlug"}`))
	req.Header.Set("Content-Type", "application/json")
	rw := httptest.NewRecorder()
	r.ServeHTTP(rw, req)
	if rw.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202; body=%s", rw.Code, rw.Body.String())
	}
	if len(queue.transitions) != 1 {
		t.Fatalf("queue transitions = %d, want 1", len(queue.transitions))
	}
	if err := queue.transitions[0](errors.New("inline queue task discarded during shutdown")); err != nil {
		t.Fatalf("discard transition error = %v", err)
	}
	for _, got := range streamRepo.jobs {
		if got.Status != streamclips.StatusFailed || !strings.Contains(got.FailureReason, "discarded during shutdown") {
			t.Fatalf("stream job after discard = status %q, reason %q; want failed discard reason", got.Status, got.FailureReason)
		}
	}
}

func TestCreateStreamJobFromURLRejectsInvalidURL(t *testing.T) {
	urls := []string{
		"not-a-url",
		"http://www.youtube.com/watch?v=abc123",
		"https://127.0.0.1/admin",
		"https://[::1]/admin",
		"https://169.254.169.254/latest/meta-data",
		"https://10.0.0.1/video.mp4",
		"https://youtube.com.evil.example/watch?v=abc123",
		"https://www.youtube.com/redirect/private-token",
		"https://user:password@www.youtube.com/watch?v=abc123",
		"https://www.youtube.com:8443/watch?v=abc123",
		"https://kick.com/aimagia",
		"https://kick.com/video/01234567-89ab-cdef-0123-456789abcdef",
	}
	for _, sourceURL := range urls {
		t.Run(sourceURL, func(t *testing.T) {
			streamRepo := newFakeStreamRepo()
			h := NewHandlers(newFakeRepo(), newFakeStorage(), &fakeQueue{},
				WithStreamRepository(streamRepo),
				WithCapabilities(Capabilities{YtdlpEnabled: true}),
			)
			r := Routes(h)
			body, err := json.Marshal(map[string]string{"source_url": sourceURL})
			if err != nil {
				t.Fatal(err)
			}
			req := httptest.NewRequest(http.MethodPost, "/api/stream-jobs", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			rw := httptest.NewRecorder()
			r.ServeHTTP(rw, req)

			if rw.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400; body=%s", rw.Code, rw.Body.String())
			}
			var response struct {
				Code string `json:"code"`
			}
			if err := json.Unmarshal(rw.Body.Bytes(), &response); err != nil {
				t.Fatal(err)
			}
			if response.Code != "invalid_source_url" {
				t.Fatalf("code = %q, want invalid_source_url", response.Code)
			}
			if len(streamRepo.jobs) != 0 {
				t.Fatalf("stream job created for an invalid url: %#v", streamRepo.jobs)
			}
		})
	}
}

func TestCreateStreamJobFromURLRejectsWhenYtdlpMissing(t *testing.T) {
	streamRepo := newFakeStreamRepo()
	h := NewHandlers(newFakeRepo(), newFakeStorage(), &fakeQueue{}, WithStreamRepository(streamRepo))
	r := Routes(h)

	req := httptest.NewRequest(http.MethodPost, "/api/stream-jobs", strings.NewReader(`{"source_url":"https://clips.twitch.tv/SomeSlug"}`))
	req.Header.Set("Content-Type", "application/json")
	rw := httptest.NewRecorder()
	r.ServeHTTP(rw, req)

	if rw.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409; body=%s", rw.Code, rw.Body.String())
	}
	if len(streamRepo.jobs) != 0 {
		t.Fatalf("stream job created while yt-dlp is unconfigured: %#v", streamRepo.jobs)
	}
}

func TestStartStreamRenderAcceptsRegistryVariantsAndRejectsUnknown(t *testing.T) {
	for _, variant := range streamclips.VariantNames() {
		t.Run(variant, func(t *testing.T) {
			streamRepo := newFakeStreamRepo()
			id := uuid.New()
			plan := streamclips.DefaultEditPlan()
			plan.Variant = variant
			plan.Clips = []streamclips.ClipRange{{ID: "clip-001", StartSeconds: 0, EndSeconds: 1}}
			layout, ok := streamclips.VariantByName(variant)
			if !ok {
				t.Fatalf("variant %q missing from registry", variant)
			}
			plan.FaceCropReviewed = !layout.FullFrame
			planJSON, err := json.Marshal(plan)
			if err != nil {
				t.Fatal(err)
			}
			streamRepo.jobs[id] = streamclips.Job{
				ID: id, Status: streamclips.StatusReady,
				SourcePath: streamclips.SourceKey(id), EditPlan: planJSON,
			}
			queue := &fakeQueue{}
			h := NewHandlers(newFakeRepo(), newFakeStorage(), queue, WithStreamRepository(streamRepo))
			r := Routes(h)

			req := httptest.NewRequest(http.MethodPost, "/api/stream-jobs/"+id.String()+"/renders/"+variant, nil)
			rw := httptest.NewRecorder()
			r.ServeHTTP(rw, req)

			if rw.Code != http.StatusAccepted {
				t.Fatalf("status = %d, want 202; body=%s", rw.Code, rw.Body.String())
			}
			if len(queue.enqueued) != 1 || queue.enqueued[0].Type() != tasks.TypeRenderStreamClip {
				t.Fatalf("queue = %#v", queue.enqueued)
			}
		})
	}

	t.Run("variant must match edit plan", func(t *testing.T) {
		streamRepo := newFakeStreamRepo()
		id := uuid.New()
		plan := streamclips.DefaultEditPlan()
		plan.Clips = []streamclips.ClipRange{{ID: "clip-001", StartSeconds: 0, EndSeconds: 1}}
		planJSON, err := json.Marshal(plan)
		if err != nil {
			t.Fatal(err)
		}
		streamRepo.jobs[id] = streamclips.Job{
			ID: id, Status: streamclips.StatusReady,
			SourcePath: streamclips.SourceKey(id), EditPlan: planJSON,
		}
		queue := &fakeQueue{}
		h := NewHandlers(newFakeRepo(), newFakeStorage(), queue, WithStreamRepository(streamRepo))
		r := Routes(h)

		req := httptest.NewRequest(
			http.MethodPost,
			"/api/stream-jobs/"+id.String()+"/renders/"+streamclips.VariantStreamerLandscape16x9,
			nil,
		)
		rw := httptest.NewRecorder()
		r.ServeHTTP(rw, req)

		if rw.Code != http.StatusConflict || !strings.Contains(rw.Body.String(), "does not match edit plan variant") {
			t.Fatalf("response = %d %s, want actionable 409", rw.Code, rw.Body.String())
		}
		if len(queue.enqueued) != 0 {
			t.Fatalf("queued mismatched render tasks = %d, want zero", len(queue.enqueued))
		}
	})

	t.Run("facecam crop must be reviewed", func(t *testing.T) {
		streamRepo := newFakeStreamRepo()
		id := uuid.New()
		plan := streamclips.DefaultEditPlan()
		plan.FaceCropReviewed = false
		plan.Clips = []streamclips.ClipRange{{ID: "clip-001", StartSeconds: 0, EndSeconds: 1}}
		planJSON, err := json.Marshal(plan)
		if err != nil {
			t.Fatal(err)
		}
		streamRepo.jobs[id] = streamclips.Job{
			ID: id, Status: streamclips.StatusReady,
			SourcePath: streamclips.SourceKey(id), EditPlan: planJSON,
		}
		queue := &fakeQueue{}
		h := NewHandlers(newFakeRepo(), newFakeStorage(), queue, WithStreamRepository(streamRepo))
		r := Routes(h)

		req := httptest.NewRequest(http.MethodPost, "/api/stream-jobs/"+id.String()+"/renders/"+plan.Variant, nil)
		rw := httptest.NewRecorder()
		r.ServeHTTP(rw, req)

		if rw.Code != http.StatusConflict || !strings.Contains(rw.Body.String(), "facecam crop requires explicit review") {
			t.Fatalf("response = %d %s, want actionable 409", rw.Code, rw.Body.String())
		}
		if len(queue.enqueued) != 0 {
			t.Fatalf("queued render tasks = %d, want zero", len(queue.enqueued))
		}
	})

	t.Run("unknown variant lists valid names", func(t *testing.T) {
		streamRepo := newFakeStreamRepo()
		id := uuid.New()
		streamRepo.jobs[id] = streamclips.Job{ID: id, Status: streamclips.StatusReady, SourcePath: streamclips.SourceKey(id)}
		h := NewHandlers(newFakeRepo(), newFakeStorage(), &fakeQueue{}, WithStreamRepository(streamRepo))
		r := Routes(h)

		req := httptest.NewRequest(http.MethodPost, "/api/stream-jobs/"+id.String()+"/renders/not-a-real-variant", nil)
		rw := httptest.NewRecorder()
		r.ServeHTTP(rw, req)

		if rw.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400; body=%s", rw.Code, rw.Body.String())
		}
		for _, name := range streamclips.VariantNames() {
			if !strings.Contains(rw.Body.String(), name) {
				t.Errorf("error body missing valid variant %q: %s", name, rw.Body.String())
			}
		}
	})
}

func TestStartStreamRenderRequiresTheApprovedEditPlanRevision(t *testing.T) {
	streamRepo := newFakeStreamRepo()
	id := uuid.New()
	plan := streamclips.DefaultEditPlan()
	plan.FaceCropReviewed = true
	plan.Clips = []streamclips.ClipRange{{ID: "clip-001", StartSeconds: 0, EndSeconds: 1}}
	plan.UpdatedAt = time.Date(2026, 7, 20, 20, 0, 0, 123, time.UTC)
	planJSON, err := json.Marshal(plan)
	if err != nil {
		t.Fatal(err)
	}
	streamRepo.jobs[id] = streamclips.Job{
		ID: id, Status: streamclips.StatusReady,
		SourcePath: streamclips.SourceKey(id), EditPlan: planJSON,
	}
	queue := &fakeQueue{}
	h := NewHandlers(newFakeRepo(), newFakeStorage(), queue, WithStreamRepository(streamRepo))
	r := Routes(h)

	staleBody := strings.NewReader(`{"expected_edit_plan_updated_at":"2026-07-20T19:59:59Z"}`)
	staleRequest := httptest.NewRequest(http.MethodPost, "/api/stream-jobs/"+id.String()+"/renders/"+plan.Variant, staleBody)
	staleRequest.Header.Set("Content-Type", "application/json")
	staleResponse := httptest.NewRecorder()
	r.ServeHTTP(staleResponse, staleRequest)

	if staleResponse.Code != http.StatusConflict || !strings.Contains(staleResponse.Body.String(), "changed after approval") {
		t.Fatalf("stale response = %d %s, want actionable 409", staleResponse.Code, staleResponse.Body.String())
	}
	if len(queue.enqueued) != 0 {
		t.Fatalf("stale revision queued %d render tasks, want zero", len(queue.enqueued))
	}

	currentBody := strings.NewReader(fmt.Sprintf(`{"expected_edit_plan_updated_at":%q}`, plan.UpdatedAt.Format(time.RFC3339Nano)))
	currentRequest := httptest.NewRequest(http.MethodPost, "/api/stream-jobs/"+id.String()+"/renders/"+plan.Variant, currentBody)
	currentRequest.Header.Set("Content-Type", "application/json")
	currentResponse := httptest.NewRecorder()
	r.ServeHTTP(currentResponse, currentRequest)

	if currentResponse.Code != http.StatusAccepted {
		t.Fatalf("current response = %d %s, want 202", currentResponse.Code, currentResponse.Body.String())
	}
	if len(queue.enqueued) != 1 {
		t.Fatalf("current revision queued %d render tasks, want one", len(queue.enqueued))
	}
}

func TestStartStreamRenderAcceptsAnEmptyOptionalJSONBody(t *testing.T) {
	streamRepo := newFakeStreamRepo()
	id := uuid.New()
	plan := streamclips.DefaultEditPlan()
	plan.FaceCropReviewed = true
	plan.Clips = []streamclips.ClipRange{{ID: "clip-001", StartSeconds: 0, EndSeconds: 1}}
	planJSON, err := json.Marshal(plan)
	if err != nil {
		t.Fatal(err)
	}
	streamRepo.jobs[id] = streamclips.Job{
		ID: id, Status: streamclips.StatusReady,
		SourcePath: streamclips.SourceKey(id), EditPlan: planJSON,
	}
	queue := &fakeQueue{}
	h := NewHandlers(newFakeRepo(), newFakeStorage(), queue, WithStreamRepository(streamRepo))
	r := Routes(h)
	req := httptest.NewRequest(http.MethodPost, "/api/stream-jobs/"+id.String()+"/renders/"+plan.Variant, strings.NewReader(""))
	req.Header.Set("Content-Type", "application/json")
	rw := httptest.NewRecorder()
	r.ServeHTTP(rw, req)

	if rw.Code != http.StatusAccepted {
		t.Fatalf("response = %d %s, want 202", rw.Code, rw.Body.String())
	}
	if len(queue.enqueued) != 1 {
		t.Fatalf("empty optional JSON body queued %d render tasks, want one", len(queue.enqueued))
	}
}

func TestStartStreamRenderMarksStateFailedWhenEnqueueFails(t *testing.T) {
	streamRepo := newFakeStreamRepo()
	store := newFakeStorage()
	id := uuid.New()
	variant := streamclips.DefaultVariant().Name
	streamRepo.jobs[id] = streamclips.Job{ID: id, Status: streamclips.StatusReady, SourcePath: streamclips.SourceKey(id), EditPlan: reviewedDefaultEditPlanJSON(t)}
	queue := &fakeQueue{err: errors.New("inline queue is full")}
	h := NewHandlers(newFakeRepo(), store, queue, WithStreamRepository(streamRepo))
	r := Routes(h)

	req := httptest.NewRequest(http.MethodPost, "/api/stream-jobs/"+id.String()+"/renders/"+variant, nil)
	rw := httptest.NewRecorder()
	r.ServeHTTP(rw, req)

	if rw.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500; body=%s", rw.Code, rw.Body.String())
	}
	key, err := streamclips.RenderStateKey(id, variant)
	if err != nil {
		t.Fatalf("RenderStateKey error = %v", err)
	}
	raw, ok := store.puts[key]
	if !ok {
		t.Fatal("failed stream render state not written")
	}
	var state streamclips.RenderState
	if err := json.Unmarshal(raw, &state); err != nil {
		t.Fatalf("unmarshal render state: %v", err)
	}
	if state.Status != streamclips.StatusFailed {
		t.Fatalf("render state status = %q, want failed", state.Status)
	}
	if state.Error != "enqueue render: inline queue is full" {
		t.Fatalf("render state error = %q", state.Error)
	}
}

func TestStartStreamRenderMarksAcceptedPendingStateFailedWhenQueueDiscardsIt(t *testing.T) {
	streamRepo := newFakeStreamRepo()
	store := newFakeStorage()
	id := uuid.New()
	variant := streamclips.DefaultVariant().Name
	streamRepo.jobs[id] = streamclips.Job{ID: id, Status: streamclips.StatusReady, SourcePath: streamclips.SourceKey(id), EditPlan: reviewedDefaultEditPlanJSON(t)}
	queue := &fakeQueue{}
	h := NewHandlers(newFakeRepo(), store, queue, WithStreamRepository(streamRepo))
	r := Routes(h)

	req := httptest.NewRequest(http.MethodPost, "/api/stream-jobs/"+id.String()+"/renders/"+variant, nil)
	rw := httptest.NewRecorder()
	r.ServeHTTP(rw, req)
	if rw.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202; body=%s", rw.Code, rw.Body.String())
	}
	if len(queue.transitions) != 1 {
		t.Fatalf("queue transitions = %d, want 1", len(queue.transitions))
	}
	if err := queue.transitions[0](errors.New("inline queue task discarded during shutdown")); err != nil {
		t.Fatalf("discard transition error = %v", err)
	}
	key, err := streamclips.RenderStateKey(id, variant)
	if err != nil {
		t.Fatal(err)
	}
	var state streamclips.RenderState
	if err := json.Unmarshal(store.puts[key], &state); err != nil {
		t.Fatalf("unmarshal render state: %v", err)
	}
	if state.Status != streamclips.StatusFailed || !strings.Contains(state.Error, "discarded during shutdown") {
		t.Fatalf("state after discard = status %q, error %q; want failed discard reason", state.Status, state.Error)
	}
}

func TestStartStreamRenderKeepsRenderingStateWhenTaskIsDuplicate(t *testing.T) {
	streamRepo := newFakeStreamRepo()
	store := newFakeStorage()
	id := uuid.New()
	variant := streamclips.DefaultVariant().Name
	streamRepo.jobs[id] = streamclips.Job{ID: id, Status: streamclips.StatusReady, SourcePath: streamclips.SourceKey(id), EditPlan: reviewedDefaultEditPlanJSON(t)}
	h := NewHandlers(newFakeRepo(), store, &fakeQueue{err: asynq.ErrDuplicateTask}, WithStreamRepository(streamRepo))
	existing, err := streamclips.NewRenderState(id, variant, streamclips.StatusRendering, nil, "", nil)
	if err != nil {
		t.Fatalf("NewRenderState error = %v", err)
	}
	if err := h.writeStreamRenderState(existing); err != nil {
		t.Fatalf("writeStreamRenderState error = %v", err)
	}
	r := Routes(h)

	req := httptest.NewRequest(http.MethodPost, "/api/stream-jobs/"+id.String()+"/renders/"+variant, nil)
	rw := httptest.NewRecorder()
	r.ServeHTTP(rw, req)

	if rw.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202; body=%s", rw.Code, rw.Body.String())
	}
	key, err := streamclips.RenderStateKey(id, variant)
	if err != nil {
		t.Fatalf("RenderStateKey error = %v", err)
	}
	raw, ok := store.puts[key]
	if !ok {
		t.Fatal("rendering stream state not written")
	}
	var state streamclips.RenderState
	if err := json.Unmarshal(raw, &state); err != nil {
		t.Fatalf("unmarshal render state: %v", err)
	}
	if state.Status != streamclips.StatusRendering || state.Error != "" {
		t.Fatalf("render state = status %q, error %q; want rendering without error", state.Status, state.Error)
	}
}

func TestStartStreamRenderPreservesRenderedStateWhenFinishedTaskIsStillDuplicate(t *testing.T) {
	streamRepo := newFakeStreamRepo()
	store := newFakeStorage()
	id := uuid.New()
	variant := streamclips.DefaultVariant().Name
	streamRepo.jobs[id] = streamclips.Job{ID: id, Status: streamclips.StatusRendered, SourcePath: streamclips.SourceKey(id), EditPlan: reviewedDefaultEditPlanJSON(t)}
	h := NewHandlers(newFakeRepo(), store, &fakeQueue{err: asynq.ErrDuplicateTask}, WithStreamRepository(streamRepo))
	previous, err := streamclips.NewRenderState(id, variant, streamclips.StatusRendered, nil, "", nil)
	if err != nil {
		t.Fatalf("NewRenderState error = %v", err)
	}
	if err := h.writeStreamRenderState(previous); err != nil {
		t.Fatalf("writeStreamRenderState error = %v", err)
	}
	r := Routes(h)

	req := httptest.NewRequest(http.MethodPost, "/api/stream-jobs/"+id.String()+"/renders/"+variant, nil)
	rw := httptest.NewRecorder()
	r.ServeHTTP(rw, req)
	if rw.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202; body=%s", rw.Code, rw.Body.String())
	}
	key, err := streamclips.RenderStateKey(id, variant)
	if err != nil {
		t.Fatalf("RenderStateKey error = %v", err)
	}
	var state streamclips.RenderState
	if err := json.Unmarshal(store.puts[key], &state); err != nil {
		t.Fatalf("unmarshal render state: %v", err)
	}
	if state.Status != streamclips.StatusRendered {
		t.Fatalf("render state status = %q, want rendered", state.Status)
	}
}

func storeBytes(t *testing.T, store storage.Storage, key string) []byte {
	t.Helper()
	rc, err := store.Open(key)
	if err != nil {
		t.Fatalf("open %s: %v", key, err)
	}
	defer rc.Close()
	b, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("read %s: %v", key, err)
	}
	return b
}

func hasAsynqOption(opts []asynq.Option, prefix string) bool {
	for _, opt := range opts {
		if strings.HasPrefix(opt.String(), prefix) {
			return true
		}
	}
	return false
}

func withIsolatedTempDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for _, key := range []string{"TMPDIR", "TMP", "TEMP"} {
		t.Setenv(key, dir)
	}
	return dir
}

func assertMultipartTempDirEmpty(t *testing.T) {
	t.Helper()
	entries, err := os.ReadDir(os.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "multipart-") || strings.HasPrefix(entry.Name(), "zv-stream-upload-") {
			t.Fatalf("temporary upload file still exists: %s", filepath.Join(os.TempDir(), entry.Name()))
		}
	}
}
