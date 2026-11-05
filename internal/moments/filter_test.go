package moments

import (
	"slices"
	"testing"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/killplan"
)

func TestBuildTagsSideAndClutch(t *testing.T) {
	ct := killplan.Player{NameInDemo: "target", TeamAtKill: "CT"}
	plan := killplan.NewPlan()
	plan.Demo.Tickrate = 64
	plan.Segments = []killplan.Segment{
		{ID: "seg-001", Round: 3, TickStart: 0, TickEnd: 640, Kills: []killplan.Kill{
			{Tick: 100, Weapon: "ak47", Killer: ct},
			{Tick: 200, Weapon: "ak47", Killer: ct, ClutchVs: 3},
			{Tick: 300, Weapon: "ak47", Killer: ct, ClutchVs: 2},
		}},
		{ID: "seg-002", Round: 4, TickStart: 700, TickEnd: 900, Utility: []killplan.UtilityThrow{
			{ID: "util-1", Thrower: killplan.Player{TeamAtKill: "T"}},
		}},
		{ID: "seg-003", Round: 5, TickStart: 1000, TickEnd: 1200, Kills: []killplan.Kill{
			{Tick: 1100, Weapon: "awp", Killer: killplan.Player{TeamAtKill: "SPEC"}},
		}},
	}

	bySegment := map[string]Moment{}
	for _, m := range Build(uuid.New(), plan).Moments {
		bySegment[m.SegmentID] = m
	}

	clutch := bySegment["seg-001"]
	if clutch.Side != SideCT || clutch.ClutchVs != 3 || !hasReason(clutch.ReasonCodes, KindClutch) {
		t.Fatalf("seg-001 side/clutch/reasons = %q/%d/%v, want CT/3/clutch", clutch.Side, clutch.ClutchVs, clutch.ReasonCodes)
	}
	if got := bySegment["seg-002"].Side; got != SideT {
		t.Fatalf("utility side = %q, want T from the thrower", got)
	}
	if got := bySegment["seg-003"]; got.Side != "" || hasReason(got.ReasonCodes, KindClutch) {
		t.Fatalf("spectator side or clutch leaked: %q %v", got.Side, got.ReasonCodes)
	}
}

func TestApplyFiltersByWeaponKindAndSide(t *testing.T) {
	list := []Moment{
		{SegmentID: "awp-ct", Side: SideCT, Events: Events{Weapons: []string{"awp"}}, ReasonCodes: []string{"awp"}},
		{SegmentID: "awp-ak-t", Side: SideT, Events: Events{Weapons: []string{"ak47", "awp"}}, ReasonCodes: []string{"clutch", "multi_kill"}},
		{SegmentID: "ace-t", Side: SideT, Events: Events{Weapons: []string{"weapon_ak47"}}, ReasonCodes: []string{"ace", "multi_kill", "quad"}},
		{SegmentID: "smoke-ct", Side: SideCT, ReasonCodes: []string{"utility_lineup"}},
	}
	cases := []struct {
		name   string
		filter Filter
		want   []string
	}{
		{"empty keeps all", Filter{}, []string{"awp-ct", "awp-ak-t", "ace-t", "smoke-ct"}},
		{"weapon needs every kill", Filter{Weapon: "awp"}, []string{"awp-ct"}},
		{"weapon prefix is ignored", Filter{Weapon: "ak47"}, []string{"ace-t"}},
		{"any of the kinds", Filter{Kinds: []string{KindClutch, KindAce}}, []string{"awp-ak-t", "ace-t"}},
		{"side", Filter{Side: SideCT}, []string{"awp-ct", "smoke-ct"}},
		{"all at once", Filter{Weapon: "ak47", Kinds: []string{KindAce}, Side: SideT}, []string{"ace-t"}},
		{"no match", Filter{Weapon: "awp", Side: SideT}, []string{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := []string{}
			for _, m := range Apply(list, tc.filter) {
				got = append(got, m.SegmentID)
			}
			if !slices.Equal(got, tc.want) {
				t.Fatalf("Apply = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestParseFilterNormalizesAndRejects(t *testing.T) {
	f, err := ParseFilter(" Weapon_AWP ", []string{"Clutch", "", "clutch", "ace"}, "ct")
	if err != nil {
		t.Fatal(err)
	}
	if f.Weapon != "awp" || f.Side != SideCT || !slices.Equal(f.Kinds, []string{KindClutch, KindAce}) {
		t.Fatalf("filter = %+v", f)
	}
	for _, bad := range []struct{ weapon, kind, side string }{
		{"awp;drop", "", ""},
		{"", "pistol_round", ""},
		{"", "", "spec"},
	} {
		if _, err := ParseFilter(bad.weapon, []string{bad.kind}, bad.side); err == nil {
			t.Errorf("ParseFilter(%q, %q, %q) accepted", bad.weapon, bad.kind, bad.side)
		}
	}
}
