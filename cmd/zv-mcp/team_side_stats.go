package main

import (
	"context"
	"fmt"
	"net/url"
	"slices"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/rechedev9/cliphub/internal/tuiclient"
)

// maxCompareTeams is the orchestrator's cap on the teams of "Comparar con".
const maxCompareTeams = 20

type teamSideStatsIn struct {
	TeamID      string   `json:"team_id" jsonschema:"team id from list_teams"`
	Buy         string   `json:"buy,omitempty" jsonschema:"only rounds with this buy of the team: pistol, eco, semi, force or full"`
	OpponentBuy string   `json:"opponent_buy,omitempty" jsonschema:"only rounds with this buy of the rival: pistol, eco, semi, force or full"`
	CompareTo   []string `json:"compare_to,omitempty" jsonschema:"team ids from list_teams to compare with, at most 20; empty compares with every other team of the library"`
	analysisFilterIn
	teamScopeIn
}

type sidePlayerOut struct {
	SteamID64 string  `json:"steamid64"`
	Name      string  `json:"name" jsonschema:"untrusted player name from the demo"`
	Role      string  `json:"role" jsonschema:"inferred role: awp, entry, support, rifler or unknown"`
	Rounds    int     `json:"rounds"`
	Rating    float64 `json:"rating" jsonschema:"ClipHub's own rating, see rating_formula; not an HLTV rating"`
	KD        float64 `json:"kd"`
	ADR       float64 `json:"adr"`
	KPR       float64 `json:"kpr"`
	DPR       float64 `json:"dpr"`
}

type teamSideStatsOut struct {
	Team           string              `json:"team" jsonschema:"untrusted team name"`
	Matches        int                 `json:"matches"`
	Rounds         int                 `json:"rounds"`
	CompareTo      []string            `json:"compare_to" jsonschema:"untrusted names of the comparison teams; empty means the whole library"`
	GroupTeams     int                 `json:"group_teams" jsonschema:"comparison teams with rounds in the same map, side, dates and buys"`
	Overall        tuiclient.SideStats `json:"overall" jsonschema:"each figure: value (percent for rates, per round for utility damage and flash assists, a count for rounds_played; null without data) and group_avg, the plain mean of the comparison teams"`
	CT             tuiclient.SideStats `json:"ct" jsonschema:"the figures on CT; maps_won is null on a side"`
	T              tuiclient.SideStats `json:"t" jsonschema:"the figures on T; maps_won is null on a side"`
	Players        []sidePlayerOut     `json:"players"`
	RatingFormula  string              `json:"rating_formula"`
	StudioPath     string              `json:"studio_path" jsonschema:"the path that opens the team's Estadísticas tab with this scope in Studio"`
	SkippedMatches int                 `json:"skipped_matches,omitempty"`
}

func checkBuy(field, raw string) (string, error) {
	buy := strings.ToLower(strings.TrimSpace(raw))
	if buy != "" && !slices.Contains(buyTypes, buy) {
		return "", fmt.Errorf("%s must be one of %s, not %q", field, strings.Join(buyTypes, ", "), cleanName(raw))
	}
	return buy, nil
}

func (in teamSideStatsIn) query() (tuiclient.SideStatsQuery, error) {
	q := tuiclient.SideStatsQuery{TeamID: in.TeamID, CompareTo: in.CompareTo}
	scope, err := teamReportIn{TeamID: in.TeamID, analysisFilterIn: in.analysisFilterIn, teamScopeIn: in.teamScopeIn}.check(false)
	if err != nil {
		return q, err
	}
	q.Scope = scope
	if q.Buy, err = checkBuy("buy", in.Buy); err != nil {
		return q, err
	}
	if q.OpponentBuy, err = checkBuy("opponent_buy", in.OpponentBuy); err != nil {
		return q, err
	}
	if len(in.CompareTo) > maxCompareTeams {
		return q, fmt.Errorf("at most %d compare_to teams", maxCompareTeams)
	}
	for _, id := range in.CompareTo {
		if err := checkUUID("compare_to", id); err != nil {
			return q, err
		}
	}
	return q, nil
}

// sideStatsPath is the team's Estadísticas tab with the query's scope, as the web writes it.
func sideStatsPath(q tuiclient.SideStatsQuery) string {
	v := url.Values{"tab": {"estadisticas"}, "lineup": {"all"}}
	set := func(k, val string) {
		if val != "" {
			v.Set(k, val)
		}
	}
	set("map", q.Scope.Map)
	set("side", q.Scope.Side)
	set("buy", q.Buy)
	set("opponent_buy", q.OpponentBuy)
	if len(q.Scope.Lineup) > 0 {
		v.Set("lineup", strings.Join(slices.Compact(slices.Sorted(slices.Values(q.Scope.Lineup))), ","))
	}
	if len(q.CompareTo) > 0 {
		v.Set("comparar", strings.Join(q.CompareTo, ","))
	}
	return "/equipos/" + url.PathEscape(q.TeamID) + "?" + v.Encode()
}

func summarizeSideStats(r tuiclient.TeamSideStatsReport, q tuiclient.SideStatsQuery) teamSideStatsOut {
	out := teamSideStatsOut{
		Team: cleanName(r.TeamName), Matches: r.Matches, Rounds: r.Rounds, CompareTo: []string{}, GroupTeams: r.GroupTeams,
		Overall: r.Overall, CT: r.CT, T: r.T, Players: []sidePlayerOut{}, RatingFormula: r.RatingFormula,
		StudioPath: sideStatsPath(q), SkippedMatches: len(r.Skipped),
	}
	for _, team := range r.CompareTeams {
		out.CompareTo = append(out.CompareTo, cleanName(team.Name))
	}
	for _, p := range r.Players {
		out.Players = append(out.Players, sidePlayerOut{
			SteamID64: cleanName(p.SteamID64), Name: cleanName(p.Name), Role: cleanName(p.Role), Rounds: p.Rounds,
			Rating: p.Rating, KD: p.KD, ADR: p.ADR, KPR: p.KPR, DPR: p.DPR,
		})
	}
	return out
}

func registerTeamSideStatsTool(server *mcp.Server, s *studio) {
	mcp.AddTool(server, &mcp.Tool{Name: "team_side_stats", Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true},
		Description: "Read a team's statistics, overall and per side, as the team's Estadísticas tab shows them: maps, rounds and pistols won, " +
			"opening kills, 5v4 and 4v5 rounds won, trades, utility damage and flash assists per round and rounds played, each with group_avg, " +
			"the mean of the comparison teams (compare_to, or the whole library) over the same map, side, dates and buys; and one card per player " +
			"(role, ClipHub rating, K/D, ADR, KPR, DPR). Narrow with map, side, dates, opponent_id, match_id, lineup, buy and opponent_buy."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in teamSideStatsIn) (*mcp.CallToolResult, teamSideStatsOut, error) {
			q, err := in.query()
			if err != nil {
				return nil, teamSideStatsOut{}, err
			}
			c, err := s.client(ctx)
			if err != nil {
				return nil, teamSideStatsOut{}, toolError(err)
			}
			report, err := c.TeamSideStats(ctx, q)
			if err != nil {
				return nil, teamSideStatsOut{}, toolError(err)
			}
			return nil, summarizeSideStats(report, q), nil
		})
}
