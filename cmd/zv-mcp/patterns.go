package main

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/rechedev9/cliphub/internal/tacticalplan"
	"github.com/rechedev9/cliphub/internal/tuiclient"
)

// The server's caps (internal/library/pattern.go and internal/tacticalplan/pattern.go).
const (
	maxPatternNameRunes = 80
	maxPatternMatches   = 50
	// maxPatternRows bounds the matched rounds an answer lists; the counts cover them all.
	maxPatternRows = 200
)

var patternBuys = []string{"pistol", "eco", "semi", "force", "full"}

type patternAreaIn struct {
	MinX  float64 `json:"min_x" jsonschema:"west edge of the zone in world units, as player_positions and team_heatmap give positions"`
	MinY  float64 `json:"min_y" jsonschema:"south edge in world units"`
	MaxX  float64 `json:"max_x"`
	MaxY  float64 `json:"max_y"`
	Level string  `json:"level,omitempty" jsonschema:"on Nuke or Vertigo: lower keeps the zone to the lower floor, default to the upper; empty is both floors"`
}

type patternBlockIn struct {
	Kind      string         `json:"kind" jsonschema:"player (a player passes through the zone), utility (a grenade lands in the zone, or is thrown from it with from_throw) or plant (the bomb is planted in the zone)"`
	Players   []string       `json:"players,omitempty" jsonschema:"SteamID64s; any of them meets the block; empty is anyone of the team"`
	Rival     bool           `json:"rival,omitempty" jsonschema:"read the rival team's players instead of the team's (Buscar en el equipo rival)"`
	Grenade   string         `json:"grenade,omitempty" jsonschema:"for utility: smoke, flash, he, molotov or decoy; empty is any"`
	FromThrow bool           `json:"from_throw,omitempty" jsonschema:"for utility: the zone is where the grenade was thrown from, not where it landed"`
	Area      *patternAreaIn `json:"area,omitempty" jsonschema:"the zone; without it the block holds anywhere on the map"`
	ClockFrom float64        `json:"clock_from,omitempty" jsonschema:"start of the block's clock window (Tramo), seconds after the round went live; the round clock 1:40 is 15"`
	ClockTo   float64        `json:"clock_to,omitempty" jsonschema:"end of the window in seconds after the round went live; 0 runs to the end of the round"`
	Invert    bool           `json:"invert,omitempty" jsonschema:"keep the rounds in which the block does not hold (Rondas en que no pasa)"`
}

type patternScopeIn struct {
	Matches     int    `json:"matches,omitempty" jsonschema:"only the team's (or the library's) most recent N matches on the map, at most 50; 0 reads them all"`
	Buy         string `json:"buy,omitempty" jsonschema:"the team's buy: pistol, eco, semi, force or full"`
	OpponentBuy string `json:"opponent_buy,omitempty" jsonschema:"the rival's buy, same values"`
}

type findPatternIn struct {
	TeamID string `json:"team_id" jsonschema:"team id from list_teams"`
	Map    string `json:"map,omitempty" jsonschema:"such as de_ancient; empty reads the team's most played map"`
	Side   string `json:"side" jsonschema:"T or CT: the side whose rounds the pattern reads"`
	From   string `json:"from,omitempty" jsonschema:"first day the matches were played, YYYY-MM-DD"`
	To     string `json:"to,omitempty" jsonschema:"last day the matches were played, YYYY-MM-DD, included"`
	teamScopeIn
	patternScopeIn
	Blocks []patternBlockIn `json:"blocks" jsonschema:"1 to 6 blocks; a round is kept when every block holds in it"`
}

type findPlayerPatternIn struct {
	TeamID string `json:"team_id" jsonschema:"team id from list_teams"`
	Map    string `json:"map,omitempty" jsonschema:"such as de_ancient; empty reads the team's most played map"`
	Side   string `json:"side" jsonschema:"T or CT"`
	From   string `json:"from,omitempty" jsonschema:"first day the matches were played, YYYY-MM-DD"`
	To     string `json:"to,omitempty" jsonschema:"last day, YYYY-MM-DD, included"`
	teamScopeIn
	patternScopeIn
	Players   []string       `json:"players,omitempty" jsonschema:"the player or players to follow, SteamID64s; empty is anyone of the team"`
	Rival     bool           `json:"rival,omitempty" jsonschema:"follow the rival team's players instead"`
	Area      *patternAreaIn `json:"area,omitempty" jsonschema:"the zone they pass through; without it every round they played counts"`
	ClockFrom float64        `json:"clock_from,omitempty" jsonschema:"start of the clock window (Tramo), seconds after the round went live"`
	ClockTo   float64        `json:"clock_to,omitempty" jsonschema:"end of the window; 0 runs to the end of the round"`
	Invert    bool           `json:"invert,omitempty" jsonschema:"keep the rounds in which they do not pass (Rondas en que no pasa)"`
}

