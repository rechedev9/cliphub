package parser

import (
	"testing"

	"github.com/rechedev9/cliphub/internal/killplan"
	"github.com/rechedev9/cliphub/internal/rules"
)

func defaultTestRules() rules.Rules {
	r := rules.Default()
	// keep the test deterministic regardless of future default changes
	r.WindowSeconds = 8
	r.PreRollSeconds = 3
	r.PostRollSeconds = 5
	r.MinKillsInWindow = 1
	return r
}

const testTickrate = 64

func mkKill(tick, round int, weapon string) RawKill {
	return RawKill{
		Tick:   tick,
		Round:  round,
		Weapon: weapon,
	}
}

func TestSegmentEmptyKillsReturnsNoSegments(t *testing.T) {
	got := Segment(nil, nil, nil, defaultTestRules(), testTickrate)
	if len(got) != 0 {
		t.Errorf("Segment(nil) = %d segments, want 0", len(got))
	}
}

func TestSegmentSingleKillProducesOneSegment(t *testing.T) {
	kills := []RawKill{mkKill(10000, 5, "awp")}
	got := Segment(kills, nil, nil, defaultTestRules(), testTickrate)
	if len(got) != 1 {
		t.Fatalf("got %d segments, want 1", len(got))
	}
	s := got[0]
	if s.Round != 5 {
		t.Errorf("Round = %d, want 5", s.Round)
	}
	if len(s.Kills) != 1 {
		t.Errorf("Kills length = %d, want 1", len(s.Kills))
	}
	// pre_roll_seconds=3 at 64 tickrate = 192 ticks before
	if s.TickStart != 10000-3*testTickrate {
		t.Errorf("TickStart = %d, want %d", s.TickStart, 10000-3*testTickrate)
	}
	// post_roll_seconds=5 = 320 ticks after
	if s.TickEnd != 10000+5*testTickrate {
		t.Errorf("TickEnd = %d, want %d", s.TickEnd, 10000+5*testTickrate)
	}
	if s.ID != "seg-001" {
		t.Errorf("ID = %q, want seg-001", s.ID)
	}
}

func TestSegmentPreservesTheTargetIdentityAtTheKillTick(t *testing.T) {
	killer := killplan.Player{
		SteamID64:  "76561197997743909",
		NameInDemo: "ZaCkETiZOR",
		TeamAtKill: "T",
	}
	got := Segment([]RawKill{{
		Tick:   4396,
		Round:  1,
		Weapon: "deagle",
		Killer: killer,
	}}, nil, nil, defaultTestRules(), testTickrate)
	if len(got) != 1 || len(got[0].Kills) != 1 {
		t.Fatalf("Segment() = %+v, want one kill", got)
	}
	if got[0].Kills[0].Killer != killer {
		t.Fatalf("Killer = %+v, want tick identity %+v", got[0].Kills[0].Killer, killer)
	}
}

func TestSegmentTwoKillsWithinWindowMergeIntoOneSegment(t *testing.T) {
	// 2 ticks_per_sec * 7 sec window = 7 sec apart. window is 8 sec → same segment.
	kills := []RawKill{
		mkKill(10000, 5, "awp"),
		mkKill(10000+7*testTickrate, 5, "awp"),
	}
	got := Segment(kills, nil, nil, defaultTestRules(), testTickrate)
	if len(got) != 1 {
		t.Fatalf("got %d segments, want 1", len(got))
	}
	if len(got[0].Kills) != 2 {
		t.Errorf("Kills length = %d, want 2", len(got[0].Kills))
	}
	if got[0].TickStart != 10000-3*testTickrate {
		t.Errorf("TickStart = %d, want %d", got[0].TickStart, 10000-3*testTickrate)
	}
	// TickEnd uses the last kill in the segment
	want := 10000 + 7*testTickrate + 5*testTickrate
	if got[0].TickEnd != want {
		t.Errorf("TickEnd = %d, want %d", got[0].TickEnd, want)
	}
}

func TestSegmentKillsOutsideWindowSplitIntoSeparateSegments(t *testing.T) {
	// 9 seconds apart > 8 second window
	kills := []RawKill{
		mkKill(10000, 5, "awp"),
		mkKill(10000+9*testTickrate, 5, "awp"),
	}
	got := Segment(kills, nil, nil, defaultTestRules(), testTickrate)
	if len(got) != 2 {
		t.Fatalf("got %d segments, want 2", len(got))
	}
	if got[0].ID != "seg-001" || got[1].ID != "seg-002" {
		t.Errorf("IDs = %q, %q; want seg-001, seg-002", got[0].ID, got[1].ID)
	}
}

