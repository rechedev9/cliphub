package store

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/library"
	"github.com/rechedev9/cliphub/internal/streamclips"
)

func TestDemoVoiceAssistantCorrectionsAreAllOrNothingAndNotLearned(t *testing.T) {
	for name, repo := range libraryRepositoriesUnderTest(t) {
		t.Run(name, func(t *testing.T) {
			f := libraryFixture{t: t, ctx: context.Background(), repo: repo}
			m := f.analyze(f.demo("demo-voice-assistant", 0), "de_mirage", side{clan: "FUT", score: 13}, side{clan: "OG", score: 8})
			first := library.DemoVoiceLine{ID: uuid.New(), SteamID64: "76561198000000001", Name: "ropz", TeamKey: "ct-start", StartMS: 1_000, EndMS: 2_000, Tick: 64, Round: 1, Text: "flesh on Palas"}
			second := library.DemoVoiceLine{ID: uuid.New(), SteamID64: "76561198000000002", Name: "b1t", TeamKey: "ct-start", StartMS: 3_000, EndMS: 4_000, Tick: 192, Round: 1, Text: "drop the vesty"}
			set := func() {
				t.Helper()
				lines := []library.DemoVoiceLine{first, second}
				if err := repo.SetDemoVoice(f.ctx, library.DemoVoiceUpdate{MatchID: m.ID, Transcript: library.DemoVoiceTranscript{State: library.TranscriptReady}, Lines: &lines}); err != nil {
					t.Fatal(err)
				}
			}
			lines := func() []library.DemoVoiceLine {
				t.Helper()
				got, err := repo.ListDemoVoiceLines(f.ctx, library.DemoVoiceLineFilter{MatchIDs: []uuid.UUID{m.ID}, All: true})
				if err != nil || len(got) != 2 {
					t.Fatalf("lines = %+v, %v", got, err)
				}
				return got
			}
			learned := func() int {
				t.Helper()
				got, err := repo.ListDemoVoiceCorrections(f.ctx)
				if err != nil {
					t.Fatal(err)
				}
				return len(got)
			}
			set()

			bad := library.DemoVoiceCorrections{MatchID: m.ID, By: library.CorrectedByAssistant, Lines: []library.VoiceLineText{{ID: first.ID, Text: "flash on Palace"}, {ID: uuid.New(), Text: "x"}}}
			if _, err := repo.CorrectDemoVoiceLines(f.ctx, bad); !errors.Is(err, library.ErrNotFound) {
				t.Fatalf("a batch with an unknown phrase = %v, want ErrNotFound", err)
			}
			if got := lines(); got[0].Text != first.Text || got[0].Corrected {
				t.Fatalf("after a refused batch = %+v; want nothing written", got[0])
			}

			batch := library.DemoVoiceCorrections{MatchID: m.ID, By: library.CorrectedByAssistant, Lines: []library.VoiceLineText{{ID: first.ID, Text: "flash on Palace"}, {ID: second.ID, Text: "drop the vest"}}}
			done, err := repo.CorrectDemoVoiceLines(f.ctx, batch)
			if err != nil || len(done) != 2 || done[0].Text != "flash on Palace" || done[1].CorrectedBy != library.CorrectedByAssistant {
				t.Fatalf("batch = %+v, %v", done, err)
			}
			for _, l := range lines() {
				if !l.Corrected || l.CorrectedBy != library.CorrectedByAssistant || l.OriginalText == "" {
					t.Fatalf("stored = %+v; want an assistant correction with the heard text beside it", l)
				}
			}
			if n := learned(); n != 0 {
				t.Fatalf("the learner reads %d assistant corrections, want none", n)
			}

			set()
			if got := lines(); got[0].Text != "flash on Palace" || got[0].CorrectedBy != library.CorrectedByAssistant {
				t.Fatalf("transcribed again = %+v; want the assistant correction kept as the assistant's", got[0])
			}

			text := "flash on Palace now"
			hand, err := repo.UpdateDemoVoiceLine(f.ctx, library.DemoVoiceLinePatch{MatchID: m.ID, ID: first.ID, Text: &text, By: library.CorrectedByUser})
			if err != nil || hand.CorrectedBy != library.CorrectedByUser {
				t.Fatalf("hand correction = %+v, %v; want it the user's", hand, err)
			}
			if n := learned(); n != 1 {
				t.Fatalf("the learner reads %d corrections, want the one by hand", n)
			}
			same := library.DemoVoiceCorrections{MatchID: m.ID, By: library.CorrectedByAssistant, Lines: []library.VoiceLineText{{ID: first.ID, Text: text}}}
			if kept, err := repo.CorrectDemoVoiceLines(f.ctx, same); err != nil || kept[0].CorrectedBy != library.CorrectedByUser || learned() != 1 {
				t.Fatalf("an assistant batch that keeps a hand correction's text = %+v, %v; want it still the user's and learned", kept, err)
			}

			undo := second.Text
			back, err := repo.UpdateDemoVoiceLine(f.ctx, library.DemoVoiceLinePatch{MatchID: m.ID, ID: second.ID, Text: &undo, By: library.CorrectedByUser})
			if err != nil || back.Corrected || back.CorrectedBy != "" || back.OriginalText != "" {
				t.Fatalf("undo = %+v, %v; want the heard text and no author", back, err)
			}
		})
	}
}

