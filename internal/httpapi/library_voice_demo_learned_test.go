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
	if call := lineAt(t, first, 1000); call.Text != "moly under Palace" {
		t.Fatalf("first demo = %+v; want the corrector to change Palas", call.DemoVoiceLine)
	}
	if n := env.correct(first, func(l demoTranscriptLine) bool { return l.SteamID64 == first.Lines[0].SteamID64 }, "moly under Palas"); n != 1 {
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
