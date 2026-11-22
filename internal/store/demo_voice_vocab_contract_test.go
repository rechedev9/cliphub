package store

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/library"
)

func TestDemoVoiceLineKeepsWhatTheCorrectorWrote(t *testing.T) {
	for name, repo := range libraryRepositoriesUnderTest(t) {
		t.Run(name, func(t *testing.T) {
			f := libraryFixture{t: t, ctx: context.Background(), repo: repo}
			m := f.analyze(f.demo("demo-voice-vocab", 0), "de_mirage", side{clan: "FUT", score: 13}, side{clan: "OG", score: 8})
			heard, written := "moly under Palas", "moly under Palace"
			fixed := library.DemoVoiceLine{ID: uuid.New(), SteamID64: "76561198000000001", Name: "ropz", TeamKey: "ct-start", StartMS: 1_000, EndMS: 2_000, Tick: 64, Round: 1,
				Text: written, OriginalText: heard, VocabText: written}
			old := library.DemoVoiceLine{ID: uuid.New(), SteamID64: "76561198000000002", Name: "b1t", TeamKey: "ct-start", StartMS: 3_000, EndMS: 4_000, Tick: 192, Round: 1, Text: "eco"}
			ready := library.DemoVoiceTranscript{State: library.TranscriptReady}
			set := func() {
				t.Helper()
				lines := []library.DemoVoiceLine{fixed, old}
				if err := repo.SetDemoVoice(f.ctx, library.DemoVoiceUpdate{MatchID: m.ID, Transcript: ready, Lines: &lines}); err != nil {
					t.Fatal(err)
				}
			}
			call := func() library.DemoVoiceLine {
				t.Helper()
				got, err := repo.ListDemoVoiceLines(f.ctx, library.DemoVoiceLineFilter{MatchIDs: []uuid.UUID{m.ID}, All: true})
				if err != nil || len(got) != 2 {
					t.Fatalf("lines = %+v, %v", got, err)
				}
				if got[1].VocabText != "" || got[1].OriginalText != "" || got[1].Text != "eco" {
					t.Fatalf("a frase the corrector left = %+v; want it unchanged", got[1])
				}
				return got[0]
			}
			patch := func(text string) library.DemoVoiceLine {
				t.Helper()
				line, err := repo.UpdateDemoVoiceLine(f.ctx, library.DemoVoiceLinePatch{MatchID: m.ID, ID: fixed.ID, Text: &text})
				if err != nil {
					t.Fatal(err)
				}
				return line
			}

			set()
			if got := call(); got.Text != written || got.VocabText != written || got.OriginalText != heard || got.Corrected {
				t.Fatalf("stored = %+v; want the corrector's text with the heard text beside it", got)
			}
			if got := patch(heard); got.Text != heard || !got.Corrected || got.OriginalText != "" || got.VocabText != written {
				t.Fatalf("undo of the corrector = %+v; want the heard text as a person's correction", got)
			}
			set()
			if got := call(); got.Text != heard || !got.Corrected || got.HeardText() != heard {
				t.Fatalf("transcribed again after an undo = %+v; want the person's text kept", got)
			}
			if got := patch(written); got.Corrected || got.OriginalText != heard || got.VocabText != written {
				t.Fatalf("back to the corrector's text = %+v; want it not corrected", got)
			}
			if got := patch("molly under palace"); !got.Corrected || got.OriginalText != heard {
				t.Fatalf("a person's correction = %+v; want it corrected over the heard text", got)
			}
			set()
			if got := call(); got.Text != "molly under palace" || !got.Corrected || got.OriginalText != heard || got.VocabText != written {
				t.Fatalf("transcribed again after a correction = %+v; want the person's text kept", got)
			}
		})
	}
}
