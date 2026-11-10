package main

import (
	"context"
	"fmt"
	"maps"
	"math"
	"net/url"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/rechedev9/cliphub/internal/tacticalplan"
	"github.com/rechedev9/cliphub/internal/tuiclient"
)

const (
	defaultHeatCells = 15
	maxHeatCells     = 50
	maxHeatZones     = 10
	// maxHeatSeconds is the orchestrator's bound on a clock window: a round, its bomb and overtime.
	maxHeatSeconds = 300
)

var heatKinds = []string{"position", "death", "kill", "utility", "timing"}

// heatUnits is what one count of each kind is, as the legend says it.
var heatUnits = map[string]string{"position": "samples", "death": "deaths", "kill": "kills", "utility": "grenades", "timing": "arrivals"}

var heatGrenades = []string{"smoke", "flash", "he", "molotov"}

// heatmapViewIn is what the heatmap counts and in which clock window (the tramo).
type heatmapViewIn struct {
	Kind      string   `json:"kind,omitempty" jsonschema:"what the map counts: position (default; each round read at 50 instants spread over the clock window, one sample per alive player at each), death, kill, utility or timing (first arrival at each spot)"`
	KillAt    string   `json:"kill_at,omitempty" jsonschema:"for kill: killer (default; where the counted players stood when they killed) or victim (where their victims fell). Team kills never count"`
	Grenade   []string `json:"grenade,omitempty" jsonschema:"for utility: only smoke, flash, he or molotov; empty counts all four"`
	ClockFrom float64  `json:"clock_from,omitempty" jsonschema:"start of the clock window (Tramo), in seconds after the round went live; 0 by default. The round clock 1:40 is 15"`
	ClockTo   float64  `json:"clock_to,omitempty" jsonschema:"end of the clock window, in seconds after the round went live; 0 runs to the end of the round"`
	FromPlant bool     `json:"from_plant,omitempty" jsonschema:"start the window at each round's plant instead; a round without a plant counts nothing"`
	Limit     int      `json:"limit,omitempty" jsonschema:"how many of the hottest cells to list; default 15, at most 50"`
}

// query checks the view and turns it into the heatmap's own query names.
func (in heatmapViewIn) query() (url.Values, error) {
	v := url.Values{}
	kind := strings.ToLower(strings.TrimSpace(in.Kind))
	if kind == "" {
		kind = heatKinds[0]
	}
	if !slices.Contains(heatKinds, kind) {
		return nil, fmt.Errorf("kind must be one of %s, not %q", strings.Join(heatKinds, ", "), cleanName(in.Kind))
	}
	v.Set("kind", kind)
	switch killAt := strings.ToLower(strings.TrimSpace(in.KillAt)); {
	case killAt == "":
	case kind != "kill":
		return nil, fmt.Errorf("kill_at only applies to kind kill")
	case killAt == "killer" || killAt == "victim":
		v.Set("kill_at", killAt)
	default:
		return nil, fmt.Errorf("kill_at must be killer or victim, not %q", cleanName(in.KillAt))
	}
	for _, raw := range in.Grenade {
		g := strings.ToLower(strings.TrimSpace(raw))
		if kind != "utility" || !slices.Contains(heatGrenades, g) {
			return nil, fmt.Errorf("grenade needs kind utility and one of %s", strings.Join(heatGrenades, ", "))
		}
		v.Add("grenade", g)
	}
	if in.ClockFrom < 0 || in.ClockTo < 0 || in.ClockFrom > maxHeatSeconds || in.ClockTo > maxHeatSeconds {
		return nil, fmt.Errorf("clock_from and clock_to must be from 0 to %d seconds", maxHeatSeconds)
	}
	if in.ClockTo > 0 && in.ClockTo < in.ClockFrom {
		return nil, fmt.Errorf("clock_to must not be before clock_from")
	}
	if in.FromPlant {
		v.Set("t_from_plant", "1")
	} else if in.ClockFrom > 0 {
		v.Set("t_from", strconv.FormatFloat(in.ClockFrom, 'f', -1, 64))
	}
	if in.ClockTo > 0 {
		v.Set("t_to", strconv.FormatFloat(in.ClockTo, 'f', -1, 64))
	}
	if in.Limit < 0 || in.Limit > maxHeatCells {
		return nil, fmt.Errorf("limit must be from 1 to %d, or 0 for the default %d", maxHeatCells, defaultHeatCells)
	}
	return v, nil
}

