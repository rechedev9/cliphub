package workers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/editor"
	"github.com/rechedev9/cliphub/internal/job"
	"github.com/rechedev9/cliphub/internal/recording"
	"github.com/rechedev9/cliphub/internal/renderplan"
	"github.com/rechedev9/cliphub/internal/rules"
	"github.com/rechedev9/cliphub/internal/tasks"
)

func TestHandleRenderVariantPersistsEmptyEditorCombinedOutput(t *testing.T) {
	id, store, worker := recordedFullDemoRender(t)
	var logs bytes.Buffer
	log.SetOutput(&logs)
	log.SetFlags(0)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })

	var sawDocument bool
	worker.runner = &fakeRunner{fn: func(_ context.Context, exe string, args ...string) ([]byte, error) {
		if filepath.Base(exe) != "zv-editor.exe" {
			t.Fatalf("exe = %q, want zv-editor.exe", exe)
		}
		if hasArg(args, "--document") {
			t.Fatalf("editor args = %#v, zv-editor does not take --document", args)
		}
		outDir := argValue(args, "--out")
		if outDir == "" {
			t.Fatal("editor args missing --out")
		}
		if _, err := os.Stat(filepath.Join(outDir, "edit-document.json")); err != nil {
			t.Fatalf("workDir edit-document.json: %v", err)
		}
		sawDocument = true
		return nil, errors.New("exit status 1")
	}}

	task, err := tasks.NewRenderVariantTask(id, editor.PresetGameplayPOV60, "", 0, nil, renderplan.RecapEditRequest())
	if err != nil {
		t.Fatal(err)
	}
	if err := worker.HandleRenderVariant(context.Background(), task); err == nil {
		t.Fatal("HandleRenderVariant error = nil, want editor exit")
	}
	if !sawDocument {
		t.Fatal("editor ran without a workDir edit-document.json")
	}

	var state renderplan.RenderVariantState
	if err := json.Unmarshal(store.files[mustRenderVariantStatusKey(t, id, editor.PresetGameplayPOV60)], &state); err != nil {
		t.Fatal(err)
	}
	if state.Status != renderplan.RenderVariantStatusFailed {
		t.Fatalf("status = %q, want %q", state.Status, renderplan.RenderVariantStatusFailed)
	}
	if !strings.Contains(state.Error, emptyCombinedOutputMarker) {
		t.Fatalf("status error = %q, want %q", state.Error, emptyCombinedOutputMarker)
	}
	if _, ok := store.files[mustRenderVariantEditDocumentKey(t, id, editor.PresetGameplayPOV60)]; ok {
		t.Fatal("durable edit-document.json was uploaded after an editor failure; recap folder absence is expected until success")
	}
	if _, ok := store.files[mustRenderVariantEditManifestKey(t, id, editor.PresetGameplayPOV60)]; ok {
		t.Fatal("durable edit-manifest.json was uploaded after an editor failure")
	}
	if !strings.Contains(logs.String(), "combined_output="+emptyCombinedOutputMarker) {
		t.Fatalf("studio.log line missing: %q", logs.String())
	}
}

func TestHandleRenderVariantPersistsTruncatedEditorCombinedOutput(t *testing.T) {
	id, store, worker := recordedFullDemoRender(t)
	dump := bytes.Repeat([]byte("ffmpeg frame\n"), 400)
	worker.runner = &fakeRunner{fn: func(context.Context, string, ...string) ([]byte, error) {
		return dump, errors.New("exit status 1")
	}}

	task, err := tasks.NewRenderVariantTask(id, editor.PresetGameplayPOV60, "", 0, nil, renderplan.RecapEditRequest())
	if err != nil {
		t.Fatal(err)
	}
	if err := worker.HandleRenderVariant(context.Background(), task); err == nil {
		t.Fatal("HandleRenderVariant error = nil, want editor exit")
	}

	var state renderplan.RenderVariantState
	if err := json.Unmarshal(store.files[mustRenderVariantStatusKey(t, id, editor.PresetGameplayPOV60)], &state); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(state.Error, truncatedCombinedOutputMark) {
		t.Fatalf("status error = %q, want truncated CombinedOutput", state.Error)
	}
	if strings.Contains(state.Error, string(dump)) {
		t.Fatal("status error kept the full ffmpeg dump")
	}
}

func recordedFullDemoRender(t *testing.T) (uuid.UUID, *fakeStorage, *RenderWorker) {
	t.Helper()
	repo := newFakeRepo()
	store := newFakeStorage()
	id := uuid.New()
	plan := minimalKillPlan()
	repo.jobs[id] = &job.Job{
		ID:            id,
		Status:        job.StatusRecorded,
		DemoPath:      "demos/test.dem",
		TargetSteamID: "76561197960265729",
		Rules:         rules.Default(),
		KillPlan:      &plan,
	}
	putJSON(t, store, recording.ResultArtifactKey(id), recordingResultWithSegment("", "C:/stale/seg-001.mp4"))
	_ = store.Put(mustSegmentClipKey(t, id, "seg-001"), bytes.NewReader([]byte("clip")))
	_ = store.Put("demos/test.dem", bytes.NewReader([]byte("demo")))
	worker := NewRenderWorker(repo, store, RenderWorkerConfig{
		WorkDir:    t.TempDir(),
		EditorPath: "zv-editor.exe",
	})
	worker.voiceExtract = func(string, string, string) (int, error) { return 0, nil }
	return id, store, worker
}
