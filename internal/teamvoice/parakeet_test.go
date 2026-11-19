package teamvoice

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/rechedev9/cliphub/internal/library"
)

func TestTranscribeReadsSpeechRegionsAndDropsWhatHasNoSpeech(t *testing.T) {
	cli, model := fakeTool(t)
	speech := []library.VoiceSpan{{StartMS: 1000, EndMS: 2600}, {StartMS: 8000, EndMS: 9000}}
	// Each region is cut 200 ms early: the first starts at 800 ms of the track, the second at 7800 ms.
	words, _ := json.Marshal([][]fakeWord{
		{{"Dos", 0.25, 0.5}, {"en", 0.55, 0.7}, {"B.", 0.75, 1.0}, {"Rotad", 1.65, 1.9}},
		{{"vale", 0, 0.15}, {"...", 0.9, 1.0}},
	})
	t.Setenv(fakeNemoEnv, string(words))
	tr := NewTranscriber(TranscriberOptions{CLI: cli, Model: model, Media: fakeDecoder(pcm(12_000, speech))})
	lines, err := tr.Transcribe(context.Background(), TranscribeJob{Audio: "track.ogg", Speech: speech})
	if err != nil {
		t.Fatal(err)
	}
	want := []Line{{StartMS: 1050, EndMS: 1800, Text: "Dos en B."}, {StartMS: 2450, EndMS: 2700, Text: "Rotad"}}
	if !reflect.DeepEqual(lines, want) {
		t.Fatalf("lines = %+v, want %+v: a pause splits a phrase, a word over silence and bare punctuation go", lines, want)
	}
}

func TestTranscribeOfSilenceRunsNothing(t *testing.T) {
	cli, model := fakeTool(t)
	t.Setenv(fakeNemoEnv, "fail")
	tr := NewTranscriber(TranscriberOptions{CLI: cli, Model: model, Media: fakeDecoder(pcm(5_000, nil))})
	lines, err := tr.Transcribe(context.Background(), TranscribeJob{Audio: "quiet.ogg"})
	if err != nil || len(lines) != 0 {
		t.Fatalf("silence = %+v, %v; want no phrase and no run", lines, err)
	}
}

func TestTranscribeReportsAFailedRun(t *testing.T) {
	cli, model := fakeTool(t)
	t.Setenv(fakeNemoEnv, "fail")
	speech := []library.VoiceSpan{{StartMS: 500, EndMS: 1500}}
	tr := NewTranscriber(TranscriberOptions{CLI: cli, Model: model, Media: fakeDecoder(pcm(3_000, speech))})
	if _, err := tr.Transcribe(context.Background(), TranscribeJob{Audio: "a.ogg", Speech: speech}); err == nil || !strings.Contains(err.Error(), "model load failed") {
		t.Fatalf("a failed run = %v, want its output in the error", err)
	}
}

func TestTranslateGivesEachPhraseItsEnglish(t *testing.T) {
	cli, model := fakeTool(t)
	t.Setenv(fakeWhisperEnv, "two on B")
	spans := []library.VoiceSpan{{StartMS: 1000, EndMS: 2000}, {StartMS: 5000, EndMS: 6000}}
	tr := NewTranslator(TranscriberOptions{CLI: cli, Model: model, Media: fakeDecoder(pcm(8_000, spans))})
	got, err := tr.Translate(context.Background(), TranslateJob{Audio: "a.ogg", Spans: spans})
	if err != nil || !reflect.DeepEqual(got, []string{"two on B", "two on B"}) {
		t.Fatalf("Translate = %q, %v", got, err)
	}
}