type savePatternIn struct {
	Name   string `json:"name" jsonschema:"what the pattern is called, such as Ida a B por banana"`
	Map    string `json:"map" jsonschema:"such as de_ancient"`
	TeamID string `json:"team_id,omitempty" jsonschema:"the team the pattern belongs to; empty saves it for the whole library, detected over every analysed match on the map"`
	Side   string `json:"side" jsonschema:"T or CT"`
	patternScopeIn
	Blocks []patternBlockIn `json:"blocks" jsonschema:"1 to 6 blocks, as find_pattern_rounds takes them"`
}

type listPatternsIn struct {
	Map    string `json:"map" jsonschema:"such as de_ancient"`
	TeamID string `json:"team_id,omitempty" jsonschema:"figures over this team's matches; empty detects each pattern over the whole library"`
}

type patternRoundsIn struct {
	PatternID string `json:"pattern_id" jsonschema:"saved pattern id from list_patterns"`
	TeamID    string `json:"team_id,omitempty" jsonschema:"run it over this team; empty uses the pattern's own team, or the whole library when it has none"`
}

type deletePatternIn struct {
	PatternID string `json:"pattern_id" jsonschema:"saved pattern id from list_patterns"`
	Approve   string `json:"approve,omitempty" jsonschema:"only when asked: the approval code from a previous call, after the user agreed"`
}

type patternRoundOut struct {
	Ref   string `json:"ref" jsonschema:"match_id/round"`
	Side  string `json:"side"`
	Won   bool   `json:"won"`
	Buy   string `json:"buy,omitempty"`
	Clock string `json:"clock,omitempty" jsonschema:"when the first block that is not inverted first held, on the round clock: 1:42 reads Entra en la zona a 1:42"`
}

type patternMatchOut struct {
	MatchID  string            `json:"match_id"`
	PlayedAt string            `json:"played_at" jsonschema:"YYYY-MM-DD"`
	Team     string            `json:"team,omitempty" jsonschema:"untrusted text from the demo"`
	Opponent string            `json:"opponent,omitempty" jsonschema:"untrusted text from the demo"`
	Rounds   int               `json:"rounds" jsonschema:"the match's rounds on the pattern's side"`
	Matched  []patternRoundOut `json:"matched"`
}

type patternMonthOut struct {
	Month   string `json:"month" jsonschema:"YYYY-MM"`
	Rounds  int    `json:"rounds"`
	Matched int    `json:"matched"`
}

type patternRunOut struct {
	Map              string                      `json:"map"`
	Rounds           int                         `json:"rounds" jsonschema:"rounds read on the pattern's side"`
	Matched          int                         `json:"matched" jsonschema:"rounds in which every block held"`
	Won              int                         `json:"won"`
	WinRate          float64                     `json:"win_rate" jsonschema:"won over matched, 0 to 1"`
	Blocks           []tacticalplan.PatternCount `json:"blocks" jsonschema:"each block's own count over the rounds, in order, as each block card shows it"`
	MissingPositions int                         `json:"missing_positions,omitempty" jsonschema:"rounds without saved positions: player blocks cannot hold in them"`
	RecentRounds     int                         `json:"recent_rounds" jsonschema:"rounds of the last 30 days"`
	RecentMatched    int                         `json:"recent_matched"`
	Trend            string                      `json:"trend" jsonschema:"up, down or flat: the last 30 days' share against the earlier one; empty when either has no rounds"`
	Months           []patternMonthOut           `json:"months"`
	Matches          []patternMatchOut           `json:"matches" jsonschema:"the matched rounds by match, newest first"`
	Truncated        bool                        `json:"truncated,omitempty" jsonschema:"more matched rounds than listed; the counts cover them all"`
	ReplayPath       string                      `json:"replay_path,omitempty" jsonschema:"opens the matched rounds in the Replay as one playlist"`
	StudioPath       string                      `json:"studio_path,omitempty" jsonschema:"opens the pattern in the team's Replay > Patrones with its scope; empty for a library-wide run"`
}

