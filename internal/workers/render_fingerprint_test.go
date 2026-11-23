package workers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/artifacts"
	"github.com/rechedev9/cliphub/internal/editor"
	"github.com/rechedev9/cliphub/internal/job"
	"github.com/rechedev9/cliphub/internal/parser"
	"github.com/rechedev9/cliphub/internal/recording"
	"github.com/rechedev9/cliphub/internal/renderplan"
	"github.com/rechedev9/cliphub/internal/rules"
	"github.com/rechedev9/cliphub/internal/subtitles"
	"github.com/rechedev9/cliphub/internal/tasks"
)

type fixedSubtitles struct {
	lookup subtitles.Lookup
	err    error
}

func (f fixedSubtitles) Subtitles(context.Context, subtitles.Request) (subtitles.Lookup, error) {
	return f.lookup, f.err
}

var errRenderedAgain = errors.New("rendered again")

// cachedRender is a job whose render of edit is cached under the fingerprint cached gives for the
// render's own inputs; it renders once with source and reports whether the cached video was reused.
func cachedRender(t *testing.T, edit renderplan.EditRequest, source SubtitleSource, cached func(base string) string) bool {
	t.Helper()
	repo, store, id, plan := newFakeRepo(), newFakeStorage(), uuid.New(), minimalKillPlan()
	repo.jobs[id] = &job.Job{ID: id, Status: job.StatusRecorded, Rules: rules.Default(), KillPlan: &plan, TargetSteamID: plan.Target.SteamID64}
	rec := recordingResultWithSegment("", "C:/stale/seg-001.mp4")
	rec.CaptureRevision = "capture-1"
	putJSON(t, store, recording.ResultArtifactKey(id), rec)
	_ = store.Put(mustSegmentClipKey(t, id, "seg-001"), bytes.NewReader([]byte("clip")))
	putJSON(t, store, artifacts.RosterKey(id), parser.RosterResult{Players: []parser.PlayerStat{{SteamID64: plan.Target.SteamID64, Name: "player", Team: "CT", Kills: 3}}})
	base, err := renderInputFingerprint(rec, &plan, editor.PresetViral60Clean, "", "", 0, nil, edit)
	if err != nil {
		t.Fatal(err)
	}
	seedLegacyRenderVariantReady(t, store, id, editor.PresetViral60Clean, editor.Result{
		Preset: editor.PresetViral60Clean, InputFingerprint: cached(base), Shorts: []editor.ShortResult{{SegmentID: "seg-001"}},
	})
	runner := &fakeRunner{fn: func(context.Context, string, ...string) ([]byte, error) { return nil, errRenderedAgain }}
	w := NewRenderWorker(repo, store, RenderWorkerConfig{WorkDir: t.TempDir(), EditorPath: "zv-editor"})
	w.runner = runner
	if source != nil {
		w.UseSubtitles(source)
	}
	task, err := tasks.NewRenderVariantTask(id, editor.PresetViral60Clean, "", 0, nil, edit, nil)
	if err != nil {
		t.Fatal(err)
	}
	// A render that runs again stops at the editor, or before it at the team voices, which need a demo.
	err = w.HandleRenderVariant(context.Background(), task)
	if err != nil && !errors.Is(err, errRenderedAgain) && !strings.Contains(err.Error(), "has no demo") {
		t.Fatal(err)
	}
	var state renderplan.RenderVariantState
	if err := json.Unmarshal(store.files[mustRenderVariantStatusKey(t, id, editor.PresetViral60Clean)], &state); err != nil {
		t.Fatal(err)
	}
	return state.Status == renderplan.RenderVariantStatusReady
}

func asIs(base string) string { return base }

// renderedWith is the fingerprint of a render that read extras.
func renderedWith(t *testing.T, extras renderExtras) func(string) string {
	return func(base string) string {
		got, err := extras.fingerprint(base)
		if err != nil {
			t.Fatal(err)
		}
		return got
	}
}

// A cached video is the video of its subtitles and player card too: one rendered before either could be
// read renders again once they can, and one rendered while they still cannot be read stays cached.
func TestRenderWorkerRendersAgainWhenItsSubtitlesOrCardsCanBeRead(t *testing.T) {
	subs := renderplan.DefaultEditRequest()
	subs.VoiceComms, subs.Subtitles = true, true
	cards := renderplan.DefaultEditRequest()
	cards.OpeningCard = "ficha"
	track := subtitles.Track{Phrases: []subtitles.Phrase{{ID: uuid.NewString(), Speaker: "player", Text: "Rotamos a B", StartTick: 64, EndTick: 128}}}
	for _, tc := range []struct {
		name   string
		edit   renderplan.EditRequest
		source SubtitleSource
		reused bool
	}{
		{name: "the transcript cannot be read", edit: subs, source: fixedSubtitles{err: subtitles.ErrTranscriptFailed}, reused: true},
		{name: "the transcript became ready", edit: subs, source: fixedSubtitles{lookup: subtitles.Ready(track)}},
		{name: "the player card can be read", edit: cards},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := cachedRender(t, tc.edit, tc.source, asIs); got != tc.reused {
				t.Fatalf("cached video reused = %v, want %v", got, tc.reused)
			}
		})
	}
}

// A corrected phrase renders the video again; the same transcript keeps it.
func TestRenderWorkerRendersAgainWhenAPhraseIsCorrected(t *testing.T) {
	edit := renderplan.DefaultEditRequest()
	edit.VoiceComms, edit.Subtitles = true, true
	id := uuid.NewString()
	said := func(text string) subtitles.Track {
		return subtitles.Track{Phrases: []subtitles.Phrase{{ID: id, Speaker: "player", Text: text, StartTick: 64, EndTick: 128}}}
	}
	rendered := said("Rotamos a B")
	cached := renderedWith(t, renderExtras{Subtitles: &rendered})
	if !cachedRender(t, edit, fixedSubtitles{lookup: subtitles.Ready(said("Rotamos a B"))}, cached) {
		t.Fatal("the same transcript rendered the video again")
	}
	if cachedRender(t, edit, fixedSubtitles{lookup: subtitles.Ready(said("Rotamos a A"))}, cached) {
		t.Fatal("a corrected phrase kept the cached video")
	}
}
