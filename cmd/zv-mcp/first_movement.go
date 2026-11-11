package main

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"math"
	"net/url"
	"slices"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/rechedev9/cliphub/internal/tacticalplan"
	"github.com/rechedev9/cliphub/internal/tuiclient"
)

const (
	defaultMovementRows = 30
	maxMovementRows     = 100
)

type firstMovementIn struct {
	TeamID string `json:"team_id" jsonschema:"team id from list_teams"`
	analysisFilterIn
	teamScopeIn
	Players   []string       `json:"players,omitempty" jsonschema:"only these players' moves, SteamID64s from get_team; empty reads the whole team"`
	Area      *patternAreaIn `json:"area,omitempty" jsonschema:"keep only the rounds in which a chosen player passes through this zone at any moment (world units, as player_positions give them); their moves are listed"`
	ClockFrom float64        `json:"clock_from,omitempty" jsonschema:"keep the moves from this second after the round went live; 0 by default. The round clock 1:40 is 15"`
	ClockTo   float64        `json:"clock_to,omitempty" jsonschema:"keep the moves up to this second; 0 runs to the end of the round"`
	Limit     int            `json:"limit,omitempty" jsonschema:"how many rows to list, default 30, at most 100"`
}

type firstMovementRow struct {
	Ref       string  `json:"ref" jsonschema:"match_id/round"`
	SteamID64 string  `json:"steamid64"`
	Player    string  `json:"player" jsonschema:"untrusted player name from the demo"`
	Seconds   float64 `json:"seconds" jsonschema:"when the player first left the spot the round started on, seconds after the round went live"`
	Clock     string  `json:"clock" jsonschema:"the same moment on the round clock, such as 1:50"`
	linkOut
}

type movePlayerOut struct {
	SteamID64 string `json:"steamid64"`
	Name      string `json:"name" jsonschema:"untrusted player name from the demo"`
	Moves     int    `json:"moves"`
}

type moveBucketOut struct {
	Span    string          `json:"span" jsonschema:"the round clock span, such as 1:55-1:45"`
	Moves   int             `json:"moves"`
	Players []movePlayerOut `json:"players"`
}

type firstMovementOut struct {
	Map        string             `json:"map"`
	Rounds     int                `json:"rounds" jsonschema:"rounds read: the newest ones of the scope, at most 40"`
	Total      int                `json:"total" jsonschema:"rounds the scope kept before that cap"`
	Matched    int                `json:"matched" jsonschema:"rounds with at least one move that passed the filters"`
	Moves      int                `json:"moves" jsonschema:"moves that passed the filters; rows lists the first ones"`
	Histogram  []moveBucketOut    `json:"histogram" jsonschema:"the moves per 10 seconds of the round clock by player, as the Buscador's histogram; empty buckets left out"`
	Rows       []firstMovementRow `json:"rows"`
	Truncated  bool               `json:"truncated,omitempty"`
	StudioPath string             `json:"studio_path" jsonschema:"opens the Buscador de patrones on the Jugadores panel with this scope, players and window"`
}

func (in firstMovementIn) check() error {
	if err := checkUUID("team_id", in.TeamID); err != nil {
		return err
	}
	for _, id := range in.Players {
		if err := checkSteamID(id); err != nil {
			return err
		}
	}
	if err := (clockWindowIn{ClockFrom: in.ClockFrom, ClockTo: in.ClockTo}).check(); err != nil {
		return err
	}
	if in.Limit < 0 || in.Limit > maxMovementRows {
		return fmt.Errorf("limit must be from 1 to %d, or 0 for the default %d", maxMovementRows, defaultMovementRows)
	}
	return nil
}

// firstMovementPath opens the Buscador de patrones on the Jugadores panel with the scope the read used.
func firstMovementPath(in firstMovementIn, mapName string, q tuiclient.AnalysisQuery) (string, error) {
	base, err := teamReplayPath(replayPathIn{TeamID: in.TeamID, Map: mapName, Query: q})
	if err != nil {
		return "", err
	}
	_, query, _ := strings.Cut(base, "?")
	v, err := url.ParseQuery(query)
	if err != nil {
		return "", err
	}
	v.Del("tab")
	v.Set("modo", "jugadores")
	for _, id := range in.Players {
		v.Add(steamFocusQuery, id)
	}
	heatmapViewIn{ClockFrom: in.ClockFrom, ClockTo: in.ClockTo}.studioQuery(v)
	v.Del("forma")
	return "/patrones/" + url.PathEscape(in.TeamID) + "?" + v.Encode(), nil
}

