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

func TestDemoVoiceLineUndoSurvivesANewCorrectorChange(t *testing.T) {
	for name, repo := range libraryRepositoriesUnderTest(t) {
		t.Run(name, func(t *testing.T) {
			f := libraryFixture{t: t, ctx: context.Background(), repo: repo}
			m := f.analyze(f.demo("demo-voice-undo-new", 0), "de_mirage", side{clan: "FUT", score: 13}, side{clan: "OG", score: 8})
			id, heard, first := uuid.New(), "A V P don't take", "A V Plant take"
			set := func(vocabText string) {
				t.Helper()
				l := library.DemoVoiceLine{ID: id, SteamID64: "76561198000000001", Name: "ropz", TeamKey: "t-start", StartMS: 1_000, EndMS: 2_000, Text: heard}
				if vocabText != "" {
					l.Text, l.OriginalText, l.VocabText = vocabText, heard, vocabText
				}
				lines := []library.DemoVoiceLine{l}
				if err := repo.SetDemoVoice(f.ctx, library.DemoVoiceUpdate{MatchID: m.ID, Transcript: library.DemoVoiceTranscript{State: library.TranscriptReady}, Lines: &lines}); err != nil {
					t.Fatal(err)
				}
			}
			set(first)
			undone := heard
			if _, err := repo.UpdateDemoVoiceLine(f.ctx, library.DemoVoiceLinePatch{MatchID: m.ID, ID: id, Text: &undone}); err != nil {
				t.Fatal(err)
			}
			set("A V P don't took")
			got, err := repo.ListDemoVoiceCorrections(f.ctx)
			if err != nil || len(got) != 1 {
				t.Fatalf("corrections = %+v, %v", got, err)
			}
			if c := got[0]; c.Heard != heard || c.Vocab != first || c.Text != heard {
				t.Fatalf("an undo after a new corrector change = %+v; want it still the undo of %q", c, first)
			}
		})
	}
}

func TestDemoVoiceLineStaysAPersonsCorrectionWhenATranscriptionAgreesWithIt(t *testing.T) {
	for name, repo := range libraryRepositoriesUnderTest(t) {
		t.Run(name, func(t *testing.T) {
			f := libraryFixture{t: t, ctx: context.Background(), repo: repo}
			m := f.analyze(f.demo("demo-voice-agrees", 0), "de_mirage", side{clan: "FUT", score: 13}, side{clan: "OG", score: 8})
			words := []library.DemoVoiceWord{{Text: "moly", StartMS: 1_000, EndMS: 1_300}, {Text: "under", StartMS: 1_300, EndMS: 1_600}, {Text: "Palas", StartMS: 1_600, EndMS: 2_000}}
			undoID, learnedID := uuid.New(), uuid.New()
			set := func(lines ...library.DemoVoiceLine) {
				t.Helper()
				if err := repo.SetDemoVoice(f.ctx, library.DemoVoiceUpdate{MatchID: m.ID, Transcript: library.DemoVoiceTranscript{State: library.TranscriptReady}, Lines: &lines}); err != nil {
					t.Fatal(err)
				}
			}
			line := func(id uuid.UUID, startMS int64, text string) library.DemoVoiceLine {
				return library.DemoVoiceLine{ID: id, SteamID64: "76561198000000001", Name: "ropz", TeamKey: "ct-start", StartMS: startMS, EndMS: startMS + 1_000, Text: text}
			}
			undo := line(undoID, 1_000, "moly under Palace")
			undo.OriginalText, undo.VocabText, undo.Words = "moly under Palas", "moly under Palace", words
			set(undo, line(learnedID, 3_000, "flesh now"))
			undone, fixed := "moly under Palas", "flash now"
			for _, p := range []library.DemoVoiceLinePatch{{ID: undoID, Text: &undone}, {ID: learnedID, Text: &fixed}} {
				p.MatchID = m.ID
				if _, err := repo.UpdateDemoVoiceLine(f.ctx, p); err != nil {
					t.Fatal(err)
				}
			}

			heard := line(undoID, 1_000, "moly under Palas")
			heard.Words = words
			learned := line(learnedID, 3_000, "flash now")
			learned.OriginalText, learned.VocabText = "flesh now", "flash now"
			set(heard, learned)
			got, err := repo.ListDemoVoiceLines(f.ctx, library.DemoVoiceLineFilter{MatchIDs: []uuid.UUID{m.ID}, All: true})
			if err != nil || len(got) != 2 {
				t.Fatalf("lines = %+v, %v", got, err)
			}
			if u := got[0]; u.Text != "moly under Palas" || !u.Corrected || u.VocabText != "moly under Palace" || len(u.Words) != 3 || u.Words[2] != words[2] {
				t.Fatalf("an undo the new transcription agrees with = %+v; want it kept as the person's correction of the corrector's text, timings kept", u)
			}
			if l := got[1]; l.Text != "flash now" || !l.Corrected || l.OriginalText != "flesh now" || l.VocabText != "" {
				t.Fatalf("a correction a learned rule now writes = %+v; want it kept as the person's correction of the heard text", l)
			}
		})
	}
}
