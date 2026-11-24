package workers

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/mediaassets"
	"github.com/rechedev9/cliphub/internal/storage"
	"github.com/rechedev9/cliphub/internal/store"
	"github.com/rechedev9/cliphub/internal/tasks"
	"github.com/rechedev9/cliphub/internal/timelineplan"
)

type timelineOwnerFixture struct {
	worker      *TimelineRenderWorker
	projects    *store.MemoryEditorProjectRepository
	files       storage.Storage
	projectID   uuid.UUID
	fingerprint string
}

func newTimelineOwnerFixture(t *testing.T, ffmpegPath string) timelineOwnerFixture {
	t.Helper()
	ctx := context.Background()
	dir := t.TempDir()
	files, err := storage.NewLocal(filepath.Join(dir, "data"))
	if err != nil {
		t.Fatal(err)
	}
	assets := store.NewMemoryEditorAssetRepository()
	projects := store.NewMemoryEditorProjectRepository()
	asset := mediaassets.Asset{FileName: "clip.mp4", Origin: mediaassets.OriginUpload, MediaKey: "editor-assets/clip.mp4"}
	if ffmpegPath != "" {
		clip := filepath.Join(dir, "clip.mp4")
		out, err := exec.Command(ffmpegPath, "-hide_banner", "-y",
			"-f", "lavfi", "-i", "color=c=red:s=640x360:d=2:r=30",
			"-c:v", "libx264", "-pix_fmt", "yuv420p", clip).CombinedOutput()
		if err != nil {
			t.Fatalf("write clip: %v: %s", err, out)
		}
		raw, err := os.ReadFile(clip)
		if err != nil {
			t.Fatal(err)
		}
		if err := files.Put(asset.MediaKey, bytes.NewReader(raw)); err != nil {
			t.Fatal(err)
		}
	}
	if err := assets.Create(ctx, &asset); err != nil {
		t.Fatal(err)
	}
	project := timelineplan.Project{Title: "owner", Status: timelineplan.StatusRendering}
	if err := projects.Create(ctx, &project); err != nil {
		t.Fatal(err)
	}
	plan := timelineplan.Document{
		SchemaVersion: timelineplan.SchemaVersion,
		Canvas:        timelineplan.Canvas{Width: 1080, Height: 1920, FPS: 60},
		Tracks: []timelineplan.Track{{ID: "v1", Kind: timelineplan.KindVideo, Items: []timelineplan.Item{{
			ID: "base", AssetID: asset.ID.String(), SourceIn: 0, SourceOut: 1,
		}}}},
	}
	if err := projects.SetPlan(ctx, project.ID, plan); err != nil {
		t.Fatal(err)
	}
	if err := projects.UpdateStatus(ctx, project.ID, timelineplan.StatusRendering, ""); err != nil {
		t.Fatal(err)
	}
	fp, err := timelineplan.Fingerprint(plan)
	if err != nil {
		t.Fatal(err)
	}
	worker := NewTimelineRenderWorker(projects, files, TimelineRenderWorkerConfig{
		WorkDir:     filepath.Join(dir, "work"),
		FFmpegPath:  ffmpegPath,
		Timeout:     "2m",
		AssetLookup: assets,
	})
	return timelineOwnerFixture{worker: worker, projects: projects, files: files, projectID: project.ID, fingerprint: fp}
}

func (f timelineOwnerFixture) seedState(t *testing.T, attemptID uuid.UUID) timelineplan.RenderState {
	t.Helper()
	published := timelineplan.RenderState{ProjectID: f.projectID, VideoKey: "editor-jobs/old/final.mp4", CoverKey: "editor-jobs/old/cover.jpg"}
	state := published.StartAttempt(timelineplan.RenderAttempt{
		ProjectID:   f.projectID,
		AttemptID:   attemptID,
		Fingerprint: f.fingerprint,
		At:          time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC),
	})
	raw, err := json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.files.Put(timelineplan.RenderStateKey(f.projectID), bytes.NewReader(raw)); err != nil {
		t.Fatal(err)
	}
	return state
}

func (f timelineOwnerFixture) run(t *testing.T, attemptID uuid.UUID) error {
	t.Helper()
	task, err := tasks.NewRenderTimelineTask(tasks.RenderTimelinePayload{ProjectID: f.projectID, AttemptID: attemptID, Fingerprint: f.fingerprint})
	if err != nil {
		t.Fatal(err)
	}
	return f.worker.HandleRenderTimeline(context.Background(), task)
}

