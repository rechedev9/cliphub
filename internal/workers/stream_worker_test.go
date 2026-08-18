package workers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"

	"github.com/rechedev9/cliphub/internal/streamclips"
	"github.com/rechedev9/cliphub/internal/tasks"
	"github.com/rechedev9/cliphub/internal/vodfetch"
)

func TestWriteStreamCoverExtractsRenderedFrame(t *testing.T) {
	runner := &fakeRunner{recordCoverCalls: true, fn: func(_ context.Context, _ string, args ...string) ([]byte, error) {
		return nil, os.WriteFile(args[len(args)-1], []byte("jpeg"), 0o600)
	}}
	w := NewStreamRenderWorker(newFakeStreamRepo(), newFakeStorage(), StreamRenderWorkerConfig{})
	w.runner = runner
	filename := filepath.Join(t.TempDir(), "cover.jpg")
	if err := w.writeStreamCover(context.Background(), "ffmpeg", "rendered.mp4", filename); err != nil {
		t.Fatal(err)
	}
	if got := runner.calls[0].args; !slices.Contains(got, "rendered.mp4") || !slices.Contains(got, "scale=720:-2") {
		t.Fatalf("cover args = %v, want rendered video and thumbnail scale", got)
	}
	if slices.Contains(runner.calls[0].args, "-ss") {
		t.Fatalf("cover args seek past the first frame: %v", runner.calls[0].args)
	}
}

func TestPublicSourceURLRemovesPrivateURLParts(t *testing.T) {
	got := publicSourceURL("https://www.twitch.tv/videos/123?utm_source=test#chapter")
	want := "https://www.twitch.tv/videos/123"
	if got != want {
		t.Fatalf("publicSourceURL() = %q, want %q", got, want)
	}
	if got := publicSourceURL("not a URL"); got != "" {
		t.Fatalf("publicSourceURL(invalid) = %q, want empty", got)
	}
	got = publicSourceURL("https://www.youtube.com/watch?v=abc123&list=PL456&utm_source=test#chapter")
	want = "https://www.youtube.com/watch?list=PL456&v=abc123"
	if got != want {
		t.Fatalf("publicSourceURL(youtube) = %q, want %q", got, want)
	}
	got = publicSourceURL("https://m.youtube.com/watch?v=mobile123&utm_source=test#chapter")
	want = "https://m.youtube.com/watch?v=mobile123"
	if got != want {
		t.Fatalf("publicSourceURL(mobile youtube) = %q, want %q", got, want)
	}
	got = publicSourceURL("https://youtube.com.evil.example/watch?v=secret&utm_source=test")
	want = "https://youtube.com.evil.example/watch"
	if got != want {
		t.Fatalf("publicSourceURL(non-youtube suffix) = %q, want %q", got, want)
	}
	got = publicSourceURL("https://kick.com/aimagia?clip=clip_01K8TRRRRPK5NL1N1FFFZ7C7&utm_source=chat#watch")
	want = "https://kick.com/aimagia?clip=clip_01K8TRRRRPK5NL1N1FFFZ7C7"
	if got != want {
		t.Fatalf("publicSourceURL(kick clip) = %q, want %q", got, want)
	}
}

// fakeStreamRepo implements StreamRenderRepository and StreamAcquireRepository
// for tests.
type fakeStreamRepo struct {
	jobs map[uuid.UUID]streamclips.Job
}

func newFakeStreamRepo(jobs ...streamclips.Job) *fakeStreamRepo {
	f := &fakeStreamRepo{jobs: map[uuid.UUID]streamclips.Job{}}
	for _, j := range jobs {
		f.jobs[j.ID] = j
	}
	return f
}

func (f *fakeStreamRepo) Get(_ context.Context, id uuid.UUID) (streamclips.Job, error) {
	j, ok := f.jobs[id]
	if !ok {
		return streamclips.Job{}, streamclips.ErrNotFound
	}
	return j, nil
}

func (f *fakeStreamRepo) UpdateStatus(_ context.Context, id uuid.UUID, s streamclips.Status, reason string) error {
	j, ok := f.jobs[id]
	if !ok {
		return streamclips.ErrNotFound
	}
	j.Status = s
	j.FailureReason = reason
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
	f.jobs[id] = j
	return nil
}

// fakeVodfetchRunner implements vodfetch.CommandRunner: it "downloads" by
// writing fixed content to the -o destination, so tests never shell out to a
// real yt-dlp binary.
type fakeVodfetchRunner struct {
	content string
	stdout  string
	stderr  string
	err     error
}

