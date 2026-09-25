package parser

import (
	"errors"
	"testing"

	"github.com/rechedev9/cliphub/internal/killplan"
	"github.com/rechedev9/cliphub/internal/rules"
)

const targetID = "76561198000000000"

func (c *Collector) Build(m PlanMeta) (killplan.Plan, error) {
	return c.build(m, SegmentModeKills)
}

func meta() PlanMeta {
	return PlanMeta{
		DemoPath:      "/tmp/demo.dem",
		SHA256:        "abc123",
		Map:           "de_inferno",
		Tickrate:      testTickrate,
		DurationTicks: 285000,
	}
}

func TestRecordKillAcceptedWeaponAdded(t *testing.T) {
	c := NewCollector(targetID, defaultTestRules())
	c.RecordTargetIdentity("MARTINEZSA", "CT")
	c.RecordKill(RawKill{Tick: 1000, Round: 3, Weapon: "awp"})

	if c.totalKillsTarget != 1 {
		t.Errorf("TotalKillsTarget = %d, want 1", c.totalKillsTarget)
	}
	if c.killsAfterFilters != 1 {
		t.Errorf("KillsAfterFilters = %d, want 1", c.killsAfterFilters)
	}
}

func TestRecordKillRejectedWeaponNotAdded(t *testing.T) {
	r := defaultTestRules()
	r.Weapons = []string{"awp"}
	c := NewCollector(targetID, r)
	c.RecordKill(RawKill{Tick: 1000, Round: 3, Weapon: "knife"})

	if c.totalKillsTarget != 1 {
		t.Errorf("TotalKillsTarget = %d, want 1 (counted before filters)", c.totalKillsTarget)
	}
	if c.killsAfterFilters != 0 {
		t.Errorf("KillsAfterFilters = %d, want 0 (filtered out)", c.killsAfterFilters)
	}
}

func TestCollectorRecapKeepsEveryWeaponDespiteShortsFilters(t *testing.T) {
	tests := []struct {
		name   string
		weapon string
	}{
		{name: "p90", weapon: "p90"},
		{name: "knife", weapon: "knife"},
		{name: "zeus", weapon: "taser"},
		{name: "future weapon", weapon: "future_cs2_weapon"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := defaultTestRules()
			r.Weapons = []string{"awp"}
			c := NewCollector(targetID, r)
			c.RecordTargetIdentity("MARTINEZSA", "CT")
			c.RecordRoundLiveStart(RoundLiveStart{Round: 5, Tick: 9000})
			c.RecordKill(RawKill{Tick: 10000, Round: 5, Weapon: tt.weapon})
			c.RecordRoundEnd(RoundEnd{Round: 5, Tick: 11000})

			killBursts, err := c.build(meta(), SegmentModeKills)
			if err != nil {
				t.Fatalf("Build kills: %v", err)
			}
			if len(killBursts.Segments) != 0 {
				t.Fatalf("kill-burst segments = %d, want 0 for custom filter", len(killBursts.Segments))
			}

			recap, err := c.build(meta(), SegmentModeRecap)
			if err != nil {
				t.Fatalf("Build recap: %v", err)
			}
			if len(recap.Segments) != 1 || len(recap.Segments[0].Kills) != 1 {
				t.Fatalf("recap segments = %+v, want one segment with one kill", recap.Segments)
			}
			if got := recap.Segments[0].Kills[0].Weapon; got != tt.weapon {
				t.Fatalf("recap weapon = %q, want %q", got, tt.weapon)
			}
			recapRules, ok := recap.Rules.(rules.Rules)
			if !ok {
				t.Fatalf("recap rules type = %T, want rules.Rules", recap.Rules)
			}
			if got := recapRules.Weapons; len(got) != 1 || got[0] != rules.AllWeapons {
				t.Fatalf("recap weapons rule = %v, want [%q]", got, rules.AllWeapons)
			}
			if got := recap.Stats.KillsAfterFilters; got != 1 {
				t.Fatalf("recap KillsAfterFilters = %d, want 1", got)
			}
		})
	}
}