func TestSegmentTransitiveChainingAcrossKills(t *testing.T) {
	// k1 at t=0, k2 at t=7s (within 8s of k1), k3 at t=14s (within 8s of k2).
	// All three should land in one segment.
	kills := []RawKill{
		mkKill(10000, 5, "awp"),
		mkKill(10000+7*testTickrate, 5, "awp"),
		mkKill(10000+14*testTickrate, 5, "awp"),
	}
	got := Segment(kills, nil, nil, defaultTestRules(), testTickrate)
	if len(got) != 1 {
		t.Fatalf("got %d segments, want 1", len(got))
	}
	if len(got[0].Kills) != 3 {
		t.Errorf("Kills length = %d, want 3", len(got[0].Kills))
	}
}

func TestSegmentMinKillsInWindowDropsSingleKillSegments(t *testing.T) {
	r := defaultTestRules()
	r.MinKillsInWindow = 2

	kills := []RawKill{
		mkKill(10000, 5, "awp"),                // alone
		mkKill(20000, 6, "awp"),                // start of a pair...
		mkKill(20000+2*testTickrate, 6, "awp"), // ...with this one
		mkKill(40000, 7, "awp"),                // alone
	}
	got := Segment(kills, nil, nil, r, testTickrate)
	if len(got) != 1 {
		t.Fatalf("got %d segments, want 1 (only the pair survives)", len(got))
	}
	if len(got[0].Kills) != 2 {
		t.Errorf("surviving segment kills = %d, want 2", len(got[0].Kills))
	}
	if got[0].ID != "seg-001" {
		t.Errorf("ID = %q, want seg-001 (renumbered after filtering)", got[0].ID)
	}
}

func TestSegmentPreRollClampedToZero(t *testing.T) {
	// kill very early — pre-roll would underflow past tick 0
	kills := []RawKill{mkKill(100, 1, "awp")}
	got := Segment(kills, nil, nil, defaultTestRules(), testTickrate)
	if len(got) != 1 {
		t.Fatalf("got %d segments, want 1", len(got))
	}
	if got[0].TickStart != 0 {
		t.Errorf("TickStart = %d, want 0 (clamped)", got[0].TickStart)
	}
}

func TestSegmentClippedAtRoundEnd(t *testing.T) {
	// kill at tick 10000, post-roll would extend to 10000 + 320 = 10320.
	// Round 5 ends at tick 10100 → segment clips to round end plus grace.
	kills := []RawKill{mkKill(10000, 5, "awp")}
	roundEnds := []RoundEnd{{Round: 5, Tick: 10100}}
	got := Segment(kills, roundEnds, nil, defaultTestRules(), testTickrate)
	if len(got) != 1 {
		t.Fatalf("got %d segments, want 1", len(got))
	}
	want := 10100 + roundEndGraceSeconds*testTickrate
	if got[0].TickEnd != want {
		t.Errorf("TickEnd = %d, want %d (round end + grace)", got[0].TickEnd, want)
	}
}

func TestSegmentUsesFirstRoundEndWhenDuplicatesExist(t *testing.T) {
	kills := []RawKill{mkKill(10000, 5, "awp")}
	roundEnds := []RoundEnd{
		{Round: 5, Tick: 10100},
		{Round: 5, Tick: 10200},
	}
	got := Segment(kills, roundEnds, nil, defaultTestRules(), testTickrate)
	if len(got) != 1 {
		t.Fatalf("got %d segments, want 1", len(got))
	}
	if got[0].TickEnd != 10100+roundEndGraceSeconds*testTickrate {
		t.Errorf("TickEnd = %d, want first round end plus grace %d", got[0].TickEnd, 10100+roundEndGraceSeconds*testTickrate)
	}
}

func TestSegmentNotClippedWhenRoundEndIsAfterPostRoll(t *testing.T) {
	// Round 5 ends way after post-roll; no clipping should happen.
	kills := []RawKill{mkKill(10000, 5, "awp")}
	roundEnds := []RoundEnd{{Round: 5, Tick: 999999}}
	got := Segment(kills, roundEnds, nil, defaultTestRules(), testTickrate)
	if len(got) != 1 {
		t.Fatalf("got %d segments, want 1", len(got))
	}
	want := 10000 + 5*testTickrate
	if got[0].TickEnd != want {
		t.Errorf("TickEnd = %d, want %d (not clipped)", got[0].TickEnd, want)
	}
}

