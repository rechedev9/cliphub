package moments

import (
	"slices"
	"testing"

	"github.com/rechedev9/cliphub/internal/killplan"
)

var (
	ropz = killplan.Target{SteamID64: "1", NameInDemo: "ropz"}
	b1t  = killplan.Target{SteamID64: "2", NameInDemo: "b1t"}
)

// directorSegment is a 10 s play at 64 tick with n plain CT kills.
func directorSegment(id string, round, start int, pov killplan.Target, n int) killplan.Segment {
	s := killplan.Segment{ID: id, Round: round, TickStart: start, TickEnd: start + 640, POV: &pov}
	for i := range n {
		s.Kills = append(s.Kills, killplan.Kill{Tick: start + 128 + i*256, Weapon: "weapon_ak47", Killer: killplan.Player{SteamID64: pov.SteamID64, TeamAtKill: "CT"}})
	}
	return s
}

func directorPlan(segs ...killplan.Segment) killplan.Plan {
	plan := killplan.NewPlan()
	plan.Demo.Tickrate = 64
	plan.Target = ropz
	plan.Segments = segs
	return plan
}

func cueOf(p DirectedPlay, kind string) (Cue, bool) {
	i := slices.IndexFunc(p.Cues, func(c Cue) bool { return c.Kind == kind })
	if i < 0 {
		return Cue{}, false
	}
	return p.Cues[i], true
}

func TestDirectPicksTheBetterOfTwoClashingPOVsWithinTheBudget(t *testing.T) {
	clutch := directorSegment("seg-002", 2, 1200, b1t, 3)
	for i := range clutch.Kills {
		clutch.Kills[i].ClutchVs = 3 - i
		clutch.Kills[i].ClutchResult = killplan.ClutchWon
	}
	entry := directorSegment("seg-003", 5, 5000, ropz, 2)
	entry.Kills[0].Entry = true
	plan := directorPlan(
		directorSegment("seg-001", 2, 1000, ropz, 2),
		clutch,
		entry,
		directorSegment("seg-004", 8, 9000, b1t, 3),
	)
	got := Direct(plan, DirectorOptions{TargetSeconds: 40})
	if want := []string{"seg-002", "seg-003", "seg-004"}; !slices.Equal(got.SegmentIDs, want) {
		t.Fatalf("segment_ids = %v, want %v", got.SegmentIDs, want)
	}
	if got.TotalSeconds != 30 || got.RoundResults {
		t.Fatalf("total/round_results = %v/%v, want 30/false", got.TotalSeconds, got.RoundResults)
	}
	if c, ok := cueOf(got.Plays[0], CueClutch); !ok || c.Count != 3 || c.Detail != CueClutchWon {
		t.Errorf("clutch cue = %+v, want a won 1v3", got.Plays[0].Cues)
	}
	if c, ok := cueOf(got.Plays[1], CueDuel); !ok || c.Detail != CueDuelOpening {
		t.Errorf("entry cue = %+v, want the opening duel", got.Plays[1].Cues)
	}
	wantPlayers := []DirectedPlayer{{SteamID64: "1", Name: "ropz", Plays: 1, Seconds: 10}, {SteamID64: "2", Name: "b1t", Plays: 2, Seconds: 20}}
	if !slices.Equal(got.Players, wantPlayers) {
		t.Errorf("players = %+v, want %+v", got.Players, wantPlayers)
	}
	for _, p := range got.Plays {
		if p.Interest < p.Score {
			t.Errorf("%s interest %v is below its score %v", p.SegmentID, p.Interest, p.Score)
		}
	}
}

func TestDirectRotatesToAnotherPlayerWhenTheirPlayIsClose(t *testing.T) {
	other := directorSegment("seg-004", 9, 9000, b1t, 2)
	other.Kills[0].Headshot, other.Kills[1].Headshot, other.Kills[0].Entry = true, true, true
	plan := directorPlan(
		directorSegment("seg-001", 2, 1000, ropz, 3),
		directorSegment("seg-002", 4, 3000, ropz, 3),
		directorSegment("seg-003", 6, 5000, ropz, 3),
		other,
	)
	got := Direct(plan, DirectorOptions{TargetSeconds: 20})
	if want := []string{"seg-001", "seg-004"}; !slices.Equal(got.SegmentIDs, want) {
		t.Fatalf("segment_ids = %v, want ropz's best and then b1t's play", got.SegmentIDs)
	}
}