type savedPatternOut struct {
	ID         string                  `json:"id"`
	Name       string                  `json:"name" jsonschema:"untrusted text the user wrote"`
	Map        string                  `json:"map"`
	TeamID     string                  `json:"team_id,omitempty" jsonschema:"empty: the pattern reads the whole library"`
	Pattern    tacticalplan.Pattern    `json:"pattern"`
	Scope      tuiclient.PatternScope  `json:"scope"`
	Stats      *tuiclient.PatternStats `json:"stats,omitempty" jsonschema:"rounds, matched, won, win_rate, recent_matched and trend, as Patrones guardados lists them"`
	StudioPath string                  `json:"studio_path,omitempty" jsonschema:"opens the pattern in its team's Replay > Patrones"`
}

type listPatternsOut struct {
	Patterns []savedPatternOut `json:"patterns"`
}

func (in patternAreaIn) area() (*tacticalplan.PatternArea, error) {
	if in.MinX >= in.MaxX || in.MinY >= in.MaxY {
		return nil, errors.New("area needs min_x below max_x and min_y below max_y")
	}
	level := strings.ToLower(strings.TrimSpace(in.Level))
	if level != "" && level != "default" && level != "lower" {
		return nil, fmt.Errorf("area level must be lower, default or empty, not %q", cleanName(in.Level))
	}
	return &tacticalplan.PatternArea{MinX: in.MinX, MinY: in.MinY, MaxX: in.MaxX, MaxY: in.MaxY, Level: level}, nil
}

func (in patternBlockIn) block() (tacticalplan.PatternBlock, error) {
	b := tacticalplan.PatternBlock{Kind: tacticalplan.PatternBlockKind(strings.ToLower(strings.TrimSpace(in.Kind))), Rival: in.Rival,
		Grenade: tacticalplan.EventKind(strings.ToLower(strings.TrimSpace(in.Grenade))), FromThrow: in.FromThrow,
		From: in.ClockFrom, To: in.ClockTo, Invert: in.Invert}
	for _, id := range in.Players {
		if err := checkSteamID(id); err != nil {
			return b, err
		}
		b.Players = append(b.Players, id)
	}
	if in.Area != nil {
		area, err := in.Area.area()
		if err != nil {
			return b, err
		}
		b.Area = area
	}
	return b, nil
}

// pattern checks the blocks as the orchestrator will.
func patternOf(side string, blocks []patternBlockIn) (tacticalplan.Pattern, error) {
	s, err := parseSide(side)
	if err != nil {
		return tacticalplan.Pattern{}, err
	}
	if s == "" {
		return tacticalplan.Pattern{}, errors.New("side must be T or CT")
	}
	p := tacticalplan.Pattern{Side: tacticalplan.Side(s)}
	for _, in := range blocks {
		b, err := in.block()
		if err != nil {
			return p, err
		}
		p.Blocks = append(p.Blocks, b)
	}
	return p, p.Check()
}

func (in patternScopeIn) scope() (tuiclient.PatternScope, error) {
	if in.Matches < 0 || in.Matches > maxPatternMatches {
		return tuiclient.PatternScope{}, fmt.Errorf("matches must be from 0 to %d", maxPatternMatches)
	}
	out := tuiclient.PatternScope{Matches: in.Matches}
	for _, buy := range []*string{&in.Buy, &in.OpponentBuy} {
		*buy = strings.ToLower(strings.TrimSpace(*buy))
		if *buy != "" && !slices.Contains(patternBuys, *buy) {
			return out, errors.New("buy and opponent_buy must be pistol, eco, semi, force or full")
		}
	}
	out.Buy, out.OpponentBuy = in.Buy, in.OpponentBuy
	return out, nil
}

func patternScopeValues(s tuiclient.PatternScope) url.Values {
	v := url.Values{}
	if s.Matches > 0 {
		v.Set("matches", strconv.Itoa(s.Matches))
	}
	if s.Buy != "" {
		v.Set("buy", s.Buy)
	}
	if s.OpponentBuy != "" {
		v.Set("opponent_buy", s.OpponentBuy)
	}
	return v
}