func TestSegmentClippedAtRoundEndWhenGroupSpansRounds(t *testing.T) {
	// A group can span a round boundary: the first kill is in round 5, the last
	// in round 6 (within the merge window). The post-roll must be clipped at the
	// end of the round where the segment actually ends (round 6), not the round
	// where it started — otherwise the clip bleeds into round 7's footage.
	kills := []RawKill{
		mkKill(10000, 5, "awp"),
		mkKill(10000+4*testTickrate, 6, "awp"), // 10256, within the 8s window
	}
	roundEnds := []RoundEnd{
		{Round: 5, Tick: 10100}, // before the last kill; must not drive the clip
		{Round: 6, Tick: 10400}, // within the post-roll; the segment must clip here
	}
	got := Segment(kills, roundEnds, nil, defaultTestRules(), testTickrate)
	if len(got) != 1 {
		t.Fatalf("got %d segments, want 1", len(got))
	}
	if got[0].TickEnd != 10400+roundEndGraceSeconds*testTickrate {
		t.Errorf("TickEnd = %d, want %d (ending round end + grace)", got[0].TickEnd, 10400+roundEndGraceSeconds*testTickrate)
	}
}

func TestSegmentRoundEndKillPostRollGrace(t *testing.T) {
	graceTicks := roundEndGraceSeconds * testTickrate
	tests := []struct {
		name        string
		kills       []RawKill
		roundEnds   []RoundEnd
		roundStarts []RoundStart
		wantEnd     int
	}{
		{
			name:      "last kill equals round end tick keeps grace post-roll",
			kills:     []RawKill{mkKill(10750, 12, "ak47")},
			roundEnds: []RoundEnd{{Round: 12, Tick: 10750}},
			wantEnd:   10750 + graceTicks,
		},
		{
			name: "grace stops before the next round freeze",
			kills: []RawKill{
				mkKill(10000, 5, "awp"),
				mkKill(10000+4*testTickrate, 6, "awp"),
			},
			roundEnds: []RoundEnd{
				{Round: 5, Tick: 10100},
				{Round: 6, Tick: 10400},
			},
			roundStarts: []RoundStart{
				{Round: 5, Tick: 9000},
				{Round: 6, Tick: 10200},
				{Round: 7, Tick: 10500},
			},
			wantEnd: 10499,
		},
		{
			name:      "unknown next round start allows full grace past round end",
			kills:     []RawKill{mkKill(10750, 12, "ak47")},
			roundEnds: []RoundEnd{{Round: 12, Tick: 10750}},
			wantEnd:   10750 + graceTicks,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Segment(tt.kills, tt.roundEnds, tt.roundStarts, defaultTestRules(), testTickrate)
			if len(got) != 1 {
				t.Fatalf("segments = %d, want 1", len(got))
			}
			lastKill := tt.kills[len(tt.kills)-1].Tick
			if got[0].TickEnd < lastKill+graceTicks {
				t.Fatalf("TickEnd = %d, want at least last kill %d + grace %d", got[0].TickEnd, lastKill, graceTicks)
			}
			if got[0].TickEnd != tt.wantEnd {
				t.Fatalf("TickEnd = %d, want %d", got[0].TickEnd, tt.wantEnd)
			}
		})
	}
}