func (f *fakeVodfetchRunner) Run(_ context.Context, _, _ string, args ...string) (string, string, error) {
	if f.err != nil {
		return "", f.stderr, f.err
	}
	dest := argValue(args, "-o")
	if dest == "" {
		return "", "", fmt.Errorf("fake yt-dlp: missing -o arg")
	}
	if err := os.WriteFile(dest, []byte(f.content), 0o600); err != nil {
		return "", "", err
	}
	return f.stdout, "", nil
}

type fakeProber struct {
	probe streamclips.SourceProbe
	err   error
}

func (f fakeProber) Probe(context.Context, string) (streamclips.SourceProbe, error) {
	return f.probe, f.err
}

func streamAcquireTask(t *testing.T, id uuid.UUID) *asynq.Task {
	t.Helper()
	task, err := tasks.NewStreamAcquireTask(id)
	if err != nil {
		t.Fatal(err)
	}
	return task
}

func TestAcquireWorkerDownloadsProbesAndMarksReady(t *testing.T) {
	id := uuid.New()
	repo := newFakeStreamRepo(streamclips.Job{ID: id, Status: streamclips.StatusAcquiring, SourceURL: "https://clips.twitch.tv/SomeSlug"})
	store := newFakeStorage()

	w := NewAcquireWorker(repo, store, AcquireWorkerConfig{WorkDir: t.TempDir()})
	w.fetcher = vodfetch.Fetcher{Runner: &fakeVodfetchRunner{content: "fake-mp4-bytes", stdout: "vaya saco..\n"}}
	w.prober = fakeProber{probe: streamclips.SourceProbe{Width: 1920, Height: 1080, DurationSeconds: 12.5}}

	if err := w.HandleStreamAcquire(context.Background(), streamAcquireTask(t, id)); err != nil {
		t.Fatalf("HandleStreamAcquire error = %v", err)
	}

	got := repo.jobs[id]
	if got.Status != streamclips.StatusReady {
		t.Fatalf("status = %s, want ready", got.Status)
	}
	if got.SourceSHA256 == "" {
		t.Fatal("source sha256 not set")
	}
	if got.Probe.Width != 1920 || got.Probe.Height != 1080 {
		t.Fatalf("probe = %#v, want 1920x1080", got.Probe)
	}
	if _, ok := store.files[streamclips.SourceKey(id)]; !ok {
		t.Fatal("storage missing uploaded source")
	}
	if got.Title != "vaya saco.." {
		t.Fatalf("title = %q, want provider title", got.Title)
	}
	if _, ok := store.files[streamclips.SourceMetadataKey(id)]; !ok {
		t.Fatal("storage missing provider metadata sidecar")
	}
	if _, ok := store.files[streamclips.EditPlanKey(id)]; !ok {
		t.Fatal("storage missing default edit plan artifact")
	}
}

func TestAcquireWorkerSeedsKickBannerPlatform(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		wantPlat string
	}{
		{
			name:     "kick clip path",
			url:      "https://kick.com/aimagia/clips/clip_01K8TRRRRPK5NL1N1FFFZ7C7",
			wantPlat: streamclips.StreamerBannerPlatformKick,
		},
		{
			name:     "kick clip query",
			url:      "https://kick.com/aimagia?clip=clip_01K8TRRRRPK5NL1N1FFFZ7C7",
			wantPlat: streamclips.StreamerBannerPlatformKick,
		},
		{
			name:     "kick vod",
			url:      "https://kick.com/xqc/videos/5c697a87-afce-4256-b01f-3c8fe71ef5cb",
			wantPlat: streamclips.StreamerBannerPlatformKick,
		},
		{
			name:     "twitch clip stays default",
			url:      "https://clips.twitch.tv/SomeSlug",
			wantPlat: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id := uuid.New()
			repo := newFakeStreamRepo(streamclips.Job{ID: id, Status: streamclips.StatusAcquiring, SourceURL: tt.url})
			store := newFakeStorage()
			w := NewAcquireWorker(repo, store, AcquireWorkerConfig{WorkDir: t.TempDir()})
			w.fetcher = vodfetch.Fetcher{Runner: &fakeVodfetchRunner{content: "fake-mp4-bytes"}}
			w.prober = fakeProber{probe: streamclips.SourceProbe{Width: 1920, Height: 1080, DurationSeconds: 8}}
			if err := w.HandleStreamAcquire(context.Background(), streamAcquireTask(t, id)); err != nil {
				t.Fatalf("HandleStreamAcquire error = %v", err)
			}
			raw, ok := store.files[streamclips.EditPlanKey(id)]
			if !ok {
				t.Fatal("storage missing default edit plan artifact")
			}
			var plan streamclips.EditPlan
			if err := json.Unmarshal(raw, &plan); err != nil {
				t.Fatalf("decode edit plan: %v", err)
			}
			if plan.StreamerBanner.Platform != tt.wantPlat {
				t.Fatalf("platform = %q, want %q", plan.StreamerBanner.Platform, tt.wantPlat)
			}
		})
	}
}

