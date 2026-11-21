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
	want := []Line{
		{StartMS: 1050, EndMS: 1800, Text: "Dos en B.", Words: []Word{{Text: "Dos", StartMS: 1050, EndMS: 1300}, {Text: "en", StartMS: 1350, EndMS: 1500}, {Text: "B.", StartMS: 1550, EndMS: 1800}}},
		{StartMS: 2450, EndMS: 2700, Text: "Rotad", Words: []Word{{Text: "Rotad", StartMS: 2450, EndMS: 2700}}},
	}
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

type recordingDecoder struct {
	pcm []byte
	job PCMJob
}

func (d *recordingDecoder) RawPCM(ctx context.Context, job PCMJob) error {
	d.job = job
	return fakeDecoder(d.pcm).RawPCM(ctx, job)
}

func TestTranscribeOfAStretchReadsOneTrackAndAnswersInSourceTime(t *testing.T) {
	cli, model := fakeTool(t)
	speech := []library.VoiceSpan{{StartMS: 1000, EndMS: 2600}}
	words, _ := json.Marshal([][]fakeWord{{{"Vaya", 0.25, 0.5}, {"tiro", 0.55, 0.9}}})
	t.Setenv(fakeNemoEnv, string(words))
	media := &recordingDecoder{pcm: pcm(12_000, speech)}
	tr := NewTranscriber(TranscriberOptions{CLI: cli, Model: model, Media: media})
	clip := library.VoiceSpan{StartMS: 60_000, EndMS: 72_000}
	lines, err := tr.Transcribe(context.Background(), TranscribeJob{Audio: "stream.mkv", Speech: speech, Track: 1, Clip: &clip})
	if err != nil {
		t.Fatal(err)
	}
	want := []Line{{StartMS: 61_050, EndMS: 61_700, Text: "Vaya tiro", Words: []Word{{Text: "Vaya", StartMS: 61_050, EndMS: 61_300}, {Text: "tiro", StartMS: 61_350, EndMS: 61_700}}}}
	if !reflect.DeepEqual(lines, want) {
		t.Fatalf("lines = %+v, want %+v on the clock of the source", lines, want)
	}
	if media.job.Src != "stream.mkv" || media.job.Track != 1 || media.job.Clip == nil || *media.job.Clip != clip {
		t.Fatalf("decoded %+v, want track 1 and only the stretch", media.job)
	}
}