func TestSegmentRecap(t *testing.T) {
	shortsPre := 3 * testTickrate
	shortsPost := 5 * testTickrate
	tests := []struct {
		name         string
		kills        []RawKill
		roundStarts  []RoundStart
		liveStarts   []RoundLiveStart
		roundEnds    []RoundEnd
		targetDeaths []TargetDeath
		rules        func() rules.Rules
		wantSegs     int
		wantRound    int
		wantKills    int
		checkRange   bool
		wantStart    int
		wantEnd      int
	}{
		{
			name:     "empty",
			wantSegs: 0,
		},
		{
			name: "same-round kills far apart are one live window, not the 8s shorts burst",
			kills: []RawKill{
				mkKill(10000, 5, "awp"),
				mkKill(10000+20*testTickrate, 5, "ak47"),
			},
			wantSegs:   1,
			wantRound:  5,
			wantKills:  2,
			checkRange: true,
			wantStart:  10000,
			wantEnd:    10000 + 20*testTickrate,
		},
		{
			name: "freeze-end to round-end excludes freeze ticks",
			kills: []RawKill{
				mkKill(10000, 5, "awp"),
				mkKill(10000+20*testTickrate, 5, "ak47"),
			},
			roundStarts: []RoundStart{{Round: 5, Tick: 9000}},
			liveStarts:  []RoundLiveStart{{Round: 5, Tick: 9500}},
			roundEnds:   []RoundEnd{{Round: 5, Tick: 14000}},
			wantSegs:    1,
			wantRound:   5,
			wantKills:   2,
			checkRange:  true,
			wantStart:   9500,
			wantEnd:     14000,
		},
		{
			name:        "zero-kill live round is kept when freeze-end and round-end exist",
			roundStarts: []RoundStart{{Round: 3, Tick: 4000}},
			liveStarts:  []RoundLiveStart{{Round: 3, Tick: 4500}},
			roundEnds:   []RoundEnd{{Round: 3, Tick: 7000}},
			wantSegs:    1,
			wantRound:   3,
			wantKills:   0,
			checkRange:  true,
			wantStart:   4500,
			wantEnd:     7000,
		},
		{
			name: "different rounds stay separate",
			kills: []RawKill{
				mkKill(10000, 5, "awp"),
				mkKill(20000, 6, "awp"),
			},
			wantSegs: 2,
		},
		{
			name: "missing freeze-end falls back to first kill through round end, not shorts preroll",
			kills: []RawKill{
				mkKill(10000, 5, "awp"),
				mkKill(10200, 5, "ak47"),
			},
			roundStarts: []RoundStart{{Round: 5, Tick: 9000}},
			roundEnds:   []RoundEnd{{Round: 5, Tick: 10300}},
			wantSegs:    1,
			wantRound:   5,
			wantKills:   2,
			checkRange:  true,
			wantStart:   10000,
			wantEnd:     10200 + roundEndGraceSeconds*testTickrate,
		},
		{
			name: "shorts minimum does not drop a full-demo round",
			kills: []RawKill{
				mkKill(10000, 5, "awp"),
				mkKill(20000, 6, "awp"),
				mkKill(20100, 6, "ak47"),
			},
			rules: func() rules.Rules {
				r := defaultTestRules()
				r.MinKillsInWindow = 2
				return r
			},
			wantSegs: 2,
		},
		{
			name:         "target death ends the POV before round end",
			kills:        []RawKill{mkKill(10000, 5, "p90")},
			roundStarts:  []RoundStart{{Round: 5, Tick: 9000}},
			liveStarts:   []RoundLiveStart{{Round: 5, Tick: 9500}},
			roundEnds:    []RoundEnd{{Round: 5, Tick: 14000}},
			targetDeaths: []TargetDeath{{Round: 5, Tick: 12000}},
			wantSegs:     1,
			wantRound:    5,
			wantKills:    1,
			checkRange:   true,
			wantStart:    9500,
			wantEnd:      12000,
		},
		{
			name: "posthumous grenade kill is excluded from a death-bounded POV",
			kills: []RawKill{
				mkKill(10000, 5, "ak47"),
				mkKill(12500, 5, "hegrenade"),
			},
			roundStarts:  []RoundStart{{Round: 5, Tick: 9000}},
			liveStarts:   []RoundLiveStart{{Round: 5, Tick: 9500}},
			roundEnds:    []RoundEnd{{Round: 5, Tick: 14000}},
			targetDeaths: []TargetDeath{{Round: 5, Tick: 12000}},
			wantSegs:     1,
			wantRound:    5,
			wantKills:    1,
			checkRange:   true,
			wantStart:    9500,
			wantEnd:      12000,
		},
		{
			name:         "zero-kill target death still produces a valid POV window",
			roundStarts:  []RoundStart{{Round: 3, Tick: 4000}},
			liveStarts:   []RoundLiveStart{{Round: 3, Tick: 4500}},
			targetDeaths: []TargetDeath{{Round: 3, Tick: 6200}},
			wantSegs:     1,
			wantRound:    3,
			checkRange:   true,
			wantStart:    4500,
			wantEnd:      6200,
		},
		{
			name:         "stale post-round death does not clip the following live round",
			roundStarts:  []RoundStart{{Round: 4, Tick: 7000}},
			liveStarts:   []RoundLiveStart{{Round: 4, Tick: 7500}},
			roundEnds:    []RoundEnd{{Round: 4, Tick: 10000}},
			targetDeaths: []TargetDeath{{Round: 4, Tick: 6900}},
			wantSegs:     1,
			wantRound:    4,
			checkRange:   true,
			wantStart:    7500,
			wantEnd:      10000,
		},
		{
			name:         "stale pre-live death does not hide the actual live death",
			roundStarts:  []RoundStart{{Round: 4, Tick: 7000}},
			liveStarts:   []RoundLiveStart{{Round: 4, Tick: 7500}},
			roundEnds:    []RoundEnd{{Round: 4, Tick: 10000}},
			targetDeaths: []TargetDeath{{Round: 4, Tick: 6900}, {Round: 4, Tick: 8200}},
			wantSegs:     1,
			wantRound:    4,
			checkRange:   true,
			wantStart:    7500,
			wantEnd:      8200,
		},
		{
			name:         "death-only metadata cannot invent a recap round",
			targetDeaths: []TargetDeath{{Round: 20, Tick: 108145}},
			wantSegs:     0,
		},
		{
			name:       "kill-only without round bounds is 1s, not the 8s shorts burst",
			kills:      []RawKill{mkKill(10000, 1, "awp")},
			wantSegs:   1,
			wantRound:  1,
			wantKills:  1,
			checkRange: true,
			wantStart:  10000,
			wantEnd:    10000 + testTickrate,
		},
		{
			name:        "zero-kill without round-end is unrecordable and dropped",
			roundStarts: []RoundStart{{Round: 3, Tick: 4000}},
			liveStarts:  []RoundLiveStart{{Round: 3, Tick: 4500}},
			wantSegs:    0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := defaultTestRules()
			if tt.rules != nil {
				r = tt.rules()
			}
			got := SegmentRecap(tt.kills, nil, tt.roundStarts, tt.liveStarts, tt.roundEnds, tt.targetDeaths, r, testTickrate)
			if len(got) != tt.wantSegs {
				t.Fatalf("segments = %d, want %d", len(got), tt.wantSegs)
			}
			if tt.wantSegs == 0 {
				return
			}
			if tt.wantRound != 0 && got[0].Round != tt.wantRound {
				t.Errorf("Round = %d, want %d", got[0].Round, tt.wantRound)
			}
			if tt.wantKills != 0 && len(got[0].Kills) != tt.wantKills {
				t.Errorf("Kills = %d, want %d", len(got[0].Kills), tt.wantKills)
			}
			if tt.checkRange && got[0].TickStart != tt.wantStart {
				t.Errorf("TickStart = %d, want %d", got[0].TickStart, tt.wantStart)
			}
			if tt.checkRange && got[0].TickEnd != tt.wantEnd {
				t.Errorf("TickEnd = %d, want %d", got[0].TickEnd, tt.wantEnd)
			}
			if len(tt.kills) > 0 && tt.checkRange {
				shortsStart := tt.kills[0].Tick - shortsPre
				if shortsStart < 1 {
					shortsStart = 1
				}
				shortsEnd := tt.kills[len(tt.kills)-1].Tick + shortsPost
				if got[0].TickStart == shortsStart && got[0].TickEnd == shortsEnd {
					t.Errorf("window collapsed to Shorts burst %d-%d", shortsStart, shortsEnd)
				}
			}
		})
	}
}

