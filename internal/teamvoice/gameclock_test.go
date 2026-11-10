package teamvoice

import (
	"testing"
	"time"

	"github.com/rechedev9/cliphub/internal/library"
)

func TestGameClockPlacesTheRecordingByRoundStarts(t *testing.T) {
	start := time.Date(2026, 11, 9, 20, 0, 0, 0, time.UTC)
	tl := Timeline{Rounds: []Round{{Number: 1, FreezeEndMS: 20_000}, {Number: 2, FreezeEndMS: 140_000}, {Number: 3, FreezeEndMS: 260_000}, {Number: 4, FreezeEndMS: 380_000}}}
	// The plugin started 95 s before round 1 went live, so audio 0 is demo -75 s.
	marks := func(number func(int) int, jitterMS ...int64) []library.RecordingRound {
		var out []library.RecordingRound
		for i, r := range tl.Rounds {
			at := start.Add(time.Duration(r.FreezeEndMS+95_000-20_000) * time.Millisecond)
			if i < len(jitterMS) {
				at = at.Add(time.Duration(jitterMS[i]) * time.Millisecond)
			}
			out = append(out, library.RecordingRound{Round: number(r.Number), LiveAt: at})
		}
		return out
	}
	same := func(n int) int { return n }
	got, ok := GameClock(GameClockInput{StartedAt: start, Rounds: marks(same, 120, -80, 0, 9000), Timeline: tl})
	if !ok || got.Method != library.VoiceMethodGameClock || got.State != library.VoiceAuto || got.OffsetMS != -75_010 || got.Confidence != 0.75 {
		t.Fatalf("GameClock = %+v, %v; want -75.01 s from 3 of 4 rounds", got, ok)
	}
	// CS2 counted a knife round the demo does not have.
	if shifted, ok := GameClock(GameClockInput{StartedAt: start, Rounds: marks(func(n int) int { return n + 1 }), Timeline: tl}); !ok || shifted.OffsetMS != -75_000 {
		t.Fatalf("shifted numbering = %+v, %v", shifted, ok)
	}
	if _, ok := GameClock(GameClockInput{StartedAt: start, Rounds: marks(same)[:1], Timeline: tl}); ok {
		t.Fatal("one round placed the recording")
	}
	scattered := marks(same, 0, 30_000, -50_000, 70_000)
	if _, ok := GameClock(GameClockInput{StartedAt: start, Rounds: scattered, Timeline: tl}); ok {
		t.Fatal("rounds that disagree placed the recording")
	}
}
