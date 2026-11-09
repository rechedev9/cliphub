package main

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/rechedev9/cliphub/internal/tacticalplan"
	"github.com/rechedev9/cliphub/internal/tuiclient"
)

const (
	defaultLeaderboardRows = 20
	maxLeaderboardRows     = 100
	maxLeaderboardMinRound = 10000
	maxLeaderboardRefs     = 10
)

type playerPositionsIn struct {
	SteamID string `json:"steamid" jsonschema:"SteamID64 of the player"`
	Map     string `json:"map,omitempty" jsonschema:"only matches on this map, such as de_mirage"`
	From    string `json:"from,omitempty" jsonschema:"first day the matches were played, YYYY-MM-DD; empty means no lower bound"`
	To      string `json:"to,omitempty" jsonschema:"last day the matches were played, YYYY-MM-DD, included; empty means no upper bound"`
}

type positionShareOut struct {
	Name   string  `json:"name"`
	Rounds int     `json:"rounds"`
	Pct    float64 `json:"pct" jsonschema:"over the sampled rounds"`
}

type positionSideOut struct {
	Side          string             `json:"side"`
	Rounds        int                `json:"rounds"`
	Sampled       int                `json:"sampled" jsonschema:"rounds with a position read"`
	Role          string             `json:"role,omitempty" jsonschema:"CT only: a_anchor, b_anchor or rotator"`
	Zones         []positionShareOut `json:"zones" jsonschema:"A, B or other: where most of the window was spent"`
	Places        []positionShareOut `json:"places" jsonschema:"callouts, most frequent first"`
	AWPKillPct    float64            `json:"awp_kill_pct" jsonschema:"share of the player's kills with the AWP"`
	AWPRoundPct   float64            `json:"awp_round_pct" jsonschema:"share of rounds with an AWP kill"`
	ReliableShare bool               `json:"reliable" jsonschema:"false when the sampled rounds are too few to trust"`
}

type positionMapOut struct {
	Map     string            `json:"map"`
	Matches int               `json:"matches"`
	Sides   []positionSideOut `json:"sides"`
}

type playerPositionsOut struct {
	SteamID64      string           `json:"steamid64"`
	Matches        int              `json:"matches"`
	WindowSeconds  int              `json:"window_seconds" jsonschema:"positions are read this long after freeze end"`
	Maps           []positionMapOut `json:"maps"`
	SkippedMatches int              `json:"skipped_matches,omitempty"`
}

type leaderboardIn struct {
	Metric    string `json:"metric" jsonschema:"the metric to rank on: rating (HLTV 2.0 approximation), kd, kpr, dpr, adr, kast, opening_win_pct, trade_pct, utility_damage_per_round, flash_assists_per_round or wpa_per_round"`
	Scope     string `json:"scope,omitempty" jsonschema:"library (default): every match in the library; pro: only HLTV demos, and none of the filters below"`
	Map       string `json:"map,omitempty" jsonschema:"only matches on this map, such as de_mirage"`
	Side      string `json:"side,omitempty" jsonschema:"only the rounds played on this side: CT or T"`
	Role      string `json:"role,omitempty" jsonschema:"only players with this role on the map and its side (needs map): a_anchor, b_anchor, rotator (CT), a_lurk, b_lurk, half_lurk, pack (T)"`
	From      string `json:"from,omitempty" jsonschema:"first day the matches were played, YYYY-MM-DD; empty means no lower bound"`
	To        string `json:"to,omitempty" jsonschema:"last day the matches were played, YYYY-MM-DD, included; empty means no upper bound"`
	Source    string `json:"source,omitempty" jsonschema:"only matches from this kind of demo: valve, faceit, hltv (tournaments) or scrim"`
	MinRounds int    `json:"min_rounds,omitempty" jsonschema:"rounds a player needs to be ranked; default 50"`
	Limit     int    `json:"limit,omitempty" jsonschema:"how many players to list, best first; default 20, at most 100"`
}