func TestStreamVoiceAssistantCorrectionsAreAllOrNothingAndNotLearned(t *testing.T) {
	for name, repo := range streamRepositoriesUnderTest(t) {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			id := newStreamForVoice(t, repo)
			first := streamclips.VoiceLine{ID: uuid.New(), StartMS: 61_000, EndMS: 62_000, Text: "moly on the left"}
			second := streamclips.VoiceLine{ID: uuid.New(), StartMS: 63_000, EndMS: 64_000, Text: "vaya tiro"}
			spans := []library.VoiceSpan{{StartMS: 60_000, EndMS: 70_000}}
			set := func() {
				t.Helper()
				lines := []streamclips.VoiceLine{first, second}
				ready := streamclips.VoiceTranscript{State: library.TranscriptReady, Spans: spans}
				if err := repo.SetStreamVoice(ctx, streamclips.VoiceUpdate{JobID: id, Transcript: ready, Replaced: spans, Lines: &lines}); err != nil {
					t.Fatal(err)
				}
			}
			lines := func() []streamclips.VoiceLine {
				t.Helper()
				got, err := repo.ListStreamVoiceLines(ctx, id)
				if err != nil || len(got) != 2 {
					t.Fatalf("lines = %+v, %v", got, err)
				}
				return got
			}
			learned := func() int {
				t.Helper()
				got, err := repo.ListStreamVoiceCorrections(ctx)
				if err != nil {
					t.Fatal(err)
				}
				return len(got)
			}
			set()

			bad := streamclips.VoiceCorrections{JobID: id, By: library.CorrectedByAssistant, Lines: []library.VoiceLineText{{ID: first.ID, Text: "molly on the left"}, {ID: uuid.New(), Text: "x"}}}
			if _, err := repo.CorrectStreamVoiceLines(ctx, bad); !errors.Is(err, streamclips.ErrVoiceLineNotFound) {
				t.Fatalf("a batch with an unknown phrase = %v, want ErrVoiceLineNotFound", err)
			}
			if got := lines(); got[0].Text != first.Text || got[0].Corrected {
				t.Fatalf("after a refused batch = %+v; want nothing written", got[0])
			}

			batch := streamclips.VoiceCorrections{JobID: id, By: library.CorrectedByAssistant, Lines: []library.VoiceLineText{{ID: first.ID, Text: "molly on the left"}, {ID: second.ID, Text: "vaya tiro, tío"}}}
			if done, err := repo.CorrectStreamVoiceLines(ctx, batch); err != nil || len(done) != 2 || done[1].Text != "vaya tiro, tío" {
				t.Fatalf("batch = %+v, %v", done, err)
			}
			for _, l := range lines() {
				if !l.Corrected || l.CorrectedBy != library.CorrectedByAssistant {
					t.Fatalf("stored = %+v; want an assistant correction", l)
				}
			}
			if n := learned(); n != 0 {
				t.Fatalf("the learner reads %d assistant corrections, want none", n)
			}
			set()
			if got := lines(); got[0].Text != "molly on the left" || got[0].CorrectedBy != library.CorrectedByAssistant {
				t.Fatalf("transcribed again = %+v; want the assistant correction kept as the assistant's", got[0])
			}

			text := "molotov on the left"
			hand, err := repo.UpdateStreamVoiceLine(ctx, streamclips.VoiceLinePatch{JobID: id, ID: first.ID, Text: &text, By: library.CorrectedByUser})
			if err != nil || hand.CorrectedBy != library.CorrectedByUser {
				t.Fatalf("hand correction = %+v, %v; want it the user's", hand, err)
			}
			if n := learned(); n != 1 {
				t.Fatalf("the learner reads %d corrections, want the one by hand", n)
			}
			same := streamclips.VoiceCorrections{JobID: id, By: library.CorrectedByAssistant, Lines: []library.VoiceLineText{{ID: first.ID, Text: text}}}
			if kept, err := repo.CorrectStreamVoiceLines(ctx, same); err != nil || kept[0].CorrectedBy != library.CorrectedByUser || learned() != 1 {
				t.Fatalf("an assistant batch that keeps a hand correction's text = %+v, %v; want it still the user's and learned", kept, err)
			}
			undo := second.Text
			if back, err := repo.UpdateStreamVoiceLine(ctx, streamclips.VoiceLinePatch{JobID: id, ID: second.ID, Text: &undo}); err != nil || back.Corrected || back.CorrectedBy != "" {
				t.Fatalf("undo = %+v, %v; want the heard text and no author", back, err)
			}
		})
	}
}
