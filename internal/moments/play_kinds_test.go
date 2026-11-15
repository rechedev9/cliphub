package moments

import (
	"testing"

	"github.com/rechedev9/cliphub/internal/killplan"
)

func rankByID(plan killplan.Plan) map[string]Moment {
	out := map[string]Moment{}
	for _, m := range Rank(plan) {
		out[m.SegmentID] = m
	}
	return out
}

func killsAt(ticks ...int) []killplan.Kill {
	out := make([]killplan.Kill, len(ticks))
	for i, tick := range ticks {
		out[i] = killplan.Kill{Tick: tick, Weapon: "ak47"}
	}
	return out
}

func TestFastMultiKillNamesTheBiggestWindow(t *testing.T) {
	tests := []struct {
		name  string
		ticks []int
		want  string
	}{
		{"double inside 1.5 s", []int{100, 196}, ReasonFastDouble},
		{"double just over 1.5 s", []int{100, 197}, ""},
		{"triple inside 10 s", []int{0, 300, 640}, ReasonFastTriple},
		{"triple over 10 s", []int{0, 300, 641}, ""},
		{"quad inside 15 s beats the double", []int{0, 50, 500, 960}, ReasonFastQuad},
		{"ace inside 45 s", []int{0, 700, 1400, 2100, 2880}, ReasonFastAce},
		{"ace over 45 s still has a fast quad", []int{0, 2000, 2300, 2600, 2950}, ReasonFastQuad},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := fastMultiKill(killplan.Segment{Kills: killsAt(tt.ticks...)}, 64); got != tt.want {
				t.Fatalf("fastMultiKill = %q, want %q", got, tt.want)
			}
		})
	}
	if got := fastMultiKill(killplan.Segment{Kills: killsAt(0, 10)}, 0); got != "" {
		t.Fatalf("no tickrate = %q, want none", got)
	}
}

func TestScoreValuesClutchSizeResultAndSpeed(t *testing.T) {
	ct := killplan.Player{TeamAtKill: "CT"}
	clutch := func(vs int, result string) []killplan.Kill {
		return []killplan.Kill{{Tick: 100, Weapon: "ak47", Killer: ct, ClutchVs: vs, ClutchResult: result}}
	}
	plan := killplan.NewPlan()
	plan.Demo.Tickrate = 64
	plan.Segments = []killplan.Segment{
		{ID: "plain", TickStart: 0, TickEnd: 640, Kills: clutch(0, "")},
		{ID: "1v1-won", TickStart: 0, TickEnd: 640, Kills: clutch(1, killplan.ClutchWon)},
		{ID: "1v3-lost", TickStart: 0, TickEnd: 640, Kills: clutch(3, killplan.ClutchLost)},
		{ID: "1v3-won", TickStart: 0, TickEnd: 640, Kills: clutch(3, killplan.ClutchWon)},
		{ID: "slow-double", TickStart: 0, TickEnd: 640, Kills: killsAt(100, 500)},
		{ID: "fast-double", TickStart: 0, TickEnd: 640, Kills: killsAt(100, 150)},
	}
	got := rankByID(plan)
	for _, pair := range [][2]string{{"plain", "1v3-lost"}, {"1v3-lost", "1v3-won"}, {"1v1-won", "1v3-won"}, {"plain", "1v1-won"}} {
		low, high := got[pair[0]], got[pair[1]]
		if high.Score <= low.Score {
			t.Fatalf("score %s = %v, want above %s = %v", pair[1], high.Score, pair[0], low.Score)
		}
	}
	if got["fast-double"].Score <= got["slow-double"].Score {
		t.Fatalf("fast double %v, want above slow double %v", got["fast-double"].Score, got["slow-double"].Score)
	}
	won, lost := got["1v3-won"], got["1v3-lost"]
	if won.ClutchWon == nil || !*won.ClutchWon || !hasReason(won.ReasonCodes, ReasonClutchWon) {
		t.Fatalf("won clutch = %v / %v, want clutch_won", won.ClutchWon, won.ReasonCodes)
	}
	if lost.ClutchWon == nil || *lost.ClutchWon || hasReason(lost.ReasonCodes, ReasonClutchWon) {
		t.Fatalf("lost clutch = %v / %v, want clutch_won false", lost.ClutchWon, lost.ReasonCodes)
	}
	if got["plain"].ClutchWon != nil {
		t.Fatalf("plain play clutch_won = %v, want nil", *got["plain"].ClutchWon)
	}
	if !hasReason(got["fast-double"].ReasonCodes, ReasonFastDouble) || hasReason(got["slow-double"].ReasonCodes, ReasonFastDouble) {
		t.Fatalf("fast/slow reasons = %v / %v", got["fast-double"].ReasonCodes, got["slow-double"].ReasonCodes)
	}
}

