package parser

import (
	"strings"
	"testing"

	"github.com/rechedev9/cliphub/internal/recapplan"
)

const holdRate = 64

func walkTrail(t *holdTrail, from, to int, at func(tick int) (float64, float64)) {
	for tick := from; tick <= to; tick++ {
		x, y := at(tick)
		t.record(tick, holdRate, []holdPosition{{steamID: 7, x: x, y: y}, {steamID: 0, x: 1, y: 1}})
	}
}

func TestHoldTrailMeasuresStillnessBeforeAKill(t *testing.T) {
	kill := 10 * holdRate
	for _, tc := range []struct {
		name    string
		from    int
		at      func(int) (float64, float64)
		covered bool
		move    float64
	}{
		{"still with a jiggle", 0, func(tick int) (float64, float64) { return 100 + float64((tick/8)%3), 200 }, true, 2},
		{"walking", 0, func(tick int) (float64, float64) { return float64(tick) * 4, 0 }, true, 4 * float64(5*holdRate+holdStep(holdRate))},
		{"seen only for 3 s", kill - 3*holdRate, func(int) (float64, float64) { return 0, 0 }, false, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var trail holdTrail
			walkTrail(&trail, tc.from, kill, tc.at)
			x, y := tc.at(kill)
			h := trail.hold(7, kill, holdRate, [3]float64{x, y, 50})
			if h.Tick != kill || h.VictimSteamID64 != "7" || h.Covered != tc.covered {
				t.Fatalf("hold = %+v, want covered %v", h, tc.covered)
			}
			if tc.covered && h.MoveUnits != tc.move {
				t.Fatalf("move = %v, want %v", h.MoveUnits, tc.move)
			}
		})
	}
}

func TestHoldTrailResetsAtRoundStartAndSkipsGaps(t *testing.T) {
	kill := 10 * holdRate
	var trail holdTrail
	walkTrail(&trail, 0, kill, func(int) (float64, float64) { return 0, 0 })
	trail.reset()
	walkTrail(&trail, kill-holdRate, kill, func(int) (float64, float64) { return 0, 0 })
	if h := trail.hold(7, kill, holdRate, [3]float64{}); h.Covered {
		t.Fatalf("samples from before the round start still count: %+v", h)
	}
	var gap holdTrail
	walkTrail(&gap, 0, kill-3*holdRate, func(int) (float64, float64) { return 0, 0 })
	walkTrail(&gap, kill-holdRate, kill, func(int) (float64, float64) { return 0, 0 })
	if h := gap.hold(7, kill, holdRate, [3]float64{}); h.Covered {
		t.Fatalf("a player missing for 2 s counts as covered: %+v", h)
	}
	if h := gap.hold(8, kill, holdRate, [3]float64{}); h.Covered || h.MoveUnits != 0 {
		t.Fatalf("an unseen player counts as covered: %+v", h)
	}
}

func TestFullDemoFactsCarryHoldEvidence(t *testing.T) {
	c := NewCollector(targetID, defaultTestRules())
	c.RecordRoundStart(RoundStart{Round: 1, Tick: 1})
	c.RecordRoundLiveStart(RoundLiveStart{Round: 1, Tick: 300})
	c.RecordRoundEnd(RoundEnd{Round: 1, Tick: 700})
	empty := c.fullDemoFacts(PlanMeta{SHA256: strings.Repeat("a", 64), Tickrate: 64, DurationTicks: 900}, nil, false)
	if empty.Holds == nil || empty.Holds.Version != recapplan.HoldEvidenceVersion || empty.Holds.Kills == nil || len(empty.Holds.Kills) != 0 {
		t.Fatalf("facts without kills must still carry empty hold evidence: %+v", empty.Holds)
	}
	hold := recapplan.KillHold{Tick: 500, VictimSteamID64: "76561198000000101", MoveUnits: 3, Covered: true}
	c.RecordKillHold(hold)
	facts := c.fullDemoFacts(PlanMeta{SHA256: strings.Repeat("a", 64), Tickrate: 64, DurationTicks: 900}, nil, false)
	if facts.Holds == nil || len(facts.Holds.Kills) != 1 || facts.Holds.Kills[0] != hold {
		t.Fatalf("holds = %+v", facts.Holds)
	}
	c.resetForMatchStart()
	if after := c.fullDemoFacts(PlanMeta{SHA256: strings.Repeat("a", 64), Tickrate: 64, DurationTicks: 900}, nil, false); len(after.Holds.Kills) != 0 {
		t.Fatal("a match start must drop warmup holds")
	}
}
