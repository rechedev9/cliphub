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
		MatchID: id, PlayedAt: playedAt,
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
	if lurker == nil || lurker.Team != "Falcons" || lurker.OpponentRank != 1 || lurker.Rounds != 4 || lurker.Roles.Lurk != 4 || lurker.Roles.LurkB != 4 {
		t.Fatalf("lurker line = %+v, want 4 lurked rounds toward B against the rank 1 team", lurker)
	}
}

func TestProPlayersBoardFiltersByRoleDaysAndOpponentRank(t *testing.T) {
	matches := []ProPlayersMatch{
		proMatch("1", proNow.AddDate(0, 0, -10), 1),
		proMatch("2", proNow.AddDate(0, 0, -20), 40),
		proMatch("3", proNow.AddDate(0, 0, -200), 1),
	}
	opts := LeaderboardOptions{Metric: MetricRating, MinRounds: 1, Role: PositionLurkB}
	rows, maps, rounds := ProPlayersBoard(matches, ProPlayersQuery{Side: SideT, Now: proNow, Roles: true}, opts)
	if len(rows) != 1 || rows[0].SteamID64 != "p0" || rows[0].Team != "Falcons" || rows[0].Rounds != 12 || rows[0].Matches != 3 || maps != 3 || rounds == 0 {
		t.Fatalf("rows = %+v maps=%d, want the lurker over three maps", rows, maps)
	}
	rows, maps, _ = ProPlayersBoard(matches, ProPlayersQuery{Side: SideT, Now: proNow, Days: 30, Roles: true}, opts)
	if len(rows) != 1 || rows[0].Matches != 2 || maps != 2 {
		t.Fatalf("30 days = %+v maps=%d, want the two recent maps", rows, maps)
	}
	rows, maps, _ = ProPlayersBoard(matches, ProPlayersQuery{Side: SideT, Now: proNow, Days: 30, Top: 30, Roles: true}, opts)
	if len(rows) != 1 || rows[0].Matches != 1 || maps != 1 {
		t.Fatalf("top 30 = %+v maps=%d, want only the map against rank 1", rows, maps)
	}
	rows, _, _ = ProPlayersBoard(matches, ProPlayersQuery{Side: SideCT, Now: proNow, Roles: true}, LeaderboardOptions{Metric: MetricRating, MinRounds: 1, Role: PositionAnchorB})
	if len(rows) != 0 {
		t.Fatalf("anchor B rows = %+v, want none: nobody played CT", rows)
	}
}

func TestProPlayersBoardUnknownRankIsNotTopN(t *testing.T) {
	unranked := proMatch("1", proNow, 0)
	rows, maps, _ := ProPlayersBoard([]ProPlayersMatch{unranked}, ProPlayersQuery{Now: proNow, Top: 50}, LeaderboardOptions{Metric: MetricKPR, MinRounds: 1})
	// Only the CT enemy faced a ranked team (rank 3); the T side faced rank 0.
	if maps != 1 || len(rows) != 1 || rows[0].SteamID64 != "p3" {
		t.Fatalf("rows = %+v maps=%d, want only the player who faced a ranked team", rows, maps)
	}
}
