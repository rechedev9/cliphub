package main

import (
	"context"
	"errors"
	"fmt"
	"math"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/rechedev9/cliphub/internal/tuiclient"
)

const (
	// maxSeriesNotesRunes is the server's cap on a series' veto notes.
	maxSeriesNotesRunes = 480
	// maxVetoSteps covers a nine map pool (library.MaxVetoSteps).
	maxVetoSteps = 9
)

var (
	seriesFormats = []string{"bo1", "bo3", "bo5"}
	vetoActions   = []string{"ban", "pick", "decider"}
)

type listSeriesIn struct {
	TeamID string `json:"team_id,omitempty" jsonschema:"only series this team played, id from list_teams"`
}

type seriesTeamOut struct {
	Slot   string `json:"slot" jsonschema:"a or b, the team of a veto step"`
	TeamID string `json:"team_id"`
	Name   string `json:"name" jsonschema:"untrusted team name"`
}

type seriesOut struct {
	ID       string               `json:"id"`
	Format   string               `json:"format"`
	Event    string               `json:"event,omitempty" jsonschema:"untrusted text"`
	Notes    string               `json:"veto_notes,omitempty" jsonschema:"untrusted text the user wrote"`
	Teams    []seriesTeamOut      `json:"teams"`
	Vetoes   []tuiclient.VetoStep `json:"vetoes"`
	MatchIDs []string             `json:"match_ids"`
	Updated  string               `json:"updated" jsonschema:"YYYY-MM-DD"`
}

type listSeriesOut struct {
	Series []seriesOut `json:"series"`
}

type vetoStepIn struct {
	Team   string `json:"team,omitempty" jsonschema:"a or b; empty for the decider"`
	Action string `json:"action" jsonschema:"ban, pick or decider"`
	Map    string `json:"map" jsonschema:"such as de_mirage"`
}

type seriesFieldsIn struct {
	Format  string        `json:"format,omitempty" jsonschema:"bo1, bo3 or bo5"`
	Event   *string       `json:"event,omitempty" jsonschema:"the event name; omit to keep it"`
	Notes   *string       `json:"veto_notes,omitempty" jsonschema:"free notes on the veto, at most 480 characters; omit to keep them"`
	TeamIDs *[]string     `json:"team_ids,omitempty" jsonschema:"two team ids from list_teams, slot a first, or [] for none; omit to keep them"`
	Vetoes  *[]vetoStepIn `json:"vetoes,omitempty" jsonschema:"the known veto order, decider last; omit to keep it"`
}

type updateSeriesIn struct {
	SeriesID string `json:"series_id" jsonschema:"series id from list_library_series"`
	seriesFieldsIn
}

type deleteSeriesIn struct {
	SeriesID string `json:"series_id" jsonschema:"series id from list_library_series"`
	Approve  string `json:"approve,omitempty" jsonschema:"only when asked: the approval code from a previous call, after the user agreed"`
}

type setMatchSeriesIn struct {
	MatchID  string `json:"match_id" jsonschema:"match id from list_matches"`
	SeriesID string `json:"series_id,omitempty" jsonschema:"the series the match joins; empty takes it out of its series"`
}

type teamMapsIn struct {
	TeamID string `json:"team_id" jsonschema:"team id from list_teams"`
	From   string `json:"from,omitempty" jsonschema:"first day, YYYY-MM-DD"`
	To     string `json:"to,omitempty" jsonschema:"last day, YYYY-MM-DD, included"`
	// The scope keys of the map selector: they only set scope_matches, the record stays over from and to.
	OpponentID  []string `json:"opponent_id,omitempty" jsonschema:"scope_matches counts only matches against these teams, ids from list_teams"`
	Lineup      []string `json:"lineup,omitempty" jsonschema:"scope_matches counts only matches the team played with these players, SteamID64s from team_lineups or get_team, at most 50"`
	MaxMatches  int      `json:"max_matches,omitempty" jsonschema:"scope_matches counts at most the team's most recent N matches on each map, 1 to 50"`
	LastDays    int      `json:"last_days,omitempty" jsonschema:"scope_matches counts only matches of the last N days: 3, 7, 14, 30, 90, 180 or 365; do not combine with from"`
	OpponentTop int      `json:"opponent_top,omitempty" jsonschema:"scope_matches counts only matches against a rival in the HLTV top N: 5, 15, 30, 100 or 200"`
	Whose       string   `json:"whose,omitempty" jsonschema:"scope_matches counts only own matches (own) or only other teams' matches (others)"`
}

