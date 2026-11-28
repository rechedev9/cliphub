package recapplan

import (
	"testing"

	"github.com/rechedev9/cliphub/internal/killplan"
)

func TestAOneRoundVideoEndsWithTheRoundWithoutTheFinalScoreboard(t *testing.T) {
	for _, tc := range []struct {
		name       string
		rounds     []int
		scoreboard bool
		end        int
		reason     string
	}{
		{"the whole match keeps the scoreboard after the last kill", nil, true, 19800, "scoreboard-tail"},
		{"several rounds keep the scoreboard after the last kill", []int{2, 25}, true, 19800, "scoreboard-tail"},
		{"one round ends with its own round tail", []int{25}, false, 19200, "round-tail"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			o := DefaultOptions()
			o.Rounds = tc.rounds
			canonical, err := CanonicalNewOptions(o)
			if err != nil {
				t.Fatal(err)
			}
			if canonical.Overlays.Scoreboard != tc.scoreboard {
				t.Fatalf("scoreboard = %v for rounds %v, want %v", canonical.Overlays.Scoreboard, tc.rounds, tc.scoreboard)
			}
			if err := canonical.ValidateCurrentFullDemoPolicy(); err != nil {
				t.Fatalf("the current policy refuses its own options: %v", err)
			}
			f := fixtureFacts()
			f.Rounds[2].Kills = []killplan.Kill{{Tick: 17800}, {Tick: 18900}}
			d, err := Plan(f, canonical, VoiceEvidence{Availability: "no_packets"}, nil, "facts")
			if err != nil {
				t.Fatal(err)
			}
			last := d.Rounds[len(d.Rounds)-1]
			if last.Number != 25 || last.RequestedEndTick != tc.end || last.CaptureEndTick != tc.end || last.EndReason != tc.reason {
				t.Fatalf("last round %d end=%d reason=%q, want 25 %d %q", last.Number, last.RequestedEndTick, last.EndReason, tc.end, tc.reason)
			}
		})
	}
}

func TestAOneRoundPlanWithTheFinalScoreboardIsPreparedAgain(t *testing.T) {
	o := DefaultOptions()
	o.Rounds = []int{25}
	canonical, err := CanonicalNewOptions(o)
	if err != nil {
		t.Fatal(err)
	}
	canonical.Overlays.Scoreboard = true
	if err := canonical.ValidateCurrentFullDemoPolicy(); err == nil {
		t.Fatal("a one round plan approved with the final scoreboard still passes the execution gate")
	}
}
