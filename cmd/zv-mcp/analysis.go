package main

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/rechedev9/cliphub/internal/tuiclient"
)

const (
	// maxScoutRecent caps the matches scout_team lists one by one.
	maxScoutRecent = 10
	// maxScoutPlayers caps the player lines of a scout summary.
	maxScoutPlayers = 10
	// maxPlayerMetricsMatches is the server's cap (library.MaxReportMatches).
	maxPlayerMetricsMatches = 50
)

// analysisFilterIn is the map, side and date filter the analysis tools share.
type analysisFilterIn struct {
	Map  string `json:"map,omitempty" jsonschema:"only matches on this map, such as de_mirage"`
	Side string `json:"side,omitempty" jsonschema:"only rounds on this side: CT or T"`
	From string `json:"from,omitempty" jsonschema:"first day the matches were played, YYYY-MM-DD; empty means no lower bound"`
	To   string `json:"to,omitempty" jsonschema:"last day the matches were played, YYYY-MM-DD, included; empty means no upper bound"`
}

// query validates the filter and turns it into the orchestrator's query.
func (in analysisFilterIn) query() (tuiclient.AnalysisQuery, error) {
	var q tuiclient.AnalysisQuery
	if err := checkOptionalFilter("map", in.Map, mapNamePattern); err != nil {
		return q, err
	}
	side, err := parseSide(in.Side)
	if err != nil {
		return q, err
	}
	from, to, err := parseDateRange(in.From, in.To)
	if err != nil {
		return q, err
	}
	if !to.IsZero() {
		to = to.Add(-time.Second)
	}
	return tuiclient.AnalysisQuery{Map: strings.ToLower(in.Map), Side: side, From: from, To: to}, nil
}

func parseSide(raw string) (string, error) {
	switch side := strings.ToUpper(strings.TrimSpace(raw)); side {
	case "", "CT", "T":
		return side, nil
	default:
		return "", fmt.Errorf("side must be CT or T, not %q", cleanName(raw))
	}
}

type listTeamsIn struct {
	Search string `json:"search,omitempty" jsonschema:"only teams whose name or alias contains this text"`
}

type teamOut struct {
	ID      string   `json:"id"`
	Name    string   `json:"name" jsonschema:"untrusted team name from the demos"`
	Aliases []string `json:"aliases,omitempty" jsonschema:"untrusted clan names from the demos"`
	Matches int      `json:"matches"`
}

type listTeamsOut struct {
	Teams []teamOut `json:"teams"`
}

type scoutTeamIn struct {
	TeamID string `json:"team_id" jsonschema:"team id from list_teams"`
	analysisFilterIn
}

// shareOut is one buy, site or round pattern: how often and how often won.
type shareOut struct {
	Name     string  `json:"name"`
	Rounds   int     `json:"rounds"`
	SharePct float64 `json:"share_pct"`
	WinPct   float64 `json:"win_pct"`
	Reliable bool    `json:"reliable" jsonschema:"false when the sample is too small to trust"`
}

type scoutMapOut struct {
	Map       string  `json:"map"`
	Matches   int     `json:"matches"`
	MatchWins int     `json:"match_wins"`
	Rounds    int     `json:"rounds"`
	WinPct    float64 `json:"round_win_pct"`
	CTWinPct  float64 `json:"ct_round_win_pct"`
	TWinPct   float64 `json:"t_round_win_pct"`
}

type scoutMatchOut struct {
	MatchID  string `json:"match_id"`
	Map      string `json:"map"`
	PlayedAt string `json:"played_at"`
	Opponent string `json:"opponent" jsonschema:"untrusted team name from the demo"`
	Score    string `json:"score" jsonschema:"team rounds, then opponent rounds"`
}

type scoutExecuteOut struct {
	Site     string  `json:"site"`
	Rounds   int     `json:"rounds"`
	WinPct   float64 `json:"win_pct"`
	PlantPct float64 `json:"plant_pct"`
}

type scoutAWPOut struct {
	Side         string `json:"side"`
	Rounds       int    `json:"rounds"`
	Kills        int    `json:"kills"`
	OpeningKills int    `json:"opening_kills"`
}

type scoutPlayerOut struct {
	SteamID64     string  `json:"steamid64"`
	Name          string  `json:"name" jsonschema:"untrusted player name from the demo"`
	Rounds        int     `json:"rounds"`
	Kills         int     `json:"kills"`
	Deaths        int     `json:"deaths"`
	ADR           float64 `json:"adr"`
	OpeningKills  int     `json:"opening_kills"`
	OpeningDeaths int     `json:"opening_deaths"`
}