func TestCollectorRecapRejectsAnEmptyWeaponName(t *testing.T) {
	c := NewCollector(targetID, defaultTestRules())
	c.RecordTargetIdentity("MARTINEZSA", "CT")
	c.RecordRoundLiveStart(RoundLiveStart{Round: 5, Tick: 9000})
	c.RecordKill(RawKill{Tick: 10000, Round: 5, Weapon: ""})
	c.RecordRoundEnd(RoundEnd{Round: 5, Tick: 11000})

	recap, err := c.build(meta(), SegmentModeRecap)
	if err != nil {
		t.Fatalf("Build recap: %v", err)
	}
	if len(recap.Segments) != 1 || len(recap.Segments[0].Kills) != 0 {
		t.Fatalf("recap segments = %+v, want the round without an unknown-weapon kill", recap.Segments)
	}
	if got := recap.Stats.KillsAfterFilters; got != 0 {
		t.Fatalf("recap KillsAfterFilters = %d, want 0", got)
	}
}

func TestCollectorRecapIgnoresShortsRoundRange(t *testing.T) {
	r := defaultTestRules()
	r.MinRound = 10
	r.MaxRound = 12
	c := NewCollector(targetID, r)
	c.RecordTargetIdentity("MARTINEZSA", "CT")
	c.RecordRoundLiveStart(RoundLiveStart{Round: 5, Tick: 9000})
	c.RecordKill(RawKill{Tick: 10000, Round: 5, Weapon: "awp"})
	c.RecordRoundEnd(RoundEnd{Round: 5, Tick: 11000})

	shorts, err := c.build(meta(), SegmentModeKills)
	if err != nil {
		t.Fatalf("Build Shorts: %v", err)
	}
	if len(shorts.Segments) != 0 {
		t.Fatalf("Shorts segments = %+v, want round 5 filtered out", shorts.Segments)
	}

	recap, err := c.build(meta(), SegmentModeRecap)
	if err != nil {
		t.Fatalf("Build recap: %v", err)
	}
	if len(recap.Segments) != 1 || len(recap.Segments[0].Kills) != 1 {
		t.Fatalf("recap segments = %+v, want round 5 and its kill", recap.Segments)
	}
	recapRules, ok := recap.Rules.(rules.Rules)
	if !ok {
		t.Fatalf("recap rules type = %T, want rules.Rules", recap.Rules)
	}
	if recapRules.MinRound != 1 || recapRules.MaxRound != 0 {
		t.Fatalf("recap round range = %d-%d, want all rounds", recapRules.MinRound, recapRules.MaxRound)
	}
}

func TestRecordKillHeadshotOnlyDropsNonHeadshots(t *testing.T) {
	r := defaultTestRules()
	r.IncludeHeadshotOnly = true
	c := NewCollector(targetID, r)
	c.RecordKill(RawKill{Tick: 1000, Round: 3, Weapon: "awp", Headshot: false})
	c.RecordKill(RawKill{Tick: 2000, Round: 3, Weapon: "awp", Headshot: true})

	if c.killsAfterFilters != 1 {
		t.Errorf("KillsAfterFilters = %d, want 1 (only headshot)", c.killsAfterFilters)
	}
}

func TestRecordKillRoundFilter(t *testing.T) {
	r := defaultTestRules()
	r.MinRound = 5
	r.MaxRound = 10
	c := NewCollector(targetID, r)
	c.RecordKill(RawKill{Tick: 1000, Round: 4, Weapon: "awp"})  // below
	c.RecordKill(RawKill{Tick: 2000, Round: 5, Weapon: "awp"})  // ok
	c.RecordKill(RawKill{Tick: 3000, Round: 10, Weapon: "awp"}) // ok
	c.RecordKill(RawKill{Tick: 4000, Round: 11, Weapon: "awp"}) // above

	if c.killsAfterFilters != 2 {
		t.Errorf("KillsAfterFilters = %d, want 2", c.killsAfterFilters)
	}
}

func TestCollectorsFailWhenTargetNeverSeen(t *testing.T) {
	// No RecordTargetIdentity and no events: every collector must report the
	// missing target instead of an empty plan.
	tests := []struct {
		name  string
		build func() (killplan.Plan, error)
	}{
		{"kills", func() (killplan.Plan, error) {
			return NewCollector(targetID, defaultTestRules()).build(meta(), SegmentModeKills)
		}},
		{"smokes", func() (killplan.Plan, error) {
			return NewSmokeCollector(targetID, defaultTestRules()).Build(meta())
		}},
		{"utility", func() (killplan.Plan, error) {
			return NewUtilityCollector(targetID, defaultTestRules()).Build(meta())
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := tc.build(); !errors.Is(err, ErrTargetNotFound) {
				t.Fatalf("Build() error = %v, want errors.Is(ErrTargetNotFound)", err)
			}
		})
	}
}