// clockLabel is seconds after the round went live on CS2's round clock.
func clockLabel(seconds float64) string {
	left := max(0, roundClockSeconds-int(seconds+0.5))
	return fmt.Sprintf("%d:%02d", left/60, left%60)
}

func summarizePattern(run tuiclient.PatternRun, title string) patternRunOut {
	r := run.Result
	out := patternRunOut{Map: cleanName(run.Map), Rounds: r.Rounds, Matched: r.Matched, Won: r.Won, WinRate: r.WinRate, Blocks: r.Blocks,
		MissingPositions: r.MissingPositions, RecentRounds: r.RecentRounds, RecentMatched: r.RecentMatched, Trend: r.Trend,
		Months: []patternMonthOut{}, Matches: []patternMatchOut{}}
	if out.Blocks == nil {
		out.Blocks = []tacticalplan.PatternCount{}
	}
	for _, m := range r.Months {
		out.Months = append(out.Months, patternMonthOut(m))
	}
	refs, rows := url.Values{}, 0
	for _, m := range r.Matches {
		if len(m.Matched) == 0 {
			continue
		}
		match := patternMatchOut{MatchID: m.MatchID.String(), PlayedAt: m.PlayedAt.Format("2006-01-02"), Team: cleanName(m.Team),
			Opponent: cleanName(m.Opponent), Rounds: m.Rounds, Matched: []patternRoundOut{}}
		for _, row := range m.Matched {
			if rows >= maxPatternRows {
				out.Truncated = true
				break
			}
			rows++
			item := patternRoundOut{Ref: fmt.Sprintf("%s/%d", match.MatchID, row.Round), Side: string(row.Side), Won: row.Won, Buy: string(row.Buy)}
			if row.Seconds != nil {
				item.Clock = clockLabel(*row.Seconds)
			}
			match.Matched = append(match.Matched, item)
			ref := fmt.Sprintf("%s:%d", match.MatchID, row.Round)
			if row.Tick > 0 {
				ref += ":" + strconv.Itoa(row.Tick)
			}
			refs.Add("r", ref)
		}
		if len(match.Matched) > 0 {
			out.Matches = append(out.Matches, match)
		}
	}
	if len(refs) > 0 {
		refs.Set("t", title)
		out.ReplayPath = "/replay?" + refs.Encode()
	}
	return out
}

// patternStudioPath opens the pattern in the team's Replay > Patrones with its scope, as the web reads it.
func patternStudioPath(teamID, mapName string, p tacticalplan.Pattern, scope tuiclient.PatternScope, dates [2]string) string {
	v := url.Values{"tab": {"replay"}, "forma": {"patrones"}}
	if mapName != "" {
		v.Set("map", mapName)
	}
	for key, value := range map[string]string{"from": dates[0], "to": dates[1], "buy": scope.Buy, "opponent_buy": scope.OpponentBuy} {
		if value != "" {
			v.Set(key, value)
		}
	}
	if scope.Matches > 0 {
		v.Set("limit", strconv.Itoa(scope.Matches))
	}
	if raw, err := json.Marshal(p); err == nil {
		v.Set("patron", string(raw))
	}
	return "/equipos/" + url.PathEscape(teamID) + "?" + v.Encode()
}

func savedPatternOf(p tuiclient.SavedPattern) savedPatternOut {
	out := savedPatternOut{ID: p.ID, Name: cleanName(p.Name), Map: p.Map, TeamID: p.TeamID, Pattern: p.Pattern, Scope: p.Scope, Stats: p.Stats}
	if p.TeamID != "" {
		out.StudioPath = patternStudioPath(p.TeamID, p.Map, p.Pattern, p.Scope, [2]string{})
	}
	return out
}

// runTeamPattern checks a team run, asks the orchestrator and words the answer.
func runTeamPattern(ctx context.Context, s *studio, run teamPatternRun) (patternRunOut, error) {
	if err := checkUUID("team_id", run.teamID); err != nil {
		return patternRunOut{}, err
	}
	q, err := teamQuery(analysisFilterIn{Map: run.mapName, From: run.from, To: run.to}, run.teamScope)
	if err != nil {
		return patternRunOut{}, err
	}
	scope, err := run.scope.scope()
	if err != nil {
		return patternRunOut{}, err
	}
	c, err := s.client(ctx)
	if err != nil {
		return patternRunOut{}, toolError(err)
	}
	got, err := c.TeamPattern(ctx, run.teamID, q, patternScopeValues(scope), run.pattern)
	if err != nil {
		return patternRunOut{}, toolError(err)
	}
	out := summarizePattern(got, "Patrón en "+cleanName(got.Map))
	out.StudioPath = patternStudioPath(run.teamID, got.Map, run.pattern, scope, [2]string{run.from, run.to})
	return out, nil
}