type mapRecordOut struct {
	Map         string         `json:"map"`
	Matches     int            `json:"matches"`
	Record      string         `json:"record" jsonschema:"wins-losses-ties"`
	RoundWinPct float64        `json:"round_win_pct"`
	CTWinPct    float64        `json:"ct_round_win_pct"`
	TWinPct     float64        `json:"t_round_win_pct"`
	Vetoes      map[string]int `json:"vetoes" jsonschema:"picked, banned, opponent_picked, opponent_banned and decider counts from the series"`
	Preference  int            `json:"preference" jsonschema:"-100 to 100: how much the team seeks this map"`
	// ScopeMatches is the N partidas of the map selector.
	ScopeMatches int `json:"scope_matches" jsonschema:"matches on this map under the scope keys, the N partidas of the map selector; 0 is a map the selector greys out (Sin partidas con esta alineación)"`
}

type teamMapsOut struct {
	Team           string         `json:"team" jsonschema:"untrusted team name"`
	Matches        int            `json:"matches"`
	Series         int            `json:"series"`
	Maps           []mapRecordOut `json:"maps"`
	SkippedMatches int            `json:"skipped_matches,omitempty"`
}

type vetoIn struct {
	TeamID     string   `json:"team_id" jsonschema:"our team, id from list_teams; slot a"`
	OpponentID string   `json:"opponent_id" jsonschema:"the rival, id from list_teams; slot b"`
	Format     string   `json:"format,omitempty" jsonschema:"bo1, bo3 (default) or bo5"`
	First      string   `json:"first,omitempty" jsonschema:"a (default) or b: who bans first"`
	Pool       []string `json:"pool,omitempty" jsonschema:"the map pool; default the active duty pool"`
	Done       []string `json:"done,omitempty" jsonschema:"maps of the steps already made, in order; the rest is recommended"`
}

type vetoMapOut struct {
	Map        string  `json:"map"`
	OurMatches int     `json:"our_matches"`
	OurRate    float64 `json:"our_round_win_pct"`
	TheirMatch int     `json:"their_matches"`
	TheirRate  float64 `json:"their_round_win_pct"`
	EdgePct    float64 `json:"edge_pct" jsonschema:"our smoothed round win rate minus theirs; positive favours us"`
	TheirBans  int     `json:"their_bans"`
	TheirPicks int     `json:"their_picks"`
}

type vetoStepOut struct {
	Team   string `json:"team" jsonschema:"a (us), b (the rival) or empty for the decider"`
	Action string `json:"action"`
	Map    string `json:"map"`
	Basis  string `json:"basis" jsonschema:"chosen: a step already made; edge: our round win rate against theirs; history: the rival's known bans and picks; leftover: the map left"`
}

type vetoOut struct {
	Team            string        `json:"team" jsonschema:"untrusted team name"`
	Opponent        string        `json:"opponent" jsonschema:"untrusted team name"`
	Format          string        `json:"format"`
	First           string        `json:"first"`
	Steps           []vetoStepOut `json:"steps"`
	Played          []string      `json:"played" jsonschema:"the maps the series would be played on, in order"`
	Maps            []vetoMapOut  `json:"maps"`
	OpponentHistory int           `json:"opponent_history" jsonschema:"known bans and picks of the rival the plan read"`
}

