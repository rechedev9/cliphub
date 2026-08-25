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
	got := Segment(nil, nil, defaultTestRules(), testTickrate)
	if len(got) != 0 {
		t.Errorf("Segment(nil) = %d segments, want 0", len(got))
	}
}

func TestSegmentSingleKillProducesOneSegment(t *testing.T) {
	kills := []RawKill{mkKill(10000, 5, "awp")}
	got := Segment(kills, nil, defaultTestRules(), testTickrate)
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
	}}, nil, defaultTestRules(), testTickrate)
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
	got := Segment(kills, nil, defaultTestRules(), testTickrate)
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
	got := Segment(kills, nil, defaultTestRules(), testTickrate)
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
	got := Segment(kills, nil, defaultTestRules(), testTickrate)
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
	got := Segment(kills, nil, r, testTickrate)
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
	got := Segment(kills, nil, defaultTestRules(), testTickrate)
	if len(got) != 1 {
		t.Fatalf("got %d segments, want 1", len(got))
	}
	if got[0].TickStart != 0 {
		t.Errorf("TickStart = %d, want 0 (clamped)", got[0].TickStart)
	}
}

func TestSegmentClippedAtRoundEnd(t *testing.T) {
	// kill at tick 10000, post-roll would extend to 10000 + 320 = 10320.
	// Round 5 ends at tick 10100 → segment should clip to 10100.
	kills := []RawKill{mkKill(10000, 5, "awp")}
	roundEnds := []RoundEnd{{Round: 5, Tick: 10100}}
	got := Segment(kills, roundEnds, defaultTestRules(), testTickrate)
	if len(got) != 1 {
		t.Fatalf("got %d segments, want 1", len(got))
	}
	if got[0].TickEnd != 10100 {
		t.Errorf("TickEnd = %d, want 10100 (clipped at round end)", got[0].TickEnd)
	}
}

func TestSegmentUsesFirstRoundEndWhenDuplicatesExist(t *testing.T) {
	kills := []RawKill{mkKill(10000, 5, "awp")}
	roundEnds := []RoundEnd{
		{Round: 5, Tick: 10100},
		{Round: 5, Tick: 10200},
	}
	got := Segment(kills, roundEnds, defaultTestRules(), testTickrate)
	if len(got) != 1 {
		t.Fatalf("got %d segments, want 1", len(got))
	}
	if got[0].TickEnd != 10100 {
		t.Errorf("TickEnd = %d, want first round end 10100", got[0].TickEnd)
	}
}

func TestSegmentNotClippedWhenRoundEndIsAfterPostRoll(t *testing.T) {
	// Round 5 ends way after post-roll; no clipping should happen.
	kills := []RawKill{mkKill(10000, 5, "awp")}
	roundEnds := []RoundEnd{{Round: 5, Tick: 999999}}
	got := Segment(kills, roundEnds, defaultTestRules(), testTickrate)
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
	got := Segment(kills, roundEnds, defaultTestRules(), testTickrate)
	if len(got) != 1 {
		t.Fatalf("got %d segments, want 1", len(got))
	}
	if got[0].TickEnd != 10400 {
		t.Errorf("TickEnd = %d, want 10400 (clipped at the ending round's end)", got[0].TickEnd)
	}
}

func TestSegmentRecap(t *testing.T) {
	shortsPre := 3 * testTickrate
	shortsPost := 5 * testTickrate
	tests := []struct {
		name        string
		kills       []RawKill
		roundStarts []RoundStart
		liveStarts  []RoundLiveStart
		roundEnds   []RoundEnd
		rules       func() rules.Rules
		wantSegs    int
		wantRound   int
		wantKills   int
		checkRange  bool
		wantStart   int
		wantEnd     int
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
			wantEnd:     10300,
		},
		{
			name: "min kills drops a lone-kill round",
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
			wantSegs:  1,
			wantRound: 6,
			wantKills: 2,
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
			got := SegmentRecap(tt.kills, nil, tt.roundStarts, tt.liveStarts, tt.roundEnds, r, testTickrate)
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
	got := SegmentRecap(kills, utility, []RoundStart{{Round: 5, Tick: 9000}}, []RoundLiveStart{{Round: 5, Tick: 9200}}, []RoundEnd{{Round: 5, Tick: 14000}}, defaultTestRules(), testTickrate)
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
	got := Segment(kills, nil, defaultTestRules(), testTickrate)
	if len(got) != 1 {
		t.Fatalf("got %d segments, want 1", len(got))
	}
	if got[0].Round != 5 {
		t.Errorf("Round = %d, want 5 (first kill's round)", got[0].Round)
	}
}