type teamPatternRun struct {
	teamID, mapName, from, to string
	teamScope                 teamScopeIn
	scope                     patternScopeIn
	pattern                   tacticalplan.Pattern
}

func registerPatternTools(server *mcp.Server, s *studio) {
	readOnly := &mcp.ToolAnnotations{ReadOnlyHint: true}

	mcp.AddTool(server, &mcp.Tool{Name: "find_player_pattern", Annotations: readOnly,
		Description: "Find the rounds in which a team's player (or players) passes through a zone in a clock window, as Replay > Patrones does with one Jugador block: " +
			"over the team's matches on one map and side, its most recent N with matches. invert keeps the rounds in which they do not pass. " +
			"The answer lists the matched rounds by match with when they entered the zone, the win rate, the last 30 days and the trend, " +
			"replay_path to open them as a playlist and studio_path to open the finder with its paths and heatmap."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in findPlayerPatternIn) (*mcp.CallToolResult, patternRunOut, error) {
			block := patternBlockIn{Kind: "player", Players: in.Players, Rival: in.Rival, Area: in.Area, ClockFrom: in.ClockFrom, ClockTo: in.ClockTo, Invert: in.Invert}
			p, err := patternOf(in.Side, []patternBlockIn{block})
			if err != nil {
				return nil, patternRunOut{}, err
			}
			out, err := runTeamPattern(ctx, s, teamPatternRun{teamID: in.TeamID, mapName: in.Map, from: in.From, to: in.To, teamScope: in.teamScopeIn, scope: in.patternScopeIn, pattern: p})
			return nil, out, err
		})

	mcp.AddTool(server, &mcp.Tool{Name: "find_pattern_rounds", Annotations: readOnly,
		Description: "Run a pattern of blocks that add up over a team's matches on one map and side, as Replay > Patrones does: player (a player passes through a zone), " +
			"utility (a grenade lands in a zone, or is thrown from it) and plant, each with an optional zone, clock window, rival and invert. " +
			"A round is kept when every block holds. The answer has each block's own count, the matched rounds by match with their outcome, " +
			"the win rate, the last 30 days, the trend and months, replay_path and studio_path."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in findPatternIn) (*mcp.CallToolResult, patternRunOut, error) {
			p, err := patternOf(in.Side, in.Blocks)
			if err != nil {
				return nil, patternRunOut{}, err
			}
			out, err := runTeamPattern(ctx, s, teamPatternRun{teamID: in.TeamID, mapName: in.Map, from: in.From, to: in.To, teamScope: in.teamScopeIn, scope: in.patternScopeIn, pattern: p})
			return nil, out, err
		})

	mcp.AddTool(server, &mcp.Tool{Name: "save_pattern", Annotations: &mcp.ToolAnnotations{},
		Description: "Save a pattern, as Guardar patrón does: for a team, or without team_id for the whole library, where Studio detects it over every analysed match on the map."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in savePatternIn) (*mcp.CallToolResult, savedPatternOut, error) {
			name := strings.TrimSpace(in.Name)
			if name == "" || utf8.RuneCountInString(name) > maxPatternNameRunes {
				return nil, savedPatternOut{}, fmt.Errorf("name must have from 1 to %d characters", maxPatternNameRunes)
			}
			if !mapNamePattern.MatchString(in.Map) {
				return nil, savedPatternOut{}, errors.New("map must be a map name such as de_ancient")
			}
			if in.TeamID != "" {
				if err := checkUUID("team_id", in.TeamID); err != nil {
					return nil, savedPatternOut{}, err
				}
			}
			p, err := patternOf(in.Side, in.Blocks)
			if err != nil {
				return nil, savedPatternOut{}, err
			}
			scope, err := in.scope()
			if err != nil {
				return nil, savedPatternOut{}, err
			}
			c, err := s.client(ctx)
			if err != nil {
				return nil, savedPatternOut{}, toolError(err)
			}
			saved, err := c.CreatePattern(ctx, tuiclient.SavedPattern{Name: name, Map: strings.ToLower(in.Map), TeamID: in.TeamID, Pattern: p, Scope: scope})
			if err != nil {
				return nil, savedPatternOut{}, toolError(err)
			}
			return nil, savedPatternOf(saved), nil
		})

	mcp.AddTool(server, &mcp.Tool{Name: "list_patterns", Annotations: readOnly,
		Description: "List a map's saved patterns with their figures, as Patrones guardados does: with team_id, over that team's matches (its patterns and the library-wide ones); " +
			"without it, each detected over the whole library. Each has its rounds, matched, won, win_rate, last 30 days and trend."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in listPatternsIn) (*mcp.CallToolResult, listPatternsOut, error) {
			if !mapNamePattern.MatchString(in.Map) {
				return nil, listPatternsOut{}, errors.New("map must be a map name such as de_ancient")
			}
			if in.TeamID != "" {
				if err := checkUUID("team_id", in.TeamID); err != nil {
					return nil, listPatternsOut{}, err
				}
			}
			c, err := s.client(ctx)
			if err != nil {
				return nil, listPatternsOut{}, toolError(err)
			}
			list, err := c.ListPatterns(ctx, tuiclient.PatternListQuery{Map: strings.ToLower(in.Map), TeamID: in.TeamID, Stats: true})
			if err != nil {
				return nil, listPatternsOut{}, toolError(err)
			}
			out := listPatternsOut{Patterns: []savedPatternOut{}}
			for _, p := range list {
				item := savedPatternOf(p)
				if in.TeamID != "" {
					item.StudioPath = patternStudioPath(in.TeamID, p.Map, p.Pattern, p.Scope, [2]string{})
				}
				out.Patterns = append(out.Patterns, item)
			}
			return nil, out, nil
		})

	mcp.AddTool(server, &mcp.Tool{Name: "pattern_rounds", Annotations: readOnly,
		Description: "Run a saved pattern: over team_id, its own team, or without one over every analysed match of its map in the library. " +
			"The answer is find_pattern_rounds's: the matched rounds by match, win rate, trend, replay_path and, over a team, studio_path."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in patternRoundsIn) (*mcp.CallToolResult, patternRunOut, error) {
			if err := checkUUID("pattern_id", in.PatternID); err != nil {
				return nil, patternRunOut{}, err
			}
			if in.TeamID != "" {
				if err := checkUUID("team_id", in.TeamID); err != nil {
					return nil, patternRunOut{}, err
				}
			}
			c, err := s.client(ctx)
			if err != nil {
				return nil, patternRunOut{}, toolError(err)
			}
			saved, err := c.GetPattern(ctx, in.PatternID)
			if err != nil {
				return nil, patternRunOut{}, toolError(err)
			}
			got, err := c.PatternRounds(ctx, in.PatternID, in.TeamID)
			if err != nil {
				return nil, patternRunOut{}, toolError(err)
			}
			out := summarizePattern(got, cleanName(saved.Name))
			if team := cmp.Or(in.TeamID, saved.TeamID); team != "" {
				out.StudioPath = patternStudioPath(team, saved.Map, saved.Pattern, saved.Scope, [2]string{})
			}
			return nil, out, nil
		})

	mcp.AddTool(server, &mcp.Tool{Name: "delete_pattern", Annotations: &mcp.ToolAnnotations{DestructiveHint: new(true)},
		Description: "Delete a saved pattern. It cannot be undone, so the user must approve it. " + deleteApprovalHint},
		func(ctx context.Context, req *mcp.CallToolRequest, in deletePatternIn) (*mcp.CallToolResult, deleteOut, error) {
			if err := checkUUID("pattern_id", in.PatternID); err != nil {
				return nil, deleteOut{}, err
			}
			c, err := s.client(ctx)
			if err != nil {
				return nil, deleteOut{}, toolError(err)
			}
			saved, err := c.GetPattern(ctx, in.PatternID)
			if err != nil {
				return nil, deleteOut{}, toolError(err)
			}
			summary := fmt.Sprintf("Borrar el patrón %s. No se puede deshacer.", quotedName(saved.Name, "sin nombre"))
			return approveAndDelete(req, approvalSpec{Tool: "delete_pattern", JobID: in.PatternID, Request: saved.UpdatedAt, Summary: summary, Given: in.Approve, Question: "¿Borrar ahora?"},
				func() error { return c.DeletePattern(ctx, in.PatternID) })
		})
}