func registerFirstMovementTool(server *mcp.Server, s *studio) {
	mcp.AddTool(server, &mcp.Tool{Name: "team_first_movement", Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true},
		Description: "Read when each player of a team first leaves the spot a round starts on, round by round, as the Primer movimiento list and histogram of the Buscador de patrones' Jugadores panel. " +
			"Filter by players, by a clock window on the move (clock_from and clock_to) and by a zone the players must pass through (area), then the team scope every team tool takes (map, side, dates, opponent_id, match_id, lineup, buys, AWP...). " +
			"It reads the newest 40 rounds of the scope. Each row links to that round with the player in focus; studio_path opens the Buscador."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in firstMovementIn) (*mcp.CallToolResult, firstMovementOut, error) {
			if err := in.check(); err != nil {
				return nil, firstMovementOut{}, err
			}
			q, err := teamQuery(in.analysisFilterIn, in.teamScopeIn)
			if err != nil {
				return nil, firstMovementOut{}, err
			}
			var area *tacticalplan.PatternArea
			if in.Area != nil {
				if area, err = in.Area.area(); err != nil {
					return nil, firstMovementOut{}, err
				}
			}
			c, err := s.client(ctx)
			if err != nil {
				return nil, firstMovementOut{}, toolError(err)
			}
			got, err := c.LibraryTeamOverlay(ctx, in.TeamID, q)
			if err != nil {
				return nil, firstMovementOut{}, toolError(err)
			}
			if got.Overlay == nil {
				return nil, firstMovementOut{}, errors.New("the team has no positions saved for these rounds")
			}
			out := summarizeMovements(in, got, area)
			if out.StudioPath, err = firstMovementPath(in, got.Map, q); err != nil {
				return nil, firstMovementOut{}, err
			}
			return nil, out, nil
		})
}

// summarizeMovements lists the filtered first moves and counts them per bucket of the round clock.
func summarizeMovements(in firstMovementIn, got tuiclient.TeamOverlay, area *tacticalplan.PatternArea) firstMovementOut {
	filter := tacticalplan.FirstMovementFilter{Players: in.Players, From: in.ClockFrom, To: in.ClockTo, Area: area}
	if got.Geometry != nil {
		filter.Geometry = *got.Geometry
	}
	moves := tacticalplan.FirstMovements(*got.Overlay, filter)
	limit := in.Limit
	if limit == 0 {
		limit = defaultMovementRows
	}
	out := firstMovementOut{Map: cleanName(got.Map), Rounds: len(got.Overlay.Rounds), Total: got.Overlay.Total, Moves: len(moves),
		Histogram: movementBuckets(moves), Rows: []firstMovementRow{}}
	rounds := map[string]bool{}
	for i, m := range moves {
		ref := roundRef(m.MatchID.String(), m.Round)
		rounds[ref] = true
		if i >= limit {
			out.Truncated = true
			continue
		}
		link := findingLink{title: fmt.Sprintf("Primer movimiento de %s, ronda %d", cleanName(m.Name), m.Round),
			refs: []tuiclient.Evidence{{MatchID: m.MatchID.String(), Round: m.Round, Tick: m.Tick}}, focus: []string{m.SteamID64}}
		out.Rows = append(out.Rows, firstMovementRow{Ref: ref, SteamID64: cleanName(m.SteamID64), Player: cleanName(m.Name),
			Seconds: math.Round(m.Seconds*10) / 10, Clock: roundClock(roundClockSeconds - int(math.Round(roundClockSeconds-m.Seconds))), linkOut: link.out()})
	}
	out.Matched = len(rounds)
	return out
}

// movementBuckets counts the moves per clockBinSeconds of the round clock by player, as the Buscador's histogram.
func movementBuckets(moves []tacticalplan.FirstMovement) []moveBucketOut {
	bins := int(math.Ceil(float64(roundClockSeconds) / clockBinSeconds))
	counts := make([]map[string]*movePlayerOut, bins)
	for _, m := range moves {
		i := min(bins-1, max(0, int(m.Seconds/clockBinSeconds)))
		if counts[i] == nil {
			counts[i] = map[string]*movePlayerOut{}
		}
		p, ok := counts[i][m.SteamID64]
		if !ok {
			p = &movePlayerOut{SteamID64: cleanName(m.SteamID64), Name: cleanName(m.Name)}
			counts[i][m.SteamID64] = p
		}
		p.Moves++
	}
	out := []moveBucketOut{}
	for i, players := range counts {
		if players == nil {
			continue
		}
		bucket := moveBucketOut{Span: roundClock(i*clockBinSeconds) + "-" + roundClock((i+1)*clockBinSeconds), Players: []movePlayerOut{}}
		for _, p := range players {
			bucket.Moves += p.Moves
			bucket.Players = append(bucket.Players, *p)
		}
		slices.SortFunc(bucket.Players, func(a, b movePlayerOut) int {
			return cmp.Or(cmp.Compare(b.Moves, a.Moves), cmp.Compare(a.SteamID64, b.SteamID64))
		})
		out = append(out, bucket)
	}
	return out
}
