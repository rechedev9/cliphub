package store

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/library"
)

// TestDemoVoiceRepositoryContract runs the transcript of a demo's own voices against both repositories.
func TestDemoVoiceRepositoryContract(t *testing.T) {
	ct, tside := side{clan: "FUT", score: 13}, side{clan: "OG", score: 8}
	for name, repo := range libraryRepositoriesUnderTest(t) {
		t.Run(name, func(t *testing.T) {
			f := libraryFixture{t: t, ctx: context.Background(), repo: repo}
			d := f.demo("demo-voice", 0)
			m := f.analyze(d, "de_mirage", ct, tside)
			other := f.analyze(f.demo("demo-voice-b", 1), "de_nuke", ct, tside)
			if _, err := repo.GetDemoVoice(f.ctx, m.ID); !errors.Is(err, library.ErrNotFound) {
				t.Fatalf("GetDemoVoice(none) = %v, want ErrNotFound", err)
			}
			queued := library.DemoVoiceTranscript{State: library.TranscriptQueued, English: library.TranscriptNone}
			if err := repo.SetDemoVoice(f.ctx, library.DemoVoiceUpdate{MatchID: uuid.New(), Transcript: queued}); !errors.Is(err, library.ErrNotFound) {
				t.Fatalf("SetDemoVoice(unknown match) = %v, want ErrNotFound", err)
			}
			if err := repo.SetDemoVoice(f.ctx, library.DemoVoiceUpdate{MatchID: other.ID, Transcript: queued}); err != nil {
				t.Fatal(err)
			}
			call := library.DemoVoiceLine{ID: uuid.New(), SteamID64: "76561198000000001", Name: "ropz", TeamKey: "ct-start", StartMS: 64_000, EndMS: 66_000, Tick: 4096, Round: 2, Text: "Stack  B site"}
			eco := library.DemoVoiceLine{ID: uuid.New(), SteamID64: "76561198000000002", Name: "b1t", TeamKey: "ct-start", StartMS: 1_000, EndMS: 2_000, Tick: 64, Round: 1, Text: "eco"}
			lines := []library.DemoVoiceLine{call, eco}
			ready := library.DemoVoiceTranscript{State: library.TranscriptReady, Model: "parakeet", English: library.TranscriptNone}
			if err := repo.SetDemoVoice(f.ctx, library.DemoVoiceUpdate{MatchID: m.ID, Transcript: ready, Lines: &lines}); err != nil {
				t.Fatal(err)
			}
			got, err := repo.ListDemoVoiceLines(f.ctx, library.DemoVoiceLineFilter{MatchIDs: []uuid.UUID{m.ID}})
			if err != nil || len(got) != 2 || got[0].ID != eco.ID || got[1].Tick != 4096 || got[1].Round != 2 || got[1].Name != "ropz" || got[1].MatchID != m.ID {
				t.Fatalf("lines = %+v, %v; want time order with player, tick and round", got, err)
			}
			fixed := "Stack B, they go B"
			line, err := repo.UpdateDemoVoiceLine(f.ctx, library.DemoVoiceLinePatch{MatchID: m.ID, ID: call.ID, Text: &fixed})
			if err != nil || line.Text != fixed || !line.Corrected {
				t.Fatalf("correction = %+v, %v", line, err)
			}
			if _, err := repo.UpdateDemoVoiceLine(f.ctx, library.DemoVoiceLinePatch{MatchID: other.ID, ID: call.ID, Text: &fixed}); !errors.Is(err, library.ErrNotFound) {
				t.Fatalf("a correction through another match = %v, want ErrNotFound", err)
			}
			english := library.DemoVoiceTranscript{State: library.TranscriptReady, Model: "parakeet", English: library.TranscriptReady}
			if err := repo.SetDemoVoice(f.ctx, library.DemoVoiceUpdate{MatchID: m.ID, Transcript: english, English: map[uuid.UUID]string{eco.ID: "save round"}}); err != nil {
				t.Fatal(err)
			}
			got, _ = repo.ListDemoVoiceLines(f.ctx, library.DemoVoiceLineFilter{MatchIDs: []uuid.UUID{m.ID}})
			if len(got) != 2 || got[0].English != "save round" || got[1].English != "" || got[1].Text != fixed {
				t.Fatalf("lines after the translation = %+v; want the English beside the corrected text", got)
			}
			if found, err := repo.ListDemoVoiceLines(f.ctx, library.DemoVoiceLineFilter{Query: "THEY go"}); err != nil || len(found) != 1 || found[0].ID != call.ID {
				t.Fatalf("search = %+v, %v; want the corrected phrase", found, err)
			}
			state, err := repo.GetDemoVoice(f.ctx, m.ID)
			if err != nil || state.State != library.TranscriptReady || state.English != library.TranscriptReady || state.Model != "parakeet" || state.MatchID != m.ID {
				t.Fatalf("GetDemoVoice = %+v, %v", state, err)
			}
			pending, err := repo.ListDemoVoices(f.ctx, []string{library.TranscriptQueued, library.TranscriptRunning})
			if err != nil || len(pending) != 1 || pending[0].MatchID != other.ID {
				t.Fatalf("ListDemoVoices = %+v, %v; want the queued match", pending, err)
			}
			if err := repo.DeleteDemo(f.ctx, d.ID); err != nil {
				t.Fatal(err)
			}
			if _, err := repo.GetDemoVoice(f.ctx, m.ID); !errors.Is(err, library.ErrNotFound) {
				t.Fatalf("transcript after the demo went = %v, want ErrNotFound", err)
			}
			if left, _ := repo.ListDemoVoiceLines(f.ctx, library.DemoVoiceLineFilter{}); len(left) != 0 {
				t.Fatalf("lines after the demo went = %+v", left)
			}
		})
	}
}
