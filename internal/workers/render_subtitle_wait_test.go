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
	"time"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/artifacts"
	"github.com/rechedev9/cliphub/internal/editor"
	"github.com/rechedev9/cliphub/internal/job"
	"github.com/rechedev9/cliphub/internal/recording"
	"github.com/rechedev9/cliphub/internal/renderplan"
	"github.com/rechedev9/cliphub/internal/rules"
	"github.com/rechedev9/cliphub/internal/subtitles"
	"github.com/rechedev9/cliphub/internal/tasks"
)

type steppedSubtitles struct {
	steps  []subtitles.Lookup
	calls  int
	onCall func(call int)
}

func (s *steppedSubtitles) Subtitles(context.Context, subtitles.Request) (subtitles.Lookup, error) {
	s.calls++
	if s.onCall != nil {
		s.onCall(s.calls)
	}
	return s.steps[min(s.calls, len(s.steps))-1], nil
}

type subtitleRenderOutcome struct {
	state     renderplan.RenderVariantState
	subtitles *subtitles.Track
}

type subtitleRenderFixture struct {
	t     *testing.T
	id    uuid.UUID
	store *fakeStorage
	w     *RenderWorker
}

func newSubtitleRenderFixture(t *testing.T) subtitleRenderFixture {
	t.Helper()
	repo, store, id, plan := newFakeRepo(), newFakeStorage(), uuid.New(), minimalKillPlan()
	repo.jobs[id] = &job.Job{ID: id, Status: job.StatusRecorded, DemoPath: "demos/test.dem", TargetSteamID: plan.Target.SteamID64, Rules: rules.Default(), KillPlan: &plan}
	putJSON(t, store, recording.ResultArtifactKey(id), recordingResultWithSegment("", "C:/stale/seg-001.mp4"))
	_ = store.Put(mustSegmentClipKey(t, id, "seg-001"), bytes.NewReader([]byte("clip")))
	_ = store.Put("demos/test.dem", bytes.NewReader([]byte("demo")))
	w := NewRenderWorker(repo, store, RenderWorkerConfig{WorkDir: t.TempDir(), EditorPath: "zv-editor", FFmpegPath: "ffmpeg"})
	w.voiceExtract = func(_, _, dir string) (int, error) {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return 0, err
		}
		return 1, os.WriteFile(filepath.Join(dir, "track.ogg"), []byte("ogg"), 0o600)
	}
	return subtitleRenderFixture{t: t, id: id, store: store, w: w}
}

func (f subtitleRenderFixture) render(source SubtitleSource, waitMax time.Duration) subtitleRenderOutcome {
	f.t.Helper()
	f.w.UseSubtitles(source)
	f.w.subtitles.waitMax, f.w.subtitles.pollInterval = waitMax, time.Millisecond
	var out subtitleRenderOutcome
	f.w.runner = &fakeRunner{fn: func(_ context.Context, _ string, args ...string) ([]byte, error) {
		if path := argValue(args, "--subtitles"); path != "" {
			track, err := subtitles.Load(path)
			if err != nil {
				f.t.Fatal(err)
			}
			out.subtitles = track
		}
		f.writeEditorResult(argValue(args, "--out"), argValue(args, "--publish-dir"))
		return []byte("rendered"), nil
	}}
	edit := renderplan.DefaultEditRequest()
	edit.VoiceComms, edit.Subtitles = true, true
	task, err := tasks.NewRenderVariantTask(f.id, editor.PresetViral60Clean, "", 0, nil, edit, nil)
	if err != nil {
		f.t.Fatal(err)
	}
	if err := f.w.HandleRenderVariant(context.Background(), task); err != nil {
		f.t.Fatalf("HandleRenderVariant error = %v", err)
	}
	if err := json.Unmarshal(f.store.files[mustRenderVariantStatusKey(f.t, f.id, editor.PresetViral60Clean)], &out.state); err != nil {
		f.t.Fatal(err)
	}
	return out
}