// studioQuery is the view as the web keeps it in the URL: cuenta, baja, granada and tramo.
func (in heatmapViewIn) studioQuery(v url.Values) {
	v.Set("forma", "calor")
	if kind := strings.ToLower(strings.TrimSpace(in.Kind)); kind != "" && kind != heatKinds[0] {
		v.Set("cuenta", kind)
	}
	if strings.EqualFold(strings.TrimSpace(in.KillAt), "victim") {
		v.Set("baja", "victim")
	}
	for _, g := range in.Grenade {
		v.Add("granada", strings.ToLower(strings.TrimSpace(g)))
	}
	switch {
	case in.FromPlant:
		v.Set("tramo", "plant")
	case in.ClockFrom > 0 || (in.ClockTo > 0 && in.ClockTo < roundClockSeconds):
		// The web's tramo is whole seconds on the round clock.
		from := int(math.Floor(in.ClockFrom))
		to := roundClockSeconds
		if in.ClockTo > 0 {
			to = min(roundClockSeconds, int(math.Ceil(in.ClockTo)))
		}
		if from < to {
			v.Set("tramo", fmt.Sprintf("%d-%d", from, to))
		}
	}
}

type heatCellOut struct {
	Callout       string   `json:"callout" jsonschema:"the place CS2 names at the cell; untrusted map text"`
	Side          string   `json:"side"`
	Level         string   `json:"level,omitempty" jsonschema:"lower on the lower floor of Nuke or Vertigo; empty on the upper floor"`
	Center        [3]int   `json:"center" jsonschema:"world x and y of the cell's centre, and the median altitude of what it counted"`
	Count         int      `json:"count"`
	Share         float64  `json:"share" jsonschema:"count over all the side counted on that floor"`
	Rounds        int      `json:"rounds" jsonschema:"rounds that added to the cell"`
	MedianSeconds float64  `json:"median_seconds" jsonschema:"seconds after the round went live; for timing, the median first arrival"`
	Examples      []string `json:"examples" jsonschema:"round refs (match_id/round) the cell counted, at most 3; for timing the earliest arrivals"`
}

type heatZoneOut struct {
	Callout string  `json:"callout" jsonschema:"untrusted map text"`
	Side    string  `json:"side"`
	Level   string  `json:"level,omitempty"`
	Count   int     `json:"count"`
	Share   float64 `json:"share"`
	Cells   int     `json:"cells"`
}

type heatSideOut struct {
	Side   string `json:"side"`
	Level  string `json:"level,omitempty"`
	Total  int    `json:"total"`
	Rounds int    `json:"rounds" jsonschema:"rounds in which the side counted anything on that floor"`
}

type heatmapOut struct {
	Map              string        `json:"map"`
	Kind             string        `json:"kind"`
	KillAt           string        `json:"kill_at,omitempty"`
	Unit             string        `json:"unit" jsonschema:"what one count is: samples, deaths, kills, grenades or arrivals"`
	Matches          int           `json:"matches"`
	Rounds           int           `json:"rounds" jsonschema:"the rounds the scope kept, whether or not they counted anything"`
	MissingPositions int           `json:"missing_positions,omitempty" jsonschema:"matches with no saved positions: they add no position or timing"`
	SkippedMatches   int           `json:"skipped_matches,omitempty"`
	CellSize         float64       `json:"cell_size" jsonschema:"world units per cell side"`
	Sides            []heatSideOut `json:"sides"`
	Zones            []heatZoneOut `json:"zones" jsonschema:"the cells summed by callout, most first, at most 10"`
	Cells            []heatCellOut `json:"cells" jsonschema:"the hottest cells, most first, as the Zonas con más actividad table lists them"`
	Truncated        bool          `json:"truncated,omitempty"`
	StudioPath       string        `json:"studio_path" jsonschema:"the path that opens this heatmap in Studio with its scope, kind and tramo"`
}

