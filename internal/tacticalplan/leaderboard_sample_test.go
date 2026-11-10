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
