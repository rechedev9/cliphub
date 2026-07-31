package parser

import (
	"errors"
	"testing"

	"github.com/rechedev9/tickcut/internal/killplan"
)

const targetID = "76561198000000000"

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

	if c.TotalKillsTarget() != 1 {
		t.Errorf("TotalKillsTarget = %d, want 1", c.TotalKillsTarget())
	}
	if c.KillsAfterFilters() != 1 {
		t.Errorf("KillsAfterFilters = %d, want 1", c.KillsAfterFilters())
	}
}

func TestRecordKillRejectedWeaponNotAdded(t *testing.T) {
	c := NewCollector(targetID, defaultTestRules())
	c.RecordKill(RawKill{Tick: 1000, Round: 3, Weapon: "knife"})

	if c.TotalKillsTarget() != 1 {
		t.Errorf("TotalKillsTarget = %d, want 1 (counted before filters)", c.TotalKillsTarget())
	}
	if c.KillsAfterFilters() != 0 {
		t.Errorf("KillsAfterFilters = %d, want 0 (filtered out)", c.KillsAfterFilters())
	}
}

func TestRecordKillHeadshotOnlyDropsNonHeadshots(t *testing.T) {
	r := defaultTestRules()
	r.IncludeHeadshotOnly = true
	c := NewCollector(targetID, r)
	c.RecordKill(RawKill{Tick: 1000, Round: 3, Weapon: "awp", Headshot: false})
	c.RecordKill(RawKill{Tick: 2000, Round: 3, Weapon: "awp", Headshot: true})

	if c.KillsAfterFilters() != 1 {
		t.Errorf("KillsAfterFilters = %d, want 1 (only headshot)", c.KillsAfterFilters())
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

	if c.KillsAfterFilters() != 2 {
		t.Errorf("KillsAfterFilters = %d, want 2", c.KillsAfterFilters())
	}
}

func TestBuildPlanFailsWhenTargetNeverSeen(t *testing.T) {
	c := NewCollector(targetID, defaultTestRules())
	// no RecordTargetIdentity, no kills

	_, err := c.Build(meta())
	if err == nil {
		t.Fatal("Build() error = nil, want error about target not seen")
	}
	if !errors.Is(err, ErrTargetNotFound) {
		t.Fatalf("Build() error = %v, want errors.Is(ErrTargetNotFound)", err)
	}
}

func TestBuildPlanWithNoKillsReturnsEmptySegments(t *testing.T) {
	c := NewCollector(targetID, defaultTestRules())
	c.RecordTargetIdentity("MARTINEZSA", "CT")

	plan, err := c.Build(meta())
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

	plan, err := c.Build(meta())
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

func TestBuildClampsSegmentEndToDemoDuration(t *testing.T) {
	c := NewCollector(targetID, defaultTestRules())
	c.RecordTargetIdentity("Target", "CT")
	// Kill well before EOF so the soft margin (2s) can trim the full post-roll
	// without colliding with the last event.
	const duration = 20_000
	const killTick = 10_000
	c.RecordKill(RawKill{Tick: killTick, Round: 1, Weapon: "ak47"})
	plan, err := c.Build(PlanMeta{Tickrate: 64, DurationTicks: duration})
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Segments) != 1 {
		t.Fatalf("segments = %#v, want 1", plan.Segments)
	}
	// Soft margin = 2s * 64 = 128 ticks → softCap 19872. Post-roll would be
	// 10000+320=10320, already under the soft cap, so end stays post-roll.
	want := killTick + 5*64
	if plan.Segments[0].TickEnd != want {
		t.Fatalf("TickEnd = %d, want post-roll %d", plan.Segments[0].TickEnd, want)
	}
}

func TestBuildPullsSegmentEndBackFromDemoEOF(t *testing.T) {
	c := NewCollector(targetID, defaultTestRules())
	c.RecordTargetIdentity("Target", "CT")
	// Post-roll would overrun the demo; soft margin must stop before EOF.
	const duration = 10_500
	const killTick = 10_200
	c.RecordKill(RawKill{Tick: killTick, Round: 1, Weapon: "ak47"})
	plan, err := c.Build(PlanMeta{Tickrate: 64, DurationTicks: duration})
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Segments) != 1 {
		t.Fatalf("segments = %#v, want 1", plan.Segments)
	}
	// softCap = 10500 - 128 = 10372; last kill 10200 is below softCap so end=10372.
	wantSoftCap := duration - 2*64
	if plan.Segments[0].TickEnd != wantSoftCap {
		t.Fatalf("TickEnd = %d, want soft-cap %d (away from demo EOF)", plan.Segments[0].TickEnd, wantSoftCap)
	}
	if plan.Segments[0].TickEnd >= duration {
		t.Fatalf("TickEnd = %d must stay before demo duration %d", plan.Segments[0].TickEnd, duration)
	}
}