type leaderboardRowOut struct {
	Rank      int     `json:"rank"`
	SteamID64 string  `json:"steamid64"`
	Name      string  `json:"name" jsonschema:"untrusted player name from the demo"`
	Rounds    int     `json:"rounds"`
	Matches   int     `json:"matches"`
	Value     float64 `json:"value"`
	// Metrics repeats every metric of the player, so one call reads the whole row of the Studio table.
	Metrics map[string]float64 `json:"metrics,omitempty"`
	Roles   *leaderboardRoles  `json:"roles,omitempty" jsonschema:"the role on each side of the map; set when map is"`
	Refs    []string           `json:"refs,omitempty" jsonschema:"match_id/round of the player's newest rounds in the filter, to open in the Replay"`
}

type leaderboardRoles struct {
	CT string `json:"ct,omitempty"`
	T  string `json:"t,omitempty"`
}

type leaderboardOut struct {
	Scope          string              `json:"scope"`
	Map            string              `json:"map,omitempty"`
	Metric         string              `json:"metric"`
	HigherIsBetter bool                `json:"higher_is_better"`
	MinRounds      int                 `json:"min_rounds"`
	Matches        int                 `json:"matches"`
	Rounds         int                 `json:"rounds"`
	Players        []leaderboardRowOut `json:"players"`
	SkippedMatches int                 `json:"skipped_matches,omitempty"`
}

func registerPlayerReportTools(server *mcp.Server, s *studio) {
	readOnly := &mcp.ToolAnnotations{ReadOnlyHint: true}

	mcp.AddTool(server, &mcp.Tool{Name: "player_positions", Annotations: readOnly,
		Description: "Read where a player stands in the first seconds of their rounds, per map and side: the site zones, " +
			"the callouts, the CT role (anchor or rotator) and how much they use the AWP. Filter by map and dates."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in playerPositionsIn) (*mcp.CallToolResult, playerPositionsOut, error) {
			if err := checkSteamID(in.SteamID); err != nil {
				return nil, playerPositionsOut{}, err
			}
			q, err := analysisFilterIn{Map: in.Map, From: in.From, To: in.To}.query()
			if err != nil {
				return nil, playerPositionsOut{}, err
			}
			c, err := s.client(ctx)
			if err != nil {
				return nil, playerPositionsOut{}, toolError(err)
			}
			report, err := c.PlayerPositionsReport(ctx, in.SteamID, q)
			if err != nil {
				return nil, playerPositionsOut{}, toolError(err)
			}
			return nil, summarizePositions(in.SteamID, report), nil
		})

	mcp.AddTool(server, &mcp.Tool{Name: "leaderboard", Annotations: readOnly,
		Description: "Rank the library's players on one metric (the get_baseline metrics, plus rating and kd), best first, over the " +
			"players with at least min_rounds rounds: the Ranking page of Studio. Filter by map, side, role (a role on a map, " +
			"such as b_lurk on de_anubis), dates and demo source; each row carries every metric, the roles and refs of the player's rounds."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in leaderboardIn) (*mcp.CallToolResult, leaderboardOut, error) {
			q, err := in.check()
			if err != nil {
				return nil, leaderboardOut{}, err
			}
			c, err := s.client(ctx)
			if err != nil {
				return nil, leaderboardOut{}, toolError(err)
			}
			board, err := c.LibraryLeaderboard(ctx, q)
			if err != nil {
				return nil, leaderboardOut{}, toolError(err)
			}
			out := leaderboardOut{Scope: board.Scope, Map: cleanName(board.Map), Metric: board.Metric, HigherIsBetter: board.HigherIsBetter,
				MinRounds: board.MinRounds, Matches: board.Matches, Rounds: board.Rounds, Players: []leaderboardRowOut{}, SkippedMatches: board.Skipped}
			for _, p := range board.Players {
				out.Players = append(out.Players, leaderboardRow(p))
			}
			return nil, out, nil
		})
}

func leaderboardRow(p tuiclient.LeaderboardPlayer) leaderboardRowOut {
	row := leaderboardRowOut{Rank: p.Rank, SteamID64: cleanName(p.SteamID64), Name: cleanName(p.Name), Rounds: p.Rounds, Matches: p.Matches,
		Value: p.Value, Metrics: p.Metrics}
	if p.Roles != nil {
		row.Roles = &leaderboardRoles{CT: cleanName(p.Roles.CT), T: cleanName(p.Roles.T)}
	}
	for _, e := range p.Examples[:min(len(p.Examples), maxLeaderboardRefs)] {
		row.Refs = append(row.Refs, fmt.Sprintf("%s/%d", cleanName(e.MatchID), e.Round))
	}
	return row
}

