package tacticalplan

import (
	"slices"
	"strings"
	"time"
)

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
// with the ranks of their team and of the team they faced and the counts their
// role is read from.
type ProPlayerSide struct {
	SteamID64     string     `json:"steamid64"`
	Name          string     `json:"name"`
	Team          string     `json:"team"`
	Side          Side       `json:"side"`
	OwnRank       int        `json:"own_rank,omitempty"`
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

// ReadProPlayersInput is a pro map to reduce: its analysis, the opening frames of its rounds and its teams.
type ReadProPlayersInput struct {
	Match   MetricsMatch
	Opening map[int][]Frame
	Info    ProPlayersInfo
}

// ReadProPlayersMatch reduces a pro map to the lines of ProPlayersMatch.
// Opening holds the opening-window frames (OpeningFrames); without them no role is read.
func ReadProPlayersMatch(in ReadProPlayersInput) ProPlayersMatch {
	m, info := in.Match, in.Info
	read := ReadMatch(m, MatchReadOptions{Roles: true, Opening: in.Opening})
	out := ProPlayersMatch{MatchID: info.MatchID, Map: m.Document.Demo.Map, Event: info.Event, PlayedAt: info.PlayedAt, Rounds: read.Rounds}
	teamKey := map[string]string{}
	for _, p := range m.Document.Players {
		teamKey[p.SteamID64] = p.TeamKey
	}
	keys := make([]string, 0, len(info.Teams))
	for key := range info.Teams {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	for _, side := range []Side{SideCT, SideT} {
		steams := make([]string, 0, len(read.totals[side]))
		for steam := range read.totals[side] {
			steams = append(steams, steam)
		}
		slices.Sort(steams)
		for _, steam := range steams {
			t := read.totals[side][steam]
			own := info.Teams[teamKey[steam]]
			line := ProPlayerSide{
				SteamID64: steam, Name: t.name, Team: own.Name, Side: side, OwnRank: own.Rank, Rounds: t.rounds, Kills: t.kills, Deaths: t.deaths,
				TradedDeaths: t.tradedDeaths, FlashAssists: t.flashAssists, UtilityDamage: t.utilityDamage, WPA: t.wpa, Box: t.box,
				Roles: read.roles[side][steam].trimmed(),
			}
			// The first other team by key is the rival; a map has two.
			for _, key := range keys {
				if key != teamKey[steam] {
					line.OpponentRank = info.Teams[key].Rank
					break
				}
			}
			out.Sides = append(out.Sides, line)
		}
	}
	return out
}

// MergeProPlayersMatches joins the parts of one map (a server restart splits a
// demo in two): lines of the same player and side add up. The first part names the map.
func MergeProPlayersMatches(parts []ProPlayersMatch) ProPlayersMatch {
	if len(parts) == 0 {
		return ProPlayersMatch{}
	}
	out := parts[0]
	out.Sides, out.Rounds = nil, 0
	type key struct {
		steam string
		side  Side
	}
	at := map[key]int{}
	for _, part := range parts {
		out.Rounds += part.Rounds
		for _, l := range part.Sides {
			k := key{l.SteamID64, l.Side}
			i, ok := at[k]
			if !ok {
				at[k] = len(out.Sides)
				l.Roles = l.Roles.trimmed()
				out.Sides = append(out.Sides, l)
				continue
			}
			sum := out.Sides[i]
			t := sum.totals()
			t.merge(l.totals())
			sum.Rounds, sum.Kills, sum.Deaths, sum.TradedDeaths = t.rounds, t.kills, t.deaths, t.tradedDeaths
			sum.FlashAssists, sum.UtilityDamage, sum.WPA, sum.Box = t.flashAssists, t.utilityDamage, t.wpa, t.box
			roles := RoleCounts{}
			roles.add(sum.Roles)
			roles.add(l.Roles)
			sum.Roles = roles.trimmed()
			out.Sides[i] = sum
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
	// Top keeps the lines against a team ranked Top or better; TeamTop keeps
	// the lines of players whose own team is ranked that high. 0 keeps all.
	Top     int
	TeamTop int
	// Event keeps the maps of one event, compared without case; empty keeps all.
	Event string
	// Roles reads the roles; the lines must come from one map.
	Roles bool
}

// ProPlayersResult is a ranked pro board: the rows, the maps and rounds read and
// the events of the maps in the period, before the event filter.
type ProPlayersResult struct {
	Rows   []LeaderboardRow
	Maps   int
	Rounds int
	Events []string
	// MapRoles are the map roles on the filtered side, when it is set and one map is read.
	MapRoles []MapRoleOption
}

// ProPlayersBoardInput is what a board ranks: the pro maps, the query that narrows them and how the rows are ranked.
type ProPlayersBoardInput struct {
	Matches []ProPlayersMatch
	Query   ProPlayersQuery
	Rows    LeaderboardOptions
}

// ProPlayersBoard ranks the players of the pro maps that pass the query, the
// way a library leaderboard does.
func ProPlayersBoard(in ProPlayersBoardInput) ProPlayersResult {
	matches, q, opts := in.Matches, in.Query, in.Rows
	var s LeaderboardSample
	events := map[string]bool{}
	for _, m := range matches {
		if q.Days > 0 && (m.PlayedAt.IsZero() || m.PlayedAt.Before(q.Now.AddDate(0, 0, -q.Days))) {
			continue
		}
		if m.Event != "" {
			events[m.Event] = true
		}
		if q.Event != "" && !strings.EqualFold(m.Event, q.Event) {
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
			if q.TeamTop > 0 && (l.OwnRank < 1 || l.OwnRank > q.TeamTop) {
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
	out := ProPlayersResult{Rows: s.Rows(opts), Maps: s.Matches, Rounds: s.Rounds, Events: make([]string, 0, len(events))}
	for event := range events {
		out.Events = append(out.Events, event)
	}
	slices.Sort(out.Events)
	if q.Roles && q.Side != "" {
		out.MapRoles = s.MapRoleOptions(q.Side, opts.MinRounds)
	}
	return out
}
