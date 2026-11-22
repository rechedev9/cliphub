package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/rechedev9/cliphub/internal/artifacts"
	"github.com/rechedev9/cliphub/internal/library"
	"github.com/rechedev9/cliphub/internal/subtitles"
	"github.com/rechedev9/cliphub/internal/tacticalplan"
	"github.com/rechedev9/cliphub/internal/teamvoice"
	"github.com/rechedev9/cliphub/internal/vocab"
)

func newMirageVocabEnv(t *testing.T) demoTranscriptEnv {
	t.Helper()
	env := newDemoTranscriptEnv(t)
	env.transcriber.state = teamvoice.TranscriberInstalled
	doc, err := env.h.readLibraryDocument(env.match.DemoID)
	if err != nil {
		t.Fatal(err)
	}
	doc.Geometry.Callouts = []tacticalplan.Callout{{Name: "Palace", Samples: 40}, {Name: "Underpass", Samples: 30}, {Name: "BombsiteA", Samples: 20}}
	doc.Players[0].Name = "ZywOo"
	raw, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	if err := env.storage.Put(artifacts.LibraryTacticalIndexKey(env.match.DemoID), bytes.NewReader(raw)); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	for _, c := range []library.CalloutName{
		{Map: "de_mirage", Token: "TRamp", Name: "Tetris"},
		{Map: "de_mirage", Token: "Shop", Name: "Tetras"},
		{Map: "de_mirage", Token: "Stairs", Name: "Sandwich"},
		{Map: "de_inferno", Token: "Banana", Name: "Bananna"},
	} {
		if err := env.repo.SetCalloutName(ctx, &c); err != nil {
			t.Fatal(err)
		}
	}
	env.transcriber.heard = []teamvoice.Line{
		{StartMS: 1000, EndMS: 3000, Text: "moly under Palas, Zaywoo tetres banana sanwich", Words: []teamvoice.Word{
			{Text: "moly", StartMS: 1000, EndMS: 1200}, {Text: "under", StartMS: 1200, EndMS: 1500}, {Text: "Palas,", StartMS: 1500, EndMS: 1900},
			{Text: "Zaywoo", StartMS: 2000, EndMS: 2300}, {Text: "tetres", StartMS: 2300, EndMS: 2500}, {Text: "banana", StartMS: 2500, EndMS: 2700},
			{Text: "sanwich", StartMS: 2700, EndMS: 3000},
		}},
		{StartMS: 4000, EndMS: 5000, Text: "Палас под", Words: []teamvoice.Word{{Text: "Палас", StartMS: 4000, EndMS: 4500}, {Text: "под", StartMS: 4500, EndMS: 5000}}},
	}
	return env
}

func (e demoTranscriptEnv) transcribe(force bool) demoTranscript {
	e.t.Helper()
	before := e.transcriber.calls.Load()
	e.json(http.MethodPost, e.base, map[string]bool{"force": force}, http.StatusAccepted, nil)
	return e.await(func(d demoTranscript) bool {
		return d.State == library.TranscriptReady && e.transcriber.calls.Load() == before+2
	})
}

func (e demoTranscriptEnv) callWords() []subtitles.PhraseWord {
	e.t.Helper()
	j := e.linkedJob()
	track, err := e.h.Subtitles(context.Background(), subtitles.Request{JobID: j.ID, Target: j.TargetSteamID})
	if err != nil {
		e.t.Fatal(err)
	}
	for _, p := range track.Phrases {
		if len(p.Words) > 2 {
			return p.Words
		}
	}
	e.t.Fatalf("no call in the subtitles %+v", track.Phrases)
	return nil
}

func callOf(t *testing.T, got demoTranscript) demoTranscriptLine {
	t.Helper()
	for _, l := range got.Lines {
		if l.StartMS == 1000 {
			return l
		}
	}
	t.Fatalf("no call at 1000 ms in %+v", got.Lines)
	return demoTranscriptLine{}
}

func TestDemoTranscriptCorrectsTheCalloutsOfTheMapAndTheNicksOfTheMatch(t *testing.T) {
	env := newMirageVocabEnv(t)
	heard := "moly under Palas, Zaywoo tetres banana sanwich"

	off := callOf(t, env.transcribe(false))
	if off.Text != heard || off.VocabText != "" || len(off.VocabChanges) != 0 || off.Corrected {
		t.Fatalf("with the corrector off = %+v; want what the transcriber heard", off)
	}
	offWords := env.callWords()

	env.h.voice.deps.VocabCorrector = true
	on := env.transcribe(true)
	call := callOf(t, on)
	want := "molly under Palace, ZywOo tetres banana sandwich"
	if call.Text != want || call.VocabText != want || call.OriginalText != heard || call.Corrected {
		t.Fatalf("with the corrector on = %+v; want %q with the heard text kept", call.DemoVoiceLine, want)
	}
	if call.ID != off.ID {
		t.Fatalf("the corrected frase changed id from %s to %s; a hidden frase would show again", off.ID, call.ID)
	}
	changes := []vocab.Change{{Heard: "moly", Written: "molly"}, {Heard: "Palas,", Written: "Palace,"}, {Heard: "Zaywoo", Written: "ZywOo"}, {Heard: "sanwich", Written: "sandwich"}}
	if len(call.VocabChanges) != len(changes) {
		t.Fatalf("changes = %+v; want %+v", call.VocabChanges, changes)
	}
	for i, c := range changes {
		if call.VocabChanges[i].Heard != c.Heard || call.VocabChanges[i].Written != c.Written {
			t.Fatalf("changes = %+v; want %+v", call.VocabChanges, changes)
		}
	}
	onWords := env.callWords()
	if len(onWords) != len(offWords) || onWords[2].Text != "Palace," || onWords[2].StartTick != offWords[2].StartTick || onWords[2].EndTick != offWords[2].EndTick ||
		onWords[3].Text != "ZywOo" || onWords[3].StartTick != offWords[3].StartTick {
		t.Fatalf("words = %+v, heard %+v; want each corrected word on the heard word's time", onWords, offWords)
	}
	for _, l := range on.Lines {
		if l.StartMS == 4000 && (l.Text != "Палас под" || l.VocabText != "" || l.OriginalText != "") {
			t.Fatalf("a Cyrillic frase = %+v; want it as heard", l)
		}
	}
}