func registerLibrarySeriesTools(server *mcp.Server, s *studio) {
	readOnly := &mcp.ToolAnnotations{ReadOnlyHint: true}

	mcp.AddTool(server, &mcp.Tool{Name: "list_library_series", Annotations: readOnly,
		Description: "List the series of the library (the BO1, BO3 and BO5 that group matches, with their teams and veto), optionally those of one team."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in listSeriesIn) (*mcp.CallToolResult, listSeriesOut, error) {
			if in.TeamID != "" {
				if err := checkUUID("team_id", in.TeamID); err != nil {
					return nil, listSeriesOut{}, err
				}
			}
			c, err := s.client(ctx)
			if err != nil {
				return nil, listSeriesOut{}, toolError(err)
			}
			series, err := c.ListLibrarySeries(ctx, in.TeamID)
			if err != nil {
				return nil, listSeriesOut{}, toolError(err)
			}
			out := listSeriesOut{Series: make([]seriesOut, 0, len(series))}
			for _, se := range series {
				out.Series = append(out.Series, seriesOf(se))
			}
			return nil, out, nil
		})

	mcp.AddTool(server, &mcp.Tool{Name: "create_library_series", Annotations: &mcp.ToolAnnotations{},
		Description: "Create a library series as Nueva serie does: format, event, the two teams (slot a first) and the known veto. " +
			"Put its matches in it with set_match_series."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in seriesFieldsIn) (*mcp.CallToolResult, seriesOut, error) {
			if in.Format == "" {
				in.Format = "bo3"
			}
			body, err := in.input()
			if err != nil {
				return nil, seriesOut{}, err
			}
			c, err := s.client(ctx)
			if err != nil {
				return nil, seriesOut{}, toolError(err)
			}
			se, err := c.CreateLibrarySeries(ctx, body)
			if err != nil {
				return nil, seriesOut{}, toolError(err)
			}
			return nil, seriesOf(se), nil
		})

	mcp.AddTool(server, &mcp.Tool{Name: "update_library_series", Annotations: &mcp.ToolAnnotations{IdempotentHint: true},
		Description: "Edit a library series: format, event, veto notes, the two teams or the known veto order."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in updateSeriesIn) (*mcp.CallToolResult, seriesOut, error) {
			if err := checkUUID("series_id", in.SeriesID); err != nil {
				return nil, seriesOut{}, err
			}
			f := in.seriesFieldsIn
			if f.Format == "" && f.Event == nil && f.Notes == nil && f.TeamIDs == nil && f.Vetoes == nil {
				return nil, seriesOut{}, errors.New("set format, event, veto_notes, team_ids or vetoes")
			}
			body, err := f.input()
			if err != nil {
				return nil, seriesOut{}, err
			}
			c, err := s.client(ctx)
			if err != nil {
				return nil, seriesOut{}, toolError(err)
			}
			se, err := c.UpdateLibrarySeries(ctx, in.SeriesID, body)
			if err != nil {
				return nil, seriesOut{}, toolError(err)
			}
			return nil, seriesOf(se), nil
		})

	mcp.AddTool(server, &mcp.Tool{Name: "delete_library_series", Annotations: &mcp.ToolAnnotations{DestructiveHint: new(true)},
		Description: "Delete a library series with its veto; its matches stay in the library without a series. It cannot be undone, so the user must approve it. " + deleteApprovalHint},
		func(ctx context.Context, req *mcp.CallToolRequest, in deleteSeriesIn) (*mcp.CallToolResult, deleteOut, error) {
			if err := checkUUID("series_id", in.SeriesID); err != nil {
				return nil, deleteOut{}, err
			}
			c, err := s.client(ctx)
			if err != nil {
				return nil, deleteOut{}, toolError(err)
			}
			se, err := c.GetLibrarySeries(ctx, in.SeriesID)
			if err != nil {
				return nil, deleteOut{}, toolError(err)
			}
			kept := "No tiene partidas."
			switch n := len(se.MatchIDs); {
			case n == 1:
				kept = "Su partida se queda en la biblioteca."
			case n > 1:
				kept = fmt.Sprintf("Sus %d partidas se quedan en la biblioteca.", n)
			}
			summary := fmt.Sprintf("Borrar la serie %s %s con su veto. %s No se puede deshacer.", strings.ToUpper(se.Format), seriesName(se), kept)
			return approveAndDelete(req, approvalSpec{Tool: "delete_library_series", JobID: in.SeriesID, Request: se.UpdatedAt, Summary: summary, Given: in.Approve, Question: "¿Borrar ahora?"},
				func() error { return c.DeleteLibrarySeries(ctx, in.SeriesID) })
		})

	mcp.AddTool(server, &mcp.Tool{Name: "set_match_series", Annotations: &mcp.ToolAnnotations{IdempotentHint: true},
		Description: "Put a library match in a series, or take it out with an empty series_id, as the match summary does."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in setMatchSeriesIn) (*mcp.CallToolResult, matchSummary, error) {
			if err := checkUUID("match_id", in.MatchID); err != nil {
				return nil, matchSummary{}, err
			}
			var seriesID *string
			if in.SeriesID != "" {
				if err := checkUUID("series_id", in.SeriesID); err != nil {
					return nil, matchSummary{}, err
				}
				seriesID = &in.SeriesID
			}
			c, err := s.client(ctx)
			if err != nil {
				return nil, matchSummary{}, toolError(err)
			}
			m, err := c.SetMatchSeries(ctx, in.MatchID, seriesID)
			if err != nil {
				return nil, matchSummary{}, toolError(err)
			}
			return nil, summarizeMatch(m), nil
		})

	mcp.AddTool(server, &mcp.Tool{Name: "team_maps", Annotations: readOnly,
		Description: "Read a team's record on every map: results, round win rate per side, how often it picks or bans the map in its series, and its preference. " +
			"scope_matches is the N partidas of the Replay, Equipos and Preparar map selector, counted with the same scope keys the bar sends (opponent_id, lineup, max_matches, last_days, opponent_top, whose, from, to)."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in teamMapsIn) (*mcp.CallToolResult, teamMapsOut, error) {
			if err := checkUUID("team_id", in.TeamID); err != nil {
				return nil, teamMapsOut{}, err
			}
			filter := analysisFilterIn{From: in.From, To: in.To}
			q, err := filter.query()
			if err != nil {
				return nil, teamMapsOut{}, err
			}
			scope, err := teamQuery(filter, teamScopeIn{OpponentID: in.OpponentID, Lineup: in.Lineup, teamRoundScopeIn: teamRoundScopeIn{
				MaxMatches: in.MaxMatches, LastDays: in.LastDays, OpponentTop: in.OpponentTop, Whose: in.Whose,
			}})
			if err != nil {
				return nil, teamMapsOut{}, err
			}
			c, err := s.client(ctx)
			if err != nil {
				return nil, teamMapsOut{}, toolError(err)
			}
			tm, err := c.LibraryTeamMaps(ctx, in.TeamID, q)
			if err != nil {
				return nil, teamMapsOut{}, toolError(err)
			}
			counts, err := c.LibraryTeamMapCounts(ctx, tuiclient.MapCountsRequest{TeamID: in.TeamID, Scope: scope})
			if err != nil {
				return nil, teamMapsOut{}, toolError(err)
			}
			return nil, summarizeTeamMaps(tm, counts), nil
		})

	mcp.AddTool(server, &mcp.Tool{Name: "simulate_veto", Annotations: readOnly,
		Description: "Simulate a BO1, BO3 or BO5 veto between our team (slot a) and a rival (slot b), as the Veto tab does: the steps already made " +
			"are kept and the rest recommended from both map records and the rival's known bans and picks."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in vetoIn) (*mcp.CallToolResult, vetoOut, error) {
			q, err := in.query()
			if err != nil {
				return nil, vetoOut{}, err
			}
			c, err := s.client(ctx)
			if err != nil {
				return nil, vetoOut{}, toolError(err)
			}
			v, err := c.SimulateVeto(ctx, in.TeamID, q)
			if err != nil {
				return nil, vetoOut{}, toolError(err)
			}
			return nil, summarizeVeto(v), nil
		})
}

