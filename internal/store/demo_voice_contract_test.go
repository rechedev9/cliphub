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
			queued := library.DemoVoiceTranscript{State: library.TranscriptQueued}
			if err := repo.SetDemoVoice(f.ctx, library.DemoVoiceUpdate{MatchID: uuid.New(), Transcript: queued}); !errors.Is(err, library.ErrNotFound) {
				t.Fatalf("SetDemoVoice(unknown match) = %v, want ErrNotFound", err)
			}
			if err := repo.SetDemoVoice(f.ctx, library.DemoVoiceUpdate{MatchID: other.ID, Transcript: queued}); err != nil {
				t.Fatal(err)
			}
			call := library.DemoVoiceLine{ID: uuid.New(), SteamID64: "76561198000000001", Name: "ropz", TeamKey: "ct-start", StartMS: 64_000, EndMS: 66_000, Tick: 4096, Round: 2, Text: "Stack  B site"}
			eco := library.DemoVoiceLine{ID: uuid.New(), SteamID64: "76561198000000002", Name: "b1t", TeamKey: "ct-start", StartMS: 1_000, EndMS: 2_000, Tick: 64, Round: 1, Text: "eco"}
			lines := []library.DemoVoiceLine{call, eco}
			ready := library.DemoVoiceTranscript{State: library.TranscriptReady, Model: "parakeet"}
			if err := repo.SetDemoVoice(f.ctx, library.DemoVoiceUpdate{MatchID: m.ID, Transcript: ready, Lines: &lines}); err != nil {
				t.Fatal(err)
			}
			got, err := repo.ListDemoVoiceLines(f.ctx, library.DemoVoiceLineFilter{MatchIDs: []uuid.UUID{m.ID}})
			if err != nil || len(got) != 2 || got[0].ID != eco.ID || got[1].Tick != 4096 || got[1].Round != 2 || got[1].Name != "ropz" || got[1].MatchID != m.ID {
				t.Fatalf("lines = %+v, %v; want time order with player, tick and round", got, err)
			}
			fixed := "Stack B, they go B"
			line, err := repo.UpdateDemoVoiceLine(f.ctx, library.DemoVoiceLinePatch{MatchID: m.ID, ID: call.ID, Text: &fixed})
			if err != nil || line.Text != fixed || !line.Corrected || line.OriginalText != "Stack  B site" {
				t.Fatalf("correction = %+v, %v", line, err)
			}
			if _, err := repo.UpdateDemoVoiceLine(f.ctx, library.DemoVoiceLinePatch{MatchID: other.ID, ID: call.ID, Text: &fixed}); !errors.Is(err, library.ErrNotFound) {
				t.Fatalf("a correction through another match = %v, want ErrNotFound", err)
			}
			if err := repo.SetDemoVoice(f.ctx, library.DemoVoiceUpdate{MatchID: m.ID, Transcript: ready}); err != nil {
				t.Fatal(err)
			}
			got, _ = repo.ListDemoVoiceLines(f.ctx, library.DemoVoiceLineFilter{MatchIDs: []uuid.UUID{m.ID}})
			if len(got) != 2 || got[1].Text != fixed || !got[1].Corrected {
				t.Fatalf("lines after a state update = %+v; want the phrases and the correction kept", got)
			}
			if found, err := repo.ListDemoVoiceLines(f.ctx, library.DemoVoiceLineFilter{Query: "THEY go"}); err != nil || len(found) != 1 || found[0].ID != call.ID {
				t.Fatalf("search = %+v, %v; want the corrected phrase", found, err)
			}
			state, err := repo.GetDemoVoice(f.ctx, m.ID)
			if err != nil || state.State != library.TranscriptReady || state.Model != "parakeet" || state.MatchID != m.ID {
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

// TestDemoVoiceTranscriptIsReadWhole: a long match holds more phrases than one page, and All reads them all.
func TestDemoVoiceTranscriptIsReadWhole(t *testing.T) {
	for name, repo := range libraryRepositoriesUnderTest(t) {
		t.Run(name, func(t *testing.T) {
			f := libraryFixture{t: t, ctx: context.Background(), repo: repo}
			m := f.analyze(f.demo("demo-voice-long", 0), "de_mirage", side{clan: "FUT", score: 13}, side{clan: "OG", score: 8})
			lines := make([]library.DemoVoiceLine, library.MaxVoiceLineList+1)
			for i := range lines {
				lines[i] = library.DemoVoiceLine{ID: uuid.New(), SteamID64: "76561198000000001", Name: "ropz", TeamKey: "ct-start", StartMS: int64(i) * 1000, EndMS: int64(i)*1000 + 500, Tick: 64 + i*64, Round: 1, Text: "go"}
			}
			ready := library.DemoVoiceTranscript{State: library.TranscriptReady}
			if err := repo.SetDemoVoice(f.ctx, library.DemoVoiceUpdate{MatchID: m.ID, Transcript: ready, Lines: &lines}); err != nil {
				t.Fatal(err)
			}
			got, err := repo.ListDemoVoiceLines(f.ctx, library.DemoVoiceLineFilter{MatchIDs: []uuid.UUID{m.ID}, All: true})
			if err != nil || len(got) != len(lines) || got[len(got)-1].ID != lines[len(lines)-1].ID {
				t.Fatalf("whole transcript = %d phrases, %v; want all %d in time order", len(got), err, len(lines))
			}
			if page, err := repo.ListDemoVoiceLines(f.ctx, library.DemoVoiceLineFilter{MatchIDs: []uuid.UUID{m.ID}}); err != nil || len(page) != library.MaxVoiceLineList {
				t.Fatalf("one page = %d phrases, %v; want %d", len(page), err, library.MaxVoiceLineList)
			}
		})
	}
}

func TestDemoVoiceLineKeepsWhatTheTranscriberHeard(t *testing.T) {
	for name, repo := range libraryRepositoriesUnderTest(t) {
		t.Run(name, func(t *testing.T) {
			f := libraryFixture{t: t, ctx: context.Background(), repo: repo}
			m := f.analyze(f.demo("demo-voice-original", 0), "de_mirage", side{clan: "FUT", score: 13}, side{clan: "OG", score: 8})
			heard := library.DemoVoiceLine{ID: uuid.New(), SteamID64: "76561198000000001", Name: "ropz", TeamKey: "ct-start", StartMS: 1_000, EndMS: 2_000, Tick: 64, Round: 1, Text: "rotamos a ve"}
			invented := library.DemoVoiceLine{ID: uuid.New(), SteamID64: "76561198000000002", Name: "b1t", TeamKey: "ct-start", StartMS: 3_000, EndMS: 4_000, Tick: 192, Round: 1, Text: "gracias por ver"}
			ready := library.DemoVoiceTranscript{State: library.TranscriptReady}
			set := func(lines ...library.DemoVoiceLine) {
				t.Helper()
				if err := repo.SetDemoVoice(f.ctx, library.DemoVoiceUpdate{MatchID: m.ID, Transcript: ready, Lines: &lines}); err != nil {
					t.Fatal(err)
				}
			}
			patch := func(p library.DemoVoiceLinePatch) library.DemoVoiceLine {
				t.Helper()
				p.MatchID = m.ID
				line, err := repo.UpdateDemoVoiceLine(f.ctx, p)
				if err != nil {
					t.Fatal(err)
				}
				return line
			}
			stored := func() []library.DemoVoiceLine {
				t.Helper()
				got, err := repo.ListDemoVoiceLines(f.ctx, library.DemoVoiceLineFilter{MatchIDs: []uuid.UUID{m.ID}, All: true})
				if err != nil || len(got) != 2 {
					t.Fatalf("lines = %+v, %v; want both phrases", got, err)
				}
				return got
			}
			set(heard, invented)
			if got := stored(); got[0].OriginalText != "" || got[0].Corrected || got[0].Discarded || got[1].Discarded {
				t.Fatalf("fresh phrases = %+v; want neither corrected nor discarded", got)
			}

			first, second := "rotamos a B", "rotamos a B ya"
			patch(library.DemoVoiceLinePatch{ID: heard.ID, Text: &first})
			if line := patch(library.DemoVoiceLinePatch{ID: heard.ID, Text: &second}); line.Text != second || line.OriginalText != heard.Text || !line.Corrected {
				t.Fatalf("second correction = %+v; want the transcriber's text kept as the original", line)
			}
			yes, no := true, false
			if line := patch(library.DemoVoiceLinePatch{ID: invented.ID, Discarded: &yes}); !line.Discarded || line.Text != invented.Text || line.Corrected {
				t.Fatalf("discard = %+v; want the phrase discarded with its text untouched", line)
			}
			if line := patch(library.DemoVoiceLinePatch{ID: heard.ID, Discarded: &yes}); !line.Discarded || line.Text != second || line.OriginalText != heard.Text {
				t.Fatalf("discard of a corrected phrase = %+v; want the correction and the original kept", line)
			}
			if got := stored(); !got[0].Discarded || got[0].OriginalText != heard.Text || !got[1].Discarded {
				t.Fatalf("stored phrases = %+v; want the discards and the original read back", got)
			}

			set(heard, invented)
			got := stored()
			if got[0].Text != second || got[0].OriginalText != heard.Text || !got[0].Corrected || !got[0].Discarded || !got[1].Discarded || got[1].Text != invented.Text {
				t.Fatalf("phrases heard again = %+v; want the correction and the discards of unchanged ids kept", got)
			}
			if line := patch(library.DemoVoiceLinePatch{ID: invented.ID, Discarded: &no}); line.Discarded {
				t.Fatalf("restore = %+v; want the phrase back", line)
			}
			if line := patch(library.DemoVoiceLinePatch{ID: heard.ID, Text: &heard.Text}); line.Corrected || line.OriginalText != "" || line.Text != heard.Text || !line.Discarded {
				t.Fatalf("a correction back to the original = %+v; want it not corrected and still discarded", line)
			}

			changed := heard
			changed.ID, changed.Text = uuid.New(), "rotamos a B"
			patch(library.DemoVoiceLinePatch{ID: heard.ID, Text: &second})
			set(changed, invented)
			if got := stored(); got[0].ID != changed.ID || got[0].Corrected || got[0].Discarded || got[0].Text != changed.Text {
				t.Fatalf("a phrase heard differently = %+v; want it as the transcriber gave it", got[0])
			}
		})
	}
}
