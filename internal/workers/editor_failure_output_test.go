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

func TestHandleRenderVariantLocksRecapCaptureToLandscape(t *testing.T) {
	id, store, worker := recordedFullDemoRender(t)
	if err := putCaptureKind(store, id, true); err != nil {
		t.Fatal(err)
	}
	var format string
	worker.runner = &fakeRunner{fn: func(_ context.Context, exe string, args ...string) ([]byte, error) {
		if filepath.Base(exe) != "zv-editor.exe" {
			t.Fatalf("exe = %q, want zv-editor.exe", exe)
		}
		format = argValue(args, "--output-format")
		return nil, errors.New("exit status 1")
	}}

	task, err := tasks.NewRenderVariantTask(id, editor.PresetGameplayPOV60, "", 0, nil, renderplan.DefaultEditRequest())
	if err != nil {
		t.Fatal(err)
	}
	if err := worker.HandleRenderVariant(context.Background(), task); err == nil {
		t.Fatal("HandleRenderVariant error = nil, want editor exit")
	}
	if format != renderplan.FormatLandscape16x9 {
		t.Fatalf("--output-format = %q, want %q on a recap capture", format, renderplan.FormatLandscape16x9)
	}
}

func TestHandleRenderVariantIngestsCompleteLandscapeAfterEmptyCombinedOutput(t *testing.T) {
	id, store, worker := recordedFullDemoRender(t)
	if err := putCaptureKind(store, id, true); err != nil {
		t.Fatal(err)
	}
	worker.runner = &fakeRunner{fn: func(_ context.Context, _ string, args ...string) ([]byte, error) {
		outDir := argValue(args, "--out")
		publishDir := argValue(args, "--publish-dir")
		if outDir == "" || publishDir == "" {
			t.Fatal("editor args missing --out or --publish-dir")
		}
		videoPath := filepath.Join(publishDir, "demo-compilation.mp4")
		coverPath := filepath.Join(publishDir, "demo-compilation.cover.jpg")
		captionPath := filepath.Join(publishDir, "demo-compilation.caption.txt")
		logPath := filepath.Join(outDir, "logs", "demo-compilation-render.log")
		for _, file := range []struct {
			path string
			body string
		}{
			{filepath.Join(outDir, "edit-manifest.json"), `{"shorts":[{"segment_id":"demo-compilation"}]}`},
			{filepath.Join(publishDir, "pack-manifest.json"), `{"items":[{"segment_id":"demo-compilation"}]}`},
			{filepath.Join(publishDir, "index.html"), `<html></html>`},
			{filepath.Join(publishDir, "publish-summary.md"), `summary`},
			{videoPath, "complete-1920x1080"},
			{coverPath, "cover"},
			{captionPath, "caption"},
			{logPath, "log"},
		} {
			if err := os.MkdirAll(filepath.Dir(file.path), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(file.path, []byte(file.body), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		rendered := editor.Result{
			Preset:      editor.PresetGameplayPOV60,
			OutputDir:   outDir,
			PublishDir:  publishDir,
			GalleryPath: filepath.Join(publishDir, "index.html"),
			SummaryPath: filepath.Join(publishDir, "publish-summary.md"),
			Error:       emptyCombinedOutputMarker,
			Shorts: []editor.ShortResult{{
				SegmentID:     "demo-compilation",
				Output:        videoPath,
				PublishPath:   videoPath,
				CoverPath:     coverPath,
				CaptionPath:   captionPath,
				RenderLogPath: logPath,
				OutputFormat:  editor.OutputFormatLandscape16x9,
				PublishArtifact: recording.RecordingArtifact{
					Width:  1920,
					Height: 1080,
					Path:   videoPath,
					Type:   "video",
					Role:   "publish",
				},
			}},
		}
		if err := writeJSONFile(filepath.Join(outDir, "shorts-result.json"), rendered); err != nil {
			t.Fatal(err)
		}
		return nil, errors.New("exit status 1")
	}}

	task, err := tasks.NewRenderVariantTask(id, editor.PresetGameplayPOV60, "", 0, nil, renderplan.RecapEditRequest())
	if err != nil {
		t.Fatal(err)
	}
	if err := worker.HandleRenderVariant(context.Background(), task); err != nil {
		t.Fatalf("HandleRenderVariant error = %v, want ingested complete 16:9", err)
	}
	var state renderplan.RenderVariantState
	if err := json.Unmarshal(store.files[mustRenderVariantStatusKey(t, id, editor.PresetGameplayPOV60)], &state); err != nil {
		t.Fatal(err)
	}
	if state.Status != renderplan.RenderVariantStatusReady && state.Status != renderplan.RenderVariantStatusReview {
		t.Fatalf("status = %q, want ready or review after ingesting complete 16:9", state.Status)
	}
	if state.EditDocumentKey == "" {
		t.Fatal("render state missing edit document after salvage ingest")
	}
	if _, ok := store.files[state.EditDocumentKey]; !ok {
		t.Fatalf("durable %s missing after salvage ingest", state.EditDocumentKey)
	}
}

func TestHandleRenderVariantIngestsAttemptFileWithoutResultJSON(t *testing.T) {
	id, store, worker := recordedFullDemoRender(t)
	if err := putCaptureKind(store, id, true); err != nil {
		t.Fatal(err)
	}
	worker.cfg.FFprobePath = "ffprobe.exe"
	var logs bytes.Buffer
	log.SetOutput(&logs)
	log.SetFlags(0)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })

	worker.runner = &fakeRunner{fn: func(_ context.Context, exe string, args ...string) ([]byte, error) {
		switch filepath.Base(exe) {
		case "zv-editor.exe":
			outDir := argValue(args, "--out")
			if outDir == "" {
				t.Fatal("editor args missing --out")
			}
			if got := argValue(args, "--output-format"); got != renderplan.FormatLandscape16x9 {
				t.Fatalf("--output-format = %q, want landscape-16x9", got)
			}
			attempt := filepath.Join(outDir, ".short-001-demo-compilation.attempt-2591725329.mp4")
			if err := os.WriteFile(attempt, bytes.Repeat([]byte("mp4"), 64), 0o600); err != nil {
				t.Fatal(err)
			}
			return nil, errors.New("exit status 1")
		case "ffprobe.exe":
			return []byte(`{"streams":[{"codec_name":"h264","width":1920,"height":1080,"r_frame_rate":"60/1","duration":"1312.0"}],"format":{"duration":"1312.0","size":"5177344000"}}`), nil
		default:
			t.Fatalf("unexpected exe %q", exe)
			return nil, nil
		}
	}}

	task, err := tasks.NewRenderVariantTask(id, editor.PresetGameplayPOV60, "", 0, nil, renderplan.DefaultEditRequest())
	if err != nil {
		t.Fatal(err)
	}
	if err := worker.HandleRenderVariant(context.Background(), task); err != nil {
		t.Fatalf("HandleRenderVariant error = %v, want ingest of attempt MP4", err)
	}
	if !strings.Contains(logs.String(), "combined_output="+emptyCombinedOutputMarker) {
		t.Fatalf("studio.log missing CombinedOutput: %q", logs.String())
	}

	var state renderplan.RenderVariantState
	if err := json.Unmarshal(store.files[mustRenderVariantStatusKey(t, id, editor.PresetGameplayPOV60)], &state); err != nil {
		t.Fatal(err)
	}
	if state.Status != renderplan.RenderVariantStatusReady && state.Status != renderplan.RenderVariantStatusReview {
		t.Fatalf("status = %q, want ready or review after ingesting attempt 16:9", state.Status)
	}
	if state.EditDocumentKey == "" {
		t.Fatal("durable edit-document key missing after attempt-file ingest")
	}
	if _, ok := store.files[state.EditDocumentKey]; !ok {
		t.Fatalf("durable edit-document.json missing at %s", state.EditDocumentKey)
	}
}

func TestHandleRenderVariantDoesNotPublishNineBySixteenFullDemo(t *testing.T) {
	id, store, worker := recordedFullDemoRender(t)
	if err := putCaptureKind(store, id, true); err != nil {
		t.Fatal(err)
	}
	worker.runner = &fakeRunner{fn: func(_ context.Context, _ string, args ...string) ([]byte, error) {
		outDir := argValue(args, "--out")
		publishDir := argValue(args, "--publish-dir")
		videoPath := filepath.Join(publishDir, "demo-compilation.mp4")
		if err := os.MkdirAll(publishDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(videoPath, []byte("portrait"), 0o644); err != nil {
			t.Fatal(err)
		}
		rendered := editor.Result{
			Preset:     editor.PresetGameplayPOV60,
			OutputDir:  outDir,
			PublishDir: publishDir,
			Shorts: []editor.ShortResult{{
				SegmentID:    "demo-compilation",
				Output:       videoPath,
				PublishPath:  videoPath,
				OutputFormat: editor.OutputFormatShort9x16,
				PublishArtifact: recording.RecordingArtifact{
					Width:  1080,
					Height: 1920,
					Path:   videoPath,
				},
			}},
		}
		if err := writeJSONFile(filepath.Join(outDir, "shorts-result.json"), rendered); err != nil {
			t.Fatal(err)
		}
		return []byte("rendered"), nil
	}}

	task, err := tasks.NewRenderVariantTask(id, editor.PresetGameplayPOV60, "", 0, nil, renderplan.DefaultEditRequest())
	if err != nil {
		t.Fatal(err)
	}
	if err := worker.HandleRenderVariant(context.Background(), task); err == nil {
		t.Fatal("HandleRenderVariant error = nil, want 9:16 Full Demo rejected")
	}
	var state renderplan.RenderVariantState
	if err := json.Unmarshal(store.files[mustRenderVariantStatusKey(t, id, editor.PresetGameplayPOV60)], &state); err != nil {
		t.Fatal(err)
	}
	if state.Status != renderplan.RenderVariantStatusFailed {
		t.Fatalf("status = %q, want failed so 9:16 is not published", state.Status)
	}
	if _, ok := store.files[mustRenderVariantEditDocumentKey(t, id, editor.PresetGameplayPOV60)]; ok {
		t.Fatal("9:16 Full Demo was uploaded")
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