func TestDirectCuesBombPressureAndMomentum(t *testing.T) {
	retake := directorSegment("seg-001", 5, 5000, ropz, 1)
	retake.Kills[0].PostPlant, retake.Kills[0].Retake = true, true
	retake.WinProb = []killplan.WinProbSpan{{TickStart: 5000, TickEnd: 5640, CTPct: 50}}
	run := directorSegment("seg-002", 8, 9000, b1t, 1)
	run.Kills[0].WinProbSwing = 0.4
	old := directorSegment("seg-003", 12, 14000, ropz, 1)
	old.WinProb = []killplan.WinProbSpan{{TickStart: 14000, TickEnd: 14640, CTPct: 75}}
	plan := directorPlan(retake, run, old)
	for n := 1; n <= 8; n++ {
		winner := "T"
		if n >= 5 {
			winner = "CT"
		}
		plan.Rounds = append(plan.Rounds, killplan.RoundResult{Number: n, Winner: winner, TargetSide: "CT"})
	}
	got := Direct(plan, DirectorOptions{TargetSeconds: 60, ExtraSeconds: 2})
	if len(got.Plays) != 3 || !got.RoundResults || got.TotalSeconds != 36 {
		t.Fatalf("direction = %+v", got)
	}
	if c, ok := cueOf(got.Plays[0], CueBomb); !ok || c.Detail != CueBombRetake || c.Value != 1 {
		t.Errorf("retake cue = %+v, want a retake at an even chance", got.Plays[0].Cues)
	}
	if c, ok := cueOf(got.Plays[0], CueMomentum); !ok || c.Detail != CueComeback || c.Count != 4 {
		t.Errorf("round 5 cue = %+v, want a comeback after 4 lost rounds", got.Plays[0].Cues)
	}
	momentum := slices.DeleteFunc(slices.Clone(got.Plays[1].Cues), func(c Cue) bool { return c.Kind != CueMomentum })
	want := []Cue{{Kind: CueMomentum, Detail: CueMomentumSwing, Value: 0.4}, {Kind: CueMomentum, Detail: CueRun, Count: 4}}
	if !slices.Equal(momentum, want) {
		t.Errorf("round 8 momentum = %+v, want %+v", momentum, want)
	}
	if c, ok := cueOf(got.Plays[2], CueBomb); !ok || c.Detail != CueBombPostPlant || c.Value != 0.5 {
		t.Errorf("older plan bomb cue = %+v, want post_plant from the win-chance span", got.Plays[2].Cues)
	}
	if _, ok := cueOf(got.Plays[2], CueMomentum); ok {
		t.Errorf("round 12 has no known result, so no momentum cue: %+v", got.Plays[2].Cues)
	}
}

func TestDirectKeepsTheFirstPlayOverTheBudgetAndSkipsDeathsAndUtility(t *testing.T) {
	long := directorSegment("seg-002", 3, 2000, ropz, 2)
	long.TickEnd = long.TickStart + 64*30
	death := directorSegment("seg-001", 1, 100, ropz, 0)
	death.Death = &killplan.Kill{Tick: 600}
	got := Direct(directorPlan(death, long), DirectorOptions{TargetSeconds: 15})
	if !slices.Equal(got.SegmentIDs, []string{"seg-002"}) || got.TotalSeconds != 30 {
		t.Fatalf("direction = %v (%v s), want only the long play", got.SegmentIDs, got.TotalSeconds)
	}
	if empty := Direct(directorPlan(), DirectorOptions{TargetSeconds: 60}); empty.SegmentIDs == nil || len(empty.Plays) != 0 {
		t.Fatalf("empty plan direction = %+v, want empty lists", empty)
	}
}

func TestDirectorOptionsNormalize(t *testing.T) {
	got, err := DirectorOptions{}.Normalize()
	if err != nil || got.TargetSeconds != DefaultDirectorSeconds {
		t.Fatalf("defaults = %+v, %v", got, err)
	}
	for _, bad := range []DirectorOptions{{TargetSeconds: 10}, {TargetSeconds: 601}, {TargetSeconds: 60, ExtraSeconds: -1}, {TargetSeconds: 60, ExtraSeconds: 16}} {
		if _, err := bad.Normalize(); err == nil {
			t.Errorf("%+v: want an error", bad)
		}
	}
}