// teamHeatmapOut is the team heatmap: the whole answer and, with by_player, one more per player.
type teamHeatmapOut struct {
	heatmapOut
	ByPlayer []playerHeat `json:"by_player,omitempty" jsonschema:"with by_player: the same answer for each player counted alone"`
}

type playerHeat struct {
	SteamID64 string     `json:"steamid64"`
	Heatmap   heatmapOut `json:"heatmap"`
}

// restrictToArea drops the cells whose centre lies outside the zone or its floor; the sides keep their full totals.
func restrictToArea(got tuiclient.SpatialHeatmap, area *tacticalplan.PatternArea) tuiclient.SpatialHeatmap {
	if area == nil || got.Heatmap == nil {
		return got
	}
	h := *got.Heatmap
	h.Cells = nil
	// A one-floor map has no floors to pick, as PatternArea.Contains reads it.
	twoFloors := got.Geometry != nil && got.Geometry.Calibration.MultiLevel()
	for _, c := range got.Heatmap.Cells {
		x, y := (float64(c.X)+0.5)*h.CellSize, (float64(c.Y)+0.5)*h.CellSize
		inside := x >= area.MinX && x <= area.MaxX && y >= area.MinY && y <= area.MaxY
		floor := area.Level == "" || !twoFloors || (area.Level == "lower") == (c.Level == "lower")
		if inside && floor {
			h.Cells = append(h.Cells, c)
		}
	}
	got.Heatmap = &h
	return got
}

// summarizeHeatmap names the hottest cells with their callout, sums them by callout and lists each side's total.
func summarizeHeatmap(got tuiclient.SpatialHeatmap, limit int) heatmapOut {
	out := heatmapOut{Map: cleanName(got.Map), Matches: len(got.Matches), SkippedMatches: len(got.Skipped), Sides: []heatSideOut{}, Zones: []heatZoneOut{}, Cells: []heatCellOut{}}
	h := got.Heatmap
	if h == nil {
		return out
	}
	out.Kind, out.KillAt, out.Unit = string(h.Kind), string(h.KillAt), heatUnits[string(h.Kind)]
	out.Rounds, out.MissingPositions, out.CellSize = h.Rounds, h.MissingPositions, h.CellSize
	totals := map[string]int{}
	for _, s := range h.Sides {
		totals[string(s.Side)+"|"+s.Level] = s.Total
		out.Sides = append(out.Sides, heatSideOut{Side: string(s.Side), Level: floorName(s.Level), Total: s.Total, Rounds: s.Rounds})
	}
	cells := slices.Clone(h.Cells)
	sort.SliceStable(cells, func(i, j int) bool {
		a, b := cells[i], cells[j]
		if a.Count != b.Count {
			return a.Count > b.Count
		}
		if a.Seconds != b.Seconds {
			return a.Seconds < b.Seconds
		}
		if a.Y != b.Y {
			return a.Y < b.Y
		}
		return a.X < b.X
	})
	zones := map[string]*heatZoneOut{}
	var order []string
	for i, c := range cells {
		center := [3]float64{(float64(c.X) + 0.5) * h.CellSize, (float64(c.Y) + 0.5) * h.CellSize, float64(c.Z)}
		callout := ""
		if got.Geometry != nil {
			callout = cleanName(tacticalplan.NearestCallout(*got.Geometry, center))
		}
		total := totals[string(c.Side)+"|"+c.Level]
		key := callout + "|" + string(c.Side) + "|" + c.Level
		zone, ok := zones[key]
		if !ok {
			zone = &heatZoneOut{Callout: callout, Side: string(c.Side), Level: floorName(c.Level)}
			zones[key] = zone
			order = append(order, key)
		}
		zone.Count += c.Count
		zone.Cells++
		zone.Share = ratio(zone.Count, total)
		if i >= limit {
			out.Truncated = true
			continue
		}
		row := heatCellOut{Callout: callout, Side: string(c.Side), Level: floorName(c.Level), Center: roundPos(center), Count: c.Count,
			Share: ratio(c.Count, total), Rounds: c.Rounds, MedianSeconds: c.Seconds, Examples: []string{}}
		for _, e := range c.Examples {
			row.Examples = append(row.Examples, fmt.Sprintf("%s/%d", e.MatchID, e.Round))
		}
		out.Cells = append(out.Cells, row)
	}
	for _, key := range order {
		out.Zones = append(out.Zones, *zones[key])
	}
	sort.SliceStable(out.Zones, func(i, j int) bool { return out.Zones[i].Count > out.Zones[j].Count })
	if len(out.Zones) > maxHeatZones {
		out.Zones = out.Zones[:maxHeatZones]
	}
	return out
}