func TestBuildPlanWithNoKillsReturnsEmptySegments(t *testing.T) {
	c := NewCollector(targetID, defaultTestRules())
	c.RecordTargetIdentity("MARTINEZSA", "CT")

	plan, err := c.build(meta(), SegmentModeKills)
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if plan.SchemaVersion != killplan.SchemaVersion {
		t.Errorf("SchemaVersion = %q, want %q", plan.SchemaVersion, killplan.SchemaVersion)
	}
	if len(plan.Segments) != 0 {
		t.Errorf("Segments length = %d, want 0", len(plan.Segments))
	}
	if plan.Target.SteamID64 != targetID {
		t.Errorf("Target.SteamID64 = %q, want %q", plan.Target.SteamID64, targetID)
	}
	if plan.Demo.Map != "de_inferno" {
		t.Errorf("Demo.Map = %q, want de_inferno", plan.Demo.Map)
	}
	if plan.Stats.KillsAfterFilters != 0 {
		t.Errorf("Stats.KillsAfterFilters = %d, want 0", plan.Stats.KillsAfterFilters)
	}
}

type matchStartIdentityCollector interface {
	RecordTargetIdentity(string, string)
	resetForMatchStart()
	Build(PlanMeta) (killplan.Plan, error)
}

func TestCollectorsPreserveWarmupTargetWhenLiveMatchHasNoEvents(t *testing.T) {
	for _, tc := range []struct {
		name string
		new  func() matchStartIdentityCollector
	}{
		{
			name: "kills",
			new: func() matchStartIdentityCollector {
				return NewCollector(targetID, defaultTestRules())
			},
		},
		{
			name: "smokes",
			new: func() matchStartIdentityCollector {
				return NewSmokeCollector(targetID, defaultTestRules())
			},
		},
		{
			name: "utility",
			new: func() matchStartIdentityCollector {
				return NewUtilityCollector(targetID, defaultTestRules())
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := tc.new()
			c.RecordTargetIdentity("Warmup Target", "T")
			c.resetForMatchStart()

			plan, err := c.Build(meta())
			if err != nil {
				t.Fatalf("Build after MatchStart error = %v", err)
			}
			if plan.Target.NameInDemo != "Warmup Target" || plan.Target.TeamAtStart != "T" {
				t.Fatalf("target = %#v, want preserved warmup identity", plan.Target)
			}
			if len(plan.Segments) != 0 {
				t.Fatalf("segments = %#v, want empty live plan", plan.Segments)
			}
		})
	}
}

func TestRecordTargetIdentityKeepsTheFirstObservedAliasAndTeam(t *testing.T) {
	c := NewCollector(targetID, defaultTestRules())
	c.RecordTargetIdentity("ZaCkETiZOR", "T")
	c.RecordTargetIdentity("zack", "CT")

	plan, err := c.build(meta(), SegmentModeKills)
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if got, want := plan.Target.NameInDemo, "ZaCkETiZOR"; got != want {
		t.Fatalf("Target.NameInDemo = %q, want first observed alias %q", got, want)
	}
	if got, want := plan.Target.TeamAtStart, "T"; got != want {
		t.Fatalf("Target.TeamAtStart = %q, want starting team %q", got, want)
	}
}

