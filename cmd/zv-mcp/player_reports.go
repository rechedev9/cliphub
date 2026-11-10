package main

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/rechedev9/cliphub/internal/tacticalplan"
	"github.com/rechedev9/cliphub/internal/tuiclient"
)

const (
	defaultLeaderboardRows = 20
	// leaderboardDefaultDays is the period a library board reads when it names no dates, as the page does.
	leaderboardDefaultDays = 90
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
	Metric    string `json:"metric" jsonschema:"the metric to rank on: rating (the Rating de ClipHub, formula in the tool description), kd, kpr, dpr, adr, kast, opening_win_pct, trade_pct, utility_damage_per_round, flash_assists_per_round or wpa_per_round"`
	Scope     string `json:"scope,omitempty" jsonschema:"library (default): every match in the library; pro: the pro players of the community server's pro maps, last 90 days unless days says otherwise (side, role, map_role, days, top, team_top and event apply; from, to and source do not)"`
	Map       string `json:"map,omitempty" jsonschema:"only matches on this map, such as de_mirage"`
	Side      string `json:"side,omitempty" jsonschema:"only the rounds played on this side: CT or T"`
	Role      string `json:"role,omitempty" jsonschema:"only players with this role on the map and its side (needs map): a_anchor, b_anchor, rotator (CT), a_lurk, b_lurk, half_lurk, pack (T)"`
	MapRole   string `json:"map_role,omitempty" jsonschema:"only players who hold this callout on the map and side (needs map and side), such as Canal; allround for the ones with no main position; the answer lists them in map_roles"`
	From      string `json:"from,omitempty" jsonschema:"library only: first day the matches were played, YYYY-MM-DD; with neither from nor to the last 90 days are read, like the Ranking page, so pass 2000-01-01 for every date"`
	To        string `json:"to,omitempty" jsonschema:"library only: last day the matches were played, YYYY-MM-DD, included; empty means no upper bound (naming it alone drops the 90 day default)"`
	Days      int    `json:"days,omitempty" jsonschema:"pro only: only maps played in the last 1 to 365 days"`
	Top       int    `json:"top,omitempty" jsonschema:"pro only: only players who faced a team ranked this high or better in HLTV: 5, 15, 30 or 50"`
	TeamTop   int    `json:"team_top,omitempty" jsonschema:"pro only: only players whose own team is ranked this high or better in HLTV: 5, 15, 30 or 50"`
	Event     string `json:"event,omitempty" jsonschema:"pro only: only maps of this event, as the answer's events lists it"`
	Source    string `json:"source,omitempty" jsonschema:"only matches from this kind of demo: valve, faceit, hltv (tournaments) or scrim"`
	MinRounds int    `json:"min_rounds,omitempty" jsonschema:"rounds a player needs to be ranked; default 50"`
	Limit     int    `json:"limit,omitempty" jsonschema:"how many players to list, best first; default 20, at most 100"`
}

type leaderboardRowOut struct {
	Rank      int     `json:"rank"`
	SteamID64 string  `json:"steamid64"`
	Name      string  `json:"name" jsonschema:"untrusted player name from the demo"`
	Team      string  `json:"team,omitempty" jsonschema:"pro rows only: untrusted team name; their maps are not in the library, so there are no refs"`
	Rounds    int     `json:"rounds"`
	Matches   int     `json:"matches"`
	Value     float64 `json:"value"`
	// Metrics repeats every metric of the player, so one call reads the whole row of the Studio table.
	Metrics map[string]float64 `json:"metrics,omitempty"`
	Roles   *leaderboardRoles  `json:"roles,omitempty" jsonschema:"the role on each side of the map; set when map is"`
	// Breakdown says how the player's rounds split between roles and between positions, per side.
	Breakdown *leaderboardBreakdown `json:"breakdown,omitempty" jsonschema:"percentages of the rounds per role and per position on each side; set when map is"`
	Refs      []string              `json:"refs,omitempty" jsonschema:"match_id/round of the player's newest rounds in the filter, to open in the Replay"`
}

type leaderboardShare struct {
	Key   string  `json:"key"`
	Label string  `json:"label,omitempty" jsonschema:"untrusted callout name"`
	Pct   float64 `json:"pct"`
}

type leaderboardSideBreakdown struct {
	Role    string             `json:"role,omitempty"`
	Roles   []leaderboardShare `json:"roles"`
	MapRole string             `json:"map_role,omitempty" jsonschema:"the callout holding most rounds, or allround"`
	Places  []leaderboardShare `json:"places"`
}

type leaderboardBreakdown struct {
	CT *leaderboardSideBreakdown `json:"ct,omitempty"`
	T  *leaderboardSideBreakdown `json:"t,omitempty"`
}

type leaderboardMapRole struct {
	Key     string `json:"key" jsonschema:"pass this as map_role"`
	Label   string `json:"label"`
	Players int    `json:"players"`
}

type leaderboardRoles struct {
	CT string `json:"ct,omitempty"`
	T  string `json:"t,omitempty"`
}