// floorName leaves the upper floor unnamed, so a one-floor map shows no level at all.
func floorName(level string) string {
	if level == "lower" {
		return level
	}
	return ""
}

func heatLimit(limit int) int {
	if limit <= 0 {
		return defaultHeatCells
	}
	return limit
}

type matchHeatmapIn struct {
	replayScopeIn
	heatmapViewIn
}

type teamHeatmapIn struct {
	TeamID string `json:"team_id" jsonschema:"team id from list_teams"`
	analysisFilterIn
	teamScopeIn
	Players  []string       `json:"players,omitempty" jsonschema:"SteamID64s to count alone, as Jugador does on the team's heatmap; every round of the team stays in the scope"`
	ByPlayer bool           `json:"by_player,omitempty" jsonschema:"with players: also answer one heatmap per player in by_player, each counting that player alone; at most 5 players"`
	Area     *patternAreaIn `json:"area,omitempty" jsonschema:"keep only the cells whose centre lies in this zone (world units, as player_positions give them); shares stay over the whole side. For the rounds in which a player passes through a zone use find_player_pattern"`
	heatmapViewIn
}

// maxHeatPlayers is a team's five: by_player asks the orchestrator once per player.
const maxHeatPlayers = 5

// teamValues is the focus and view of a team heatmap; the scope, buys included, travels in the AnalysisQuery.
func (in teamHeatmapIn) teamValues() (url.Values, error) {
	v, err := in.heatmapViewIn.query()
	if err != nil {
		return nil, err
	}
	for _, id := range in.playerIDs() {
		if err := checkSteamID(id); err != nil {
			return nil, err
		}
		v.Add("player", id)
	}
	if n := len(in.playerIDs()); in.ByPlayer && (n == 0 || n > maxHeatPlayers) {
		return nil, fmt.Errorf("by_player needs 1 to %d different players", maxHeatPlayers)
	}
	return v, nil
}

// playerIDs are the focus players without repeats, in the order given.
func (in teamHeatmapIn) playerIDs() []string {
	var ids []string
	for _, id := range in.Players {
		if !slices.Contains(ids, id) {
			ids = append(ids, id)
		}
	}
	return ids
}

// playerHeatValues is the heatmap query with one player as the whole focus.
func playerHeatValues(heat url.Values, id string) url.Values {
	one := maps.Clone(heat)
	one["player"] = []string{id}
	return one
}

// teamHeatmapPath opens the team's Replay on Mapa de calor on the resolved map with the
// full team scope, as teamReplayPath writes it, plus the focus and the heatmap's view.
func teamHeatmapPath(in teamHeatmapIn, mapName string, q tuiclient.AnalysisQuery) (string, error) {
	base, err := teamReplayPath(replayPathIn{TeamID: in.TeamID, Map: mapName, Query: q})
	if err != nil {
		return "", err
	}
	path, query, _ := strings.Cut(base, "?")
	v, err := url.ParseQuery(query)
	if err != nil {
		return "", err
	}
	for _, id := range in.Players {
		v.Add("jugador", id)
	}
	in.heatmapViewIn.studioQuery(v)
	return path + "?" + v.Encode(), nil
}