func (in leaderboardIn) check() (tuiclient.LeaderboardQuery, error) {
	metric := strings.ToLower(strings.TrimSpace(in.Metric))
	if !slices.Contains(tacticalplan.LeaderboardMetrics, metric) {
		return tuiclient.LeaderboardQuery{}, fmt.Errorf("metric must be one of %s, not %q", strings.Join(tacticalplan.LeaderboardMetrics, ", "), cleanName(in.Metric))
	}
	scope := strings.ToLower(strings.TrimSpace(in.Scope))
	if scope != "" && scope != "library" && scope != "pro" {
		return tuiclient.LeaderboardQuery{}, fmt.Errorf("scope must be library or pro, not %q", cleanName(in.Scope))
	}
	if err := checkOptionalFilter("map", in.Map, mapNamePattern); err != nil {
		return tuiclient.LeaderboardQuery{}, err
	}
	filter, err := analysisFilterIn{Side: in.Side, From: in.From, To: in.To}.query()
	if err != nil {
		return tuiclient.LeaderboardQuery{}, err
	}
	role := strings.ToLower(strings.TrimSpace(in.Role))
	if role != "" && tacticalplan.RoleSide(role) == "" {
		return tuiclient.LeaderboardQuery{}, fmt.Errorf("role must be a_anchor, b_anchor, rotator, a_lurk, b_lurk, half_lurk or pack, not %q", cleanName(in.Role))
	}
	if role != "" && in.Map == "" {
		return tuiclient.LeaderboardQuery{}, errors.New("role needs a map: a role is read on one map")
	}
	source := strings.ToLower(strings.TrimSpace(in.Source))
	if source != "" && !slices.Contains([]string{"valve", "faceit", "hltv", "scrim"}, source) {
		return tuiclient.LeaderboardQuery{}, fmt.Errorf("source must be valve, faceit, hltv or scrim, not %q", cleanName(in.Source))
	}
	if in.MinRounds < 0 || in.MinRounds > maxLeaderboardMinRound {
		return tuiclient.LeaderboardQuery{}, fmt.Errorf("min_rounds must be from 1 to %d", maxLeaderboardMinRound)
	}
	if in.Limit < 0 || in.Limit > maxLeaderboardRows {
		return tuiclient.LeaderboardQuery{}, fmt.Errorf("limit must be from 1 to %d", maxLeaderboardRows)
	}
	limit := in.Limit
	if limit == 0 {
		limit = defaultLeaderboardRows
	}
	return tuiclient.LeaderboardQuery{Metric: metric, Scope: scope, Map: strings.ToLower(in.Map), MinRounds: in.MinRounds, Limit: limit,
		Side: filter.Side, Role: role, From: filter.From, To: filter.To, Source: source}, nil
}

func summarizePositions(steamID string, r tuiclient.PlayerPositions) playerPositionsOut {
	out := playerPositionsOut{SteamID64: steamID, Matches: r.MatchCount, WindowSeconds: r.WindowSeconds, Maps: []positionMapOut{}, SkippedMatches: len(r.Skipped)}
	for _, m := range r.Maps {
		mo := positionMapOut{Map: cleanName(m.Map), Matches: m.Matches, Sides: []positionSideOut{}}
		for _, side := range m.Sides {
			so := positionSideOut{Side: cleanName(side.Side), Rounds: side.Rounds, Sampled: side.Sampled, Role: cleanName(side.Role),
				Zones: []positionShareOut{}, Places: []positionShareOut{}, AWPKillPct: side.AWPKills.Pct, AWPRoundPct: side.AWPRounds.Pct,
				ReliableShare: side.Sampled >= r.MinReliable}
			for _, z := range side.Zones {
				so.Zones = append(so.Zones, positionShareOut{Name: cleanName(z.Zone), Rounds: z.Rounds.Count, Pct: z.Rounds.Pct})
			}
			for _, p := range side.Places {
				name := p.Label
				if name == "" {
					name = p.Place
				}
				so.Places = append(so.Places, positionShareOut{Name: cleanName(name), Rounds: p.Rounds.Count, Pct: p.Rounds.Pct})
			}
			mo.Sides = append(mo.Sides, so)
		}
		out.Maps = append(out.Maps, mo)
	}
	return out
}