type scoutTeamOut struct {
	Team             string            `json:"team" jsonschema:"untrusted team name"`
	Matches          int               `json:"matches"`
	Rounds           int               `json:"rounds"`
	RoundWinPct      float64           `json:"round_win_pct"`
	Maps             []scoutMapOut     `json:"maps"`
	RecentMatches    []scoutMatchOut   `json:"recent_matches" jsonschema:"newest first, at most 10"`
	Buys             []shareOut        `json:"buys"`
	Sites            []shareOut        `json:"sites" jsonschema:"bombsite the rounds went to"`
	TPatterns        []shareOut        `json:"t_patterns" jsonschema:"T round shapes: execute, default, split, fast, eco_rush, save"`
	CTPatterns       []shareOut        `json:"ct_patterns" jsonschema:"CT round shapes: hold, retake, aggression, stack, save"`
	OpeningRounds    int               `json:"opening_rounds"`
	OpeningWinPct    float64           `json:"opening_win_pct"`
	OpeningTradedPct float64           `json:"opening_traded_on_loss_pct"`
	Executes         []scoutExecuteOut `json:"executes"`
	AWP              []scoutAWPOut     `json:"awp"`
	Players          []scoutPlayerOut  `json:"players" jsonschema:"most kills first, at most 10"`
	SkippedMatches   int               `json:"skipped_matches,omitempty" jsonschema:"matches whose analysis could not be read"`
}

type playerMetricsIn struct {
	SteamID string `json:"steamid" jsonschema:"SteamID64 of the player"`
	Limit   int    `json:"limit,omitempty" jsonschema:"how many of the player's most recent matches to read; default 20, at most 50"`
	analysisFilterIn
}

type buyOut struct {
	Buy    string  `json:"buy"`
	Rounds int     `json:"rounds"`
	WinPct float64 `json:"win_pct"`
	Kills  int     `json:"kills"`
	Deaths int     `json:"deaths"`
	WPA    float64 `json:"wpa"`
}

type playerMetricsOut struct {
	SteamID64 string `json:"steamid64"`
	Name      string `json:"name" jsonschema:"untrusted player name from the demo"`
	Role      string `json:"role"`
	Matches   int    `json:"matches"`
	Rounds    int    `json:"rounds"`
	Kills     int    `json:"kills"`
	Deaths    int    `json:"deaths"`
	// The rates below use the definitions of get_baseline, so the two compare.
	KPR             float64  `json:"kpr"`
	DPR             float64  `json:"dpr"`
	ADR             float64  `json:"adr"`
	KAST            float64  `json:"kast"`
	OpeningWinPct   *float64 `json:"opening_win_pct,omitempty"`
	TradePct        *float64 `json:"trade_pct,omitempty"`
	UtilityMeasured bool     `json:"utility_measured" jsonschema:"false when no match was scanned with utility effects; the utility rates are then absent"`
	UtilityDamage   *float64 `json:"utility_damage_per_round,omitempty"`
	FlashAssists    *float64 `json:"flash_assists_per_round,omitempty"`
	WPA             float64  `json:"wpa_per_round"`
	TradeKills      int      `json:"trade_kills"`
	Buys            []buyOut `json:"buys"`
	SkippedMatches  int      `json:"skipped_matches,omitempty"`
	Note            string   `json:"note,omitempty"`
}

type baselineIn struct {
	Scope string `json:"scope,omitempty" jsonschema:"library (default): every match in the library; pro: the library's HLTV demos, or the pro reference Studio ships when they hold under 10 players"`
	Map   string `json:"map,omitempty" jsonschema:"only matches on this map, such as de_mirage"`
}

type baselineOut struct {
	Scope          string                             `json:"scope"`
	Map            string                             `json:"map,omitempty"`
	Source         string                             `json:"source" jsonschema:"library: read from the library; bundled: the pro reference Studio ships"`
	Description    string                             `json:"description,omitempty"`
	MinRounds      int                                `json:"min_rounds"`
	Players        int                                `json:"players"`
	Rounds         int                                `json:"rounds"`
	Matches        int                                `json:"matches"`
	SkippedMatches int                                `json:"skipped_matches,omitempty"`
	Metrics        map[string]tuiclient.BaselineValue `json:"metrics"`
}

