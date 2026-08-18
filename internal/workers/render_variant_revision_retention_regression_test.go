package workers

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"testing"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/editor"
	"github.com/rechedev9/cliphub/internal/job"
	"github.com/rechedev9/cliphub/internal/recording"
	"github.com/rechedev9/cliphub/internal/renderplan"
	"github.com/rechedev9/cliphub/internal/rules"
	"github.com/rechedev9/cliphub/internal/storage"
	"github.com/rechedev9/cliphub/internal/tasks"
)

func TestSuccessfulRenderVariantRerenderKeepsPreviousAndWinningRevisionsReadable(t *testing.T) {
	repo := newFakeRepo()
	store, err := storage.NewLocal(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	jobID := uuid.New()
	plan := minimalKillPlan()
	repo.jobs[jobID] = &job.Job{
		ID:       jobID,
		Status:   job.StatusRecorded,
		Rules:    rules.Default(),
		KillPlan: &plan,
	}
	putRevisionRetentionJSON(t, store, recording.ResultArtifactKey(jobID), recordingResultWithSegment("", "C:/stale/seg-001.mp4"))
	if err := store.Put(mustSegmentClipKey(t, jobID, "seg-001"), bytes.NewReader([]byte("clip"))); err != nil {
		t.Fatal(err)
	}

	runner := &fakeRunner{fn: func(_ context.Context, _ string, args ...string) ([]byte, error) {
		writeSuccessfulSingleShortRenderOutput(t, args)
		return []byte("rendered"), nil
	}}
	worker := NewRenderWorker(repo, store, RenderWorkerConfig{
		WorkDir:    t.TempDir(),
		EditorPath: "zv-editor",
	})
	worker.runner = runner

	run := func(edit renderplan.EditRequest) {
		t.Helper()
		task, err := tasks.NewRenderVariantTask(jobID, editor.PresetViral60Clean, "", 0, nil, edit)
		if err != nil {
			t.Fatal(err)
		}
		if err := worker.HandleRenderVariant(context.Background(), task); err != nil {
			t.Fatalf("HandleRenderVariant error = %v", err)
		}
	}

	run(renderplan.DefaultEditRequest())
	previous := readRevisionRetentionState(t, store, jobID, editor.PresetViral60Clean)

	rerenderEdit := renderplan.DefaultEditRequest()
	rerenderEdit.Transition = renderplan.TransitionDip
	run(rerenderEdit)
	winner := readRevisionRetentionState(t, store, jobID, editor.PresetViral60Clean)

	if previous.ArtifactPrefix == "" || winner.ArtifactPrefix == "" || previous.ArtifactPrefix == winner.ArtifactPrefix {
		t.Fatalf("revision prefixes: previous=%q winner=%q", previous.ArtifactPrefix, winner.ArtifactPrefix)
	}
	assertRenderVariantRevisionReadable(t, store, previous)
	assertRenderVariantRevisionReadable(t, store, winner)
}

func putRevisionRetentionJSON(t *testing.T, store storage.Storage, key string, value any) {
	t.Helper()
	body, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Put(key, bytes.NewReader(body)); err != nil {
		t.Fatal(err)
	}
}

func readRevisionRetentionState(t *testing.T, store storage.Storage, id uuid.UUID, variant string) renderplan.RenderVariantState {
	t.Helper()
	key := mustRenderVariantStatusKey(t, id, variant)
	r, err := store.Open(key)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	var state renderplan.RenderVariantState
	if err := json.NewDecoder(r).Decode(&state); err != nil {
		t.Fatal(err)
	}
	return state
}

func assertRenderVariantRevisionReadable(t *testing.T, store storage.Storage, state renderplan.RenderVariantState) {
	t.Helper()
	keys := []string{
		state.RenderResultKey,
		state.EditDocumentKey,
		state.EditManifestKey,
		state.PackManifestKey,
		state.GalleryKey,
		state.PublishSummaryKey,
	}
	for _, kind := range []renderplan.RenderVariantArtifactKind{
		renderplan.RenderVariantArtifactVideo,
		renderplan.RenderVariantArtifactCaption,
	} {
		ref, err := renderplan.NewRenderVariantArtifactRefForState(state, kind, "seg-001")
		if err != nil {
			t.Fatal(err)
		}
		keys = append(keys, ref.Key)
	}
	for _, key := range keys {
		r, err := store.Open(key)
		if err != nil {
			t.Fatalf("open revision artifact %s: %v", key, err)
		}
		if _, readErr := io.ReadAll(r); readErr != nil {
			_ = r.Close()
			t.Fatalf("read revision artifact %s: %v", key, readErr)
		}
		if err := r.Close(); err != nil {
			t.Fatalf("close revision artifact %s: %v", key, err)
		}
	}
}