func registerHeatmapTools(server *mcp.Server, s *studio) {
	mcp.AddTool(server, &mcp.Tool{Name: "match_heatmap", Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true},
		Description: "Read the Mapa de calor of one match's Replay: over the rounds a replay_rounds scope keeps, where the players stood (position), died (death), " +
			"killed from or dropped their victims (kill with kill_at), landed grenades (utility) or first arrived (timing), inside a clock window (Tramo). " +
			"focus counts only those players, as a click on a player does. The answer lists the hottest cells with their callout and counts, the same cells summed " +
			"by callout, and studio_path to open it in Studio."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in matchHeatmapIn) (*mcp.CallToolResult, heatmapOut, error) {
			if err := checkUUID("match_id", in.MatchID); err != nil {
				return nil, heatmapOut{}, err
			}
			heat, err := in.heatmapViewIn.query()
			if err != nil {
				return nil, heatmapOut{}, err
			}
			c, err := s.client(ctx)
			if err != nil {
				return nil, heatmapOut{}, toolError(err)
			}
			doc, err := c.LibraryTacticalPlan(ctx, in.MatchID)
			if err != nil {
				return nil, heatmapOut{}, toolError(err)
			}
			scope, err := in.values(doc)
			if err != nil {
				return nil, heatmapOut{}, err
			}
			query := url.Values{}
			for key, list := range scope {
				query[key] = list
			}
			for key, list := range heat {
				query[key] = list
			}
			got, err := c.LibraryMatchHeatmap(ctx, in.MatchID, query)
			if err != nil {
				return nil, heatmapOut{}, toolError(err)
			}
			out := summarizeHeatmap(got, heatLimit(in.Limit))
			scope.Set("tab", "replay")
			in.heatmapViewIn.studioQuery(scope)
			out.StudioPath = "/partidas/" + url.PathEscape(in.MatchID) + "?" + scope.Encode()
			return nil, out, nil
		})

	mcp.AddTool(server, &mcp.Tool{Name: "team_heatmap", Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true},
		Description: "Read a team's Mapa de calor over its matches on one map (the most played when map is empty), as the team's Replay shows it: " +
			"the same kinds, clock window and kill_at as match_heatmap, filtered by side, dates, rivals (opponent_id), matches (match_id) and the whole team scope of team_rounds (buys, AWP, advantage, last N matches...). " +
			"players counts only those players (SteamID64s), across demos where their slots change; by_player adds one heatmap per player and area keeps the cells inside a zone. The answer lists the hottest cells with their " +
			"callout and counts, the same cells summed by callout, and studio_path to open it in Studio."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in teamHeatmapIn) (*mcp.CallToolResult, teamHeatmapOut, error) {
			if err := checkUUID("team_id", in.TeamID); err != nil {
				return nil, teamHeatmapOut{}, err
			}
			q, err := teamQuery(in.analysisFilterIn, in.teamScopeIn)
			if err != nil {
				return nil, teamHeatmapOut{}, err
			}
			heat, err := in.teamValues()
			if err != nil {
				return nil, teamHeatmapOut{}, err
			}
			if _, err := teamHeatmapPath(in, "", q); err != nil {
				return nil, teamHeatmapOut{}, err
			}
			var area *tacticalplan.PatternArea
			if in.Area != nil {
				if area, err = in.Area.area(); err != nil {
					return nil, teamHeatmapOut{}, err
				}
			}
			c, err := s.client(ctx)
			if err != nil {
				return nil, teamHeatmapOut{}, toolError(err)
			}
			got, err := c.LibraryTeamHeatmap(ctx, in.TeamID, q, heat)
			if err != nil {
				return nil, teamHeatmapOut{}, toolError(err)
			}
			out := teamHeatmapOut{heatmapOut: summarizeHeatmap(restrictToArea(got, area), heatLimit(in.Limit))}
			if in.ByPlayer {
				// Every player is read on the map the main answer resolved, so a tie between maps cannot split them.
				pinned := q
				pinned.Map = got.Map
				ids := in.playerIDs()
				for _, id := range ids {
					alone := got
					if len(ids) > 1 {
						if alone, err = c.LibraryTeamHeatmap(ctx, in.TeamID, pinned, playerHeatValues(heat, id)); err != nil {
							return nil, teamHeatmapOut{}, fmt.Errorf("heatmap of player %s: %w", id, toolError(err))
						}
					}
					out.ByPlayer = append(out.ByPlayer, playerHeat{SteamID64: id, Heatmap: summarizeHeatmap(restrictToArea(alone, area), heatLimit(in.Limit))})
				}
			}
			if out.StudioPath, err = teamHeatmapPath(in, got.Map, q); err != nil {
				return nil, teamHeatmapOut{}, err
			}
			return nil, out, nil
		})
}