func (in seriesFieldsIn) input() (tuiclient.SeriesInput, error) {
	var body tuiclient.SeriesInput
	if in.Format != "" {
		if !slices.Contains(seriesFormats, in.Format) {
			return body, errors.New("format must be bo1, bo3 or bo5")
		}
		body.Format = &in.Format
	}
	if in.Event != nil {
		event := strings.TrimSpace(*in.Event)
		if event != "" {
			var err error
			if event, err = checkUserText("event", event); err != nil {
				return body, err
			}
		}
		body.Event = &event
	}
	if in.Notes != nil {
		notes := strings.TrimSpace(*in.Notes)
		if utf8.RuneCountInString(notes) > maxSeriesNotesRunes {
			return body, fmt.Errorf("veto_notes take at most %d characters", maxSeriesNotesRunes)
		}
		body.Veto = &notes
	}
	if in.TeamIDs != nil {
		ids := *in.TeamIDs
		if len(ids) != 0 && len(ids) != 2 {
			return body, errors.New("team_ids takes two teams, or none")
		}
		for _, id := range ids {
			if err := checkUUID("team_ids", id); err != nil {
				return body, err
			}
		}
		body.TeamIDs = &ids
	}
	if in.Vetoes != nil {
		if len(*in.Vetoes) > maxVetoSteps {
			return body, fmt.Errorf("a veto has at most %d steps", maxVetoSteps)
		}
		steps := make([]tuiclient.VetoStep, 0, len(*in.Vetoes))
		for i, st := range *in.Vetoes {
			team, action, mapName := strings.ToLower(st.Team), strings.ToLower(st.Action), strings.ToLower(st.Map)
			teamOK := team == "a" || team == "b"
			if action == "decider" {
				teamOK = team == ""
			}
			if !slices.Contains(vetoActions, action) || !teamOK {
				return body, fmt.Errorf("veto step %d: team must be a or b and action ban, pick or decider (no team)", i+1)
			}
			if err := checkOptionalFilter("map", mapName, mapNamePattern); err != nil || mapName == "" {
				return body, fmt.Errorf("veto step %d: map must be a map name such as de_mirage", i+1)
			}
			steps = append(steps, tuiclient.VetoStep{Team: team, Action: action, Map: mapName})
		}
		body.Vetoes = &steps
	}
	return body, nil
}

