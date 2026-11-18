package workers

import (
	"bytes"
	"context"
	"encoding/json"
	"path"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/artifacts"
	"github.com/rechedev9/cliphub/internal/captureretention"
	"github.com/rechedev9/cliphub/internal/editor"
	"github.com/rechedev9/cliphub/internal/job"
	"github.com/rechedev9/cliphub/internal/recapplan"
	"github.com/rechedev9/cliphub/internal/renderplan"
	"github.com/rechedev9/cliphub/internal/storage"
)

func TestFullDemoRenderKeepsTheCapturesOfTheNewestLongVideos(t *testing.T) {
	store, repo := newLocalStore(t), newFakeRepo()
	clips := make([]string, 4)
	for i := range clips {
		_, clips[i] = seedRenderedFullDemo(t, store, repo, time.Now().Add(time.Hour+time.Duration(i)*time.Minute))
	}
	w := NewRenderWorker(repo, store, RenderWorkerConfig{})
	setCaptureRetention(t, store, 3)
	w.releaseFullDemoCaptures(context.Background(), uuid.Nil)
	for i, clip := range clips {
		if want := i > 0; clipExists(t, store, clip) != want {
			t.Errorf("job %d of 4 (oldest first): captures kept = %v, want %v", i, !want, want)
		}
	}
	setCaptureRetention(t, store, 0)
	w.releaseFullDemoCaptures(context.Background(), uuid.Nil)
	for i, clip := range clips {
		if clipExists(t, store, clip) {
			t.Errorf("job %d kept its captures with retention 0", i)
		}
	}
}

func TestFullDemoCaptureRetentionSparesBusyJobsAndTheLabSwitch(t *testing.T) {
	store, repo := newLocalStore(t), newFakeRepo()
	setCaptureRetention(t, store, 0)
	_, idle := seedRenderedFullDemo(t, store, repo, time.Now().Add(time.Hour))
	busyID, busy := seedRenderedFullDemo(t, store, repo, time.Now().Add(2*time.Hour))
	repo.jobs[busyID].Status = job.StatusRecording
	NewRenderWorker(repo, store, RenderWorkerConfig{KeepFullDemoCaptures: true}).releaseFullDemoCaptures(context.Background(), uuid.Nil)
	if !clipExists(t, store, idle) || !clipExists(t, store, busy) {
		t.Fatal("ZV_KEEP_FULL_DEMO_CAPTURES must keep every capture")
	}
	NewRenderWorker(repo, store, RenderWorkerConfig{}).releaseFullDemoCaptures(context.Background(), uuid.Nil)
	if clipExists(t, store, idle) || !clipExists(t, store, busy) {
		t.Fatalf("idle kept = %v, recording kept = %v; want only the recording job's captures", clipExists(t, store, idle), clipExists(t, store, busy))
	}
}

// A 16:9 summary renders to the same variant as a Full Demo but has no captures to keep.
func TestFullDemoCaptureRetentionCountsOnlyFullDemos(t *testing.T) {
	store, repo := newLocalStore(t), newFakeRepo()
	_, clip := seedRenderedFullDemo(t, store, repo, time.Now().Add(time.Hour))
	for i := range 3 {
		id := uuid.New()
		repo.jobs[id] = &job.Job{ID: id, Status: job.StatusDone}
		writeReadyRender(t, store, id, time.Now().Add(2*time.Hour+time.Duration(i)*time.Minute), nil)
	}
	setCaptureRetention(t, store, 3)
	NewRenderWorker(repo, store, RenderWorkerConfig{}).releaseFullDemoCaptures(context.Background(), uuid.Nil)
	if !clipExists(t, store, clip) {
		t.Fatal("three newer summaries took the retention slots and freed the Full Demo's captures")
	}
}

func TestFullDemoCaptureRetentionSurvivesAJobItCannotRead(t *testing.T) {
	store, repo := newLocalStore(t), newFakeRepo()
	setCaptureRetention(t, store, 0)
	broken := uuid.New()
	key, err := renderplan.RenderVariantStateKey(broken, editor.PresetGameplayPOV60)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Put(key, bytes.NewReader([]byte("{not json"))); err != nil {
		t.Fatal(err)
	}
	_, clip := seedRenderedFullDemo(t, store, repo, time.Now().Add(time.Hour))
	NewRenderWorker(repo, store, RenderWorkerConfig{}).releaseFullDemoCaptures(context.Background(), uuid.Nil)
	if clipExists(t, store, clip) {
		t.Fatal("an unreadable job stopped the release of the others")
	}
}

func newLocalStore(t *testing.T) *storage.Local {
	t.Helper()
	store, err := storage.NewLocal(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return store
}

func setCaptureRetention(t *testing.T, store storage.Storage, keep int) {
	t.Helper()
	if err := captureretention.Save(store, keep); err != nil {
		t.Fatal(err)
	}
}

// seedRenderedFullDemo stores a ready long video rendered at renderedAt and one round clip it covers.
func seedRenderedFullDemo(t *testing.T, store storage.Storage, repo *fakeRepo, renderedAt time.Time) (uuid.UUID, string) {
	t.Helper()
	id := uuid.New()
	repo.jobs[id] = &job.Job{ID: id, Status: job.StatusDone}
	clip := path.Join(artifacts.FullDemoCapturePrefix(id, uuid.New()), "segments", "round-001.mp4")
	if err := store.Put(clip, bytes.NewReader([]byte("round"))); err != nil {
		t.Fatal(err)
	}
	writeReadyRender(t, store, id, renderedAt, approvedFullDemo(t))
	return id, clip
}

// approvedFullDemo is an approved Full Demo plan, as a long video's render state names it.
func approvedFullDemo(t *testing.T) *recapplan.Snapshot {
	t.Helper()
	result, _, _ := fullDemoPublicationFixture(t, "unused")
	doc := *result.Plan.FullDemo
	return &recapplan.Snapshot{Document: doc, Approval: recapplan.Approval{PlanHash: doc.PlanHash, AllowSafeTailTrim: true, Timestamp: time.Now().UTC()}}
}

// writeReadyRender stores a ready 16:9 long video: a Full Demo with fullDemo, else a summary.
func writeReadyRender(t *testing.T, store storage.Storage, id uuid.UUID, renderedAt time.Time, fullDemo *recapplan.Snapshot) {
	t.Helper()
	loadout, err := renderplan.LoadoutForVariant(editor.PresetGameplayPOV60)
	if err != nil {
		t.Fatal(err)
	}
	state, err := renderplan.NewRenderVariantStateForLoadout(renderplan.NewRenderVariantStateForLoadoutOptions{JobID: id, Loadout: loadout, Status: renderplan.RenderVariantStatusReady, RevisionID: uuid.New(), Now: renderedAt, FullDemo: fullDemo})
	if err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	key, err := renderplan.RenderVariantStateKey(id, editor.PresetGameplayPOV60)
	if err != nil {
		t.Fatal(err)
	}
	for k, body := range map[string][]byte{key: b, state.ArtifactPrefix + "/videos/demo-compilation.mp4": []byte("video")} {
		if err := store.Put(k, bytes.NewReader(body)); err != nil {
			t.Fatal(err)
		}
	}
}

func clipExists(t *testing.T, store storage.Storage, key string) bool {
	t.Helper()
	exists, err := store.Exists(key)
	if err != nil {
		t.Fatal(err)
	}
	return exists
}