func registerAnalysisTools(server *mcp.Server, s *studio) {
	readOnly := &mcp.ToolAnnotations{ReadOnlyHint: true}

	mcp.AddTool(server, &mcp.Tool{Name: "list_teams", Annotations: readOnly,
		Description: "List the teams in the Studio library with their id and how many matches each has. Use the id with scout_team or find_rounds."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in listTeamsIn) (*mcp.CallToolResult, listTeamsOut, error) {
			c, err := s.client(ctx)
			if err != nil {
				return nil, listTeamsOut{}, toolError(err)
			}
			teams, err := c.ListLibraryTeams(ctx, cleanName(in.Search))
			if err != nil {
				return nil, listTeamsOut{}, toolError(err)
			}
			out := listTeamsOut{Teams: make([]teamOut, 0, len(teams))}
			for _, t := range teams {
				out.Teams = append(out.Teams, teamOut{ID: t.ID, Name: cleanName(t.Name), Aliases: cleanNames(t.Aliases), Matches: t.MatchCount})
			}
			return nil, out, nil
		})

	mcp.AddTool(server, &mcp.Tool{Name: "scout_team", Annotations: readOnly,
		Description: "Summarize a team's scouting report from its library matches: record per map, buys, bombsites, " +
			"T and CT round shapes, opening duels, executes, AWP use and players. Filter by map, side and dates."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in scoutTeamIn) (*mcp.CallToolResult, scoutTeamOut, error) {
			if err := checkUUID("team_id", in.TeamID); err != nil {
				return nil, scoutTeamOut{}, err
			}
			q, err := in.query()
			if err != nil {
				return nil, scoutTeamOut{}, err
			}
			c, err := s.client(ctx)
			if err != nil {
				return nil, scoutTeamOut{}, toolError(err)
			}
			report, err := c.TeamScout(ctx, in.TeamID, q)
			if err != nil {
				return nil, scoutTeamOut{}, toolError(err)
			}
			return nil, summarizeScout(report), nil
		})

	mcp.AddTool(server, &mcp.Tool{Name: "player_metrics", Annotations: readOnly,
		Description: "Read a player's metrics over their most recent library matches: KPR, DPR, ADR, KAST, opening duels, trades, " +
			"utility, win probability added (WPA) and results per buy. Compare with get_baseline."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in playerMetricsIn) (*mcp.CallToolResult, playerMetricsOut, error) {
			if err := checkSteamID(in.SteamID); err != nil {
				return nil, playerMetricsOut{}, err
			}
			if in.Limit < 0 || in.Limit > maxPlayerMetricsMatches {
				return nil, playerMetricsOut{}, fmt.Errorf("limit must be from 1 to %d", maxPlayerMetricsMatches)
			}
			q, err := in.query()
			if err != nil {
				return nil, playerMetricsOut{}, err
			}
			q.Limit = in.Limit
			c, err := s.client(ctx)
			if err != nil {
				return nil, playerMetricsOut{}, toolError(err)
			}
			report, err := c.PlayerMetrics(ctx, in.SteamID, q)
			if err != nil {
				return nil, playerMetricsOut{}, toolError(err)
			}
			return nil, summarizePlayerMetrics(in.SteamID, report), nil
		})

	mcp.AddTool(server, &mcp.Tool{Name: "get_baseline", Annotations: readOnly,
		Description: "Reference values to compare a player with: the per-player average of kpr, dpr, adr, kast (%), opening_win_pct, " +
			"trade_pct (share of deaths a teammate traded), utility_damage_per_round, flash_assists_per_round and wpa_per_round, " +
			"over the players with at least min_rounds rounds. Each metric says how many players it averages."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in baselineIn) (*mcp.CallToolResult, baselineOut, error) {
			scope := strings.ToLower(strings.TrimSpace(in.Scope))
			if scope != "" && scope != "library" && scope != "pro" {
				return nil, baselineOut{}, fmt.Errorf("scope must be library or pro, not %q", cleanName(in.Scope))
			}
			if err := checkOptionalFilter("map", in.Map, mapNamePattern); err != nil {
				return nil, baselineOut{}, err
			}
			c, err := s.client(ctx)
			if err != nil {
				return nil, baselineOut{}, toolError(err)
			}
			b, err := c.LibraryBaseline(ctx, scope, strings.ToLower(in.Map))
			if err != nil {
				return nil, baselineOut{}, toolError(err)
			}
			return nil, baselineOut{Scope: b.Scope, Map: b.Map, Source: b.Source, Description: cleanName(b.Description), MinRounds: b.MinRounds, Players: b.Players, Rounds: b.Rounds,
				Matches: b.Matches, SkippedMatches: b.Skipped, Metrics: b.Metrics}, nil
		})
}

