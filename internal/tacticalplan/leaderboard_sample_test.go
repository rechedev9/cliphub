package tacticalplan

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

// roleMatch has a T team of three (slots 0 to 2) and one CT (slot 3) over four rounds.
// Slot 0 stays 2000 units from its mates and leans to B on T, and holds B on CT in a mirror setup.
func roleMatch(id string, tSide bool) (MetricsMatch, map[int][]Frame) {
	d := NewDocument()
	d.Demo.Map = "de_anubis"
	d.Players = []Player{
		{Slot: 0, SteamID64: "p0", Name: "lurker"}, {Slot: 1, SteamID64: "p1", Name: "mate1"},
		{Slot: 2, SteamID64: "p2", Name: "mate2"}, {Slot: 3, SteamID64: "p3", Name: "enemy"},
	}
	d.Geometry = MapGeometry{Callouts: []Callout{
		{Name: "BombsiteA", Level: "default", Center: [2]float64{3000, 3000}},
		{Name: "BombsiteB", Level: "default", Center: [2]float64{-3000, -3000}},
	}}
	frames := map[int][]Frame{}
	side, other := SideT, SideCT
	if !tSide {
		side, other = SideCT, SideT
	}
	for n := 1; n <= 4; n++ {
		d.Rounds = append(d.Rounds, Round{Number: n, TickFreezeEnd: n * 100, Players: []PlayerRound{
			{Slot: 0, Side: side, Survived: true}, {Slot: 1, Side: side, Survived: true}, {Slot: 2, Side: side, Survived: true}, {Slot: 3, Side: other, Survived: true},
		}})
		var window []Frame
		for i := 0; i < 4; i++ {
			window = append(window, Frame{Tick: i * 8, Samples: []Sample{
				{Slot: 0, X: -2900, Y: -2900, Flags: FlagAlive}, {Slot: 1, X: 0, Y: 0, Flags: FlagAlive},
				{Slot: 2, X: 50, Y: 0, Flags: FlagAlive}, {Slot: 3, X: 3000, Y: 3000, Flags: FlagAlive},
			}})
		}
		frames[n] = window
	}
	return MetricsMatch{MatchID: uuid.MustParse(id), PlayedAt: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), Document: d}, frames
}

func TestLeaderboardReadsTRolesFromDistanceAndLean(t *testing.T) {
	m, frames := roleMatch("00000000-0000-0000-0000-0000000000d1", true)
	read := ReadMatch(m, MatchReadOptions{Roles: true, Opening: frames})
	var s LeaderboardSample
	s.Add(read, SideT)
	rows := s.Rows(LeaderboardOptions{Metric: MetricKPR, MinRounds: 1})
	roles := map[string]PlayerRoles{}
	for _, row := range rows {
		if row.Roles == nil {
			t.Fatalf("row %s has no roles", row.Name)
		}
		roles[row.SteamID64] = *row.Roles
	}
	if roles["p0"].T != PositionLurkB {
		t.Fatalf("far player's T role = %q, want %q", roles["p0"].T, PositionLurkB)
	}
	if roles["p1"].T != PositionPack {
		t.Fatalf("grouped mate's T role = %q, want %q", roles["p1"].T, PositionPack)
	}
}

func TestLeaderboardTeammateDistanceIsThreeDimensional(t *testing.T) {
	m, frames := roleMatch("00000000-0000-0000-0000-0000000000d4", true)
	// Slot 0 sits right under its mates: 2D distance is nil, but a floor away in 3D.
	for _, round := range frames {
		for _, frame := range round {
			frame.Samples[0].X, frame.Samples[0].Y, frame.Samples[0].Z = 0, 0, 0
			frame.Samples[1].Z, frame.Samples[2].Z = 2000, 2000
		}
	}
	var s LeaderboardSample
	s.Add(ReadMatch(m, MatchReadOptions{Roles: true, Opening: frames}), SideT)
	for _, row := range s.Rows(LeaderboardOptions{Metric: MetricKPR, MinRounds: 1}) {
		if row.SteamID64 == "p0" && (row.Roles == nil || row.Roles.T == "" || row.Roles.T == PositionPack) {
			t.Fatalf("player a floor under the pack has role %+v, want a lurk role", row.Roles)
		}
	}
}

