package httpapi

import (
	"net/http"
	"slices"
	"testing"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/library"
	"github.com/rechedev9/cliphub/internal/teamvoice"
	"github.com/rechedev9/cliphub/internal/vocab"
)

func newStreamVocabEnv(t *testing.T) streamTranscriptEnv {
	t.Helper()
	env := newStreamTranscriptEnv(t)
	env.transcriber.state = teamvoice.TranscriberInstalled
	env.h.voice.deps.VocabCorrector = true
	return env
}

func (e streamTranscriptEnv) transcribeAt(base string, startSeconds float64, heard string) streamTranscript {
	e.t.Helper()
	e.transcriber.mu.Lock()
	e.transcriber.heard = heard
	e.transcriber.mu.Unlock()
	before := len(e.transcriber.clips())
	e.json(http.MethodPost, base+"/transcript", streamSpans{Spans: []streamSpan{{StartSeconds: startSeconds, EndSeconds: startSeconds + 5}}, Force: true}, http.StatusAccepted, nil)
	return e.await(base, func(s streamTranscript) bool {
		return s.State == library.TranscriptReady && len(e.transcriber.clips()) > before
	})
}

func (e streamTranscriptEnv) correctStream(base string, line streamTranscriptLine, text string) {
	e.t.Helper()
	e.json(http.MethodPatch, base+"/transcript/lines/"+line.ID.String(), map[string]string{"text": text}, http.StatusOK, nil)
}

func streamLineAt(t *testing.T, got streamTranscript, startMS int64) streamTranscriptLine {
	t.Helper()
	for _, l := range got.Lines {
		if l.StartMS == startMS {
			return l
		}
	}
	t.Fatalf("no frase at %d ms in %+v", startMS, got.Lines)
	return streamTranscriptLine{}
}

func TestStreamSpanIsCorrectedWithTheLexiconAndNoCallouts(t *testing.T) {
	env := newStreamVocabEnv(t)
	base := env.stream(micStream)
	got := streamLineAt(t, env.transcribeAt(base, 62, "moly to the conector"), 61_000)
	if got.Text != "molly to the conector" || got.VocabText != got.Text || got.OriginalText != "moly to the conector" || got.Corrected {
		t.Fatalf("frase = %+v; want the grenade corrected, the Mirage callout left and the heard text kept", got.VoiceLine)
	}
	if want := []vocab.Change{{Heard: "moly", Written: "molly"}}; !slices.Equal(got.VocabChanges, want) {
		t.Fatalf("vocab changes = %+v, want %+v", got.VocabChanges, want)
	}
	lines, err := env.streams.ListStreamVoiceLines(t.Context(), got.JobID)
	if err != nil || len(lines) != 1 {
		t.Fatalf("stored = %+v, %v", lines, err)
	}
	want := []library.DemoVoiceWord{{Text: "molly", StartMS: 61_000, EndMS: 61_200}, {Text: "to", StartMS: 61_200, EndMS: 61_400},
		{Text: "the", StartMS: 61_400, EndMS: 61_600}, {Text: "conector", StartMS: 61_600, EndMS: 61_800}}
	if !slices.Equal(lines[0].Words, want) {
		t.Fatalf("words = %+v; want the corrected word on the timing of the heard one", lines[0].Words)
	}
	if lines[0].ID != streamLineAt(t, env.transcribeAt(base, 62, "moly to the conector"), 61_000).ID {
		t.Fatal("the frase id changed when the corrector rewrote the frase")
	}
}

func TestStreamSpanIsNotCorrectedWithTheSwitchOff(t *testing.T) {
	env := newStreamVocabEnv(t)
	env.h.voice.deps.VocabCorrector = false
	base := env.stream(micStream)
	if got := streamLineAt(t, env.transcribeAt(base, 62, "moly to the conector"), 61_000); got.Text != "moly to the conector" || got.VocabText != "" {
		t.Fatalf("frase = %+v; want it as heard", got.VoiceLine)
	}
}

func TestStreamSpanLearnsFromStreamAndDemoCorrections(t *testing.T) {
	env := newStreamVocabEnv(t)
	base := env.stream(micStream)
	first := env.transcribeAt(base, 62, "vesty on moly")
	env.correctStream(base, streamLineAt(t, first, 61_000), "vest on moly")
	m := env.seedLibraryMatch("stream-vocab-demo", "Alpha", "Bravo")
	demo := []library.DemoVoiceLine{{ID: uuid.New(), SteamID64: "76561198000000001", Name: "ropz", StartMS: 1_000, EndMS: 2_000,
		Text: "vest here", Corrected: true, OriginalText: "vesty here"}}
	if err := env.repo.SetDemoVoice(t.Context(), library.DemoVoiceUpdate{MatchID: m.ID, Transcript: library.DemoVoiceTranscript{State: library.TranscriptReady}, Lines: &demo}); err != nil {
		t.Fatal(err)
	}

	got := streamLineAt(t, env.transcribeAt(env.stream(micStream), 202, "Vesty, moly now"), 201_000)
	if got.Text != "Vest, moly now" || got.OriginalText != "Vesty, moly now" || got.Corrected {
		t.Fatalf("next stream = %+v; want the correction made twice applied and the undone change not made", got.VoiceLine)
	}
}

func TestStreamSpanKeepsTheCorrectionOfAPerson(t *testing.T) {
	env := newStreamVocabEnv(t)
	base := env.stream(micStream)
	line := streamLineAt(t, env.transcribeAt(base, 62, "moly now"), 61_000)
	env.correctStream(base, line, "molotov now")
	got := streamLineAt(t, env.transcribeAt(base, 62, "moly now"), 61_000)
	if got.ID != line.ID || got.Text != "molotov now" || !got.Corrected || got.OriginalText != "moly now" || got.VocabText != "molly now" {
		t.Fatalf("after transcribing again = %+v; want the person's correction kept", got.VoiceLine)
	}
}
