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

type teamRoundsIn struct {
	TeamID string `json:"team_id" jsonschema:"team id from list_teams"`
	Form   string `json:"form,omitempty" jsonschema:"which form of the team's Replay studio_path opens: calor (default), recorridos or granadas"`
	analysisFilterIn
	teamScopeIn
}

type teamRoundsMatchOut struct {
	MatchID  string        `json:"match_id"`
	Map      string        `json:"map"`
	PlayedAt string        `json:"played_at" jsonschema:"YYYY-MM-DD"`
	Opponent string        `json:"opponent" jsonschema:"untrusted team name from the demo"`
	Kept     int           `json:"kept" jsonschema:"rounds of this match the scope keeps"`
	Total    int           `json:"total" jsonschema:"rounds the team played in it"`
	Rounds   []roundRefOut `json:"rounds"`
}

type teamRoundsOut struct {
	Team         string               `json:"team" jsonschema:"untrusted team name"`
	Rounds       int                  `json:"rounds" jsonschema:"rounds the scope keeps over every match, the count the team Replay shows"`
	Matches      []teamRoundsMatchOut `json:"matches" jsonschema:"newest match first"`
	Unranked     int                  `json:"unranked,omitempty" jsonschema:"matches opponent_top left out because the rival has no HLTV rank"`
	Skipped      int                  `json:"skipped,omitempty" jsonschema:"other matches that could not be read"`
	StudioPath   string               `json:"studio_path" jsonschema:"the path that opens the team's Replay in Studio with this scope"`
	PlaylistPath string               `json:"playlist_path,omitempty" jsonschema:"the path that plays the kept rounds one after another in the Replay, at most 200"`
}

// teamReplayForms are the forms of the team's Replay, as the URL names them.
var teamReplayForms = []string{"calor", "recorridos", "granadas"}

// teamReplayPath is the team's Replay tab with the scope the web's scope bar writes.
func teamReplayPath(teamID, form string, q tuiclient.AnalysisQuery) string {
	v := url.Values{"tab": {"replay"}, "lineup": {"all"}}
	if form != "" && form != teamReplayForms[0] {
		v.Set("forma", form)
	}
	for key, value := range map[string]string{"map": q.Map, "side": q.Side} {
		if value != "" {
			v.Set(key, value)
		}
	}
	if !q.From.IsZero() {
		v.Set("from", q.From.Format(time.DateOnly))
	}
	if !q.To.IsZero() {
		v.Set("to", q.To.Format(time.DateOnly))
	}
	if len(q.Opponents) == 1 {
		v.Set("opponent", q.Opponents[0])
	}
	if len(q.Lineup) > 0 {
		v.Set("lineup", strings.Join(slices.Compact(slices.Sorted(slices.Values(q.Lineup))), ","))
	}
	for key, list := range q.Scope {
		switch {
		case key == "max_matches":
			v.Set("limit", list[0])
		case (key == "buy" || key == "opponent_buy") && len(list) > 1:
			// The bar holds one buy, or half buys: semi and force together.
			if sorted := slices.Sorted(slices.Values(list)); slices.Equal(sorted, []string{"force", "semi"}) {
				v.Set(key, "half")
			}
		default:
			v.Set(key, list[0])
		}
	}
	return "/equipos/" + url.PathEscape(teamID) + "?" + v.Encode()
}

func registerTeamRoundsTool(server *mcp.Server, s *studio) {
	mcp.AddTool(server, &mcp.Tool{Name: "team_rounds", Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true},
		Description: "Read the rounds a team's Replay keeps over several matches with the scope bar's filters: map, side, period (from, to or last_days), " +
			"last N matches, rival, lineup, opponent level, whose matches, buys of both sides, AWP of both sides, numeric advantage and a round range. " +
			"Every team tool (scout_team, team_bombsites, team_playbook, team_prep, team_grenades, detect_team_plays) takes the same filters and counts these rounds. " +
			"studio_path opens the Replay on this scope; playlist_path plays the rounds one after another."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in teamRoundsIn) (*mcp.CallToolResult, teamRoundsOut, error) {
			if err := checkUUID("team_id", in.TeamID); err != nil {
				return nil, teamRoundsOut{}, err
			}
			form := strings.ToLower(strings.TrimSpace(in.Form))
			if form != "" && !slices.Contains(teamReplayForms, form) {
				return nil, teamRoundsOut{}, fmt.Errorf("form must be one of %s", strings.Join(teamReplayForms, ", "))
			}
			q, err := teamQuery(in.analysisFilterIn, in.teamScopeIn)
			if err != nil {
				return nil, teamRoundsOut{}, err
			}
			c, err := s.client(ctx)
			if err != nil {
				return nil, teamRoundsOut{}, toolError(err)
			}
			got, err := c.TeamRounds(ctx, in.TeamID, q)
			if err != nil {
				return nil, teamRoundsOut{}, toolError(err)
			}
			return nil, teamRoundsOutOf(got, teamReplayPath(in.TeamID, form, q)), nil
		})
}

func teamRoundsOutOf(got tuiclient.TeamRounds, studioPath string) teamRoundsOut {
	out := teamRoundsOut{Team: cleanName(got.TeamName), Rounds: got.Rounds, Matches: []teamRoundsMatchOut{}, StudioPath: studioPath}
	for _, s := range got.Skipped {
		if s.Reason == "opponent_unranked" {
			out.Unranked++
		} else {
			out.Skipped++
		}
	}
	var refs []string
	for _, m := range got.Matches {
		row := teamRoundsMatchOut{MatchID: m.MatchID, Map: cleanName(m.Map), PlayedAt: m.PlayedAt.Format(time.DateOnly),
			Opponent: cleanName(m.Opponent), Kept: len(m.Rounds), Total: m.Total, Rounds: []roundRefOut{}}
		for _, r := range m.Rounds {
			ref := fmt.Sprintf("%s/%d", m.MatchID, r.Round)
			refs = append(refs, ref)
			row.Rounds = append(row.Rounds, roundRefOut{Ref: ref, MatchID: m.MatchID, Round: r.Round, Map: row.Map, PlayedAt: row.PlayedAt,
				Opponent: row.Opponent, Side: r.Side, Buy: r.Buy, OpponentBuy: r.OpponentBuy, Won: r.Winner == r.Side, Site: r.Site})
		}
		out.Matches = append(out.Matches, row)
	}
	if len(refs) > 0 {
		path, err := playlistPath(refs[:min(len(refs), maxPlaylistRounds)], out.Team)
		if err == nil {
			out.PlaylistPath = path
		}
	}
	return out
}
