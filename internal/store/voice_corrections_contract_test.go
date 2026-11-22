package store

import (
	"context"
	"reflect"
	"testing"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/library"
	"github.com/rechedev9/cliphub/internal/streamclips"
)

func TestDemoVoiceCorrectionsAreTheCorrectedFrasesOfEveryMatch(t *testing.T) {
	for name, repo := range libraryRepositoriesUnderTest(t) {
		t.Run(name, func(t *testing.T) {
			f := libraryFixture{t: t, ctx: context.Background(), repo: repo}
			a := f.analyze(f.demo("corrections-a", 0), "de_mirage", side{clan: "FUT", score: 13}, side{clan: "OG", score: 8})
			b := f.analyze(f.demo("corrections-b", 1), "de_inferno", side{clan: "FUT", score: 13}, side{clan: "OG", score: 8})
			if got, err := repo.ListDemoVoiceCorrections(f.ctx); err != nil || len(got) != 0 {
				t.Fatalf("corrections of an empty library = %+v, %v", got, err)
			}
			line := func(startMS int64, text string) library.DemoVoiceLine {
				return library.DemoVoiceLine{ID: uuid.New(), SteamID64: "76561198000000001", Name: "ropz", TeamKey: "ct-start", StartMS: startMS, EndMS: startMS + 500, Text: text,
					Words: []library.DemoVoiceWord{{Text: text, StartMS: startMS, EndMS: startMS + 500}}}
			}
			fixed := line(1_000, "vest here")
			fixed.Corrected, fixed.OriginalText = true, "vesty here"
			undone := line(2_000, "moly")
			undone.Corrected, undone.VocabText = true, "molly"
			machine := line(3_000, "molly now")
			machine.OriginalText, machine.VocabText = "moly now", "molly now"
			hidden := line(4_000, "uh")
			hidden.Discarded = true
			other := line(500, "flash")
			other.Corrected, other.OriginalText = true, "flesh"
			for id, lines := range map[uuid.UUID][]library.DemoVoiceLine{a.ID: {machine, fixed, hidden, undone}, b.ID: {other}} {
				if err := repo.SetDemoVoice(f.ctx, library.DemoVoiceUpdate{MatchID: id, Transcript: library.DemoVoiceTranscript{State: library.TranscriptReady}, Lines: &lines}); err != nil {
					t.Fatal(err)
				}
			}

			got, err := repo.ListDemoVoiceCorrections(f.ctx)
			if err != nil {
				t.Fatal(err)
			}
			want := map[uuid.UUID][]library.VoiceCorrection{
				a.ID: {{SourceID: a.ID, Heard: "vesty here", Text: "vest here"}, {SourceID: a.ID, Heard: "moly", Vocab: "molly", Text: "moly"}},
				b.ID: {{SourceID: b.ID, Heard: "flesh", Text: "flash"}},
			}
			bySource := map[uuid.UUID][]library.VoiceCorrection{}
			for _, c := range got {
				bySource[c.SourceID] = append(bySource[c.SourceID], c)
			}
			if !reflect.DeepEqual(bySource, want) {
				t.Fatalf("corrections = %+v; want only the frases a person corrected, in demo time, with what was heard and what the corrector wrote", got)
			}
		})
	}
}

func TestStreamVoiceCorrectionsAreTheCorrectedFrasesOfEveryStream(t *testing.T) {
	for name, repo := range streamRepositoriesUnderTest(t) {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			a, b := newStreamForVoice(t, repo), newStreamForVoice(t, repo)
			if got, err := repo.ListStreamVoiceCorrections(ctx); err != nil || len(got) != 0 {
				t.Fatalf("corrections without transcripts = %+v, %v", got, err)
			}
			fixed := streamclips.VoiceLine{ID: uuid.New(), StartMS: 61_000, EndMS: 62_000, Text: "vest on moly", Corrected: true, OriginalText: "vesty on moly", VocabText: "vesty on molly"}
			heard := streamclips.VoiceLine{ID: uuid.New(), StartMS: 63_000, EndMS: 64_000, Text: "vaya tiro"}
			other := streamclips.VoiceLine{ID: uuid.New(), StartMS: 1_000, EndMS: 2_000, Text: "flash", Corrected: true, OriginalText: "flesh"}
			spans := []library.VoiceSpan{{StartMS: 0, EndMS: 70_000}}
			for id, lines := range map[uuid.UUID][]streamclips.VoiceLine{a: {heard, fixed}, b: {other}} {
				ready := streamclips.VoiceTranscript{State: library.TranscriptReady, Spans: spans}
				if err := repo.SetStreamVoice(ctx, streamclips.VoiceUpdate{JobID: id, Transcript: ready, Replaced: spans, Lines: &lines}); err != nil {
					t.Fatal(err)
				}
			}

			got, err := repo.ListStreamVoiceCorrections(ctx)
			if err != nil {
				t.Fatal(err)
			}
			want := map[uuid.UUID][]library.VoiceCorrection{
				a: {{SourceID: a, Heard: "vesty on moly", Vocab: "vesty on molly", Text: "vest on moly"}},
				b: {{SourceID: b, Heard: "flesh", Text: "flash"}},
			}
			bySource := map[uuid.UUID][]library.VoiceCorrection{}
			for _, c := range got {
				bySource[c.SourceID] = append(bySource[c.SourceID], c)
			}
			if !reflect.DeepEqual(bySource, want) {
				t.Fatalf("corrections = %+v; want only the frases a person corrected, with what was heard and what the corrector wrote", got)
			}
		})
	}
}
