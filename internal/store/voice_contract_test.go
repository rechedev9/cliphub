package store

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/library"
)

// TestVoiceRepositoryContract runs a match's team voice against both repositories.
func TestVoiceRepositoryContract(t *testing.T) {
	ct, tside := side{clan: "FUT", score: 13}, side{clan: "OG", score: 8}
	guess := library.VoiceSync{State: library.VoiceUnsynced, Method: library.VoiceMethodRoundStart, OffsetMS: 12_000, AutoOffsetMS: 12_000}
	cases := []struct {
		name string
		run  func(t *testing.T, f libraryFixture)
	}{
		{
			name: "tracks number on, keep their speech, and a second import keeps the first sync",
			run: func(t *testing.T, f libraryFixture) {
				m := f.analyze(f.demo("voice", 0), "de_mirage", ct, tside)
				if _, err := f.repo.GetMatchVoice(f.ctx, m.ID); !errors.Is(err, library.ErrNotFound) {
					t.Fatalf("GetMatchVoice(none) = %v, want ErrNotFound", err)
				}
				if _, err := f.repo.AddVoiceTracks(f.ctx, library.VoiceAdd{MatchID: uuid.New(), Tracks: []library.VoiceTrack{{StoredKey: "x"}}}); !errors.Is(err, library.ErrNotFound) {
					t.Fatalf("AddVoiceTracks(unknown match) = %v, want ErrNotFound", err)
				}
				first, err := f.repo.AddVoiceTracks(f.ctx, library.VoiceAdd{MatchID: m.ID, Sync: guess, Tracks: []library.VoiceTrack{
					{Label: "TeamSpeak", FileName: "ts3.wav", Channel: -1, StoredKey: "library/a/voice/1.ogg", DurationMS: 90_000, SizeBytes: 10,
						Source: library.VoiceSourceUpload, Speech: []library.VoiceSpan{{StartMS: 100, EndMS: 900}}},
				}})
				if err != nil || len(first.Tracks) != 1 || first.Tracks[0].Track != 1 || first.Sync != guess || first.Transcript.State != library.TranscriptNone {
					t.Fatalf("first add = %+v, %v", first, err)
				}
				second, err := f.repo.AddVoiceTracks(f.ctx, library.VoiceAdd{MatchID: m.ID, Sync: library.VoiceSync{State: library.VoiceManual}, Tracks: []library.VoiceTrack{
					{Label: "mopoz", Channel: 0, StoredKey: "library/a/voice/2.ogg", DurationMS: 120_000},
				}})
				if err != nil || len(second.Tracks) != 2 || second.Tracks[1].Track != 2 || second.Sync != guess || second.DurationMS() != 120_000 {
					t.Fatalf("second add = %+v, %v; want track 2 and the first sync kept", second, err)
				}
				got, err := f.repo.GetMatchVoice(f.ctx, m.ID)
				if err != nil || got.Tracks[0].StoredKey != "library/a/voice/1.ogg" || len(got.Tracks[0].Speech) != 1 || got.Tracks[0].Speech[0].EndMS != 900 ||
					len(got.Tracks[1].Speech) != 0 || got.Tracks[1].Speech == nil {
					t.Fatalf("GetMatchVoice = %+v, %v", got, err)
				}
				got.Tracks[0].Speech[0].EndMS = 1
				if again, _ := f.repo.GetMatchVoice(f.ctx, m.ID); again.Tracks[0].Speech[0].EndMS != 900 {
					t.Fatal("a read shares its speech slice with the store")
				}
			},
		},
		{
			name: "sync and speakers edit in place, and an unknown track is not found",
			run: func(t *testing.T, f libraryFixture) {
				m := f.analyze(f.demo("voice", 0), "de_mirage", ct, tside)
				if _, err := f.repo.SetVoiceSync(f.ctx, m.ID, guess); !errors.Is(err, library.ErrNotFound) {
					t.Fatalf("SetVoiceSync(no voice) = %v, want ErrNotFound", err)
				}
				if _, err := f.repo.AddVoiceTracks(f.ctx, library.VoiceAdd{MatchID: m.ID, Sync: guess, Tracks: []library.VoiceTrack{{StoredKey: "k"}}}); err != nil {
					t.Fatal(err)
				}
				manual := library.VoiceSync{State: library.VoiceManual, Method: library.VoiceMethodManual, OffsetMS: -4_500, AutoOffsetMS: 12_000, Confidence: 1, TeamKey: "t-start"}
				v, err := f.repo.SetVoiceSync(f.ctx, m.ID, manual)
				if err != nil || v.Sync != manual {
					t.Fatalf("SetVoiceSync = %+v, %v", v.Sync, err)
				}
				label, steam := "mopoz", "76561198000000001"
				v, err = f.repo.UpdateVoiceTrack(f.ctx, library.VoiceTrackPatch{MatchID: m.ID, Track: 1, Label: &label, SteamID64: &steam})
				if err != nil || v.Tracks[0].Label != "mopoz" || v.Tracks[0].SteamID64 != steam {
					t.Fatalf("UpdateVoiceTrack = %+v, %v", v.Tracks, err)
				}
				if _, err := f.repo.UpdateVoiceTrack(f.ctx, library.VoiceTrackPatch{MatchID: m.ID, Track: 9, Label: &label}); !errors.Is(err, library.ErrNotFound) {
					t.Fatalf("UpdateVoiceTrack(9) = %v, want ErrNotFound", err)
				}
			},
		},
		{
			name: "transcript lines replace, search as a phrase across matches, and go with the match",
			run: func(t *testing.T, f libraryFixture) {
				d := f.demo("voice-a", 0)
				a := f.analyze(d, "de_mirage", ct, tside)
				b := f.analyze(f.demo("voice-b", 1), "de_nuke", ct, tside)
				for _, m := range []library.Match{a, b} {
					if _, err := f.repo.AddVoiceTracks(f.ctx, library.VoiceAdd{MatchID: m.ID, Sync: guess, Tracks: []library.VoiceTrack{{StoredKey: "k"}}}); err != nil {
						t.Fatal(err)
					}
				}
				at := time.Date(2026, 11, 9, 20, 0, 0, 123_456_789, time.UTC)
				lines := []library.VoiceLine{{Track: 1, StartMS: 5_000, EndMS: 6_000, Text: "Stack  B, they go B"}, {Track: 1, StartMS: 1_000, EndMS: 2_000, Text: "eco"}}
				if err := f.repo.SetVoiceTranscript(f.ctx, library.VoiceTranscriptUpdate{MatchID: a.ID,
					Transcript: library.VoiceTranscript{State: library.TranscriptReady, Language: "es", Model: "small", UpdatedAt: at}, Lines: &lines}); err != nil {
					t.Fatal(err)
				}
				other := []library.VoiceLine{{Track: 1, StartMS: 0, EndMS: 1, Text: "stack b otra vez"}}
				if err := f.repo.SetVoiceTranscript(f.ctx, library.VoiceTranscriptUpdate{MatchID: b.ID, Transcript: library.VoiceTranscript{State: library.TranscriptRunning}, Lines: &other}); err != nil {
					t.Fatal(err)
				}
				v, err := f.repo.GetMatchVoice(f.ctx, a.ID)
				if err != nil || v.Transcript.State != library.TranscriptReady || v.Transcript.Language != "es" || !v.Transcript.UpdatedAt.Equal(at.Truncate(time.Millisecond)) {
					t.Fatalf("transcript = %+v, %v", v.Transcript, err)
				}
				all, err := f.repo.ListVoiceLines(f.ctx, library.VoiceLineFilter{MatchIDs: []uuid.UUID{a.ID}})
				if err != nil || len(all) != 2 || all[0].Text != "eco" || all[1].MatchID != a.ID {
					t.Fatalf("lines of a = %+v, %v; want audio order", all, err)
				}
				for _, tc := range []struct {
					filter library.VoiceLineFilter
					want   int
				}{
					{library.VoiceLineFilter{Query: "STACK b"}, 2}, {library.VoiceLineFilter{Query: "b stack"}, 0},
					{library.VoiceLineFilter{Query: "stack b", MatchIDs: []uuid.UUID{b.ID}}, 1}, {library.VoiceLineFilter{Limit: 1}, 1},
				} {
					if got, err := f.repo.ListVoiceLines(f.ctx, tc.filter); err != nil || len(got) != tc.want {
						t.Fatalf("ListVoiceLines(%+v) = %d, %v; want %d", tc.filter, len(got), err, tc.want)
					}
				}
				running, err := f.repo.ListVoicesByTranscript(f.ctx, []string{library.TranscriptQueued, library.TranscriptRunning})
				if err != nil || len(running) != 1 || running[0] != b.ID {
					t.Fatalf("ListVoicesByTranscript = %v, %v", running, err)
				}
				if err := f.repo.SetVoiceTranscript(f.ctx, library.VoiceTranscriptUpdate{MatchID: a.ID, Transcript: library.VoiceTranscript{State: library.TranscriptQueued}}); err != nil {
					t.Fatal(err)
				}
				if kept, _ := f.repo.ListVoiceLines(f.ctx, library.VoiceLineFilter{MatchIDs: []uuid.UUID{a.ID}}); len(kept) != 2 {
					t.Fatalf("a state change without lines kept %d lines, want 2", len(kept))
				}
				sums, err := f.repo.ListVoiceSummaries(f.ctx)
				if err != nil || len(sums) != 2 {
					t.Fatalf("ListVoiceSummaries = %+v, %v; want both matches", sums, err)
				}
				for _, s := range sums {
					if s.Tracks != 1 || s.Sync != guess {
						t.Fatalf("summary %+v, want one track and the sync %+v", s, guess)
					}
				}
				if err := f.repo.DeleteDemo(f.ctx, d.ID); err != nil {
					t.Fatal(err)
				}
				if sums, err := f.repo.ListVoiceSummaries(f.ctx); err != nil || len(sums) != 1 || sums[0].MatchID != b.ID {
					t.Fatalf("summaries after the demo went = %+v, %v; want only b", sums, err)
				}
				if _, err := f.repo.GetMatchVoice(f.ctx, a.ID); !errors.Is(err, library.ErrNotFound) {
					t.Fatalf("voice after the demo went = %v, want ErrNotFound", err)
				}
				if got, _ := f.repo.ListVoiceLines(f.ctx, library.VoiceLineFilter{Query: "stack"}); len(got) != 1 || got[0].MatchID != b.ID {
					t.Fatalf("lines after the demo went = %+v", got)
				}
				if err := f.repo.DeleteMatchVoice(f.ctx, b.ID); err != nil {
					t.Fatal(err)
				}
				if _, err := f.repo.GetMatchVoice(f.ctx, b.ID); !errors.Is(err, library.ErrNotFound) {
					t.Fatalf("voice after DeleteMatchVoice = %v", err)
				}
				if sums, err := f.repo.ListVoiceSummaries(f.ctx); err != nil || len(sums) != 0 {
					t.Fatalf("summaries after DeleteMatchVoice = %+v, %v; want none", sums, err)
				}
			},
		},
	}
	for _, tc := range cases {
		for name, repo := range libraryRepositoriesUnderTest(t) {
			t.Run(tc.name+"/"+name, func(t *testing.T) {
				tc.run(t, libraryFixture{t: t, ctx: context.Background(), repo: repo})
			})
		}
	}
}
