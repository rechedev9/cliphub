package parser

import (
	"slices"
	"testing"

	"github.com/rechedev9/cliphub/internal/killplan"
)

// preRollPlan is a kill plan cut by Segment with the analysis defaults (3 s before, 5 s after, 8 s merge window).
func preRollPlan(kills ...RawKill) killplan.Plan {
	plan := killplan.NewPlan()
	plan.Demo.Tickrate = testTickrate
	plan.Segments = Segment(kills, nil, nil, defaultTestRules(), testTickrate)
	return plan
}

func TestWithPreRollStartsEachPlayThatLongBeforeItsFirstKill(t *testing.T) {
	plan := preRollPlan(mkKill(10000, 5, "awp"))
	for _, seconds := range []int{3, 5, 8} {
		got := WithPreRoll(plan, seconds, []RoundStart{{Round: 5, Tick: 2000}})
		if len(got.Segments) != 1 {
			t.Fatalf("%d s: %d plays, want 1", seconds, len(got.Segments))
		}
		if want := 10000 - seconds*testTickrate; got.Segments[0].TickStart != want {
			t.Errorf("%d s: TickStart = %d, want %d", seconds, got.Segments[0].TickStart, want)
		}
		if got.Segments[0].TickEnd != 10000+5*testTickrate {
			t.Errorf("%d s: TickEnd = %d, want the 5 s after the kill unchanged", seconds, got.Segments[0].TickEnd)
		}
	}
}

func TestWithPreRollNeverStartsBeforeTheRoundStart(t *testing.T) {
	plan := preRollPlan(mkKill(3000, 2, "ak47"))
	starts := []RoundStart{{Round: 2, Tick: 2800}}
	if got := WithPreRoll(plan, 8, starts).Segments[0].TickStart; got != 2800 {
		t.Errorf("8 s: TickStart = %d, want the round start 2800", got)
	}
	if got := WithPreRoll(plan, 3, starts).Segments[0].TickStart; got != 3000-3*testTickrate {
		t.Errorf("3 s: TickStart = %d, want %d", got, 3000-3*testTickrate)
	}
}

func TestWithPreRollMergesPlaysOfOneRoundThatNowOverlap(t *testing.T) {
	// 9 s apart: two plays with the analysis defaults, one once 5 s come before each kill.
	second := 10000 + 9*testTickrate
	plan := preRollPlan(mkKill(10000, 5, "awp"), mkKill(second, 5, "awp"))
	if len(plan.Segments) != 2 {
		t.Fatalf("fixture: %d plays, want 2", len(plan.Segments))
	}
	if got := WithPreRoll(plan, 3, nil); len(got.Segments) != 2 {
		t.Fatalf("3 s: %d plays, want the two of the analysis", len(got.Segments))
	}
	got := WithPreRoll(plan, 5, nil)
	if len(got.Segments) != 1 {
		t.Fatalf("5 s: %d plays, want one merged play", len(got.Segments))
	}
	merged := got.Segments[0]
	if merged.ID != "seg-001" || len(merged.Kills) != 2 || merged.TickStart != 10000-5*testTickrate || merged.TickEnd != second+5*testTickrate {
		t.Errorf("merged play = %s with %d kills over %d-%d, want seg-001 with both kills over %d-%d",
			merged.ID, len(merged.Kills), merged.TickStart, merged.TickEnd, 10000-5*testTickrate, second+5*testTickrate)
	}
	for _, ids := range [][]string{{"seg-001", "seg-002"}, {"seg-002"}, {"seg-002", "seg-001"}} {
		if got := PreRollIDs(plan, 5, ids); !slices.Equal(got, []string{"seg-001"}) {
			t.Errorf("PreRollIDs(%v) = %v, want [seg-001]", ids, got)
		}
	}
	if got := PreRollIDs(plan, 3, []string{"seg-002", "seg-001"}); !slices.Equal(got, []string{"seg-002", "seg-001"}) {
		t.Errorf("3 s: PreRollIDs = %v, want the selection as chosen", got)
	}
}

func TestWithPreRollNeverMergesPlaysOfTwoRounds(t *testing.T) {
	plan := preRollPlan(mkKill(10000, 5, "awp"), mkKill(10000+10*testTickrate, 6, "awp"))
	if len(plan.Segments) != 2 {
		t.Fatalf("fixture: %d plays, want 2", len(plan.Segments))
	}
	if got := WithPreRoll(plan, 8, nil); len(got.Segments) != 2 {
		t.Errorf("8 s: %d plays, want one per round", len(got.Segments))
	}
}