func TestSegmentRecapUtilityDoesNotMoveKnownRoundStart(t *testing.T) {
	got := SegmentRecap(
		[]RawKill{mkKill(400, 1, "ak47")},
		[]RawUtilityThrow{{Type: SmokeGrenadeType, Round: 1, ThrowTick: 800, PopTick: 900}},
		[]RoundStart{{Round: 1, Tick: 1}},
		[]RoundLiveStart{{Round: 1, Tick: 50}},
		[]RoundEnd{{Round: 1, Tick: 1000}},
		nil,
		defaultTestRules(),
		testTickrate,
	)
	if len(got) != 1 {
		t.Fatalf("segments = %d, want 1", len(got))
	}
	if got[0].TickStart != 50 {
		t.Fatalf("TickStart = %d, want 50 (freeze-end kept; utility must not pull into freeze)", got[0].TickStart)
	}
	if got[0].TickEnd != 1000 {
		t.Fatalf("TickEnd = %d, want 1000 (round end cap)", got[0].TickEnd)
	}
}

func TestSegmentRecapDropsUtilityThrownDuringFreeze(t *testing.T) {
	got := SegmentRecap(
		[]RawKill{mkKill(10000, 5, "ak47")},
		[]RawUtilityThrow{
			{Type: SmokeGrenadeType, Round: 5, ThrowTick: 9100, PopTick: 9300},
			{Type: FlashbangType, Round: 5, ThrowTick: 11000, PopTick: 11100},
		},
		[]RoundStart{{Round: 5, Tick: 9000}},
		[]RoundLiveStart{{Round: 5, Tick: 9200}},
		[]RoundEnd{{Round: 5, Tick: 14000}},
		nil,
		defaultTestRules(),
		testTickrate,
	)
	if len(got) != 1 {
		t.Fatalf("segments = %d, want 1", len(got))
	}
	if got[0].TickStart != 9200 || got[0].TickEnd != 14000 {
		t.Fatalf("window = %d-%d, want freeze-end 9200 to round-end 14000", got[0].TickStart, got[0].TickEnd)
	}
	if len(got[0].Utility) != 1 || got[0].Utility[0].ThrowTick != 11000 {
		t.Fatalf("utility = %#v, want only the live-round flash", got[0].Utility)
	}
}