func (in vetoIn) query() (tuiclient.VetoQuery, error) {
	var q tuiclient.VetoQuery
	if err := checkUUID("team_id", in.TeamID); err != nil {
		return q, err
	}
	if err := checkUUID("opponent_id", in.OpponentID); err != nil {
		return q, err
	}
	if in.TeamID == in.OpponentID {
		return q, errors.New("the opponent must be another team")
	}
	if in.Format != "" && !slices.Contains(seriesFormats, in.Format) {
		return q, errors.New("format must be bo1, bo3 or bo5")
	}
	if in.First != "" && in.First != "a" && in.First != "b" {
		return q, errors.New("first must be a or b")
	}
	if len(in.Pool) > maxVetoSteps || len(in.Done) > maxVetoSteps {
		return q, fmt.Errorf("a veto has at most %d maps", maxVetoSteps)
	}
	for _, m := range append(slices.Clone(in.Pool), in.Done...) {
		if err := checkOptionalFilter("map", m, mapNamePattern); err != nil || m == "" {
			return q, fmt.Errorf("pool and done take map names such as de_mirage, not %q", cleanName(m))
		}
	}
	lower := func(maps []string) []string {
		out := make([]string, 0, len(maps))
		for _, m := range maps {
			out = append(out, strings.ToLower(m))
		}
		return out
	}
	return tuiclient.VetoQuery{OpponentID: in.OpponentID, Format: in.Format, First: in.First, Pool: lower(in.Pool), Done: lower(in.Done)}, nil
}

