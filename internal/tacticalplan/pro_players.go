package tacticalplan

import "time"

// ProPlayersTeam is a team of a pro map as HLTV lists it; Rank is its HLTV
// world rank at the match, 0 when unknown.
type ProPlayersTeam struct {
	Name string `json:"name"`
	Rank int    `json:"rank,omitempty"`
}

// ProPlayersInfo is what the match record adds to a pro map's document. Teams
// are keyed by the document's team key ("ct-start", "t-start").
type ProPlayersInfo struct {
	MatchID  string
	Event    string
	PlayedAt time.Time
	Teams    map[string]ProPlayersTeam
}

// ProPlayerSide is one player's additive totals on one side of one pro map,
// with the rank of the team they faced and the counts their role is read from.
type ProPlayerSide struct {
	SteamID64     string     `json:"steamid64"`
	Name          string     `json:"name"`
	Team          string     `json:"team"`
	Side          Side       `json:"side"`
	OpponentRank  int        `json:"opponent_rank,omitempty"`
	Rounds        int        `json:"rounds"`
	Kills         int        `json:"kills"`
	Deaths        int        `json:"deaths"`
	TradedDeaths  int        `json:"traded_deaths"`
	FlashAssists  int        `json:"flash_assists"`
	UtilityDamage int        `json:"utility_damage"`
	WPA           float64    `json:"wpa"`
	Box           BoxScore   `json:"box"`
	Roles         RoleCounts `json:"role_counts"`
}

func (l ProPlayerSide) totals() playerTotals {
	return playerTotals{
		steamID: l.SteamID64, name: l.Name, rounds: l.Rounds, kills: l.Kills, deaths: l.Deaths, tradedDeaths: l.TradedDeaths,
		flashAssists: l.FlashAssists, utilityDamage: l.UtilityDamage, wpa: l.WPA, box: l.Box,
	}
}

// ProPlayersMatch is one pro map reduced to its players' lines per side, so the
// server can answer any filter without reading the demo again.
type ProPlayersMatch struct {
	MatchID  string          `json:"match_id"`
	Map      string          `json:"map"`
	Event    string          `json:"event,omitempty"`
	PlayedAt time.Time       `json:"played_at"`
	Rounds   int             `json:"rounds"`
	Sides    []ProPlayerSide `json:"sides"`
}

// ReadProPlayersMatch reduces a pro map to the lines of ProPlayersMatch.
// opening holds the opening-window frames (OpeningFrames); without them no role is read.
func ReadProPlayersMatch(m MetricsMatch, opening map[int][]Frame, info ProPlayersInfo) ProPlayersMatch {
	read := ReadMatch(m, MatchReadOptions{Roles: true, Opening: opening})
	out := ProPlayersMatch{MatchID: info.MatchID, Map: m.Document.Demo.Map, Event: info.Event, PlayedAt: info.PlayedAt, Rounds: read.Rounds}
	teamKey := map[string]string{}
	for _, p := range m.Document.Players {
		teamKey[p.SteamID64] = p.TeamKey
	}
	for _, side := range []Side{SideCT, SideT} {
		for steam, t := range read.totals[side] {
			own := info.Teams[teamKey[steam]]
			line := ProPlayerSide{
				SteamID64: steam, Name: t.name, Team: own.Name, Side: side, Rounds: t.rounds, Kills: t.kills, Deaths: t.deaths,
				TradedDeaths: t.tradedDeaths, FlashAssists: t.flashAssists, UtilityDamage: t.utilityDamage, WPA: t.wpa, Box: t.box,
				Roles: read.roles[side][steam],
			}
			for key, other := range info.Teams {
				if key != teamKey[steam] {
					line.OpponentRank = other.Rank
				}
			}
			out.Sides = append(out.Sides, line)
		}
	}
	return out
}

// ProPlayersQuery narrows the pro maps a board reads.
type ProPlayersQuery struct {
	Side Side
	// Days keeps the maps played within this many days of Now; 0 keeps all.
	Days int
	Now  time.Time
	// Top keeps the lines against a team ranked Top or better; 0 keeps all.
	Top int
	// Roles reads the roles; the lines must come from one map.
	Roles bool
}

// ProPlayersBoard ranks the players of the pro maps that pass the query, the
// way a library leaderboard does, and says how many maps and rounds it read.
func ProPlayersBoard(matches []ProPlayersMatch, q ProPlayersQuery, opts LeaderboardOptions) (rows []LeaderboardRow, maps, rounds int) {
	var s LeaderboardSample
	for _, m := range matches {
		if q.Days > 0 && (m.PlayedAt.IsZero() || m.PlayedAt.Before(q.Now.AddDate(0, 0, -q.Days))) {
			continue
		}
		read := MatchRead{
			Rounds: m.Rounds, HasRoles: q.Roles, teams: map[string]string{},
			totals: map[Side]map[string]playerTotals{SideCT: {}, SideT: {}}, roles: map[Side]map[string]RoleCounts{SideCT: {}, SideT: {}},
		}
		kept := false
		for _, l := range m.Sides {
			if q.Top > 0 && (l.OpponentRank < 1 || l.OpponentRank > q.Top) {
				continue
			}
			kept = kept || q.Side == "" || l.Side == q.Side
			read.totals[l.Side][l.SteamID64] = l.totals()
			read.roles[l.Side][l.SteamID64] = l.Roles
			read.teams[l.SteamID64] = l.Team
		}
		if kept {
			s.Add(read, q.Side)
		}
	}
	return s.Rows(opts), s.Matches, s.Rounds
}