func (f timelineOwnerFixture) state(t *testing.T) timelineplan.RenderState {
	t.Helper()
	state, _, err := f.worker.readState(f.projectID)
	if err != nil {
		t.Fatal(err)
	}
	return state
}

func (f timelineOwnerFixture) projectStatus(t *testing.T) timelineplan.Status {
	t.Helper()
	p, err := f.projects.Get(context.Background(), f.projectID)
	if err != nil {
		t.Fatal(err)
	}
	return p.Status
}

func timelineFFmpeg(t *testing.T) string {
	t.Helper()
	path, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg not found on PATH")
	}
	return path
}

func TestTimelineRenderFailureWritesOnlyForCurrentAttempt(t *testing.T) {
	t.Parallel()
	current := newTimelineOwnerFixture(t, "")
	attempt := uuid.New()
	current.seedState(t, attempt)
	if err := current.run(t, attempt); err == nil {
		t.Fatal("render without ffmpeg succeeded")
	}
	got := current.state(t)
	if got.Status != timelineplan.StatusFailed || got.AttemptID != attempt || got.Error == "" || got.VideoKey != "editor-jobs/old/final.mp4" {
		t.Fatalf("current attempt state = %+v, want failed for its attempt with the published keys", got)
	}
	if status := current.projectStatus(t); status != timelineplan.StatusFailed {
		t.Fatalf("project status = %s, want failed", status)
	}

	superseded := newTimelineOwnerFixture(t, "")
	newer := superseded.seedState(t, uuid.New())
	if err := superseded.run(t, uuid.New()); err == nil {
		t.Fatal("render without ffmpeg succeeded")
	}
	if got := superseded.state(t); got.Status != timelineplan.StatusRendering || got.AttemptID != newer.AttemptID || got.Error != "" {
		t.Fatalf("superseded attempt overwrote state: %+v", got)
	}
	if status := superseded.projectStatus(t); status != timelineplan.StatusRendering {
		t.Fatalf("project status = %s, want rendering", status)
	}
}

func TestTimelineRenderSuccessPublishesOnlyForCurrentAttempt(t *testing.T) {
	t.Parallel()
	ffmpegPath := timelineFFmpeg(t)

	current := newTimelineOwnerFixture(t, ffmpegPath)
	attempt := uuid.New()
	current.seedState(t, attempt)
	if err := current.run(t, attempt); err != nil {
		t.Fatalf("render: %v", err)
	}
	got := current.state(t)
	if got.Status != timelineplan.StatusRendered || got.AttemptID != attempt || got.VideoKey == "editor-jobs/old/final.mp4" || got.ResultKey == "" {
		t.Fatalf("current attempt state = %+v, want rendered for its attempt with new keys", got)
	}
	if status := current.projectStatus(t); status != timelineplan.StatusRendered {
		t.Fatalf("project status = %s, want rendered", status)
	}

	superseded := newTimelineOwnerFixture(t, ffmpegPath)
	newer := superseded.seedState(t, uuid.New())
	if err := superseded.run(t, uuid.New()); err != nil {
		t.Fatalf("superseded render: %v", err)
	}
	if got := superseded.state(t); got.Status != timelineplan.StatusRendering || got.AttemptID != newer.AttemptID || got.VideoKey != newer.VideoKey {
		t.Fatalf("superseded attempt overwrote state: %+v", got)
	}
	if status := superseded.projectStatus(t); status != timelineplan.StatusRendering {
		t.Fatalf("project status = %s, want rendering", status)
	}
}

func TestTimelineRenderLegacyPayloadStillWritesFailure(t *testing.T) {
	t.Parallel()
	f := newTimelineOwnerFixture(t, "")
	f.seedState(t, uuid.New())
	if err := f.run(t, uuid.Nil); err == nil {
		t.Fatal("render without ffmpeg succeeded")
	}
	if got := f.state(t); got.Status != timelineplan.StatusFailed || got.VideoKey != "editor-jobs/old/final.mp4" {
		t.Fatalf("legacy payload state = %+v, want failed with the published keys", got)
	}
}

func TestTimelineRenderFailureWithUnreadableStateStillFailsTheProject(t *testing.T) {
	t.Parallel()
	f := newTimelineOwnerFixture(t, "")
	if err := f.files.Put(timelineplan.RenderStateKey(f.projectID), bytes.NewReader([]byte("{"))); err != nil {
		t.Fatal(err)
	}
	if err := f.run(t, uuid.New()); err == nil {
		t.Fatal("render without ffmpeg succeeded")
	}
	if status := f.projectStatus(t); status != timelineplan.StatusFailed {
		t.Fatalf("project status = %s, want failed", status)
	}
}