func TestBuildSegmentEndRespectsDemoEOFMargin(t *testing.T) {
	// Tickrate 64: post-roll is 5s (320 ticks), the soft EOF margin is 2s
	// (128 ticks) and the hard headroom keeps TickEnd one tick before EOF.
	// Landing on DurationTicks used to capture the glitchy last frames and
	// could miss record-end.
	tests := []struct {
		name     string
		duration int
		killTick int
		round    int
		weapon   string
		wantEnd  int
	}{
		{
			// Post-roll 10320 is already under the soft cap 19872.
			name: "post-roll well before EOF is not clipped", duration: 20_000, killTick: 10_000,
			round: 1, weapon: "ak47", wantEnd: 10_000 + 5*64,
		},
		{
			// Post-roll 10520 overruns EOF; the kill is below soft cap 10372.
			name: "post-roll past EOF pulls back to the soft cap", duration: 10_500, killTick: 10_200,
			round: 1, weapon: "ak47", wantEnd: 10_500 - 2*64,
		},
		{
			// Regression guard for a late-match kill: post-roll 50020 > duration.
			name: "late-match post-roll past EOF stops at the soft cap", duration: 50_000, killTick: 49_700,
			round: 12, weapon: "awp", wantEnd: 50_000 - 2*64,
		},
		{
			// The soft cap is before the kill, so a short tail (kill+64=10014)
			// is kept and clamped to the hard headroom duration-1.
			name: "kill inside the EOF margin keeps a short tail", duration: 10_000, killTick: 9_950,
			round: 1, weapon: "ak47", wantEnd: 10_000 - 1,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := NewCollector(targetID, defaultTestRules())
			c.RecordTargetIdentity("Target", "CT")
			c.RecordKill(RawKill{Tick: tc.killTick, Round: tc.round, Weapon: tc.weapon})
			plan, err := c.build(PlanMeta{Tickrate: 64, DurationTicks: tc.duration}, SegmentModeKills)
			if err != nil {
				t.Fatal(err)
			}
			if len(plan.Segments) != 1 {
				t.Fatalf("segments = %#v, want 1", plan.Segments)
			}
			end := plan.Segments[0].TickEnd
			if end != tc.wantEnd {
				t.Fatalf("TickEnd = %d, want %d", end, tc.wantEnd)
			}
			if end >= tc.duration {
				t.Fatalf("TickEnd = %d must stay before demo duration %d", end, tc.duration)
			}
			if end <= tc.killTick {
				t.Fatalf("TickEnd = %d must still cover the kill at %d", end, tc.killTick)
			}
		})
	}
}

func TestBuildPlanAssemblesSegments(t *testing.T) {
	c := NewCollector(targetID, defaultTestRules())
	c.RecordTargetIdentity("MARTINEZSA", "CT")
	c.RecordKill(RawKill{Tick: 10000, Round: 5, Weapon: "awp"})
	c.RecordKill(RawKill{Tick: 10000 + 2*testTickrate, Round: 5, Weapon: "awp"})

	plan, err := c.build(meta(), SegmentModeKills)
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if len(plan.Segments) != 1 {
		t.Fatalf("Segments length = %d, want 1", len(plan.Segments))
	}
	if len(plan.Segments[0].Kills) != 2 {
		t.Errorf("Kills in segment = %d, want 2", len(plan.Segments[0].Kills))
	}
	if plan.Stats.SegmentsCreated != 1 {
		t.Errorf("Stats.SegmentsCreated = %d, want 1", plan.Stats.SegmentsCreated)
	}
	if plan.Stats.KillsAfterFilters != 2 {
		t.Errorf("Stats.KillsAfterFilters = %d, want 2", plan.Stats.KillsAfterFilters)
	}
	if plan.Stats.DurationSecondsTotal <= 0 {
		t.Errorf("Stats.DurationSecondsTotal = %v, want > 0", plan.Stats.DurationSecondsTotal)
	}
}

