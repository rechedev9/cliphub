package main

import (
	"context"
	"fmt"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/rechedev9/cliphub/internal/tuiclient"
)

// maxCompareTeams is the orchestrator's cap on the teams of "Comparar con".
const maxCompareTeams = 20

type teamSideStatsIn struct {
	TeamID    string   `json:"team_id" jsonschema:"team id from list_teams"`
	CompareTo []string `json:"compare_to,omitempty" jsonschema:"team ids from list_teams to compare with, at most 20; empty compares with every other team of the library"`
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

type sideMatchOut struct {
	MatchID  string `json:"match_id"`
	Map      string `json:"map"`
	PlayedAt string `json:"played_at"`
	Opponent string `json:"opponent" jsonschema:"untrusted team name"`
	Rounds   int    `json:"rounds" jsonschema:"rounds of this match the scope keeps"`
	Won      bool   `json:"won" jsonschema:"the team won the map; a draw is false"`
}

type teamSideStatsOut struct {
	Team           string              `json:"team" jsonschema:"untrusted team name"`
	Matches        int                 `json:"matches"`
	Rounds         int                 `json:"rounds"`
	CompareTo      []string            `json:"compare_to" jsonschema:"untrusted names of the comparison teams; empty means the whole library"`
	CompareSkipped []string            `json:"compare_skipped" jsonschema:"compare_to ids that are no team of the library, left out"`
	GroupTeams     int                 `json:"group_teams" jsonschema:"comparison teams with rounds in the same map, side, dates and buys"`
	GroupMatches   int                 `json:"group_matches" jsonschema:"comparison matches read, at most 20 per team"`
	GroupCapped    bool                `json:"group_capped" jsonschema:"true when a team had more matches than the cap, so only its newest count"`
	AWPEstimated   int                 `json:"awp_estimated,omitempty" jsonschema:"matches whose AWP filter was estimated from AWP kills and hits: their analysis predates the AWP flag"`
	GroupMinRounds int                 `json:"group_min_rounds" jsonschema:"a team enters a figure's mean only with this many rounds behind it; each figure's group_teams says how many did"`
	Overall        tuiclient.SideStats `json:"overall" jsonschema:"each figure: value (percent for rates, per round for utility damage and flash assists, a count for rounds_played; null without data) and group_avg, the plain mean of the comparison teams"`
	CT             tuiclient.SideStats `json:"ct" jsonschema:"the figures on CT; maps_won is null on a side"`
	T              tuiclient.SideStats `json:"t" jsonschema:"the figures on T; maps_won is null on a side"`
	Players        []sidePlayerOut     `json:"players"`
	MatchList      []sideMatchOut      `json:"match_list" jsonschema:"the matches the figures stand on, newest first"`
	RatingFormula  string              `json:"rating_formula"`
	StudioPath     string              `json:"studio_path" jsonschema:"the path that opens the team's Estadísticas tab in Studio with the filters it shows: map, side, dates, rival, lineup and buys"`
	SkippedMatches int                 `json:"skipped_matches,omitempty"`
}

func (in teamSideStatsIn) query() (tuiclient.SideStatsQuery, error) {
	q := tuiclient.SideStatsQuery{TeamID: in.TeamID, CompareTo: in.CompareTo}
	scope, err := teamReportIn{TeamID: in.TeamID, analysisFilterIn: in.analysisFilterIn, teamScopeIn: in.teamScopeIn}.check(false)
	if err != nil {
		return q, err
	}
	q.Scope = scope
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

// sideStatsPath is the team's Estadísticas tab with the call's scope, as the web writes it;
// the tab's bar holds one buy per side (or half), so other buy lists have no path.
func sideStatsPath(q tuiclient.SideStatsQuery) (string, error) {
	v := url.Values{"tab": {"estadisticas"}, "lineup": {"all"}}
	for key, value := range map[string]string{"map": q.Scope.Map, "side": q.Scope.Side} {
		if value != "" {
			v.Set(key, value)
		}
	}
	if !q.Scope.From.IsZero() {
		v.Set("from", q.Scope.From.Format(time.DateOnly))
	}
	if !q.Scope.To.IsZero() {
		v.Set("to", q.Scope.To.Format(time.DateOnly))
	}
	if len(q.Scope.Opponents) == 1 {
		v.Set("opponent", q.Scope.Opponents[0])
	}
	if len(q.Scope.Lineup) > 0 {
		v.Set("lineup", strings.Join(slices.Compact(slices.Sorted(slices.Values(q.Scope.Lineup))), ","))
	}
	for _, key := range []string{"buy", "opponent_buy"} {
		if list := q.Scope.Scope[key]; len(list) > 0 {
			buy, err := barBuy(key, list)
			if err != nil {
				return "", err
			}
			v.Set(key, buy)
		}
	}
	if len(q.CompareTo) > 0 {
		v.Set("comparar", strings.Join(slices.Compact(slices.Sorted(slices.Values(q.CompareTo))), ","))
	}
	return "/equipos/" + url.PathEscape(q.TeamID) + "?" + v.Encode(), nil
}

func summarizeSideStats(r tuiclient.TeamSideStatsReport, path string) teamSideStatsOut {
	out := teamSideStatsOut{
		Team: cleanName(r.TeamName), Matches: r.Matches, Rounds: r.Rounds, CompareTo: []string{}, CompareSkipped: cleanNames(r.CompareSkipped),
		GroupTeams: r.GroupTeams, GroupMatches: r.GroupMatches, GroupCapped: r.GroupCapped, GroupMinRounds: r.GroupMinRounds, AWPEstimated: r.AWPEstimated,
		Overall: r.Overall, CT: r.CT, T: r.T, Players: []sidePlayerOut{}, MatchList: []sideMatchOut{}, RatingFormula: r.RatingFormula,
		StudioPath: path, SkippedMatches: len(r.Skipped),
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
	for _, m := range r.MatchList {
		out.MatchList = append(out.MatchList, sideMatchOut{
			MatchID: cleanName(m.MatchID), Map: cleanName(m.Map), PlayedAt: cleanName(m.PlayedAt), Opponent: cleanName(m.Opponent), Rounds: m.Rounds, Won: m.Won,
		})
	}
	return out
}

func registerTeamSideStatsTool(server *mcp.Server, s *studio) {
	mcp.AddTool(server, &mcp.Tool{Name: "team_side_stats", Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true},
		Description: "Read a team's statistics, overall and per side, as the team's Estadísticas tab shows them: maps, rounds and pistols won, " +
			"opening kills, 5v4 and 4v5 rounds won, trades, utility damage and flash assists per round and rounds played, each with group_avg, " +
			"the mean of the comparison teams (compare_to, or the whole library) over the same map, side, dates and buys; and one card per player " +
			"(role, ClipHub rating, K/D, ADR, KPR, DPR). Narrow with map, side, dates, opponent_id, match_id, lineup and the team scope: buy and opponent_buy " +
				"(lists; semi with force is a half buy), max_matches, last_days, opponent_top, whose, awp, opponent_awp, advantage and round_from/round_to."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in teamSideStatsIn) (*mcp.CallToolResult, teamSideStatsOut, error) {
			q, err := in.query()
			if err != nil {
				return nil, teamSideStatsOut{}, err
			}
			path, err := sideStatsPath(q)
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
			return nil, summarizeSideStats(report, path), nil
		})
}