func seriesOf(se tuiclient.LibrarySeries) seriesOut {
	out := seriesOut{ID: se.ID, Format: se.Format, Event: cleanName(se.Event), Notes: cleanText(se.Veto, maxSeriesNotesRunes),
		Teams: []seriesTeamOut{}, Vetoes: []tuiclient.VetoStep{}, MatchIDs: se.MatchIDs, Updated: se.UpdatedAt.Format(time.DateOnly)}
	for _, t := range se.Teams {
		out.Teams = append(out.Teams, seriesTeamOut{Slot: t.Slot, TeamID: t.TeamID, Name: cleanName(t.Name)})
	}
	for _, v := range se.Vetoes {
		out.Vetoes = append(out.Vetoes, tuiclient.VetoStep{Team: cleanName(v.Team), Action: cleanName(v.Action), Map: cleanName(v.Map)})
	}
	if out.MatchIDs == nil {
		out.MatchIDs = []string{}
	}
	return out
}

// seriesName names a series by its teams for approval text.
func seriesName(se tuiclient.LibrarySeries) string {
	names := make([]string, 0, len(se.Teams))
	for _, t := range se.Teams {
		names = append(names, quotedName(t.Name, "sin nombre"))
	}
	if len(names) == 0 {
		return "sin equipos"
	}
	return strings.Join(names, " contra ")
}

func summarizeTeamMaps(tm tuiclient.TeamMaps, counts tuiclient.TeamMapCounts) teamMapsOut {
	out := teamMapsOut{Team: cleanName(tm.TeamName), Matches: tm.Matches, Series: tm.Series, Maps: []mapRecordOut{}, SkippedMatches: len(tm.Skipped)}
	inScope := map[string]int{}
	for _, c := range counts.Maps {
		inScope[c.Map] = c.Matches
	}
	for _, m := range tm.Maps {
		r := mapRecordOut{Map: cleanName(m.Map), Matches: m.Matches, Record: fmt.Sprintf("%d-%d-%d", m.Wins, m.Losses, m.Ties),
			RoundWinPct: pct(m.RoundsWon, m.RoundsWon+m.RoundsLost), Vetoes: map[string]int{}, Preference: m.Preference.Score,
			ScopeMatches: inScope[m.Map]}
		for _, side := range m.Sides {
			switch side.Side {
			case "CT":
				r.CTWinPct = pct(side.Won.Count, side.Rounds)
			case "T":
				r.TWinPct = pct(side.Won.Count, side.Rounds)
			}
		}
		for k, v := range m.Veto {
			if v.Count > 0 {
				r.Vetoes[cleanName(k)] = v.Count
			}
		}
		out.Maps = append(out.Maps, r)
	}
	return out
}

func summarizeVeto(v tuiclient.VetoSimulation) vetoOut {
	p := v.Plan
	out := vetoOut{Team: cleanName(v.TeamName), Opponent: cleanName(v.OpponentName), Format: p.Format, First: p.First,
		Steps: []vetoStepOut{}, Played: cleanNames(p.Played), Maps: []vetoMapOut{}, OpponentHistory: p.OpponentHistory}
	for _, st := range p.Steps {
		out.Steps = append(out.Steps, vetoStepOut{Team: st.Team, Action: cleanName(st.Action), Map: cleanName(st.Map), Basis: cleanText(st.Basis, maxErrorRunes)})
	}
	for _, m := range p.Maps {
		out.Maps = append(out.Maps, vetoMapOut{Map: cleanName(m.Map), OurMatches: m.OurMatches, OurRate: sharePct(m.OurRate),
			TheirMatch: m.TheirMatches, TheirRate: sharePct(m.TheirRate), EdgePct: math.Round(m.Edge*1000) / 10,
			TheirBans: m.TheirBans, TheirPicks: m.TheirPicks})
	}
	return out
}
