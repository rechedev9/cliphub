package workers

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"slices"
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

// labRenderFixture renders a two-segment selection of a recorded job through a
// fake editor and returns the arguments the editor received.
func labRenderFixture(t *testing.T) (*RenderWorker, *fakeStorage, uuid.UUID, []string) {
	t.Helper()
	repo := newFakeRepo()
	store := newFakeStorage()
	id := uuid.New()
	plan := multiSegmentKillPlan("seg-001", "seg-002", "seg-003")
	repo.jobs[id] = &job.Job{ID: id, Status: job.StatusRecorded, Rules: rules.Default(), KillPlan: &plan}
	recordingPlan, err := recording.NewPlanFromKillPlan(plan, "demo.dem", "out", recording.DefaultStreamConfig())
	if err != nil {
		t.Fatal(err)
	}
	rec := recording.RecordingResult{Plan: recordingPlan, CaptureMode: recording.CaptureModeReal, CaptureVerified: true}
	for _, sid := range []string{"seg-001", "seg-002", "seg-003"} {
		rec.Artifacts = append(rec.Artifacts, recording.RecordingArtifact{SegmentID: sid, Role: "segment", Type: "video", Path: sid + ".mp4", SizeBytes: 4})
		if err := store.Put(mustSegmentClipKey(t, id, sid), bytes.NewReader([]byte("clip-"+sid))); err != nil {
			t.Fatal(err)
		}
	}
	rec.CaptureInputFingerprint, _ = recording.CaptureInputFingerprint(rec.Plan)
	if err := putRecordingResult(store, id, rec); err != nil {
		t.Fatal(err)
	}
	selection := []string{"seg-003", "seg-001"}
	var seenArgs []string
	runner := &fakeRunner{fn: func(_ context.Context, _ string, args ...string) ([]byte, error) {
		seenArgs = args
		outDir, publishDir := argValue(args, "--out"), argValue(args, "--publish-dir")
		write := func(path, body string) {
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		var shorts []editor.ShortResult
		for _, sid := range selection {
			short := editor.ShortResult{
				SegmentID:     sid,
				Output:        filepath.Join(publishDir, sid+".mp4"),
				PublishPath:   filepath.Join(publishDir, sid+".mp4"),
				CoverPath:     filepath.Join(publishDir, sid+".cover.jpg"),
				CaptionPath:   filepath.Join(publishDir, sid+".caption.txt"),
				RenderLogPath: filepath.Join(outDir, "logs", sid+"-render.log"),
			}
			for _, path := range []string{short.Output, short.CoverPath, short.CaptionPath, short.RenderLogPath} {
				write(path, "media")
			}
			shorts = append(shorts, short)
		}
		write(filepath.Join(outDir, "edit-manifest.json"), segmentIDOnlyDocJSON(t, "shorts", selection))
		write(filepath.Join(publishDir, "pack-manifest.json"), segmentIDOnlyDocJSON(t, "items", selection))
		write(filepath.Join(publishDir, "index.html"), `<html></html>`)
		write(filepath.Join(publishDir, "publish-summary.md"), `summary`)
		rendered := editor.Result{Preset: editor.PresetViral60Clean, OutputDir: outDir, PublishDir: publishDir, GalleryPath: filepath.Join(publishDir, "index.html"), SummaryPath: filepath.Join(publishDir, "publish-summary.md"), Shorts: shorts}
		if err := writeJSONFile(filepath.Join(outDir, "shorts-result.json"), rendered); err != nil {
			t.Fatal(err)
		}
		return []byte("rendered"), nil
	}}
	w := NewRenderWorker(repo, store, RenderWorkerConfig{WorkDir: t.TempDir(), EditorPath: "zv-editor", FFmpegPath: "ffmpeg"})
	w.runner = runner
	task, err := tasks.NewRenderVariantTask(id, editor.PresetViral60Clean, "", 0, nil, renderplan.DefaultEditRequest(), selection)
	if err != nil {
		t.Fatal(err)
	}
	if err := w.HandleRenderVariant(context.Background(), task); err != nil {
		t.Fatalf("HandleRenderVariant error = %v", err)
	}
	return w, store, id, seenArgs
}

func TestLabBundleReplaysTheLastRenderInputsWithoutTouchingRenderState(t *testing.T) {
	w, store, id, renderArgs := labRenderFixture(t)
	statusKey := mustRenderVariantStatusKey(t, id, editor.PresetViral60Clean)
	stateBefore := slices.Clone(store.files[statusKey])
	renderWorkDir := filepath.Dir(argValue(renderArgs, "--out"))

	dir := filepath.Join(t.TempDir(), "bundle")
	bundle, err := w.PrepareLabBundle(context.Background(), id, editor.PresetViral60Clean, dir)
	if err != nil {
		t.Fatalf("PrepareLabBundle error = %v", err)
	}

	// The render stage directory is gone; with that prefix mapped to the
	// bundle, the editor must receive exactly the arguments the render ran,
	// except the Studio progress file the worker adds when it runs the editor.
	var want []string
	for i := 0; i < len(renderArgs); i++ {
		if renderArgs[i] == "--progress-out" {
			i++
			continue
		}
		want = append(want, strings.ReplaceAll(renderArgs[i], renderWorkDir, bundle.Dir))
	}
	if !slices.Equal(bundle.Args, want) {
		t.Fatalf("bundle args differ from the render's editor args\n got %q\nwant %q", bundle.Args, want)
	}
	var onDisk LabBundle
	if err := readJSONFile(filepath.Join(dir, editor.LabBundleFile), &onDisk); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(onDisk.Args, bundle.Args) {
		t.Fatalf("%s args = %q, want %q", editor.LabBundleFile, onDisk.Args, bundle.Args)
	}
	var localized recording.RecordingResult
	if err := readJSONFile(argValue(bundle.Args, "--recording-result"), &localized); err != nil {
		t.Fatal(err)
	}
	if got := recording.EditorialSegmentIDs(localized); !slices.Equal(got, []string{"seg-003", "seg-001"}) {
		t.Fatalf("bundled recording compiles %v, want the rendered selection", got)
	}
	for _, artifact := range localized.Artifacts {
		body, err := os.ReadFile(artifact.Path)
		if err != nil || string(body) != "clip-"+artifact.SegmentID {
			t.Fatalf("bundled clip %s = %q, %v; want the stored clip", artifact.Path, body, err)
		}
	}
	if !bytes.Equal(store.files[statusKey], stateBefore) {
		t.Fatalf("render state changed while preparing a lab bundle")
	}
}

func TestLabBundleNeedsACommittedRender(t *testing.T) {
	w, _, id, _ := labRenderFixture(t)
	_, err := w.PrepareLabBundle(context.Background(), id, editor.PresetGameplayPOV60, t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "render it once first") {
		t.Fatalf("PrepareLabBundle for an unrendered variant error = %v, want a render-first error", err)
	}
}