func TestCollectorRecapPistolRoundStaysOneContinuousLiveWindow(t *testing.T) {
	c := NewCollector(targetID, defaultTestRules())
	c.RecordTargetIdentity("MARTINEZSA", "CT")
	c.RecordRoundStart(RoundStart{Round: 1, Tick: 8000})
	c.RecordRoundLiveStart(RoundLiveStart{Round: 1, Tick: 9200})
	c.RecordKill(RawKill{Tick: 9400, Round: 1, Weapon: "usp_silencer"})
	c.RecordKill(RawKill{Tick: 9400 + 20*testTickrate, Round: 1, Weapon: "usp_silencer"})
	c.RecordKill(RawKill{Tick: 9400 + 36*testTickrate, Round: 1, Weapon: "usp_silencer"})
	c.RecordRoundEnd(RoundEnd{Round: 1, Tick: 14000})

	shorts, err := c.build(meta(), SegmentModeKills)
	if err != nil {
		t.Fatalf("shorts build: %v", err)
	}
	recap, err := c.build(meta(), SegmentModeRecap)
	if err != nil {
		t.Fatalf("recap build: %v", err)
	}
	if len(shorts.Segments) != 3 {
		t.Fatalf("shorts segments = %d, want 3 kill bursts", len(shorts.Segments))
	}
	if len(recap.Segments) != 1 {
		t.Fatalf("recap segments = %d, want one live pistol round (not a jump-cut montage)", len(recap.Segments))
	}
	seg := recap.Segments[0]
	if seg.TickStart >= 9200 {
		t.Fatalf("intro freeze/buy countdown skipped: TickStart = %d", seg.TickStart)
	}
	if seg.TickStart < 8000 {
		t.Fatalf("intro freeze pulled before round start: TickStart = %d", seg.TickStart)
	}
	wantEnd := 14000 + (OutroBannerSeconds+OutroScoreboardSeconds)*testTickrate
	if seg.TickEnd != wantEnd {
		t.Fatalf("TickEnd = %d, want %d (win banner then scoreboard)", seg.TickEnd, wantEnd)
	}
	if len(seg.Kills) != 3 {
		t.Fatalf("recap kills = %d, want 3 in the same window", len(seg.Kills))
	}
	if len(seg.Utility) != 0 {
		t.Fatalf("utility rewritten: %#v", seg.Utility)
	}
}

func TestCollectorBuildEmitsWiderRecapWindows(t *testing.T) {
	c := NewCollector(targetID, defaultTestRules())
	c.RecordTargetIdentity("MARTINEZSA", "CT")
	c.RecordRoundStart(RoundStart{Round: 5, Tick: 8000})
	c.RecordRoundLiveStart(RoundLiveStart{Round: 5, Tick: 8500})
	c.RecordKill(RawKill{Tick: 10000, Round: 5, Weapon: "awp"})
	c.RecordRoundEnd(RoundEnd{Round: 5, Tick: 14000})

	kills, err := c.build(meta(), SegmentModeKills)
	if err != nil {
		t.Fatalf("kills build: %v", err)
	}
	recap, err := c.build(meta(), SegmentModeRecap)
	if err != nil {
		t.Fatalf("recap build: %v", err)
	}
	if len(kills.Segments) != 1 || len(recap.Segments) != 1 {
		t.Fatalf("segments kills=%d recap=%d, want 1 and 1", len(kills.Segments), len(recap.Segments))
	}
	if recap.Segments[0].TickStart >= kills.Segments[0].TickStart {
		t.Fatalf("recap start %d, want earlier than kill burst %d", recap.Segments[0].TickStart, kills.Segments[0].TickStart)
	}
	if recap.Segments[0].TickEnd <= kills.Segments[0].TickEnd {
		t.Fatalf("recap end %d, want later than kill burst %d", recap.Segments[0].TickEnd, kills.Segments[0].TickEnd)
	}
}

func TestBuildPlanRoundEndClipping(t *testing.T) {
	c := NewCollector(targetID, defaultTestRules())
	c.RecordTargetIdentity("MARTINEZSA", "CT")
	c.RecordKill(RawKill{Tick: 10000, Round: 5, Weapon: "awp"})
	c.RecordRoundEnd(RoundEnd{Round: 5, Tick: 10100})

	plan, err := c.build(meta(), SegmentModeKills)
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if plan.Segments[0].TickEnd != 10100+roundEndGraceSeconds*testTickrate {
		t.Errorf("TickEnd = %d, want %d (round end + grace)", plan.Segments[0].TickEnd, 10100+roundEndGraceSeconds*testTickrate)
	}
}

func TestSortRawKillsByTickKeepsStableOrder(t *testing.T) {
	kills := []RawKill{
		{Tick: 20, Victim: killplan.Player{NameInDemo: "late"}},
		{Tick: 10, Victim: killplan.Player{NameInDemo: "first"}},
		{Tick: 10, Victim: killplan.Player{NameInDemo: "second"}},
	}

	sortRawKillsByTick(kills)

	got := []string{
		kills[0].Victim.NameInDemo,
		kills[1].Victim.NameInDemo,
		kills[2].Victim.NameInDemo,
	}
	want := []string{"first", "second", "late"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("order = %v, want %v", got, want)
		}
	}
}
