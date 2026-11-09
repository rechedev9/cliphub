package tacticalplan

import (
	"testing"
	"time"
)

var proNow = time.Date(2026, 11, 9, 12, 0, 0, 0, time.UTC)

func proMatch(id string, playedAt time.Time, rankOfCT int) ProPlayersMatch {
	m, frames := roleMatch("00000000-0000-0000-0000-0000000000e1", true)
	for i := range m.Document.Players {
		m.Document.Players[i].TeamKey = "t-start"
	}
	m.Document.Players[3].TeamKey = "ct-start"
	return ReadProPlayersMatch(m, frames, ProPlayersInfo{
		MatchID: id, PlayedAt: playedAt, Event: "IEM Cologne",
		Teams: map[string]ProPlayersTeam{"t-start": {Name: "Falcons", Rank: 3}, "ct-start": {Name: "Vitality", Rank: rankOfCT}},
	})
}

func TestProPlayersMatchKeepsTeamRanksAndRoles(t *testing.T) {
	m := proMatch("111", proNow, 1)
	var lurker *ProPlayerSide
	for i, l := range m.Sides {
		if l.SteamID64 == "p0" && l.Side == SideT {
			lurker = &m.Sides[i]
		}
	}
	if lurker == nil || lurker.Team != "Falcons" || lurker.OpponentRank != 1 || lurker.OwnRank != 3 || lurker.Rounds != 4 || lurker.Roles.Lurk != 4 || lurker.Roles.LurkB != 4 {
		t.Fatalf("lurker line = %+v, want 4 lurked rounds toward B against the rank 1 team", lurker)
	}
}

func TestProPlayersBoardFiltersByRoleDaysEventAndRanks(t *testing.T) {
	matches := []ProPlayersMatch{
		proMatch("1", proNow.AddDate(0, 0, -10), 1),
		proMatch("2", proNow.AddDate(0, 0, -20), 40),
		proMatch("3", proNow.AddDate(0, 0, -200), 1),
	}
	matches[1].Event = "BLAST Open"
	opts := LeaderboardOptions{Metric: MetricRating, MinRounds: 1, Role: PositionLurkB}
	res := ProPlayersBoard(matches, ProPlayersQuery{Side: SideT, Now: proNow, Roles: true}, opts)
	if len(res.Rows) != 1 || res.Rows[0].SteamID64 != "p0" || res.Rows[0].Team != "Falcons" || res.Rows[0].Rounds != 12 || res.Rows[0].Matches != 3 || res.Maps != 3 {
		t.Fatalf("rows = %+v maps=%d, want the lurker over three maps", res.Rows, res.Maps)
	}
	res = ProPlayersBoard(matches, ProPlayersQuery{Side: SideT, Now: proNow, Days: 30, Roles: true}, opts)
	if len(res.Rows) != 1 || res.Rows[0].Matches != 2 || res.Maps != 2 {
		t.Fatalf("30 days = %+v maps=%d, want the two recent maps", res.Rows, res.Maps)
	}
	res = ProPlayersBoard(matches, ProPlayersQuery{Side: SideT, Now: proNow, Days: 30, Top: 30, Roles: true}, opts)
	if len(res.Rows) != 1 || res.Rows[0].Matches != 1 || res.Maps != 1 {
		t.Fatalf("top 30 = %+v maps=%d, want only the map against rank 1", res.Rows, res.Maps)
	}
	res = ProPlayersBoard(matches, ProPlayersQuery{Side: SideT, Now: proNow, Days: 30, Event: "blast open", Roles: true}, opts)
	if res.Maps != 1 || len(res.Events) != 2 {
		t.Fatalf("event = %+v, want the one BLAST map and both events listed", res)
	}
	res = ProPlayersBoard(matches, ProPlayersQuery{Side: SideT, Now: proNow, TeamTop: 2, Roles: true}, opts)
	if len(res.Rows) != 0 {
		t.Fatalf("team top 2 = %+v, want nobody: Falcons are ranked 3", res.Rows)
	}
	res = ProPlayersBoard(matches, ProPlayersQuery{Side: SideCT, Now: proNow, Roles: true}, LeaderboardOptions{Metric: MetricRating, MinRounds: 1, Role: PositionAnchorB})
	if len(res.Rows) != 0 {
		t.Fatalf("anchor B rows = %+v, want none: nobody played CT", res.Rows)
	}
}

func TestProPlayersBoardUnknownRankIsNotTopN(t *testing.T) {
	unranked := proMatch("1", proNow, 0)
	res := ProPlayersBoard([]ProPlayersMatch{unranked}, ProPlayersQuery{Now: proNow, Top: 50}, LeaderboardOptions{Metric: MetricKPR, MinRounds: 1})
	if res.Maps != 1 || len(res.Rows) != 1 || res.Rows[0].SteamID64 != "p3" {
		t.Fatalf("rows = %+v maps=%d, want only the player who faced a ranked team", res.Rows, res.Maps)
	}
}

func TestMergeProPlayersMatchesAddsTheRestartParts(t *testing.T) {
	a, b := proMatch("1", proNow, 1), proMatch("1", proNow, 1)
	merged := MergeProPlayersMatches([]ProPlayersMatch{a, b})
	if merged.Rounds != a.Rounds+b.Rounds || len(merged.Sides) != len(a.Sides) {
		t.Fatalf("merged = %d rounds %d lines, want the rounds added and the lines joined", merged.Rounds, len(merged.Sides))
	}
	for _, l := range merged.Sides {
		if l.SteamID64 == "p0" && l.Side == SideT && (l.Rounds != 8 || l.Roles.Sampled != 8 || l.Roles.LurkB != 8) {
			t.Fatalf("lurker = %+v, want two parts added", l)
		}
	}
}

func TestMapRolesNameWhereTheWindowIsSpent(t *testing.T) {
	m, frames := roleMatch("00000000-0000-0000-0000-0000000000e2", true)
	m.Document.Geometry.Callouts = append(m.Document.Geometry.Callouts,
		Callout{Name: "Canal", Level: "default", Center: [2]float64{-2900, -2900}}, Callout{Name: "Middle", Level: "default", Center: [2]float64{0, 0}})
	var s LeaderboardSample
	s.Add(ReadMatch(m, MatchReadOptions{Roles: true, Opening: frames}), SideT)
	rows := s.Rows(LeaderboardOptions{Metric: MetricKPR, MinRounds: 1, MapRole: "Canal", MapRoleSide: SideT})
	if len(rows) != 1 || rows[0].SteamID64 != "p0" {
		t.Fatalf("Canal rows = %+v, want only the far player", rows)
	}
	if b := rows[0].Breakdown; b == nil || b.T == nil || b.T.MapRole != "Canal" || b.T.Places[0].Pct != 100 || b.T.Roles[1].Key != PositionLurkB || b.T.Roles[1].Pct != 100 {
		t.Fatalf("breakdown = %+v, want 100 percent at Canal as a B lurker", rows[0].Breakdown)
	}
	if len(rows[0].Examples) != 4 {
		t.Fatalf("examples = %+v, want the four rounds at Canal", rows[0].Examples)
	}
	opts := s.MapRoleOptions(SideT, 1)
	if len(opts) != 2 || opts[0].Key != "Middle" && opts[0].Key != "Canal" {
		t.Fatalf("options = %+v", opts)
	}
	if got := s.Rows(LeaderboardOptions{Metric: MetricKPR, MinRounds: 1, MapRole: MapRoleAllround, MapRoleSide: SideT}); len(got) != 0 {
		t.Fatalf("allround rows = %+v, want none: everyone holds a place", got)
	}
}
