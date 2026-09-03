package workers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"

	"github.com/rechedev9/cliphub/internal/editor"
	"github.com/rechedev9/cliphub/internal/job"
	"github.com/rechedev9/cliphub/internal/killplan"
	"github.com/rechedev9/cliphub/internal/obs"
	"github.com/rechedev9/cliphub/internal/recording"
	"github.com/rechedev9/cliphub/internal/renderplan"
	"github.com/rechedev9/cliphub/internal/rules"
	"github.com/rechedev9/cliphub/internal/tasks"
)

// multiSegmentKillPlan builds a kill plan with one segment per id, so tests can
// assert that the recorder is only handed the segments a reel selected.
func multiSegmentKillPlan(ids ...string) killplan.Plan {
	plan := minimalKillPlan()
	plan.Segments = nil
	for i, id := range ids {
		start := 64 * (i + 1)
		plan.Segments = append(plan.Segments, killplan.Segment{
			ID:        id,
			Round:     i + 1,
			TickStart: start,
			TickEnd:   start + 64,
		})
	}
	return plan
}

// planRecorderRunner mimics zv-recorder: it records exactly the segments present
// in the kill plan it is handed (writing one clip per segment plus the result),
// and records the segment ids it saw into seen so the test can assert scoping.
func planRecorderRunner(t *testing.T, seen *[]string) *fakeRunner {
	t.Helper()
	return &fakeRunner{fn: func(_ context.Context, _ string, args ...string) ([]byte, error) {
		outDir := argValue(args, "--out")
		if err := os.MkdirAll(outDir, 0o755); err != nil {
			t.Fatal(err)
		}
		var plan killplan.Plan
		if err := readJSONFile(argValue(args, "--killplan"), &plan); err != nil {
			t.Fatalf("read killplan: %v", err)
		}
		scriptPath := filepath.Join(outDir, "recording.js")
		if err := os.WriteFile(scriptPath, []byte("script"), 0o644); err != nil {
			t.Fatal(err)
		}
		stream := recording.DefaultStreamConfig()
		stream.HUDMode = recording.HUDMode(argValue(args, "--hud"))
		stream.PortraitSafeKillfeed = hasArg(args, "--portrait-safe-killfeed")
		recordingPlan, err := recording.NewPlanFromKillPlan(plan, argValue(args, "--demo"), outDir, stream)
		if err != nil {
			t.Fatalf("build recording plan: %v", err)
		}
		result := recording.RecordingResult{
			Plan:            recordingPlan,
			Script:          scriptPath,
			CaptureMode:     recording.CaptureModeReal,
			CaptureVerified: true,
		}
		result.CaptureInputFingerprint, _ = recording.CaptureInputFingerprint(result.Plan)
		for _, s := range plan.Segments {
			if seen != nil {
				*seen = append(*seen, s.ID)
			}
			segPath := filepath.Join(outDir, "segments", s.ID+".mp4")
			if err := os.MkdirAll(filepath.Dir(segPath), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(segPath, []byte("clip"), 0o644); err != nil {
				t.Fatal(err)
			}
			result.Artifacts = append(result.Artifacts, recording.RecordingArtifact{
				SegmentID: s.ID,
				Role:      "segment",
				Type:      "video",
				Path:      segPath,
				SizeBytes: 4,
			})
		}
		if err := writeJSONFile(filepath.Join(outDir, "recording-result.json"), result); err != nil {
			t.Fatal(err)
		}
		return []byte("recorded"), nil
	}}
}

func storedRecordingResult(t *testing.T, store *fakeStorage, id uuid.UUID) recording.RecordingResult {
	t.Helper()
	result, err := decodeStoredRecordingResult(store, id)
	if err != nil {
		t.Fatalf("decode stored recording result: %v", err)
	}
	return result
}

func TestMergeRecordingPerformancePreservesEveryPhysicalRun(t *testing.T) {
	previous := &recording.RecordingPerformance{
		Version: 1,
		Runs: []recording.RecordingRunPerformance{{
			CaptureSegmentIDs:   []string{"seg-001"},
			BeforeResultWriteMS: 100,
		}},
	}
	next := &recording.RecordingPerformance{
		Version: 1,
		Runs: []recording.RecordingRunPerformance{{
			CaptureSegmentIDs:   []string{"seg-002"},
			BeforeResultWriteMS: 200,
		}},
	}

	got := mergeRecordingPerformance(previous, next)

	if got == nil || got.Version != 1 || len(got.Runs) != 2 {
		t.Fatalf("merged performance = %#v", got)
	}
	if got.Runs[0].CaptureSegmentIDs[0] != "seg-001" || got.Runs[1].CaptureSegmentIDs[0] != "seg-002" {
		t.Fatalf("merged runs = %#v", got.Runs)
	}
	incompatible := mergeRecordingPerformance(previous, &recording.RecordingPerformance{
		Version: 2,
		Runs:    []recording.RecordingRunPerformance{{CaptureSegmentIDs: []string{"new"}}},
	})
	if incompatible.Version != 2 || len(incompatible.Runs) != 1 || incompatible.Runs[0].CaptureSegmentIDs[0] != "new" {
		t.Fatalf("incompatible versions merged = %#v", incompatible)
	}
	if mergeRecordingPerformance(nil, nil) != nil {
		t.Fatal("nil legacy performance should remain nil")
	}
}

// segmentIDOnly is a minimal edit/pack manifest entry: every downstream check
// in this render path (validateRenderManifestSegmentIDs) only reads segment
// ids, so the fake editor's fixtures need not populate the rest of the shape.
type segmentIDOnly struct {
	SegmentID string `json:"segment_id"`
}

func segmentIDOnlyDocJSON(t *testing.T, listKey string, ids []string) string {
	t.Helper()
	docs := make([]segmentIDOnly, len(ids))
	for i, id := range ids {
		docs[i] = segmentIDOnly{SegmentID: id}
	}
	b, err := json.Marshal(map[string][]segmentIDOnly{listKey: docs})
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// TestRenderWorkerNarrowsToSelectedSegments is the render-side regression for
// the bug this task fixes: a Studio generate for one (or a few) reel segments
// must render exactly that selection, not every segment the job has ever
// recorded. It covers both ends of CLAUDE.md's requirement: a single selected
// segment renders its own short with no --compile-segments, and two or more
// selected segments compile only those, in the requested order.
func TestRenderWorkerNarrowsToSelectedSegments(t *testing.T) {
	cases := []struct {
		name       string
		segmentIDs []string
		// wantCapture is the narrowed plan in capture (tick) order, which the
		// recording plan contract requires regardless of the requested order.
		wantCapture    []string
		wantCompileArg bool
	}{
		{name: "single segment renders its own short", segmentIDs: []string{"seg-002"}, wantCapture: []string{"seg-002"}, wantCompileArg: false},
		{name: "multi segment compiles only the selection in the requested order", segmentIDs: []string{"seg-003", "seg-001"}, wantCapture: []string{"seg-001", "seg-003"}, wantCompileArg: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := newFakeRepo()
			store := newFakeStorage()
			id := uuid.New()
			plan := multiSegmentKillPlan("seg-001", "seg-002", "seg-003")
			repo.jobs[id] = &job.Job{ID: id, Status: job.StatusRecorded, Rules: rules.Default(), KillPlan: &plan}

			// A job-level recording result that accumulated all three segments
			// across separate reels (mergeRecordingResults), mirroring the bug
			// report evidence: capture-selection.json names one segment, but the
			// durable recording result names every segment ever recorded.
			recordingPlan, err := recording.NewPlanFromKillPlan(plan, "demo.dem", "out", recording.DefaultStreamConfig())
			if err != nil {
				t.Fatal(err)
			}
			rec := recording.RecordingResult{
				Plan:            recordingPlan,
				CaptureMode:     recording.CaptureModeReal,
				CaptureVerified: true,
			}
			for _, sid := range []string{"seg-001", "seg-002", "seg-003"} {
				rec.Artifacts = append(rec.Artifacts, recording.RecordingArtifact{
					SegmentID: sid, Role: "segment", Type: "video", Path: sid + ".mp4", SizeBytes: 4,
				})
				if err := store.Put(mustSegmentClipKey(t, id, sid), bytes.NewReader([]byte("clip-"+sid))); err != nil {
					t.Fatal(err)
				}
			}
			rec.CaptureInputFingerprint, _ = recording.CaptureInputFingerprint(rec.Plan)
			if err := putRecordingResult(store, id, rec); err != nil {
				t.Fatal(err)
			}

			var seenArgs []string
			var seenRecordingResult recording.RecordingResult
			var seenEditDocument renderplan.EditDocument
			runner := &fakeRunner{fn: func(_ context.Context, _ string, args ...string) ([]byte, error) {
				seenArgs = args
				if err := readJSONFile(argValue(args, "--recording-result"), &seenRecordingResult); err != nil {
					t.Fatal(err)
				}
				outDir := argValue(args, "--out")
				publishDir := argValue(args, "--publish-dir")
				// The render worker writes edit-document.json before invoking the
				// editor, so it is already on disk here (and outDir is torn down
				// once render() returns).
				if err := readJSONFile(filepath.Join(outDir, "edit-document.json"), &seenEditDocument); err != nil {
					t.Fatal(err)
				}

				var shorts []editor.ShortResult
				for _, sid := range tc.segmentIDs {
					videoPath := filepath.Join(publishDir, sid+".mp4")
					coverPath := filepath.Join(publishDir, sid+".cover.jpg")
					captionPath := filepath.Join(publishDir, sid+".caption.txt")
					logPath := filepath.Join(outDir, "logs", sid+"-render.log")
					for _, file := range []struct{ path, body string }{
						{videoPath, "video"}, {coverPath, "cover"}, {captionPath, "caption"}, {logPath, "log"},
					} {
						if err := os.MkdirAll(filepath.Dir(file.path), 0o755); err != nil {
							t.Fatal(err)
						}
						if err := os.WriteFile(file.path, []byte(file.body), 0o644); err != nil {
							t.Fatal(err)
						}
					}
					shorts = append(shorts, editor.ShortResult{
						SegmentID:     sid,
						Output:        videoPath,
						PublishPath:   videoPath,
						CoverPath:     coverPath,
						CaptionPath:   captionPath,
						RenderLogPath: logPath,
					})
				}
				for _, file := range []struct{ path, body string }{
					{filepath.Join(outDir, "edit-manifest.json"), segmentIDOnlyDocJSON(t, "shorts", tc.segmentIDs)},
					{filepath.Join(publishDir, "pack-manifest.json"), segmentIDOnlyDocJSON(t, "items", tc.segmentIDs)},
					{filepath.Join(publishDir, "index.html"), `<html></html>`},
					{filepath.Join(publishDir, "publish-summary.md"), `summary`},
				} {
					if err := os.MkdirAll(filepath.Dir(file.path), 0o755); err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(file.path, []byte(file.body), 0o644); err != nil {
						t.Fatal(err)
					}
				}
				rendered := editor.Result{
					Preset:      editor.PresetViral60Clean,
					OutputDir:   outDir,
					PublishDir:  publishDir,
					GalleryPath: filepath.Join(publishDir, "index.html"),
					SummaryPath: filepath.Join(publishDir, "publish-summary.md"),
					Shorts:      shorts,
				}
				if err := writeJSONFile(filepath.Join(outDir, "shorts-result.json"), rendered); err != nil {
					t.Fatal(err)
				}
				return []byte("rendered"), nil
			}}

			w := NewRenderWorker(repo, store, RenderWorkerConfig{
				WorkDir:    t.TempDir(),
				EditorPath: "zv-editor",
				FFmpegPath: "ffmpeg",
			})
			w.runner = runner

			task, err := tasks.NewRenderVariantTask(id, editor.PresetViral60Clean, "", 0, nil, renderplan.DefaultEditRequest(), tc.segmentIDs)
			if err != nil {
				t.Fatal(err)
			}
			if err := w.HandleRenderVariant(context.Background(), task); err != nil {
				t.Fatalf("HandleRenderVariant error = %v", err)
			}

			// The recording-result.json handed to zv-editor must list exactly the
			// requested segments — never every segment the job has ever recorded.
			// Capture order stays chronological (the plan contract); the requested
			// order travels as the editorial order the editor compiles.
			if got := recording.SegmentIDs(seenRecordingResult); !slices.Equal(got, tc.wantCapture) {
				t.Fatalf("recording-result handed to editor lists capture segments %v, want %v", got, tc.wantCapture)
			}
			if got := recording.EditorialSegmentIDs(seenRecordingResult); !slices.Equal(got, tc.segmentIDs) {
				t.Fatalf("recording-result handed to editor compiles segments %v, want requested order %v", got, tc.segmentIDs)
			}
			if err := seenRecordingResult.Plan.Validate(); err != nil {
				t.Fatalf("narrowed recording plan handed to editor is invalid: %v", err)
			}
			if len(seenRecordingResult.Artifacts) != len(tc.segmentIDs) {
				t.Fatalf("recording-result artifacts = %#v, want exactly the %d requested segment(s)", seenRecordingResult.Artifacts, len(tc.segmentIDs))
			}

			// The edit document's selection must match the request too, not the
			// job's full recorded history.
			if !slices.Equal(seenEditDocument.Selection.SegmentIDs, tc.segmentIDs) {
				t.Fatalf("edit document selection = %v, want %v", seenEditDocument.Selection.SegmentIDs, tc.segmentIDs)
			}

			if hasArg(seenArgs, "--compile-segments") != tc.wantCompileArg {
				t.Fatalf("editor args --compile-segments present = %v, want %v: %#v", hasArg(seenArgs, "--compile-segments"), tc.wantCompileArg, seenArgs)
			}
			if tc.wantCompileArg {
				if got, want := argValue(seenArgs, "--segments"), strings.Join(tc.segmentIDs, ","); got != want {
					t.Fatalf("--segments = %q, want %q", got, want)
				}
			}

			var state renderplan.RenderVariantState
			if err := json.Unmarshal(store.files[mustRenderVariantStatusKey(t, id, editor.PresetViral60Clean)], &state); err != nil {
				t.Fatal(err)
			}
			if got, want := state.Status, renderplan.RenderVariantStatusReady; got != want {
				t.Fatalf("render state = %q, want %q", got, want)
			}
		})
	}
}

func TestRenderWorkerFailsRenderWhenSelectedSegmentIsNotRecorded(t *testing.T) {
	repo := newFakeRepo()
	store := newFakeStorage()
	id := uuid.New()
	plan := multiSegmentKillPlan("seg-001")
	repo.jobs[id] = &job.Job{ID: id, Status: job.StatusRecorded, Rules: rules.Default(), KillPlan: &plan}

	recordingPlan, err := recording.NewPlanFromKillPlan(plan, "demo.dem", "out", recording.DefaultStreamConfig())
	if err != nil {
		t.Fatal(err)
	}
	rec := recording.RecordingResult{
		Plan:            recordingPlan,
		CaptureMode:     recording.CaptureModeReal,
		CaptureVerified: true,
		Artifacts: []recording.RecordingArtifact{{
			SegmentID: "seg-001", Role: "segment", Type: "video", Path: "seg-001.mp4", SizeBytes: 4,
		}},
	}
	rec.CaptureInputFingerprint, _ = recording.CaptureInputFingerprint(rec.Plan)
	if err := putRecordingResult(store, id, rec); err != nil {
		t.Fatal(err)
	}
	if err := store.Put(mustSegmentClipKey(t, id, "seg-001"), bytes.NewReader([]byte("clip"))); err != nil {
		t.Fatal(err)
	}

	runner := &fakeRunner{fn: func(context.Context, string, ...string) ([]byte, error) {
		t.Fatal("editor must not run when a requested segment was never recorded")
		return nil, nil
	}}
	w := NewRenderWorker(repo, store, RenderWorkerConfig{
		WorkDir:    t.TempDir(),
		EditorPath: "zv-editor",
		FFmpegPath: "ffmpeg",
	})
	w.runner = runner

	task, err := tasks.NewRenderVariantTask(id, editor.PresetViral60Clean, "", 0, nil, renderplan.DefaultEditRequest(), []string{"seg-999"})
	if err != nil {
		t.Fatal(err)
	}
	err = w.HandleRenderVariant(context.Background(), task)
	if err == nil {
		t.Fatal("HandleRenderVariant error = nil, want a failure naming the missing segment")
	}
	if !strings.Contains(err.Error(), "seg-999") {
		t.Fatalf("error = %q, want it to name the missing segment seg-999", err.Error())
	}
	// The stored capture cannot serve this selection, so the failure must be
	// the stable re-record class, land in the durable render state Studio
	// polls (not stay Queued forever), and hit the obs journal exactly once.
	if !recording.IsNotReusableMessage(err.Error()) {
		t.Fatalf("error = %q, want the %q prefix so Studio re-records", err.Error(), recording.NotReusablePrefix)
	}
	var state renderplan.RenderVariantState
	if err := json.Unmarshal(store.files[mustRenderVariantStatusKey(t, id, editor.PresetViral60Clean)], &state); err != nil {
		t.Fatalf("render state missing or unreadable after failed narrowing: %v", err)
	}
	if state.Status != renderplan.RenderVariantStatusFailed {
		t.Fatalf("render state = %q, want %q", state.Status, renderplan.RenderVariantStatusFailed)
	}
	if !strings.Contains(state.Error, "seg-999") || !recording.IsNotReusableMessage(state.Error) {
		t.Fatalf("render state error = %q, want a not-reusable failure naming seg-999", state.Error)
	}
	journal := obs.Default()
	if journal == nil {
		t.Fatal("obs.Default is nil")
	}
	found, err := journal.SelectErrors(id.String(), obs.ClassRecordingNotReusable)
	if err != nil {
		t.Fatalf("SelectErrors: %v", err)
	}
	if len(found) != 1 || found[0].Task != tasks.TypeRenderVariant || found[0].Stage != obs.StageWorker {
		t.Fatalf("obs journal for %s/%s = %#v, want exactly one %s event", id, obs.ClassRecordingNotReusable, found, tasks.TypeRenderVariant)
	}
}

func TestRecordWorkerFiltersKillPlanToSelectedSegment(t *testing.T) {
	repo := newFakeRepo()
	store := newFakeStorage()
	id := uuid.New()
	plan := multiSegmentKillPlan("seg-001", "seg-002", "seg-003")
	repo.jobs[id] = &job.Job{ID: id, Status: job.StatusParsed, DemoPath: "demos/test.dem", Rules: rules.Default(), KillPlan: &plan}
	_ = store.Put("demos/test.dem", bytes.NewReader([]byte("demo")))

	var seen []string
	w := newRecordWorkerForTest(repo, store, t)
	w.runner = planRecorderRunner(t, &seen)

	if err := w.HandleRecordDemo(context.Background(), recordTaskFor(t, id, []string{"seg-002"})); err != nil {
		t.Fatalf("HandleRecordDemo error = %v", err)
	}

	if len(seen) != 1 || seen[0] != "seg-002" {
		t.Fatalf("recorder saw segments %v, want [seg-002] only", seen)
	}
	if _, ok := store.files[mustSegmentClipKey(t, id, "seg-002")]; !ok {
		t.Fatal("storage missing seg-002 clip")
	}
	for _, unwanted := range []string{"seg-001", "seg-003"} {
		if _, ok := store.files[mustSegmentClipKey(t, id, unwanted)]; ok {
			t.Fatalf("storage unexpectedly has %s clip", unwanted)
		}
	}
}

func TestRecordWorkerRecordsAllSegmentsWhenNoneSelected(t *testing.T) {
	repo := newFakeRepo()
	store := newFakeStorage()
	id := uuid.New()
	plan := multiSegmentKillPlan("seg-001", "seg-002", "seg-003")
	repo.jobs[id] = &job.Job{ID: id, Status: job.StatusParsed, DemoPath: "demos/test.dem", Rules: rules.Default(), KillPlan: &plan}
	_ = store.Put("demos/test.dem", bytes.NewReader([]byte("demo")))

	var seen []string
	w := newRecordWorkerForTest(repo, store, t)
	w.runner = planRecorderRunner(t, &seen)

	if err := w.HandleRecordDemo(context.Background(), recordTaskFor(t, id, nil)); err != nil {
		t.Fatalf("HandleRecordDemo error = %v", err)
	}
	if len(seen) != 3 {
		t.Fatalf("recorder saw %v, want all 3 segments", seen)
	}
}

func TestRecordWorkerRerecordsAndAccumulatesAcrossReels(t *testing.T) {
	repo := newFakeRepo()
	store := newFakeStorage()
	id := uuid.New()
	plan := multiSegmentKillPlan("seg-001", "seg-002", "seg-003")
	repo.jobs[id] = &job.Job{ID: id, Status: job.StatusParsed, DemoPath: "demos/test.dem", Rules: rules.Default(), KillPlan: &plan}
	_ = store.Put("demos/test.dem", bytes.NewReader([]byte("demo")))

	var seen []string
	w := newRecordWorkerForTest(repo, store, t)
	w.runner = planRecorderRunner(t, &seen)

	// First reel records seg-001.
	if err := w.HandleRecordDemo(context.Background(), recordTaskFor(t, id, []string{"seg-001"})); err != nil {
		t.Fatalf("first record error = %v", err)
	}
	// Second reel for a different clip must re-run the recorder, not skip.
	if err := w.HandleRecordDemo(context.Background(), recordTaskFor(t, id, []string{"seg-002"})); err != nil {
		t.Fatalf("second record error = %v", err)
	}
	if want := []string{"seg-001", "seg-002"}; len(seen) != 2 || seen[0] != want[0] || seen[1] != want[1] {
		t.Fatalf("recorder saw %v across reels, want %v (re-record, not skip)", seen, want)
	}

	// The job-level result must accumulate both segments so the render can find
	// either clip; without the merge the second run would clobber the first.
	result := storedRecordingResult(t, store, id)
	got := map[string]bool{}
	for _, sid := range recording.SegmentIDs(result) {
		got[sid] = true
	}
	if !got["seg-001"] || !got["seg-002"] {
		t.Fatalf("stored result segments = %v, want both seg-001 and seg-002", recording.SegmentIDs(result))
	}
	for _, sid := range []string{"seg-001", "seg-002"} {
		if _, ok := store.files[mustSegmentClipKey(t, id, sid)]; !ok {
			t.Fatalf("storage missing %s clip after second reel", sid)
		}
	}
}

// TestRecordWorkerCapturesOnlyMissingSegmentsWithinReelSelection is the B1
// regression: a reel selection that partially overlaps an already-recorded,
// profile-compatible capture must send the recorder only the segments still
// missing a clip, not the whole selection. mergeRecordingResults folds the
// reused segment back into the durable result untouched.
func TestRecordWorkerCapturesOnlyMissingSegmentsWithinReelSelection(t *testing.T) {
	repo := newFakeRepo()
	store := newFakeStorage()
	id := uuid.New()
	plan := multiSegmentKillPlan("seg-001", "seg-002", "seg-003")
	repo.jobs[id] = &job.Job{ID: id, Status: job.StatusParsed, DemoPath: "demos/test.dem", Rules: rules.Default(), KillPlan: &plan}
	_ = store.Put("demos/test.dem", bytes.NewReader([]byte("demo")))

	var seen []string
	w := newRecordWorkerForTest(repo, store, t)
	w.runner = planRecorderRunner(t, &seen)

	// A first reel already captured seg-001.
	if err := w.HandleRecordDemo(context.Background(), recordTaskFor(t, id, []string{"seg-001"})); err != nil {
		t.Fatalf("first record error = %v", err)
	}
	if len(seen) != 1 || seen[0] != "seg-001" {
		t.Fatalf("recorder saw %v after first reel, want [seg-001]", seen)
	}

	// A later reel selects all three segments. Only seg-002 and seg-003 lack a
	// compatible durable clip, so the recorder must be handed only those two.
	seen = nil
	if err := w.HandleRecordDemo(context.Background(), recordTaskFor(t, id, []string{"seg-001", "seg-002", "seg-003"})); err != nil {
		t.Fatalf("second record error = %v", err)
	}
	if want := []string{"seg-002", "seg-003"}; len(seen) != 2 || seen[0] != want[0] || seen[1] != want[1] {
		t.Fatalf("recorder saw %v for the partially-covered selection, want %v (only the missing subset)", seen, want)
	}

	result := storedRecordingResult(t, store, id)
	if got := recording.SegmentIDs(result); len(got) != 3 || got[0] != "seg-001" || got[1] != "seg-002" || got[2] != "seg-003" {
		t.Fatalf("stored result segments = %v, want all three merged", got)
	}
	if err := result.Plan.Validate(); err != nil {
		t.Fatalf("merged plan is invalid: %v", err)
	}
	for _, sid := range []string{"seg-001", "seg-002", "seg-003"} {
		if _, ok := store.files[mustSegmentClipKey(t, id, sid)]; !ok {
			t.Fatalf("storage missing %s clip after incremental capture", sid)
		}
	}
}

func TestRecordWorkerSkipsWhenSelectedSegmentAlreadyRecorded(t *testing.T) {
	repo := newFakeRepo()
	store := newFakeStorage()
	id := uuid.New()
	plan := multiSegmentKillPlan("seg-001", "seg-002")
	repo.jobs[id] = &job.Job{ID: id, Status: job.StatusParsed, DemoPath: "demos/test.dem", Rules: rules.Default(), KillPlan: &plan}
	_ = store.Put("demos/test.dem", bytes.NewReader([]byte("demo")))

	var seen []string
	w := newRecordWorkerForTest(repo, store, t)
	w.runner = planRecorderRunner(t, &seen)

	for i := range 2 {
		if err := w.HandleRecordDemo(context.Background(), recordTaskFor(t, id, []string{"seg-001"})); err != nil {
			t.Fatalf("record %d error = %v", i, err)
		}
	}
	if len(seen) != 1 {
		t.Fatalf("recorder ran %d times for the same segment, want 1 (idempotent skip)", len(seen))
	}
}

func TestRecordWorkerInvalidatesGameplayCaptureWhenPortraitSafetyChanges(t *testing.T) {
	repo := newFakeRepo()
	store := newFakeStorage()
	id := uuid.New()
	plan := multiSegmentKillPlan("seg-001", "seg-002")
	repo.jobs[id] = &job.Job{ID: id, Status: job.StatusParsed, DemoPath: "demos/test.dem", Rules: rules.Default(), KillPlan: &plan}
	_ = store.Put("demos/test.dem", bytes.NewReader([]byte("demo")))

	var seen []string
	w := newRecordWorkerForTest(repo, store, t)
	w.runner = planRecorderRunner(t, &seen)

	// The old Full HUD profile records one segment without a portrait-safe native
	// killfeed. A later portrait-safe Full HUD request must not reuse or merge it.
	if err := w.HandleRecordDemo(context.Background(), recordTaskWithCaptureProfile(t, id, "gameplay", []string{"seg-001"}, false)); err != nil {
		t.Fatalf("unsafe record error = %v", err)
	}
	if err := w.HandleRecordDemo(context.Background(), recordTaskWithCaptureProfile(t, id, "gameplay", []string{"seg-002"}, true)); err != nil {
		t.Fatalf("portrait-safe record error = %v", err)
	}
	if len(seen) != 2 {
		t.Fatalf("recorder runs = %d, want 2 after portrait profile changed", len(seen))
	}
	result := storedRecordingResult(t, store, id)
	if got := recording.SegmentIDs(result); len(got) != 1 || got[0] != "seg-002" {
		t.Fatalf("segments after profile change = %v, want only [seg-002]", got)
	}
	if !result.Plan.Stream.PortraitSafeKillfeed {
		t.Fatal("stored profile is not portrait-safe")
	}

	// Re-recording seg-001 under the new profile may now accumulate with seg-002;
	// a fourth identical request proves the resulting profile remains idempotent.
	portraitTask := func(segmentID string) *asynq.Task {
		return recordTaskWithCaptureProfile(t, id, "gameplay", []string{segmentID}, true)
	}
	if err := w.HandleRecordDemo(context.Background(), portraitTask("seg-001")); err != nil {
		t.Fatalf("portrait-safe backfill error = %v", err)
	}
	if err := w.HandleRecordDemo(context.Background(), portraitTask("seg-001")); err != nil {
		t.Fatalf("portrait-safe retry error = %v", err)
	}
	if len(seen) != 3 {
		t.Fatalf("recorder runs = %d, want 3 after identical retry skips", len(seen))
	}
	result = storedRecordingResult(t, store, id)
	if got := recording.SegmentIDs(result); len(got) != 2 || got[0] != "seg-001" || got[1] != "seg-002" {
		t.Fatalf("segments after compatible merge = %v, want [seg-001 seg-002]", got)
	}
	if err := result.Plan.Validate(); err != nil {
		t.Fatalf("merged plan is invalid: %v", err)
	}
	if got := result.Plan.EditorialSegmentIDs; len(got) != 2 || got[0] != "seg-001" || got[1] != "seg-002" {
		t.Fatalf("merged editorial order = %v, want [seg-001 seg-002]", got)
	}
}

func TestRecordWorkerFailedReelPreservesPriorReelResult(t *testing.T) {
	repo := newFakeRepo()
	store := newFakeStorage()
	id := uuid.New()
	plan := multiSegmentKillPlan("seg-001", "seg-002")
	repo.jobs[id] = &job.Job{ID: id, Status: job.StatusParsed, DemoPath: "demos/test.dem", Rules: rules.Default(), KillPlan: &plan}
	_ = store.Put("demos/test.dem", bytes.NewReader([]byte("demo")))

	w := newRecordWorkerForTest(repo, store, t)

	// First reel records seg-001 successfully.
	w.runner = planRecorderRunner(t, nil)
	if err := w.HandleRecordDemo(context.Background(), recordTaskFor(t, id, []string{"seg-001"})); err != nil {
		t.Fatalf("first record error = %v", err)
	}

	// Second reel for seg-002 fails inside the recorder.
	w.runner = &fakeRunner{fn: func(_ context.Context, _ string, args ...string) ([]byte, error) {
		outDir := argValue(args, "--out")
		if err := os.MkdirAll(outDir, 0o755); err != nil {
			t.Fatal(err)
		}
		failed := recording.RecordingResult{
			Plan: recording.RecordingPlan{
				DemoPath: "demo.dem", OutputDir: outDir, TargetSteamID64: "76561197960265729",
				TargetAccountID: 1, Tickrate: 64, Stream: recording.DefaultStreamConfig(),
				Segments: []recording.RecordingSegment{{ID: "seg-002", TickStart: 128, TickEnd: 192}},
			},
			Error: "recorder boom",
		}
		if err := writeJSONFile(filepath.Join(outDir, "recording-result.json"), failed); err != nil {
			t.Fatal(err)
		}
		return nil, errors.New("recorder boom")
	}}
	if err := w.HandleRecordDemo(context.Background(), recordTaskFor(t, id, []string{"seg-002"})); err == nil {
		t.Fatal("second record should have failed")
	}

	// The first reel's segment must survive the failed second reel.
	result := storedRecordingResult(t, store, id)
	if result.Error != "" {
		t.Fatalf("stored result Error = %q, want preserved good result", result.Error)
	}
	ids := recording.SegmentIDs(result)
	if len(ids) != 1 || ids[0] != "seg-001" {
		t.Fatalf("stored result segments = %v, want [seg-001] preserved", ids)
	}
	if _, ok := store.files[mustSegmentClipKey(t, id, "seg-001")]; !ok {
		t.Fatal("seg-001 clip lost after failed second reel")
	}
}

func TestRecordWorkerInvalidSuccessfulAttemptPreservesPriorResult(t *testing.T) {
	repo := newFakeRepo()
	store := newFakeStorage()
	id := uuid.New()
	plan := multiSegmentKillPlan("seg-001", "seg-002")
	repo.jobs[id] = &job.Job{ID: id, Status: job.StatusParsed, DemoPath: "demos/test.dem", Rules: rules.Default(), KillPlan: &plan}
	_ = store.Put("demos/test.dem", bytes.NewReader([]byte("demo")))

	w := newRecordWorkerForTest(repo, store, t)
	w.runner = planRecorderRunner(t, nil)
	if err := w.HandleRecordDemo(context.Background(), recordTaskFor(t, id, []string{"seg-001"})); err != nil {
		t.Fatalf("first record error = %v", err)
	}

	w.runner = &fakeRunner{fn: func(_ context.Context, _ string, args ...string) ([]byte, error) {
		outDir := argValue(args, "--out")
		scriptPath := filepath.Join(outDir, "recording.js")
		segmentPath := filepath.Join(outDir, "segments", "seg-002.mp4")
		if err := os.MkdirAll(filepath.Dir(segmentPath), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(scriptPath, []byte("script"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(segmentPath, []byte("clip"), 0o644); err != nil {
			t.Fatal(err)
		}
		result := recordingResultForRunnerArgs(t, args, scriptPath, segmentPath)
		result.Plan.TargetNameInDemo = "different-player"
		result.CaptureInputFingerprint, _ = recording.CaptureInputFingerprint(result.Plan)
		if err := writeJSONFile(filepath.Join(outDir, "recording-result.json"), result); err != nil {
			t.Fatal(err)
		}
		return []byte("recorded"), nil
	}}
	if err := w.HandleRecordDemo(context.Background(), recordTaskFor(t, id, []string{"seg-002"})); err == nil {
		t.Fatal("invalid successful attempt should have failed")
	}

	result := storedRecordingResult(t, store, id)
	if ids := recording.SegmentIDs(result); len(ids) != 1 || ids[0] != "seg-001" {
		t.Fatalf("stored result segments = %v, want prior [seg-001]", ids)
	}
	if _, ok := store.files[mustSegmentClipKey(t, id, "seg-002")]; ok {
		t.Fatal("invalid seg-002 was published before attempt validation")
	}
}

func TestRenderCoversToleratesPlanSegmentWithoutClip(t *testing.T) {
	store := newFakeStorage()
	id := uuid.New()
	// Partial capture: the plan lists seg-001 and seg-002 but only seg-001 has a
	// clip. The editor only renders clip-bearing segments, so coverage must not
	// demand a short for seg-002 (which would loop the render forever).
	rec := recording.RecordingResult{
		Plan:      recording.RecordingPlan{Segments: []recording.RecordingSegment{{ID: "seg-001"}, {ID: "seg-002"}}},
		Artifacts: []recording.RecordingArtifact{{SegmentID: "seg-001", Role: "segment", Type: "video"}},
	}
	if err := putRecordingResult(store, id, rec); err != nil {
		t.Fatal(err)
	}
	covered, err := renderCoversRecordedSegments(store, id, editor.Result{
		Shorts: []editor.ShortResult{{SegmentID: "seg-001"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !covered {
		t.Fatal("a plan segment without a clip must not make render coverage unsatisfiable")
	}
}

func TestRecordingOutputsReadyRejectsCapturesWithoutVerifiedPOVContract(t *testing.T) {
	store := newFakeStorage()
	id := uuid.New()
	stream := recording.DefaultStreamConfig()
	expectedPlan, err := recording.NewPlanFromKillPlan(minimalKillPlan(), "profile.dem", "profile", stream)
	if err != nil {
		t.Fatal(err)
	}
	result := recording.RecordingResult{
		Plan: expectedPlan,
		Artifacts: []recording.RecordingArtifact{{
			SegmentID: "seg-001",
			Role:      "segment",
			Type:      "video",
		}},
	}
	if err := putRecordingResult(store, id, result); err != nil {
		t.Fatal(err)
	}
	if err := store.Put(recording.ScriptArtifactKey(id), bytes.NewReader([]byte("legacy script"))); err != nil {
		t.Fatal(err)
	}
	if err := store.Put(mustSegmentClipKey(t, id, "seg-001"), bytes.NewReader([]byte("legacy clip"))); err != nil {
		t.Fatal(err)
	}

	missing, _, err := recordingOutputsReady(store, id, []string{"seg-001"}, expectedPlan)
	if err != nil {
		t.Fatal(err)
	}
	if len(missing) == 0 {
		t.Fatal("capture without observer-SteamID verification was reused")
	}

	legacy := result
	legacy.Plan.CaptureContract = recording.LegacyCaptureContractVersion
	legacy.Plan.KillPlanSchemaVersion = ""
	legacy.Plan.DemoSHA256 = ""
	legacy.Plan.DemoDurationTicks = 0
	legacy.Plan.EditorialSegmentIDs = nil
	legacy.CaptureVerified = true
	if err := putRecordingResult(store, id, legacy); err != nil {
		t.Fatal(err)
	}
	missing, _, err = recordingOutputsReady(store, id, []string{"seg-001"}, expectedPlan)
	if err != nil {
		t.Fatal(err)
	}
	if len(missing) != 0 {
		t.Fatal("verified durable V1 capture was stranded after the V2 upgrade")
	}

	result.CaptureMode = recording.CaptureModeReal
	result.CaptureVerified = true
	result.CaptureInputFingerprint, _ = recording.CaptureInputFingerprint(result.Plan)
	if err := putRecordingResult(store, id, result); err != nil {
		t.Fatal(err)
	}
	missing, _, err = recordingOutputsReady(store, id, []string{"seg-001"}, expectedPlan)
	if err != nil {
		t.Fatal(err)
	}
	if len(missing) != 0 {
		t.Fatal("capture with current observer-SteamID contract was not reusable")
	}

	staleSegments := append([]recording.RecordingSegment(nil), result.Plan.Segments...)
	staleSegments[0].TickStart++
	result.Plan.Segments = staleSegments
	result.CaptureInputFingerprint, _ = recording.CaptureInputFingerprint(result.Plan)
	if err := putRecordingResult(store, id, result); err != nil {
		t.Fatal(err)
	}
	missing, _, err = recordingOutputsReady(store, id, []string{"seg-001"}, expectedPlan)
	if err != nil {
		t.Fatal(err)
	}
	if len(missing) == 0 {
		t.Fatal("capture with stale segment bounds was reused")
	}
}

func TestRecordingOutputsReadyRejectsCenteredFullHUDProfile(t *testing.T) {
	store := newFakeStorage()
	id := uuid.New()
	expected := recording.DefaultStreamConfig()
	expected.HUDMode = recording.HUDModeGameplay
	expected.PortraitSafeKillfeed = true
	expectedPlan, err := recording.NewPlanFromKillPlan(minimalKillPlan(), "profile.dem", "profile", expected)
	if err != nil {
		t.Fatal(err)
	}

	centered := expected
	centered.DeathnoticeSafeZoneX = 0.28
	centered.DeathnoticeSafeZoneY = 0.82
	resultPlan := expectedPlan
	resultPlan.Stream = centered
	result := recording.RecordingResult{
		Plan: resultPlan,
		Artifacts: []recording.RecordingArtifact{{
			SegmentID: "seg-001",
			Role:      "segment",
			Type:      "video",
		}},
		CaptureMode:     recording.CaptureModeReal,
		CaptureVerified: true,
	}
	result.CaptureInputFingerprint, _ = recording.CaptureInputFingerprint(result.Plan)
	if err := putRecordingResult(store, id, result); err != nil {
		t.Fatal(err)
	}
	if err := store.Put(recording.ScriptArtifactKey(id), bytes.NewReader([]byte("centered HUD script"))); err != nil {
		t.Fatal(err)
	}
	if err := store.Put(mustSegmentClipKey(t, id, "seg-001"), bytes.NewReader([]byte("centered HUD clip"))); err != nil {
		t.Fatal(err)
	}

	missing, _, err := recordingOutputsReady(store, id, []string{"seg-001"}, expectedPlan)
	if err != nil {
		t.Fatal(err)
	}
	if len(missing) == 0 {
		t.Fatal("Full HUD capture with global safe-zone compression was reused")
	}
}

func TestRecordingOutputsReadyReusesLegacyZeroPlaybackTimescale(t *testing.T) {
	store := newFakeStorage()
	id := uuid.New()
	stream := recording.DefaultStreamConfig()
	expectedPlan, err := recording.NewPlanFromKillPlan(minimalKillPlan(), "profile.dem", "profile", stream)
	if err != nil {
		t.Fatal(err)
	}
	stored := expectedPlan
	stored.Runtime.PlaybackTimescale = 0
	result := recording.RecordingResult{
		Plan: stored,
		Artifacts: []recording.RecordingArtifact{{
			SegmentID: "seg-001",
			Role:      "segment",
			Type:      "video",
		}},
		CaptureMode:     recording.CaptureModeReal,
		CaptureVerified: true,
	}
	result.CaptureInputFingerprint, _ = recording.CaptureInputFingerprint(result.Plan)
	if err := putRecordingResult(store, id, result); err != nil {
		t.Fatal(err)
	}
	if err := store.Put(recording.ScriptArtifactKey(id), bytes.NewReader([]byte("script"))); err != nil {
		t.Fatal(err)
	}
	if err := store.Put(mustSegmentClipKey(t, id, "seg-001"), bytes.NewReader([]byte("clip"))); err != nil {
		t.Fatal(err)
	}

	missing, _, err := recordingOutputsReady(store, id, []string{"seg-001"}, expectedPlan)
	if err != nil {
		t.Fatal(err)
	}
	if len(missing) != 0 {
		t.Fatalf("stored playback_timescale 0 was treated as incompatible: missing %v", missing)
	}
}

// TestRecordingOutputsReadyReturnsOnlyTheMissingSubset is the B1 unit-level
// regression: recordingOutputsReady must report exactly which requested
// segments still lack a durable, profile-compatible clip instead of an
// all-or-nothing readiness bit.
func TestRecordingOutputsReadyReturnsOnlyTheMissingSubset(t *testing.T) {
	plan := multiSegmentKillPlan("seg-001", "seg-002", "seg-003")
	stream := recording.DefaultStreamConfig()
	expectedPlan, err := recording.NewPlanFromKillPlan(plan, "profile.dem", "profile", stream)
	if err != nil {
		t.Fatal(err)
	}

	// A prior compatible run recorded seg-001 and seg-002 only; seg-003 was
	// never captured.
	recordedPlan := expectedPlan
	recordedPlan.Segments = expectedPlan.Segments[:2]
	recordedPlan.EditorialSegmentIDs = expectedPlan.EditorialSegmentIDs[:2]
	result := recording.RecordingResult{
		Plan: recordedPlan,
		Artifacts: []recording.RecordingArtifact{
			{SegmentID: "seg-001", Role: "segment", Type: "video"},
			{SegmentID: "seg-002", Role: "segment", Type: "video"},
		},
		CaptureMode:     recording.CaptureModeReal,
		CaptureVerified: true,
	}
	result.CaptureInputFingerprint, _ = recording.CaptureInputFingerprint(result.Plan)

	cases := []struct {
		name      string
		requested []string
		want      []string
	}{
		{name: "fully covered selection reports nothing missing", requested: []string{"seg-001"}, want: nil},
		{name: "fully covered multi-segment selection reports nothing missing", requested: []string{"seg-001", "seg-002"}, want: nil},
		{name: "partially covered selection reports only the uncaptured segment", requested: []string{"seg-001", "seg-002", "seg-003"}, want: []string{"seg-003"}},
		{name: "fully uncovered selection reports the whole selection", requested: []string{"seg-003"}, want: []string{"seg-003"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			store := newFakeStorage()
			id := uuid.New()
			if err := putRecordingResult(store, id, result); err != nil {
				t.Fatal(err)
			}
			if err := store.Put(recording.ScriptArtifactKey(id), bytes.NewReader([]byte("script"))); err != nil {
				t.Fatal(err)
			}
			for _, sid := range []string{"seg-001", "seg-002"} {
				if err := store.Put(mustSegmentClipKey(t, id, sid), bytes.NewReader([]byte("clip"))); err != nil {
					t.Fatal(err)
				}
			}

			missing, _, err := recordingOutputsReady(store, id, tc.requested, expectedPlan)
			if err != nil {
				t.Fatal(err)
			}
			if len(missing) != len(tc.want) {
				t.Fatalf("missing = %v, want %v", missing, tc.want)
			}
			for i, sid := range tc.want {
				if missing[i] != sid {
					t.Fatalf("missing = %v, want %v", missing, tc.want)
				}
			}
		})
	}
}

func TestRecordingProfilesCompatibleRejectsLegacyPOVContract(t *testing.T) {
	plan, err := recording.NewPlanFromKillPlan(minimalKillPlan(), "profile.dem", "profile", recording.DefaultStreamConfig())
	if err != nil {
		t.Fatal(err)
	}
	current := recording.RecordingResult{Plan: plan, CaptureMode: recording.CaptureModeReal, CaptureVerified: true}
	current.CaptureInputFingerprint, _ = recording.CaptureInputFingerprint(current.Plan)
	legacy := current
	legacy.Plan.CaptureContract = ""

	if recordingProfilesCompatible(legacy, current) {
		t.Fatal("legacy capture could be merged into a verified POV recording")
	}
	if !recordingProfilesCompatible(current, current) {
		t.Fatal("matching current capture profiles should remain compatible")
	}
}

func TestRenderVariantOutputsReadyRequiresSegmentCoverage(t *testing.T) {
	store := newFakeStorage()
	id := uuid.New()

	// Recording result holds two segments (two reels recorded).
	rec := recording.RecordingResult{
		Plan: recording.RecordingPlan{Segments: []recording.RecordingSegment{{ID: "seg-001"}, {ID: "seg-002"}}},
		Artifacts: []recording.RecordingArtifact{
			{SegmentID: "seg-001", Role: "segment", Type: "video"},
			{SegmentID: "seg-002", Role: "segment", Type: "video"},
		},
	}
	if err := putRecordingResult(store, id, rec); err != nil {
		t.Fatal(err)
	}

	covered, err := renderCoversRecordedSegments(store, id, editor.Result{
		Shorts: []editor.ShortResult{{SegmentID: "seg-001"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if covered {
		t.Fatal("render covering only seg-001 should NOT cover a 2-segment recording")
	}

	covered, err = renderCoversRecordedSegments(store, id, editor.Result{
		Shorts: []editor.ShortResult{{SegmentID: "seg-001"}, {SegmentID: "seg-002"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !covered {
		t.Fatal("render covering both segments should be considered covered")
	}

	// A compilation render is always treated as covered (different render mode).
	covered, err = renderCoversRecordedSegments(store, id, editor.Result{
		Shorts: []editor.ShortResult{{SegmentID: compilationSegmentID}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !covered {
		t.Fatal("compilation render should be treated as covered")
	}
}

func TestRenderVariantOutputsReadyRequiresMatchingInputFingerprint(t *testing.T) {
	store := newFakeStorage()
	id := uuid.New()
	plan := minimalKillPlan()
	rec := recordingResultWithSegment("", "C:/stale/seg-001.mp4")
	rec.CaptureRevision = "capture-1"
	if err := putRecordingResult(store, id, rec); err != nil {
		t.Fatal(err)
	}
	edit := renderplan.DefaultEditRequest()
	fingerprint, err := renderInputFingerprint(rec, &plan, editor.PresetViral60Clean, "", "", 0, nil, edit)
	if err != nil {
		t.Fatal(err)
	}
	seedLegacyRenderVariantReady(t, store, id, editor.PresetViral60Clean, editor.Result{
		Preset:           editor.PresetViral60Clean,
		InputFingerprint: fingerprint,
		Shorts:           []editor.ShortResult{{SegmentID: "seg-001"}},
	})

	ready, _, _, err := renderVariantOutputsReady(store, id, editor.PresetViral60Clean, fingerprint)
	if err != nil || !ready {
		t.Fatalf("matching inputs ready/error = %v/%v, want true/nil", ready, err)
	}

	recaptured := rec
	recaptured.CaptureRevision = "capture-2"
	changedCapture, err := renderInputFingerprint(recaptured, &plan, editor.PresetViral60Clean, "", "", 0, nil, edit)
	if err != nil {
		t.Fatal(err)
	}
	changedEdit := edit
	changedEdit.Transition = renderplan.TransitionWhip
	changedTreatment, err := renderInputFingerprint(rec, &plan, editor.PresetViral60Clean, "", "", 0, nil, changedEdit)
	if err != nil {
		t.Fatal(err)
	}
	musicPath := filepath.Join(t.TempDir(), "phonk.wav")
	if err := os.WriteFile(musicPath, []byte("music-v1"), 0o600); err != nil {
		t.Fatal(err)
	}
	changedMusic, err := renderInputFingerprint(rec, &plan, editor.PresetViral60Clean, "phonk", musicPath, 0, nil, edit)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(musicPath, []byte("music-v2"), 0o600); err != nil {
		t.Fatal(err)
	}
	changedMusicContent, err := renderInputFingerprint(rec, &plan, editor.PresetViral60Clean, "phonk", musicPath, 0, nil, edit)
	if err != nil {
		t.Fatal(err)
	}
	if changedMusic == changedMusicContent {
		t.Fatal("music content change did not change render fingerprint")
	}
	changedMusicVolume, err := renderInputFingerprint(rec, &plan, editor.PresetViral60Clean, "phonk", musicPath, 0.5, nil, edit)
	if err != nil {
		t.Fatal(err)
	}
	if changedMusicVolume == changedMusicContent {
		t.Fatal("music volume change did not change render fingerprint")
	}
	gameVol := 0.2
	changedGameVolume, err := renderInputFingerprint(rec, &plan, editor.PresetViral60Clean, "phonk", musicPath, 0.5, &gameVol, edit)
	if err != nil {
		t.Fatal(err)
	}
	if changedGameVolume == changedMusicVolume {
		t.Fatal("game volume change did not change render fingerprint")
	}
	voiceEdit := edit
	voice := 0.85
	voiceEdit.VoiceVolume = &voice
	changedVoiceVolume, err := renderInputFingerprint(rec, &plan, editor.PresetViral60Clean, "phonk", musicPath, 0.5, nil, voiceEdit)
	if err != nil {
		t.Fatal(err)
	}
	if changedVoiceVolume == changedMusicVolume {
		t.Fatal("voice volume change did not change render fingerprint")
	}

	for name, candidate := range map[string]string{
		"capture revision": changedCapture,
		"edit treatment":   changedTreatment,
		"music":            changedMusic,
		"music content":    changedMusicContent,
		"music volume":     changedMusicVolume,
		"game volume":      changedGameVolume,
		"voice volume":     changedVoiceVolume,
		"legacy empty":     "",
	} {
		t.Run(name, func(t *testing.T) {
			ready, _, _, err := renderVariantOutputsReady(store, id, editor.PresetViral60Clean, candidate)
			if err != nil {
				t.Fatal(err)
			}
			if ready {
				t.Fatal("stale render inputs were reused")
			}
		})
	}
}