func TestSegmentRecapAttachesUtilityFacts(t *testing.T) {
	kills := []RawKill{mkKill(10000, 5, "ak47")}
	utility := []RawUtilityThrow{{
		Type:          SmokeGrenadeType,
		Round:         5,
		ThrowTick:     9400,
		PopTick:       11000,
		ThrowPos:      [3]float64{1, 2, 3},
		LandingPos:    [3]float64{10, 20, 30},
		ThrowAction:   "jumpthrow",
		ThrowClick:    "left",
		ViewYaw:       45,
		ViewPitch:     -12,
		ThrowEyePos:   [3]float64{1, 2, 64},
		ThrowPlace:    "TSpawn",
		LandingSource: "smoke_start",
	}}
	got := SegmentRecap(kills, utility, []RoundStart{{Round: 5, Tick: 9000}}, []RoundLiveStart{{Round: 5, Tick: 9200}}, []RoundEnd{{Round: 5, Tick: 14000}}, nil, defaultTestRules(), testTickrate)
	if len(got) != 1 {
		t.Fatalf("segments = %d, want 1", len(got))
	}
	if got[0].TickStart != 9200 || got[0].TickEnd != 14000 {
		t.Fatalf("window = %d-%d, want live round (freeze excluded)", got[0].TickStart, got[0].TickEnd)
	}
	if len(got[0].Utility) != 1 {
		t.Fatalf("utility = %d, want 1", len(got[0].Utility))
	}
	u := got[0].Utility[0]
	if u.ThrowAction != "jumpthrow" || u.ThrowClick != "left" || u.ThrowPlace != "TSpawn" {
		t.Fatalf("utility labels = %+v", u)
	}
	if u.ThrowPos != [3]float64{1, 2, 3} || u.LandingPos != [3]float64{10, 20, 30} || u.ThrowEyePos != [3]float64{1, 2, 64} {
		t.Fatalf("utility positions = throw=%v land=%v eyes=%v", u.ThrowPos, u.LandingPos, u.ThrowEyePos)
	}
	if u.ViewYaw != 45 || u.ViewPitch != -12 || u.LandingSource != "smoke_start" {
		t.Fatalf("utility aim/source = %+v", u)
	}
}

func TestThrowClickFromButtons(t *testing.T) {
	tests := []struct {
		left, right bool
		want        string
	}{
		{false, false, ""},
		{true, false, "left"},
		{false, true, "right"},
		{true, true, "both"},
	}
	for _, tt := range tests {
		if got := throwClickFromButtons(tt.left, tt.right); got != tt.want {
			t.Fatalf("throwClickFromButtons(%v,%v) = %q, want %q", tt.left, tt.right, got, tt.want)
		}
	}
}

func TestTrackedUtilityTypesIncludeExplosiveAndDecoy(t *testing.T) {
	for _, typ := range []string{SmokeGrenadeType, FlashbangType, MolotovType, IncendiaryGrenadeType, HeGrenadeType, DecoyType} {
		if !isTrackedUtilityType(typ) {
			t.Fatalf("%s should be tracked", typ)
		}
	}
}

func TestSegmentRoundIsFirstKillsRound(t *testing.T) {
	// Edge case: two kills span a round boundary (unusual but possible if a
	// kill counted at the end of one round and the next sits at the very start).
	// The segment's Round should follow the first kill.
	kills := []RawKill{
		mkKill(10000, 5, "awp"),
		mkKill(10000+4*testTickrate, 6, "awp"),
	}
	got := Segment(kills, nil, nil, defaultTestRules(), testTickrate)
	if len(got) != 1 {
		t.Fatalf("got %d segments, want 1", len(got))
	}
	if got[0].Round != 5 {
		t.Errorf("Round = %d, want 5 (first kill's round)", got[0].Round)
	}
}

func TestWithIntroFreezeKeepsFirstRoundBuyCountdown(t *testing.T) {
	live := []killplan.Segment{
		{ID: "seg-001", Round: 1, TickStart: 9500, TickEnd: 14000},
		{ID: "seg-002", Round: 2, TickStart: 16000, TickEnd: 20000},
	}
	got := WithIntroFreeze(live, []RoundStart{{Round: 1, Tick: 8000}, {Round: 2, Tick: 15000}}, testTickrate)
	if len(got) != 2 {
		t.Fatalf("segments = %d", len(got))
	}
	wantFirst := 9500 - IntroFreezeSeconds*testTickrate
	if wantFirst < 8000 {
		wantFirst = 8000
	}
	if got[0].TickStart != wantFirst {
		t.Fatalf("first TickStart = %d, want %d (freeze/buy countdown kept)", got[0].TickStart, wantFirst)
	}
	if got[0].TickStart >= 9500 {
		t.Fatal("first-round freeze/buy countdown was skipped")
	}
	if got[1].TickStart != 16000 {
		t.Fatalf("second TickStart = %d, want 16000 (later rounds stay freeze-end)", got[1].TickStart)
	}
	if len(got[0].Utility) != 0 {
		t.Fatalf("utility rewritten: %#v", got[0].Utility)
	}
}