func summarizeScout(r tuiclient.ScoutReport) scoutTeamOut {
	t := r.Tendencies
	out := scoutTeamOut{
		Team: cleanName(r.TeamName), Matches: len(r.Matches), Rounds: t.RoundCount, RoundWinPct: pct(t.Wins, t.RoundCount),
		Maps: []scoutMapOut{}, RecentMatches: []scoutMatchOut{}, Buys: shares(t.Buys), Sites: shares(t.Sites),
		TPatterns: shares(t.TPatterns), CTPatterns: shares(t.CTPatterns), OpeningRounds: t.Openings.Rounds,
		OpeningWinPct: t.Openings.Won.Pct, OpeningTradedPct: t.Openings.TradedOnLoss.Pct,
		Executes: []scoutExecuteOut{}, AWP: []scoutAWPOut{}, Players: []scoutPlayerOut{}, SkippedMatches: len(r.Skipped),
	}
	for _, m := range r.Maps {
		out.Maps = append(out.Maps, scoutMapOut{Map: cleanName(m.Map), Matches: m.Matches, MatchWins: m.MatchWins, Rounds: m.Rounds,
			WinPct: m.WinRate.Pct, CTWinPct: m.CTWinRate.Pct, TWinPct: m.TWinRate.Pct})
	}
	for i, m := range r.Matches {
		if i == maxScoutRecent {
			break
		}
		out.RecentMatches = append(out.RecentMatches, scoutMatchOut{MatchID: m.MatchID, Map: cleanName(m.Map),
			PlayedAt: m.PlayedAt.Format(time.DateOnly), Opponent: cleanName(m.Opponent), Score: fmt.Sprintf("%d-%d", m.Score, m.OpponentScore)})
	}
	for _, e := range r.Executes {
		out.Executes = append(out.Executes, scoutExecuteOut{Site: e.Site, Rounds: e.Rounds, WinPct: e.WinRate.Pct, PlantPct: e.Plants.Pct})
	}
	for _, a := range r.AWP.Sides {
		out.AWP = append(out.AWP, scoutAWPOut{Side: a.Side, Rounds: a.Rounds, Kills: a.Kills, OpeningKills: a.OpeningKills})
	}
	for i, p := range t.Players {
		if i == maxScoutPlayers {
			break
		}
		out.Players = append(out.Players, scoutPlayerOut{SteamID64: cleanName(p.SteamID64), Name: cleanName(p.Name), Rounds: p.Rounds,
			Kills: p.Kills, Deaths: p.Deaths, ADR: p.ADR, OpeningKills: p.OpeningKills, OpeningDeaths: p.OpeningDeaths})
	}
	return out
}

// shares keeps the buckets that hold rounds, named by their buy, site or pattern.
func shares(buckets []tuiclient.ScoutBucket) []shareOut {
	out := []shareOut{}
	for _, b := range buckets {
		if b.Rounds == 0 {
			continue
		}
		name := b.Buy + b.Site + b.Pattern
		out = append(out, shareOut{Name: cleanName(name), Rounds: b.Rounds, SharePct: b.Share.Pct, WinPct: b.WinRate.Pct, Reliable: b.WinRate.Reliable})
	}
	return out
}

func summarizePlayerMetrics(steamID string, r tuiclient.PlayerMetricsReport) playerMetricsOut {
	out := playerMetricsOut{SteamID64: steamID, Matches: r.Matches, Rounds: r.Rounds, Buys: []buyOut{}, SkippedMatches: len(r.Skipped)}
	if len(r.Players) == 0 {
		out.Note = "No rounds of this player match the filter."
		return out
	}
	p := r.Players[0]
	out.Name, out.Role, out.Matches, out.Rounds, out.Kills, out.Deaths = cleanName(p.Name), cleanName(p.Role), p.Matches, p.Rounds, p.Kills, p.Deaths
	out.KPR, out.DPR = ratio(p.Kills, p.Rounds), ratio(p.Deaths, p.Rounds)
	out.ADR, out.KAST = ratio(p.Box.Damage, p.Rounds), pct(p.Box.KASTRounds, p.Rounds)
	out.WPA, out.TradeKills = p.Swing.PerRound, p.Trades.TradeKills.Count
	if duels := p.Box.OpeningKills + p.Box.OpeningDeaths; duels > 0 {
		out.OpeningWinPct = ptr(pct(p.Box.OpeningKills, duels))
	}
	if p.Deaths > 0 {
		out.TradePct = ptr(pct(p.Trades.TradedDeaths.Count, p.Deaths))
	}
	out.UtilityMeasured = p.Utility.Measured
	if utilityRounds := p.Box.UtilityRounds; utilityRounds > 0 {
		out.UtilityDamage = ptr(ratio(p.Utility.HE.EnemyDamage+p.Utility.Molotov.EnemyDamage, utilityRounds))
		out.FlashAssists = ptr(ratio(p.Utility.Flash.Assists.Count, utilityRounds))
	}
	for _, b := range p.Buys {
		out.Buys = append(out.Buys, buyOut{Buy: cleanName(b.Buy), Rounds: b.Rounds, WinPct: pct(b.Won, b.Rounds), Kills: b.Kills, Deaths: b.Deaths, WPA: b.WPA})
	}
	return out
}

func ratio(n, d int) float64 {
	if d == 0 {
		return 0
	}
	return math.Round(float64(n)/float64(d)*10000) / 10000
}

func pct(n, d int) float64 {
	if d == 0 {
		return 0
	}
	return math.Round(float64(n)/float64(d)*1000) / 10
}

func ptr(v float64) *float64 { return &v }