func TestAcquireWorkerFailureRecordsReasonAndObs(t *testing.T) {
	id := uuid.New()
	repo := newFakeStreamRepo(streamclips.Job{ID: id, Status: streamclips.StatusAcquiring, SourceURL: "https://clips.twitch.tv/SomeSlug"})
	store := newFakeStorage()

	w := NewAcquireWorker(repo, store, AcquireWorkerConfig{WorkDir: t.TempDir()})
	w.fetcher = vodfetch.Fetcher{Runner: &fakeVodfetchRunner{stderr: "HTTP Error 404: Not Found", err: fmt.Errorf("exit status 1")}}

	err := w.HandleStreamAcquire(context.Background(), streamAcquireTask(t, id))
	if err == nil {
		t.Fatal("HandleStreamAcquire error = nil, want error")
	}

	got := repo.jobs[id]
	if got.Status != streamclips.StatusFailed {
		t.Fatalf("status = %s, want failed", got.Status)
	}
	if got.FailureReason == "" {
		t.Fatal("failure reason not set")
	}
	// The stored reason must be the clean, user-facing "not found" message, not
	// the raw yt-dlp stderr the user reported seeing dumped into the failed card.
	if !strings.Contains(got.FailureReason, "No encontramos un vídeo") {
		t.Fatalf("failure reason = %q, want the friendly not-found message", got.FailureReason)
	}
	if strings.Contains(got.FailureReason, "HTTP Error 404") {
		t.Fatalf("failure reason = %q, leaked the raw yt-dlp stderr", got.FailureReason)
	}
}

func TestAcquireFailureClassAndFriendlyReason(t *testing.T) {
	tests := []struct {
		err       error
		wantClass string
		wantText  string
	}{
		{err: vodfetch.ErrNotFound, wantClass: "not_found", wantText: "No encontramos un vídeo"},
		{err: vodfetch.ErrAuthRequired, wantClass: "auth_required", wantText: "inicio de sesión"},
		{err: vodfetch.ErrUnavailable, wantClass: "unavailable", wantText: "no está disponible"},
		{err: vodfetch.ErrBlocked, wantClass: "blocked", wantText: "protección anti-bots"},
		{err: vodfetch.ErrTooLarge, wantClass: "too_large", wantText: "límite máximo"},
		{err: errors.New("boom"), wantClass: "error", wantText: "clip o VOD público"},
	}
	for _, tt := range tests {
		t.Run(tt.wantClass, func(t *testing.T) {
			if got := acquireFailureClass(tt.err); got != tt.wantClass {
				t.Fatalf("acquireFailureClass() = %q, want %q", got, tt.wantClass)
			}
			reason := friendlyAcquireReason(tt.err)
			if !strings.Contains(reason, tt.wantText) {
				t.Fatalf("friendlyAcquireReason() = %q, want substring %q", reason, tt.wantText)
			}
		})
	}
}

func TestAcquireWorkerRejectsOversizedDownload(t *testing.T) {
	id := uuid.New()
	repo := newFakeStreamRepo(streamclips.Job{ID: id, Status: streamclips.StatusAcquiring, SourceURL: "https://clips.twitch.tv/OversizedClip"})
	store := newFakeStorage()

	w := NewAcquireWorker(repo, store, AcquireWorkerConfig{WorkDir: t.TempDir(), MaxBytes: 4})
	w.fetcher.Runner = &fakeVodfetchRunner{content: "12345"}

	err := w.HandleStreamAcquire(context.Background(), streamAcquireTask(t, id))
	if !errors.Is(err, vodfetch.ErrTooLarge) {
		t.Fatalf("HandleStreamAcquire error = %v, want errors.Is(_, ErrTooLarge)", err)
	}
	got := repo.jobs[id]
	if got.Status != streamclips.StatusFailed {
		t.Fatalf("status = %s, want failed", got.Status)
	}
	if !strings.Contains(got.FailureReason, "límite máximo") {
		t.Fatalf("failure reason = %q, want size-limit guidance", got.FailureReason)
	}
	if _, ok := store.files[streamclips.SourceKey(id)]; ok {
		t.Fatal("oversized source was uploaded to storage")
	}
}

