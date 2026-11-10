package store

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/library"
)

// TestVoiceRecordingRepositoryContract runs the plugin recordings and team speakers against both repositories.
func TestVoiceRecordingRepositoryContract(t *testing.T) {
	ct, tside := side{clan: "FUT", score: 13}, side{clan: "OG", score: 8}
	start := time.Date(2026, 11, 9, 20, 0, 0, 0, time.UTC)
	rec := func(id string, at time.Time) library.VoiceRecording {
		return library.VoiceRecording{ID: id, StartedAt: at, EndedAt: at.Add(40 * time.Minute), Map: "de_mirage", Server: "Scrims", Trigger: "auto",
			ScoreCT: 13, ScoreT: 8, Tracks: []library.RecordingTrack{{File: "01-mopoz", Nickname: "mopoz", UID: "abc=", Self: true, DurationMS: 2_400_000}},
			Rounds: []library.RecordingRound{{Round: 1, LiveAt: at.Add(95 * time.Second)}}}
	}
	cases := []struct {
		name string
		run  func(t *testing.T, f libraryFixture)
	}{
		{
			name: "recordings list newest first, attach to a match, and wait again when the match goes",
			run: func(t *testing.T, f libraryFixture) {
				if err := f.repo.CreateVoiceRecording(f.ctx, rec("20261109-200000-aaaa", start)); err != nil {
					t.Fatal(err)
				}
				if err := f.repo.CreateVoiceRecording(f.ctx, rec("20261109-210000-bbbb", start.Add(time.Hour))); err != nil {
					t.Fatal(err)
				}
				if err := f.repo.CreateVoiceRecording(f.ctx, rec("20261109-200000-aaaa", start)); !errors.Is(err, library.ErrConflict) {
					t.Fatalf("second create = %v, want ErrConflict", err)
				}
				list, err := f.repo.ListVoiceRecordings(f.ctx)
				if err != nil || len(list) != 2 || list[0].ID != "20261109-210000-bbbb" || list[1].MatchID != nil {
					t.Fatalf("list = %+v, %v", list, err)
				}
				got := list[1]
				if !got.StartedAt.Equal(start) || len(got.Rounds) != 1 || !got.Rounds[0].LiveAt.Equal(start.Add(95*time.Second)) ||
					len(got.Tracks) != 1 || got.Tracks[0].UID != "abc=" || !got.Tracks[0].Self || got.ScoreCT != 13 {
					t.Fatalf("stored recording = %+v", got)
				}
				d := f.demo("rec", 0)
				m := f.analyze(d, "de_mirage", ct, tside)
				missing := uuid.New()
				got.MatchID = &missing
				if _, err := f.repo.UpdateVoiceRecording(f.ctx, got); !errors.Is(err, library.ErrNotFound) {
					t.Fatalf("attach to an unknown match = %v, want ErrNotFound", err)
				}
				got.MatchID, got.Note = &m.ID, "Asociada sola"
				got.Tracks[0].MatchTrack = 1
				attached, err := f.repo.UpdateVoiceRecording(f.ctx, got)
				if err != nil || attached.MatchID == nil || *attached.MatchID != m.ID || attached.Note != "Asociada sola" || attached.Tracks[0].MatchTrack != 1 {
					t.Fatalf("attach = %+v, %v", attached, err)
				}
				if _, err := f.repo.UpdateVoiceRecording(f.ctx, library.VoiceRecording{ID: "nope"}); !errors.Is(err, library.ErrNotFound) {
					t.Fatalf("update unknown = %v, want ErrNotFound", err)
				}
				if err := f.repo.DeleteDemo(f.ctx, d.ID); err != nil {
					t.Fatal(err)
				}
				if again, err := f.repo.GetVoiceRecording(f.ctx, got.ID); err != nil || again.MatchID != nil {
					t.Fatalf("after the match went = %+v, %v; want waiting", again, err)
				}
				if err := f.repo.DeleteVoiceRecording(f.ctx, got.ID); err != nil {
					t.Fatal(err)
				}
				if _, err := f.repo.GetVoiceRecording(f.ctx, got.ID); !errors.Is(err, library.ErrNotFound) {
					t.Fatalf("get deleted = %v", err)
				}
				if err := f.repo.DeleteVoiceRecording(f.ctx, got.ID); !errors.Is(err, library.ErrNotFound) {
					t.Fatalf("delete twice = %v", err)
				}
			},
		},
		{
			name: "speakers are kept per team, replaced in place and folded into a merged team",
			run: func(t *testing.T, f libraryFixture) {
				m := f.analyze(f.demo("spk", 0), "de_mirage", ct, tside)
				fut, og := *teamOf(t, m, "ct-start").TeamID, *teamOf(t, m, "t-start").TeamID
				if err := f.repo.SaveVoiceSpeakers(f.ctx, []library.VoiceSpeaker{{TeamID: uuid.New(), SpeakerKey: "uid:x", SteamID64: "1"}}); !errors.Is(err, library.ErrNotFound) {
					t.Fatalf("unknown team = %v, want ErrNotFound", err)
				}
				if err := f.repo.SaveVoiceSpeakers(f.ctx, []library.VoiceSpeaker{
					{TeamID: fut, SpeakerKey: "uid:a", Nickname: "mopoz", SteamID64: "76561198000000001"},
					{TeamID: og, SpeakerKey: "nick:b", Nickname: "b", SteamID64: "76561198000000002"},
				}); err != nil {
					t.Fatal(err)
				}
				if err := f.repo.SaveVoiceSpeakers(f.ctx, []library.VoiceSpeaker{{TeamID: fut, SpeakerKey: "uid:a", Nickname: "mopoz2", SteamID64: "76561198000000003"}}); err != nil {
					t.Fatal(err)
				}
				got, err := f.repo.ListVoiceSpeakers(f.ctx, []uuid.UUID{fut})
				if err != nil || len(got) != 1 || got[0].SteamID64 != "76561198000000003" || got[0].Nickname != "mopoz2" {
					t.Fatalf("speakers of FUT = %+v, %v", got, err)
				}
				if none, err := f.repo.ListVoiceSpeakers(f.ctx, nil); err != nil || len(none) != 0 {
					t.Fatalf("no teams = %+v, %v", none, err)
				}
				if _, err := f.repo.MergeTeams(f.ctx, og, fut); err != nil {
					t.Fatal(err)
				}
				merged, err := f.repo.ListVoiceSpeakers(f.ctx, []uuid.UUID{fut, og})
				if err != nil || len(merged) != 2 || merged[0].TeamID != fut || merged[1].TeamID != fut {
					t.Fatalf("after the merge = %+v, %v", merged, err)
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