func TestBuildNeverLandsSegmentEndOnAbsoluteDurationWhenMarginApplies(t *testing.T) {
	// Regression guard: post-roll past EOF used to clamp TickEnd == DurationTicks,
	// which captures the glitchy last frames and can miss record-end.
	c := NewCollector(targetID, defaultTestRules())
	c.RecordTargetIdentity("Target", "CT")
	const (
		tickrate = 64
		duration = 50_000
		killTick = 49_700 // post-roll 49_700+320=50020 > duration
	)
	c.RecordKill(RawKill{Tick: killTick, Round: 12, Weapon: "awp"})
	plan, err := c.Build(PlanMeta{Tickrate: tickrate, DurationTicks: duration})
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Segments) != 1 {
		t.Fatalf("segments = %#v, want 1", plan.Segments)
	}
	end := plan.Segments[0].TickEnd
	if end >= duration {
		t.Fatalf("TickEnd = %d lands on/after absolute duration %d; want soft margin headroom", end, duration)
	}
	softCap := duration - 2*tickrate
	if end != softCap {
		t.Fatalf("TickEnd = %d, want soft-cap %d", end, softCap)
	}
	if end <= killTick {
		t.Fatalf("TickEnd = %d must still cover the kill at %d", end, killTick)
	}
}

func TestBuildKeepsShortTailWhenKillIsInsideEOFMargin(t *testing.T) {
	c := NewCollector(targetID, defaultTestRules())
	c.RecordTargetIdentity("Target", "CT")
	// Kill inside the last 2s of the demo: soft cap is before the kill, so we
	// keep a short clean tail and still leave 1 tick of headroom before EOF.
	const duration = 10_000
	const killTick = 9_950
	c.RecordKill(RawKill{Tick: killTick, Round: 1, Weapon: "ak47"})
	plan, err := c.Build(PlanMeta{Tickrate: 64, DurationTicks: duration})
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Segments) != 1 {
		t.Fatalf("segments = %#v, want 1", plan.Segments)
	}
	// short tail would be kill+64=10014, but hard headroom is duration-1=9999.
	want := duration - 1
	if plan.Segments[0].TickEnd != want {
		t.Fatalf("TickEnd = %d, want EOF headroom %d", plan.Segments[0].TickEnd, want)
	}
	if plan.Segments[0].TickEnd <= killTick {
		t.Fatalf("TickEnd = %d must still cover the kill at %d", plan.Segments[0].TickEnd, killTick)
	}
}

func TestBuildPlanAssemblesSegments(t *testing.T) {
	c := NewCollector(targetID, defaultTestRules())
	c.RecordTargetIdentity("MARTINEZSA", "CT")
	c.RecordKill(RawKill{Tick: 10000, Round: 5, Weapon: "awp"})
	c.RecordKill(RawKill{Tick: 10000 + 2*testTickrate, Round: 5, Weapon: "awp"})

	plan, err := c.Build(meta())
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

func TestBuildPlanRoundEndClipping(t *testing.T) {
	c := NewCollector(targetID, defaultTestRules())
	c.RecordTargetIdentity("MARTINEZSA", "CT")
	c.RecordKill(RawKill{Tick: 10000, Round: 5, Weapon: "awp"})
	c.RecordRoundEnd(RoundEnd{Round: 5, Tick: 10100})

	plan, err := c.Build(meta())
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if plan.Segments[0].TickEnd != 10100 {
		t.Errorf("TickEnd = %d, want 10100 (clipped)", plan.Segments[0].TickEnd)
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