func TestWithIntroFreezeDoesNotPullBuyTimeNadesIntoTheSegment(t *testing.T) {
	live := SegmentRecap(
		[]RawKill{mkKill(10000, 1, "ak47")},
		[]RawUtilityThrow{{Type: SmokeGrenadeType, Round: 1, ThrowTick: 8200, PopTick: 8400}},
		[]RoundStart{{Round: 1, Tick: 8000}},
		[]RoundLiveStart{{Round: 1, Tick: 9200}},
		[]RoundEnd{{Round: 1, Tick: 14000}},
		nil,
		defaultTestRules(),
		testTickrate,
	)
	if len(live) != 1 || live[0].TickStart != 9200 {
		t.Fatalf("live recap = %#v, want freeze-end 9200", live)
	}
	if len(live[0].Utility) != 0 {
		t.Fatalf("live utility = %#v, want freeze nade dropped", live[0].Utility)
	}
	got := WithIntroFreeze(live, []RoundStart{{Round: 1, Tick: 8000}}, testTickrate)
	if got[0].TickStart >= 9200 {
		t.Fatalf("intro freeze missing: TickStart = %d", got[0].TickStart)
	}
	if len(got[0].Utility) != 0 {
		t.Fatalf("buy-time nade attached after freeze prefix: %#v", got[0].Utility)
	}
}

func TestSegmentRecapSetsLiveEndTickBeforeOutroHold(t *testing.T) {
	roundEnds := []RoundEnd{{Round: 18, Tick: 152626}}
	liveStarts := []RoundLiveStart{{Round: 18, Tick: 146271}}
	roundStarts := []RoundStart{{Round: 18, Tick: 145900}, {Round: 19, Tick: 153500}}
	got := SegmentRecap(nil, nil, roundStarts, liveStarts, roundEnds, nil, defaultTestRules(), testTickrate)
	if len(got) != 1 {
		t.Fatalf("segments = %d, want 1", len(got))
	}
	if got[0].LiveEndTick != 152626 {
		t.Fatalf("LiveEndTick = %d, want round live end 152626", got[0].LiveEndTick)
	}
	if got[0].TickEnd != 152626 {
		t.Fatalf("TickEnd before outro = %d, want 152626", got[0].TickEnd)
	}
	held := WithOutroHold(got, roundStarts, nil, testTickrate)
	last := held[len(held)-1]
	wantRecordEnd := 152626 + (OutroBannerSeconds+OutroScoreboardSeconds)*testTickrate
	if last.TickEnd != wantRecordEnd {
		t.Fatalf("TickEnd after outro hold = %d, want %d", last.TickEnd, wantRecordEnd)
	}
	if last.LiveEndTick != 152626 {
		t.Fatalf("LiveEndTick after outro hold = %d, want preserved live end 152626", last.LiveEndTick)
	}
}

func TestSegmentRecapPreservesPostKillPayoffAtRoundEnd(t *testing.T) {
	tests := []struct {
		name           string
		killTick       int
		nextRoundStart int
		deaths         []TargetDeath
		wantEnd        int
	}{
		{
			name:           "winning kill gets two second payoff",
			killTick:       14000,
			nextRoundStart: 15000,
			wantEnd:        14000 + roundEndGraceSeconds*testTickrate,
		},
		{
			name:           "kill one second before round end gets remaining payoff",
			killTick:       14000 - testTickrate,
			nextRoundStart: 15000,
			wantEnd:        14000 + testTickrate,
		},
		{
			name:           "earlier kill needs no extra round footage",
			killTick:       14000 - 3*testTickrate,
			nextRoundStart: 15000,
			wantEnd:        14000,
		},
		{
			name:           "next freeze caps payoff",
			killTick:       14000,
			nextRoundStart: 14000 + testTickrate,
			wantEnd:        14000 + testTickrate - 1,
		},
		{
			name:           "target death at round end prevents deathcam",
			killTick:       14000,
			nextRoundStart: 15000,
			deaths:         []TargetDeath{{Round: 1, Tick: 14000}},
			wantEnd:        14000,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SegmentRecap(
				[]RawKill{mkKill(tt.killTick, 1, "ak47")},
				nil,
				[]RoundStart{{Round: 1, Tick: 8000}, {Round: 2, Tick: tt.nextRoundStart}},
				[]RoundLiveStart{{Round: 1, Tick: 9200}},
				[]RoundEnd{{Round: 1, Tick: 14000}},
				tt.deaths,
				defaultTestRules(),
				testTickrate,
			)
			if len(got) != 1 {
				t.Fatalf("segments = %d, want 1", len(got))
			}
			if got[0].LiveEndTick != 14000 {
				t.Fatalf("LiveEndTick = %d, want 14000", got[0].LiveEndTick)
			}
			if got[0].TickEnd != tt.wantEnd {
				t.Fatalf("TickEnd = %d, want %d", got[0].TickEnd, tt.wantEnd)
			}
		})
	}
}