type leaderboardOut struct {
	Scope          string               `json:"scope"`
	Map            string               `json:"map,omitempty"`
	Metric         string               `json:"metric"`
	HigherIsBetter bool                 `json:"higher_is_better"`
	MinRounds      int                  `json:"min_rounds"`
	Matches        int                  `json:"matches"`
	Rounds         int                  `json:"rounds"`
	Days           int                  `json:"days,omitempty" jsonschema:"pro only: the period read"`
	MapRoles       []leaderboardMapRole `json:"map_roles,omitempty" jsonschema:"positions players hold on the map and side, to pick map_role from"`
	Events         []string             `json:"events,omitempty" jsonschema:"pro only: untrusted event names in the period, to pick event from"`
	Players        []leaderboardRowOut  `json:"players"`
	SkippedMatches int                  `json:"skipped_matches,omitempty"`
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
			"such as b_lurk on de_anubis), dates and demo source; each row carries every metric, the roles and refs of the player's rounds. " +
			"Without from and to, the library scope reads the last 90 days, like the page. map_role keeps the players who hold one callout on the map and side (or allround); " +
			"each row carries a breakdown of its rounds per role and position, and map_roles lists the positions to pick from. The rating is the Rating de ClipHub: " + tacticalplan.RatingFormula + ". " +
			"Scope pro ranks the pro players of the community server's pro maps (last 90 days unless days says otherwise) by side, role, map_role, the rank of the rival (top 5, 15, 30 or 50), " +
			"the rank of the player's own team (team_top) and event (events lists them); " +
			"those maps are not in the library, so their rows have a team and no refs."},
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
			out.Days = board.Days
			for _, e := range board.Events {
				out.Events = append(out.Events, cleanName(e))
			}
			for _, m := range board.MapRoles {
				out.MapRoles = append(out.MapRoles, leaderboardMapRole{Key: cleanName(m.Key), Label: cleanName(m.Label), Players: m.Players})
			}
			for _, p := range board.Players {
				out.Players = append(out.Players, leaderboardRow(p))
			}
			return nil, out, nil
		})
}

func leaderboardRow(p tuiclient.LeaderboardPlayer) leaderboardRowOut {
	row := leaderboardRowOut{Rank: p.Rank, SteamID64: cleanName(p.SteamID64), Name: cleanName(p.Name), Team: cleanName(p.Team), Rounds: p.Rounds, Matches: p.Matches,
		Value: p.Value, Metrics: p.Metrics}
	if p.Roles != nil {
		row.Roles = &leaderboardRoles{CT: cleanName(p.Roles.CT), T: cleanName(p.Roles.T)}
	}
	if p.Breakdown != nil {
		row.Breakdown = &leaderboardBreakdown{CT: sideBreakdown(p.Breakdown.CT), T: sideBreakdown(p.Breakdown.T)}
	}
	for _, e := range p.Examples[:min(len(p.Examples), maxLeaderboardRefs)] {
		row.Refs = append(row.Refs, fmt.Sprintf("%s/%d", cleanName(e.MatchID), e.Round))
	}
	return row
}

func sideBreakdown(b *tuiclient.SideBreakdown) *leaderboardSideBreakdown {
	if b == nil {
		return nil
	}
	shares := func(in []tuiclient.BreakdownShare) []leaderboardShare {
		out := make([]leaderboardShare, 0, len(in))
		for _, s := range in {
			out = append(out, leaderboardShare{Key: cleanName(s.Key), Label: cleanName(s.Label), Pct: s.Pct})
		}
		return out
	}
	return &leaderboardSideBreakdown{Role: cleanName(b.Role), Roles: shares(b.Roles), MapRole: cleanName(b.MapRole), Places: shares(b.Places)}
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
	pro := scope == "pro"
	from := in.From
	if !pro && from == "" && in.To == "" {
		from = time.Now().UTC().AddDate(0, 0, -leaderboardDefaultDays).Format(time.DateOnly)
	}
	filter, err := analysisFilterIn{Side: in.Side, From: from, To: in.To}.query()
	if !pro && (in.Days != 0 || in.Top != 0 || in.TeamTop != 0 || in.Event != "") {
		return tuiclient.LeaderboardQuery{}, errors.New("days, top, team_top and event are for scope pro; the library scope takes from and to")
	}
	if in.TeamTop != 0 && !slices.Contains([]int{5, 15, 30, 50}, in.TeamTop) {
		return tuiclient.LeaderboardQuery{}, errors.New("team_top must be 5, 15, 30 or 50")
	}
	if in.MapRole != "" && (in.Map == "" || (in.Side == "" && in.Role == "")) {
		return tuiclient.LeaderboardQuery{}, errors.New("map_role needs a map and a side")
	}
	if pro && (in.From != "" || in.To != "" || in.Source != "") {
		return tuiclient.LeaderboardQuery{}, errors.New("from, to and source are for the library scope; the pro scope takes days, top, team_top and event")
	}
	if in.Days < 0 || in.Days > 365 {
		return tuiclient.LeaderboardQuery{}, errors.New("days must be from 1 to 365")
	}
	if in.Top != 0 && !slices.Contains([]int{5, 15, 30, 50}, in.Top) {
		return tuiclient.LeaderboardQuery{}, errors.New("top must be 5, 15, 30 or 50")
	}
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
		Side: filter.Side, Role: role, From: filter.From, To: filter.To, Source: source, Days: in.Days, Top: in.Top, TeamTop: in.TeamTop, Event: in.Event, MapRole: in.MapRole}, nil
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