func TestLeaderboardRoleFilterKeepsOnlyThatRole(t *testing.T) {
	m, frames := roleMatch("00000000-0000-0000-0000-0000000000d2", true)
	var s LeaderboardSample
	s.Add(ReadMatch(m, MatchReadOptions{Roles: true, Opening: frames}), SideT)
	rows := s.Rows(LeaderboardOptions{Metric: MetricKPR, MinRounds: 1, Role: PositionLurkB})
	if len(rows) != 1 || rows[0].SteamID64 != "p0" || rows[0].Rank != 1 {
		t.Fatalf("lurk B rows = %+v, want only the far player", rows)
	}
	if len(rows[0].Examples) != 4 || rows[0].Examples[0].Round != 1 {
		t.Fatalf("examples = %+v, want the player's four T rounds", rows[0].Examples)
	}
	if got := s.Rows(LeaderboardOptions{Metric: MetricKPR, MinRounds: 5, Role: PositionLurkB}); len(got) != 0 {
		t.Fatalf("min rounds 5 kept %+v", got)
	}
}

func TestLeaderboardCTAnchorUsesTheSideOnly(t *testing.T) {
	m, frames := roleMatch("00000000-0000-0000-0000-0000000000d3", false)
	for _, round := range frames {
		for _, frame := range round {
			frame.Samples[0].X, frame.Samples[0].Y = -3000, -3000
		}
	}
	var s LeaderboardSample
	s.Add(ReadMatch(m, MatchReadOptions{Roles: true, Opening: frames}), SideCT)
	rows := s.Rows(LeaderboardOptions{Metric: MetricKPR, MinRounds: 1, Role: PositionAnchorB})
	if len(rows) != 1 || rows[0].SteamID64 != "p0" {
		t.Fatalf("anchor B rows = %+v", rows)
	}
	var tOnly LeaderboardSample
	tOnly.Add(ReadMatch(m, MatchReadOptions{}), SideT)
	if got := tOnly.Rows(LeaderboardOptions{Metric: MetricKPR, MinRounds: 1}); len(got) != 1 || got[0].SteamID64 != "p3" {
		t.Fatalf("T rows = %+v, want only the enemy that played T", got)
	}
}

func TestMapRolesLabelSiteRolesThenCalloutsThenTodoterreno(t *testing.T) {
	m, frames := roleMatch("00000000-0000-0000-0000-0000000000d5", true)
	var s LeaderboardSample
	s.Add(ReadMatch(m, MatchReadOptions{Roles: true, Opening: frames}), "")
	roles := s.MapRoles()
	if r := roles["p0"].T; r == nil || r.Role != PositionLurkB || r.Label != "Lurk de B" {
		t.Fatalf("lurker's T map role = %+v, want Lurk de B", r)
	}
	if r := roles["p1"].T; r == nil || r.Role != PositionPack || r.Label == "" || r.Label == MapRoleAllroundLabel {
		t.Fatalf("pack mate's T map role = %+v, want the callout it holds", r)
	}
	if roles["p0"].CT != nil {
		t.Fatalf("a player without CT rounds has CT role %+v", roles["p0"].CT)
	}
	spread := RoleCounts{Sampled: 4, Other: 4, Places: map[string]int{"Mid": 1, "Long": 1, "Short": 1, "Cat": 1}}
	if got := spread.sideMapRole(SideCT); got == nil || got.Role != PositionRotator || got.Label != MapRoleAllroundLabel {
		t.Fatalf("rotator with no callout at 40%% = %+v, want Todoterreno", got)
	}
}

// ctRead is one map's CT reads: each player's role counts over four rounds.
func ctRead(mapName string, counts map[string]RoleCounts) MatchRead {
	r := MatchRead{
		Rounds: 4, Map: mapName, HasRoles: true,
		totals: map[Side]map[string]playerTotals{SideCT: {}, SideT: {}}, roles: map[Side]map[string]RoleCounts{SideCT: {}, SideT: {}},
	}
	for steam, c := range counts {
		r.totals[SideCT][steam] = playerTotals{steamID: steam, name: steam, rounds: 4, kills: 4, deaths: 2}
		r.roles[SideCT][steam] = c
	}
	return r
}

func steamIDs(rows []LeaderboardRow) []string {
	out := []string{}
	for _, r := range rows {
		out = append(out, r.SteamID64)
	}
	return out
}

func TestAllroundOverEveryMapReadsEachMapApart(t *testing.T) {
	spread := RoleCounts{Sampled: 4, Other: 4, Places: map[string]int{"Long": 1, "Short": 1, "Cat": 1, "Mid": 1}}
	var s LeaderboardSample
	for _, m := range []string{"de_dust2", "de_mirage", "de_nuke"} {
		// "held" holds one callout per map: pooled over three maps no callout reaches 40%.
		held := RoleCounts{Sampled: 4, Other: 4, Places: map[string]int{m + "_spot": 4}}
		s.Add(ctRead(m, map[string]RoleCounts{"held": held, "spread": spread}), SideCT)
	}
	rows := s.Rows(LeaderboardOptions{Metric: MetricKPR, MinRounds: 1, MapRole: MapRoleAllround, MapRoleSide: SideCT, AnyMap: true})
	if got := steamIDs(rows); len(got) != 1 || got[0] != "spread" {
		t.Fatalf("allround over every map = %v, want only the player spread on each map", got)
	}
	if rows[0].Roles != nil || rows[0].Breakdown != nil {
		t.Fatalf("a board over every map mixes maps in roles %+v breakdown %+v", rows[0].Roles, rows[0].Breakdown)
	}
}

