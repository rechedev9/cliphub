package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/rechedev9/cliphub/internal/artifacts"
	"github.com/rechedev9/cliphub/internal/tacticalplan"
	"github.com/rechedev9/cliphub/internal/teamvoice"
)

func heardLine(startMS int64, text string) teamvoice.Line {
	fields := strings.Fields(text)
	words := make([]teamvoice.Word, len(fields))
	for i, f := range fields {
		words[i] = teamvoice.Word{Text: f, StartMS: startMS + int64(i)*200, EndMS: startMS + int64(i+1)*200}
	}
	return teamvoice.Line{StartMS: startMS, EndMS: startMS + int64(len(fields))*200, Text: text, Words: words}
}

func (e demoTranscriptEnv) onMirage() demoTranscriptEnv {
	e.t.Helper()
	doc, err := e.h.readLibraryDocument(e.match.DemoID)
	if err != nil {
		e.t.Fatal(err)
	}
	doc.Demo.Map = "de_mirage"
	doc.Geometry.Callouts = []tacticalplan.Callout{{Name: "Palace", Samples: 40}, {Name: "Underpass", Samples: 30}}
	raw, err := json.Marshal(doc)
	if err != nil {
		e.t.Fatal(err)
	}
	if err := e.storage.Put(artifacts.LibraryTacticalIndexKey(e.match.DemoID), bytes.NewReader(raw)); err != nil {
		e.t.Fatal(err)
	}
	return e
}

func (e demoTranscriptEnv) correct(got demoTranscript, filter func(demoTranscriptLine) bool, text string) int {
	e.t.Helper()
	n := 0
	for _, l := range got.Lines {
		if filter(l) {
			e.json(http.MethodPatch, e.base+"/lines/"+l.ID.String(), map[string]string{"text": text}, http.StatusOK, nil)
			n++
		}
	}
	return n
}

func lineAt(t *testing.T, got demoTranscript, startMS int64) demoTranscriptLine {
	t.Helper()
	for _, l := range got.Lines {
		if l.StartMS == startMS {
			return l
		}
	}
	t.Fatalf("no frase at %d ms in %+v", startMS, got.Lines)
	return demoTranscriptLine{}
}

func newLearningEnv(t *testing.T) demoTranscriptEnv {
	t.Helper()
	env := newDemoTranscriptEnv(t).onMirage()
	env.transcriber.state = teamvoice.TranscriberInstalled
	env.h.voice.deps.VocabCorrector = true
	return env
}

func TestDemoTranscriptDoesNotMakeAChangeTheUserUndidInTheNextDemo(t *testing.T) {
	env := newLearningEnv(t)
	env.transcriber.heard = []teamvoice.Line{heardLine(1000, "moly under Palas")}
	first := env.transcribe(false)
	if call := lineAt(t, first, 1000); call.Text != "molly under Palace" {
		t.Fatalf("first demo = %+v; want the corrector to change Palas", call.DemoVoiceLine)
	}
	if n := env.correct(first, func(l demoTranscriptLine) bool { return l.SteamID64 == first.Lines[0].SteamID64 }, "molly under Palas"); n != 1 {
		t.Fatalf("undid %d frases, want one", n)
	}

	next := env.withMatch("next-demo").onMirage()
	next.transcriber.heard = []teamvoice.Line{heardLine(1000, "Palas or underpas")}
	call := lineAt(t, next.transcribe(false), 1000)
	if call.Text != "Palas or underpass" {
		t.Fatalf("next demo = %+v; want the undone change not made and the other callouts still corrected", call.DemoVoiceLine)
	}
}

func TestDemoTranscriptAppliesACorrectionMadeByHandInTwoFrases(t *testing.T) {
	env := newLearningEnv(t)
	env.transcriber.heard = []teamvoice.Line{heardLine(1000, "flesh on Palas"), heardLine(3000, "drop the vesty")}
	first := env.transcribe(false)
	if n := env.correct(first, func(l demoTranscriptLine) bool { return l.StartMS == 1000 }, "flash on palacio"); n != 2 {
		t.Fatalf("corrected %d frases, want one per player", n)
	}
	if n := env.correct(first, func(l demoTranscriptLine) bool { return l.StartMS == 3000 && l.SteamID64 == first.Lines[0].SteamID64 }, "drop the vest"); n != 1 {
		t.Fatalf("corrected %d frases, want one", n)
	}

	next := env.withMatch("next-demo").onMirage()
	next.transcriber.heard = []teamvoice.Line{heardLine(1000, "Flesh, Palas now"), heardLine(3000, "vesty here")}
	got := next.transcribe(false)
	call := lineAt(t, got, 1000)
	if call.Text != "Flash, Palacio now" || call.OriginalText != "Flesh, Palas now" || call.Corrected {
		t.Fatalf("next demo = %+v; want the correction made twice by hand applied over the map callout", call.DemoVoiceLine)
	}
	if once := lineAt(t, got, 3000); once.Text != "vesty here" || once.VocabText != "" {
		t.Fatalf("next demo = %+v; want a correction made in one frase not applied", once.DemoVoiceLine)
	}
}

func TestDemoTranscriptKeepsAnUndoAfterTheDemoIsTranscribedAgain(t *testing.T) {
	env := newLearningEnv(t)
	env.transcriber.heard = []teamvoice.Line{heardLine(1000, "moly under Palas")}
	first := env.transcribe(false)
	env.correct(first, func(l demoTranscriptLine) bool { return l.SteamID64 == first.Lines[0].SteamID64 }, "molly under Palas")
	again := lineAt(t, env.transcribe(true), 1000)
	if again.Text != "molly under Palas" || !again.Corrected {
		t.Fatalf("transcribed again after the undo = %+v; want the undo kept as the person's correction", again.DemoVoiceLine)
	}

	next := env.withMatch("next-demo").onMirage()
	next.transcriber.heard = []teamvoice.Line{heardLine(1000, "Palas or underpas")}
	if call := lineAt(t, next.transcribe(false), 1000); call.Text != "Palas or underpass" {
		t.Fatalf("next demo = %+v; want the undone change still not made", call.DemoVoiceLine)
	}
}

func TestDemoTranscriptKeepsALearnedCorrectionAfterTheDemoIsTranscribedAgain(t *testing.T) {
	a := newLearningEnv(t)
	a.transcriber.heard = []teamvoice.Line{heardLine(1000, "vesty here")}
	first := a.transcribe(false)
	a.correct(first, func(l demoTranscriptLine) bool { return l.SteamID64 == first.Lines[0].SteamID64 }, "vest here")
	b := a.withMatch("second-demo").onMirage()
	b.transcriber.heard = []teamvoice.Line{heardLine(1000, "vesty now")}
	second := b.transcribe(false)
	b.correct(second, func(l demoTranscriptLine) bool { return l.SteamID64 == second.Lines[0].SteamID64 }, "vest now")

	a.transcriber.heard = []teamvoice.Line{heardLine(1000, "vesty here")}
	if again := lineAt(t, a.transcribe(true), 1000); again.Text != "vest here" || !again.Corrected || again.OriginalText != "vesty here" {
		t.Fatalf("transcribed again = %+v; want the person's correction kept as a correction", again.DemoVoiceLine)
	}

	c := a.withMatch("third-demo").onMirage()
	c.transcriber.heard = []teamvoice.Line{heardLine(1000, "vesty go")}
	if call := lineAt(t, c.transcribe(false), 1000); call.Text != "vest go" {
		t.Fatalf("third demo = %+v; want the correction made in two demos still applied", call.DemoVoiceLine)
	}
}