func TestAcquireWorkerIsIdempotentWhenSourceAlreadyExists(t *testing.T) {
	id := uuid.New()
	repo := newFakeStreamRepo(streamclips.Job{ID: id, Status: streamclips.StatusAcquiring, SourceURL: "https://clips.twitch.tv/SomeSlug"})
	store := newFakeStorage()
	_ = store.Put(streamclips.SourceKey(id), strings.NewReader("already-downloaded"))
	if err := putJSONToStorage(store, streamclips.SourceMetadataKey(id), acquiredSourceMetadata{Title: "Título recuperado"}); err != nil {
		t.Fatal(err)
	}

	runner := &fakeVodfetchRunner{content: "should-not-be-used"}
	w := NewAcquireWorker(repo, store, AcquireWorkerConfig{WorkDir: t.TempDir()})
	w.fetcher = vodfetch.Fetcher{Runner: runner}
	w.prober = fakeProber{probe: streamclips.SourceProbe{Width: 1280, Height: 720}}

	if err := w.HandleStreamAcquire(context.Background(), streamAcquireTask(t, id)); err != nil {
		t.Fatalf("HandleStreamAcquire error = %v", err)
	}

	if string(store.files[streamclips.SourceKey(id)]) != "already-downloaded" {
		t.Fatalf("source artifact overwritten: %q", store.files[streamclips.SourceKey(id)])
	}
	if repo.jobs[id].Status != streamclips.StatusReady {
		t.Fatalf("status = %s, want ready", repo.jobs[id].Status)
	}
	if got := repo.jobs[id].Title; got != "Título recuperado" {
		t.Fatalf("title = %q, want recovered provider title", got)
	}
}

// --- StreamRenderWorker render pass ----------------------------------------

// newReadyStreamJob stores a source artifact and returns the job id together
// with a one-clip edit plan the render tests can mutate.
func newReadyStreamJob(t *testing.T, store *fakeStorage) (uuid.UUID, streamclips.EditPlan) {
	t.Helper()
	id := uuid.New()
	_ = store.Put(streamclips.SourceKey(id), strings.NewReader("source-bytes"))
	plan := streamclips.DefaultEditPlan()
	plan.Clips = []streamclips.ClipRange{{ID: "clip-001", StartSeconds: 0, EndSeconds: 2, Title: "one"}}
	return id, plan
}

func TestStreamRenderWorkerMigratesAlreadyQueuedLegacyDuration(t *testing.T) {
	store := newFakeStorage()
	id := uuid.New()
	store.files[streamclips.SourceKey(id)] = []byte("source")
	plan := streamclips.DefaultEditPlan()
	plan.Clips = []streamclips.ClipRange{{ID: "legacy", StartSeconds: 0, EndSeconds: 20}}
	planJSON, err := json.Marshal(plan)
	if err != nil {
		t.Fatal(err)
	}
	repo := newFakeStreamRepo(streamclips.Job{
		ID:         id,
		Status:     streamclips.StatusReady,
		SourcePath: streamclips.SourceKey(id),
		Probe:      streamclips.SourceProbe{DurationSeconds: 15.15},
		EditPlan:   planJSON,
	})
	runner := &fakeRunner{fn: func(_ context.Context, _ string, args ...string) ([]byte, error) {
		out := args[len(args)-1]
		if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
			return nil, err
		}
		return nil, os.WriteFile(out, []byte("video"), 0o644)
	}}
	w := NewStreamRenderWorker(repo, store, StreamRenderWorkerConfig{WorkDir: t.TempDir(), FFmpegPath: "ffmpeg"})
	w.runner = runner
	task, err := tasks.NewRenderStreamClipTask(id, plan.Variant)
	if err != nil {
		t.Fatal(err)
	}
	if err := w.HandleRenderStreamClip(context.Background(), task); err != nil {
		t.Fatalf("HandleRenderStreamClip error = %v", err)
	}
	if got, want := argValue(runner.calls[0].args, "-t"), "15.150000000"; got != want {
		t.Fatalf("render -t = %q, want migrated duration %q", got, want)
	}
}