func TestDemoTranscriptCorrectorNeverOverwritesAPersonsCorrection(t *testing.T) {
	env := newMirageVocabEnv(t)
	env.h.voice.deps.VocabCorrector = true
	heard := "moly under Palas, Zaywoo tetres banana sanwich"
	call := callOf(t, env.transcribe(false))
	path := env.base + "/lines/" + call.ID.String()

	var undone library.DemoVoiceLine
	env.json(http.MethodPatch, path, map[string]string{"text": heard}, http.StatusOK, &undone)
	if undone.Text != heard || !undone.Corrected {
		t.Fatalf("undo of the corrector = %+v; want the heard text as a person's correction", undone)
	}
	if again := callOf(t, env.transcribe(true)); again.Text != heard || !again.Corrected || again.VocabText != undone.VocabText {
		t.Fatalf("transcribed again after an undo = %+v; want the undo kept as the person's correction of what the corrector wrote", again.DemoVoiceLine)
	}

	own := "molly under palace, ZywOo"
	env.json(http.MethodPatch, path, map[string]string{"text": own}, http.StatusOK, nil)
	again := callOf(t, env.transcribe(true))
	if again.Text != own || !again.Corrected || again.OriginalText != heard {
		t.Fatalf("transcribed again after a correction = %+v; want the person's text kept", again.DemoVoiceLine)
	}

	var machine library.DemoVoiceLine
	env.json(http.MethodPatch, path, map[string]string{"text": again.VocabText}, http.StatusOK, &machine)
	if machine.Corrected || machine.OriginalText != heard {
		t.Fatalf("back to the corrector's text = %+v; want it not a person's correction", machine)
	}
}

func (e demoTranscriptEnv) setMap(name string) {
	e.t.Helper()
	doc, err := e.h.readLibraryDocument(e.match.DemoID)
	if err != nil {
		e.t.Fatal(err)
	}
	doc.Demo.Map = name
	raw, err := json.Marshal(doc)
	if err != nil {
		e.t.Fatal(err)
	}
	if err := e.storage.Put(artifacts.LibraryTacticalIndexKey(e.match.DemoID), bytes.NewReader(raw)); err != nil {
		e.t.Fatal(err)
	}
}

func TestDemoTranscriptCorrectsTheCS2LexiconAndOnlyTheCommunityCalloutsOfTheDemosMap(t *testing.T) {
	env := newMirageVocabEnv(t)
	env.h.voice.deps.VocabCorrector = true
	heard := "Go kontakt, moly jungel coffens flesh"
	env.transcriber.heard = []teamvoice.Line{{StartMS: 1000, EndMS: 3000, Text: heard}}

	if got := callOf(t, env.transcribe(false)); got.Text != "Go contact, molly jungle coffens flesh" || got.OriginalText != heard {
		t.Fatalf("on Mirage = %+v; want the calls, the grenades and the Mirage callouts written, an Inferno callout and a common word left", got.DemoVoiceLine)
	}

	env.setMap("de_inferno")
	if got := callOf(t, env.transcribe(true)); got.Text != "Go contact, molly jungel coffins flesh" || got.OriginalText != heard {
		t.Fatalf("on Inferno = %+v; want the Inferno callouts written and a Mirage callout left", got.DemoVoiceLine)
	}
}

func TestDemoTranscriptLeavesRightWordsAndStillWritesTheCallsAndCallouts(t *testing.T) {
	env := newMirageVocabEnv(t)
	env.h.voice.deps.VocabCorrector = true
	doc, err := env.h.readLibraryDocument(env.match.DemoID)
	if err != nil {
		t.Fatal(err)
	}
	doc.Demo.Map = "de_ancient"
	doc.Geometry.Callouts = []tacticalplan.Callout{{Name: "Palace", Samples: 40}, {Name: "Stairs", Samples: 30}}
	raw, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	if err := env.storage.Put(artifacts.LibraryTacticalIndexKey(env.match.DemoID), bytes.NewReader(raw)); err != nil {
		t.Fatal(err)
	}
	heard := "he starts in the cove, asp vamos, moly under Palas, Go kontakt"
	env.transcriber.heard = []teamvoice.Line{{StartMS: 1000, EndMS: 3000, Text: heard}}
	want := "he starts in the cove, asp vamos, molly under Palace, Go contact"
	if got := callOf(t, env.transcribe(false)); got.Text != want || got.OriginalText != heard {
		t.Fatalf("frase = %+v; want %q: right English and Spanish words left, the calls and callouts written", got.DemoVoiceLine, want)
	}
}

func TestDemoTranscriptLeavesEnglishWordsThatSoundLikeACS2Term(t *testing.T) {
	env := newMirageVocabEnv(t)
	env.h.voice.deps.VocabCorrector = true
	heard := "safe clock planned famous ego tread lot"
	env.transcriber.heard = []teamvoice.Line{{StartMS: 1000, EndMS: 3000, Text: heard}}
	if got := callOf(t, env.transcribe(false)); got.Text != heard || got.VocabText != "" {
		t.Fatalf("English words = %+v; want them as heard", got.DemoVoiceLine)
	}
}