func TestBigPlaysKeepTheirOrderInsteadOfTyingAtTheTop(t *testing.T) {
	plan := killplan.NewPlan()
	plan.Demo.Tickrate = 64
	plan.Segments = []killplan.Segment{
		{ID: "seg-001", TickStart: 0, TickEnd: 6400, Kills: killsAt(0, 1000, 2000, 3000)},
		{ID: "seg-002", TickStart: 0, TickEnd: 6400, Kills: killsAt(0, 1000, 2000, 3000, 4000)},
	}
	ranked := Rank(plan)
	if ranked[0].SegmentID != "seg-002" || ranked[0].Score >= 1 || ranked[0].Score <= ranked[1].Score {
		t.Fatalf("ranked = %s %v, %s %v; want the 5K first, below 1", ranked[0].SegmentID, ranked[0].Score, ranked[1].SegmentID, ranked[1].Score)
	}
}

func TestKillFlagsBecomeReasonsAndWeighTheScore(t *testing.T) {
	plan := killplan.NewPlan()
	plan.Demo.Tickrate = 64
	plan.Segments = []killplan.Segment{
		{ID: "plain", TickStart: 0, TickEnd: 640, Kills: []killplan.Kill{{Tick: 100, Weapon: "awp"}}},
		{ID: "noscope", TickStart: 0, TickEnd: 640, Kills: []killplan.Kill{{Tick: 100, Weapon: "awp", NoScope: true}}},
		{ID: "smoke", TickStart: 0, TickEnd: 640, Kills: []killplan.Kill{{Tick: 100, Weapon: "awp", ThroughSmoke: true}}},
		{ID: "blind", TickStart: 0, TickEnd: 640, Kills: []killplan.Kill{{Tick: 100, Weapon: "awp", AttackerBlind: true}}},
	}
	got := rankByID(plan)
	for id, reason := range map[string]string{"noscope": ReasonNoScope, "smoke": ReasonThroughSmoke, "blind": ReasonAttackerBlind} {
		m := got[id]
		if !hasReason(m.ReasonCodes, reason) || m.Score <= got["plain"].Score {
			t.Fatalf("%s = %v score %v, want reason %q above plain %v", id, m.ReasonCodes, m.Score, reason, got["plain"].Score)
		}
		f, err := ParseFilter("", []string{reason}, "")
		if err != nil {
			t.Fatal(err)
		}
		if kept := Apply(Rank(plan), f); len(kept) != 1 || kept[0].SegmentID != id {
			t.Fatalf("kind %s kept %d plays, want only %s", reason, len(kept), id)
		}
	}
	if m := got["noscope"]; m.Events.NoScopes != 1 {
		t.Fatalf("noscope events = %+v", m.Events)
	}
}

func TestWinProbSwingAddsUpAndRaisesTheScore(t *testing.T) {
	plan := killplan.NewPlan()
	plan.Demo.Tickrate = 64
	plan.Segments = []killplan.Segment{
		{ID: "seg-001", TickStart: 0, TickEnd: 640, Kills: []killplan.Kill{{Tick: 100, WinProbSwing: 0.05}, {Tick: 300, WinProbSwing: 0.1}}},
		{ID: "seg-002", TickStart: 0, TickEnd: 640, Kills: []killplan.Kill{{Tick: 100, WinProbSwing: 0.2}, {Tick: 300, WinProbSwing: 0.35}}},
	}
	ranked := Rank(plan)
	if ranked[0].SegmentID != "seg-002" || ranked[0].WinProbSwing != 0.55 || ranked[1].WinProbSwing != 0.15 {
		t.Fatalf("ranked = %s %v, %s %v; want seg-002 first with 0.55", ranked[0].SegmentID, ranked[0].WinProbSwing, ranked[1].SegmentID, ranked[1].WinProbSwing)
	}
}

func TestParseFilterAcceptsTheNamedKinds(t *testing.T) {
	f, err := ParseFilter("", []string{"Fast_Triple", "clutch_won", "fast_triple"}, "")
	if err != nil || len(f.Kinds) != 2 || f.Kinds[0] != ReasonFastTriple || f.Kinds[1] != ReasonClutchWon {
		t.Fatalf("filter = %+v, %v; want fast_triple and clutch_won once", f, err)
	}
	if _, err := ParseFilter("", []string{"triple"}, ""); err == nil {
		t.Fatal("unknown kind passed")
	}
}