func TestAllroundFilterAgreesWithTheSiteRoleLabel(t *testing.T) {
	anchor := RoleCounts{Sampled: 4, InA: 4, Places: map[string]int{"BombsiteA": 1, "Pit": 1, "Long": 1, "Short": 1}}
	rotator := RoleCounts{Sampled: 4, Other: 4, Places: map[string]int{"Mid": 1, "Cat": 1, "Lower": 1, "Ramp": 1}}
	var s LeaderboardSample
	s.Add(ctRead("de_dust2", map[string]RoleCounts{"anchor": anchor, "rotator": rotator}), SideCT)
	if r := s.MapRoles()["anchor"].CT; r == nil || r.Label != "Ancla A" {
		t.Fatalf("anchor card = %+v, want Ancla A", r)
	}
	rows := s.Rows(LeaderboardOptions{Metric: MetricKPR, MinRounds: 1, MapRole: MapRoleAllround, MapRoleSide: SideCT})
	if got := steamIDs(rows); len(got) != 1 || got[0] != "rotator" {
		t.Fatalf("CT todoterreno = %v, want the rotator only, not the anchor labelled Ancla A", got)
	}
	if opts := s.MapRoleOptions(SideCT, 1); len(opts) != 1 || opts[0].Key != MapRoleAllround || opts[0].Players != 1 {
		t.Fatalf("map role options = %+v, want one Todoterreno player", opts)
	}
}

func TestRowsFilterOnLibraryRole(t *testing.T) {
	m, frames := roleMatch("00000000-0000-0000-0000-0000000000d6", true)
	var s LeaderboardSample
	s.Add(ReadMatch(m, MatchReadOptions{Roles: true, Opening: frames}), "")
	rows := s.Rows(LeaderboardOptions{Metric: MetricKPR, MinRounds: 1, LibraryRoles: map[string]string{"p0": "awp", "p1": "rifler"}, LibraryRole: "awp"})
	if len(rows) != 1 || rows[0].SteamID64 != "p0" || rows[0].LibraryRole != "awp" {
		t.Fatalf("awp rows = %+v, want only p0", rows)
	}
	if ex := s.PlayerExamples(LeaderboardOptions{Role: PositionLurkB}); len(ex["p0"]) != 4 || len(ex["p1"]) != 0 {
		t.Fatalf("lurk B examples = %+v, want p0's four rounds only", ex)
	}
}

func TestRatingAndKD(t *testing.T) {
	totals := playerTotals{rounds: 20, kills: 20, deaths: 10, box: BoxScore{Damage: 1600, Assists: 4, KASTRounds: 15}}
	values := totals.values()
	if values[MetricKD] != 2 {
		t.Fatalf("kd = %v, want 2", values[MetricKD])
	}
	// 0.0073*75 + 0.3591 - 0.2665 + 0.2372*(2.13+0.084-0.41) + 0.0032*80 + 0.1587
	if r := values[MetricRating]; r < 1.45 || r > 1.55 {
		t.Fatalf("rating = %v, want about 1.5", r)
	}
}

func TestRoleSide(t *testing.T) {
	if RoleSide(PositionAnchorB) != SideCT || RoleSide(PositionHalfLurk) != SideT || RoleSide("x") != "" {
		t.Fatal("RoleSide put a role on the wrong side")
	}
}

func TestLeaderboardRoundWithoutACalloutIsNotSampled(t *testing.T) {
	m, frames := roleMatch("00000000-0000-0000-0000-0000000000d5", true)
	m.Document.Geometry = MapGeometry{}
	var s LeaderboardSample
	s.Add(ReadMatch(m, MatchReadOptions{Roles: true, Opening: frames}), SideT)
	for _, row := range s.Rows(LeaderboardOptions{Metric: MetricKPR, MinRounds: 1}) {
		if (row.Roles != nil && (row.Roles.CT != "" || row.Roles.T != "")) || row.Breakdown != nil {
			t.Fatalf("row %s has roles %+v without any callout to place its rounds", row.Name, row.Roles)
		}
	}
}