func (f subtitleRenderFixture) writeEditorResult(outDir, publishDir string) {
	write := func(path, body string) {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			f.t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			f.t.Fatal(err)
		}
	}
	short := editor.ShortResult{SegmentID: "seg-001", Output: filepath.Join(publishDir, "seg-001.mp4"), PublishPath: filepath.Join(publishDir, "seg-001.mp4"),
		CoverPath: filepath.Join(publishDir, "seg-001.cover.jpg"), CaptionPath: filepath.Join(publishDir, "seg-001.caption.txt"), RenderLogPath: filepath.Join(outDir, "logs", "seg-001-render.log")}
	for _, path := range []string{short.Output, short.CoverPath, short.CaptionPath, short.RenderLogPath} {
		write(path, "media")
	}
	write(filepath.Join(outDir, "edit-manifest.json"), segmentIDOnlyDocJSON(f.t, "shorts", []string{"seg-001"}))
	write(filepath.Join(publishDir, "pack-manifest.json"), segmentIDOnlyDocJSON(f.t, "items", []string{"seg-001"}))
	write(filepath.Join(publishDir, "index.html"), `<html></html>`)
	write(filepath.Join(publishDir, "publish-summary.md"), `summary`)
	rendered := editor.Result{Preset: editor.PresetViral60Clean, OutputDir: outDir, PublishDir: publishDir, GalleryPath: filepath.Join(publishDir, "index.html"),
		SummaryPath: filepath.Join(publishDir, "publish-summary.md"), Shorts: []editor.ShortResult{short}}
	if err := writeJSONFile(filepath.Join(outDir, "shorts-result.json"), rendered); err != nil {
		f.t.Fatal(err)
	}
}

func (f subtitleRenderFixture) renderStage() string {
	var progress editor.EditorProgress
	if err := json.Unmarshal(f.store.files[artifacts.RenderProgressKey(f.id)], &progress); err != nil {
		return ""
	}
	return progress.Stage
}

func TestRenderWaitsForTheTranscriptionItsSubtitlesRead(t *testing.T) {
	f := newSubtitleRenderFixture(t)
	said := subtitles.Track{Phrases: []subtitles.Phrase{{ID: uuid.NewString(), Speaker: "player", Text: "Rotamos a B", StartTick: 64, EndTick: 128}}}
	stageWhileWaiting := ""
	source := &steppedSubtitles{steps: []subtitles.Lookup{subtitles.Pending(0.25), subtitles.Pending(0.6), subtitles.Ready(said)}}
	source.onCall = func(call int) {
		if call == 3 {
			stageWhileWaiting = f.renderStage()
		}
	}
	got := f.render(source, time.Minute)
	if source.calls != 3 {
		t.Fatalf("the render read the transcript %d times, want until it was ready (3)", source.calls)
	}
	if stageWhileWaiting != "Esperando la transcripción de las voces (60 %)" {
		t.Fatalf("render stage while the transcription runs = %q", stageWhileWaiting)
	}
	if got.subtitles == nil || len(got.subtitles.Phrases) != 1 || got.subtitles.Phrases[0].Text != "Rotamos a B" {
		t.Fatalf("the editor burned %+v, want the phrase the transcription finished", got.subtitles)
	}
	if got.state.Status != renderplan.RenderVariantStatusReady || slices.ContainsFunc(got.state.Warnings, func(w string) bool { return strings.HasPrefix(w, "Salió sin subtítulos") }) {
		t.Fatalf("render state = %s %q, want ready with its subtitles", got.state.Status, got.state.Warnings)
	}
}

func TestRenderWithoutItsSubtitlesSaysWhy(t *testing.T) {
	for _, tc := range []struct {
		name    string
		source  SubtitleSource
		warning string
	}{
		{name: "the transcription failed", source: fixedSubtitles{err: subtitles.ErrTranscriptFailed}, warning: subtitlesFailedWarning},
		{name: "the demo voices cannot be transcribed", source: fixedSubtitles{err: errors.New("local transcription is not installed")}, warning: subtitlesMissingWarning},
		{name: "the transcription outlasts the wait", source: fixedSubtitles{lookup: subtitles.Pending(0.5)}, warning: "Salió sin subtítulos: la transcripción de las voces no terminó"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := newSubtitleRenderFixture(t).render(tc.source, 5*time.Millisecond)
			if got.subtitles != nil {
				t.Fatalf("the editor burned %+v, want no subtitles", got.subtitles)
			}
			if got.state.Status != renderplan.RenderVariantStatusReady {
				t.Fatalf("render state = %s %q, want ready", got.state.Status, got.state.Error)
			}
			if !slices.ContainsFunc(got.state.Warnings, func(w string) bool { return strings.HasPrefix(w, tc.warning) }) {
				t.Fatalf("render warnings = %q, want one that starts %q", got.state.Warnings, tc.warning)
			}
		})
	}
}