func TestWithOutroHoldKeepsWinBannerThenScoreboardAndSkipsDeathcam(t *testing.T) {
	tests := []struct {
		name          string
		segs          []killplan.Segment
		roundStarts   []RoundStart
		deaths        []TargetDeath
		wantLastEnd   int
		wantUnchanged bool
	}{
		{
			name: "alive through round end extends banner then scoreboard",
			segs: []killplan.Segment{
				{ID: "seg-001", Round: 1, TickStart: 9200, TickEnd: 14000},
				{ID: "seg-002", Round: 2, TickStart: 16000, TickEnd: 22000},
			},
			roundStarts: []RoundStart{{Round: 1, Tick: 8000}, {Round: 2, Tick: 15000}},
			wantLastEnd: 22000 + (OutroBannerSeconds+OutroScoreboardSeconds)*testTickrate,
		},
		{
			name: "death-clipped last round is not extended into deathcam",
			segs: []killplan.Segment{
				{ID: "seg-001", Round: 1, TickStart: 9200, TickEnd: 11000},
			},
			roundStarts:   []RoundStart{{Round: 1, Tick: 8000}},
			deaths:        []TargetDeath{{Round: 1, Tick: 11000}},
			wantLastEnd:   11000,
			wantUnchanged: true,
		},
		{
			name: "hold stops before the next round start",
			segs: []killplan.Segment{
				{ID: "seg-001", Round: 1, TickStart: 9200, TickEnd: 14000},
			},
			roundStarts: []RoundStart{{Round: 1, Tick: 8000}, {Round: 2, Tick: 14500}},
			wantLastEnd: 14499,
		},
		{
			name: "post-kill grace does not lengthen the total outro hold",
			segs: []killplan.Segment{
				{ID: "seg-001", Round: 1, TickStart: 16000, TickEnd: 22128, LiveEndTick: 22000},
			},
			roundStarts: []RoundStart{{Round: 1, Tick: 15000}},
			wantLastEnd: 22000 + (OutroBannerSeconds+OutroScoreboardSeconds)*testTickrate,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := WithOutroHold(tt.segs, tt.roundStarts, tt.deaths, testTickrate)
			if len(got) != len(tt.segs) {
				t.Fatalf("segments = %d", len(got))
			}
			last := got[len(got)-1]
			if last.TickEnd != tt.wantLastEnd {
				t.Fatalf("last TickEnd = %d, want %d", last.TickEnd, tt.wantLastEnd)
			}
			if tt.wantUnchanged && last.TickEnd != tt.segs[len(tt.segs)-1].TickEnd {
				t.Fatalf("deathcam hold changed TickEnd to %d", last.TickEnd)
			}
			if len(last.Utility) != 0 {
				t.Fatalf("utility rewritten: %#v", last.Utility)
			}
		})
	}
}

func TestSegmentRecapIsOneContinuousLiveRoundNotAJumpCut(t *testing.T) {
	// imfcnd pistol: CT spawn → A ramp → under A. Three USP kills sit more
	// than the Shorts 8s window apart; a highlight stitch would emit three
	// segments. Recap-plan live rounds stay one continuous window.
	spawn := 9400
	ramp := spawn + 20*testTickrate
	underA := ramp + 16*testTickrate
	kills := []RawKill{
		mkKill(spawn, 1, "usp_silencer"),
		mkKill(ramp, 1, "usp_silencer"),
		mkKill(underA, 1, "usp_silencer"),
	}
	roundStarts := []RoundStart{{Round: 1, Tick: 8000}}
	liveStarts := []RoundLiveStart{{Round: 1, Tick: 9200}}
	roundEnds := []RoundEnd{{Round: 1, Tick: 14000}}
	r := defaultTestRules()

	got := SegmentRecap(kills, nil, roundStarts, liveStarts, roundEnds, nil, r, testTickrate)
	if len(got) != 1 {
		t.Fatalf("recap segments = %d, want one continuous live round (not a stitch)", len(got))
	}
	if got[0].TickStart != 9200 || got[0].TickEnd != 14000 {
		t.Fatalf("recap window = %d-%d, want freeze-end 9200 to round-end 14000", got[0].TickStart, got[0].TickEnd)
	}
	if len(got[0].Kills) != 3 {
		t.Fatalf("recap kills = %d, want all three in the same window", len(got[0].Kills))
	}
	for i := 1; i < len(got); i++ {
		if got[i].TickStart < got[i-1].TickEnd {
			t.Fatalf("recap overlapped %d-%d then %d-%d", got[i-1].TickStart, got[i-1].TickEnd, got[i].TickStart, got[i].TickEnd)
		}
	}

	shorts := Segment(kills, roundEnds, nil, r, testTickrate)
	if len(shorts) != 3 {
		t.Fatalf("shorts segments = %d, want 3 kill bursts so the recap contrast is real", len(shorts))
	}
}
