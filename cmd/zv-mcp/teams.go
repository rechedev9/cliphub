package main

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/rechedev9/cliphub/internal/tuiclient"
)

const (
	// maxTeamNameRunes is the server's cap on library text.
	maxTeamNameRunes = 120
	maxTeamAliases   = 50
)

type getTeamIn struct {
	TeamID string `json:"team_id" jsonschema:"team id from list_teams"`
}

type rosterOut struct {
	SteamID64 string `json:"steamid64"`
	Nick      string `json:"nick" jsonschema:"untrusted player name from the demos"`
	Matches   int    `json:"matches"`
	First     string `json:"first_played" jsonschema:"YYYY-MM-DD"`
	Last      string `json:"last_played" jsonschema:"YYYY-MM-DD"`
}

type teamDetailOut struct {
	teamOut
	Roster []rosterOut `json:"roster" jsonschema:"players who played for the team, most recent first"`
}

type updateTeamIn struct {
	TeamID  string    `json:"team_id" jsonschema:"team id from list_teams"`
	Name    string    `json:"name,omitempty" jsonschema:"new team name; empty keeps it"`
	Aliases *[]string `json:"aliases,omitempty" jsonschema:"the full new alias list (clan names that resolve to this team): start from get_team and keep the ones still used, or demos under a dropped clan name make a new team; omit to keep it"`
}

type mergeTeamsIn struct {
	TeamID     string `json:"team_id" jsonschema:"the team to fold in; it disappears"`
	IntoTeamID string `json:"into_team_id" jsonschema:"the team that keeps its id and gets every match, alias and player"`
	Approve    string `json:"approve,omitempty" jsonschema:"only when asked: the approval code from a previous call, after the user agreed"`
}

type mergeTeamsOut struct {
	NeedsApproval bool     `json:"needs_approval,omitempty"`
	ApprovalCode  string   `json:"approval_code,omitempty"`
	Summary       string   `json:"summary"`
	Team          *teamOut `json:"team,omitempty"`
}

type assignTeamIn struct {
	MatchID  string `json:"match_id" jsonschema:"match id from list_matches"`
	Side     string `json:"side" jsonschema:"the side key from get_match: ct-start or t-start"`
	TeamID   string `json:"team_id,omitempty" jsonschema:"existing team id from list_teams"`
	TeamName string `json:"team_name,omitempty" jsonschema:"team name or alias; a new team is created when none has it"`
	Clear    bool   `json:"clear,omitempty" jsonschema:"true removes the side's team; set neither team_id nor team_name"`
	// RememberAlias makes later demos with the same clan name pick this team.
	RememberAlias bool `json:"remember_alias,omitempty" jsonschema:"true adds the side's clan name to the team's aliases, so later demos resolve to it"`
}

type sideOut struct {
	Key        string `json:"key" jsonschema:"ct-start or t-start, the side argument of assign_match_team"`
	StartSide  string `json:"start_side"`
	ClanName   string `json:"clan_name,omitempty" jsonschema:"untrusted clan name from the demo"`
	TeamID     string `json:"team_id,omitempty"`
	TeamName   string `json:"team_name,omitempty" jsonschema:"untrusted team name"`
	Assignment string `json:"assignment" jsonschema:"auto: from the clan name; manual: set by the user and kept on re-analysis; none"`
}

type assignTeamOut struct {
	MatchID string    `json:"match_id"`
	Sides   []sideOut `json:"sides"`
}

func registerTeamTools(server *mcp.Server, s *studio) {
	mcp.AddTool(server, &mcp.Tool{Name: "get_team", Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true},
		Description: "Read one team with its aliases and its roster: who played for it, in how many matches and between which dates."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in getTeamIn) (*mcp.CallToolResult, teamDetailOut, error) {
			if err := checkUUID("team_id", in.TeamID); err != nil {
				return nil, teamDetailOut{}, err
			}
			c, err := s.client(ctx)
			if err != nil {
				return nil, teamDetailOut{}, toolError(err)
			}
			t, err := c.GetLibraryTeam(ctx, in.TeamID)
			if err != nil {
				return nil, teamDetailOut{}, toolError(err)
			}
			out := teamDetailOut{teamOut: teamOf(t.LibraryTeam), Roster: []rosterOut{}}
			for _, r := range t.Roster {
				out.Roster = append(out.Roster, rosterOut{SteamID64: cleanName(r.SteamID64), Nick: cleanName(r.Nick), Matches: r.Matches,
					First: r.FirstPlayedAt.Format(time.DateOnly), Last: r.LastPlayedAt.Format(time.DateOnly)})
			}
			return nil, out, nil
		})

	mcp.AddTool(server, &mcp.Tool{Name: "update_team", Annotations: &mcp.ToolAnnotations{IdempotentHint: true},
		Description: "Rename a team or replace its aliases, as the team page does. Aliases are the clan names that resolve to the team; " +
			"matches whose clan name becomes an alias follow it, and a demo whose clan name is no alias makes a new team. " +
			"An alias another team owns is refused; a name is not checked, so two teams can share one."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in updateTeamIn) (*mcp.CallToolResult, teamOut, error) {
			patch, err := in.patch()
			if err != nil {
				return nil, teamOut{}, err
			}
			c, err := s.client(ctx)
			if err != nil {
				return nil, teamOut{}, toolError(err)
			}
			t, err := c.UpdateLibraryTeam(ctx, in.TeamID, patch)
			if err != nil {
				return nil, teamOut{}, toolError(err)
			}
			return nil, teamOf(t), nil
		})

	mcp.AddTool(server, &mcp.Tool{Name: "merge_teams", Annotations: &mcp.ToolAnnotations{DestructiveHint: new(true)},
		Description: "Merge two teams that are the same, as Fusionar does on the team page: team_id is folded into into_team_id with its " +
			"matches, aliases and players, and disappears. It cannot be undone, so the user must approve it. " + deleteApprovalHint},
		func(ctx context.Context, req *mcp.CallToolRequest, in mergeTeamsIn) (*mcp.CallToolResult, mergeTeamsOut, error) {
			if err := checkUUID("team_id", in.TeamID); err != nil {
				return nil, mergeTeamsOut{}, err
			}
			if err := checkUUID("into_team_id", in.IntoTeamID); err != nil {
				return nil, mergeTeamsOut{}, err
			}
			if in.TeamID == in.IntoTeamID {
				return nil, mergeTeamsOut{}, errors.New("team_id and into_team_id are the same team")
			}
			c, err := s.client(ctx)
			if err != nil {
				return nil, mergeTeamsOut{}, toolError(err)
			}
			from, err := c.GetLibraryTeam(ctx, in.TeamID)
			if err != nil {
				return nil, mergeTeamsOut{}, toolError(err)
			}
			into, err := c.GetLibraryTeam(ctx, in.IntoTeamID)
			if err != nil {
				return nil, mergeTeamsOut{}, toolError(err)
			}
			fromName, intoName := quotedName(from.Name, "sin nombre"), quotedName(into.Name, "sin nombre")
			summary := fmt.Sprintf("Fusionar el equipo %s (%s) en %s. %s desaparece y no se puede deshacer.",
				fromName, plural(from.MatchCount, "partida", "partidas"), intoName, fromName)
			spec := approvalSpec{Tool: "merge_teams", JobID: in.TeamID, Request: in.IntoTeamID, Summary: summary, Given: in.Approve, Question: "¿Fusionar ahora?"}
			ok, err := sessionApprovals.approve(req, spec)
			if err != nil {
				return nil, mergeTeamsOut{}, err
			}
			if ok.Ask != nil {
				return ok.Ask, mergeTeamsOut{}, nil
			}
			if !ok.Approved {
				return nil, mergeTeamsOut{NeedsApproval: true, ApprovalCode: ok.Code, Summary: summary}, nil
			}
			merged, err := c.MergeLibraryTeam(ctx, in.TeamID, in.IntoTeamID)
			if err != nil {
				return nil, mergeTeamsOut{}, toolError(err)
			}
			team := teamOf(merged)
			return nil, mergeTeamsOut{Summary: summary, Team: &team}, nil
		})

	mcp.AddTool(server, &mcp.Tool{Name: "assign_match_team", Annotations: &mcp.ToolAnnotations{IdempotentHint: true},
		Description: "Correct which team played one side of a match, as the match summary does. Pick an existing team_id, a team_name " +
			"(found by name or alias, created when new) or clear. A manual side survives re-analysis. Read the side keys with get_match."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in assignTeamIn) (*mcp.CallToolResult, assignTeamOut, error) {
			a, err := in.assignment()
			if err != nil {
				return nil, assignTeamOut{}, err
			}
			c, err := s.client(ctx)
			if err != nil {
				return nil, assignTeamOut{}, toolError(err)
			}
			m, err := c.AssignMatchTeam(ctx, in.MatchID, in.Side, a)
			if err != nil {
				return nil, assignTeamOut{}, toolError(err)
			}
			return nil, assignTeamOut{MatchID: m.ID, Sides: sidesOf(m)}, nil
		})
}

func (in updateTeamIn) patch() (tuiclient.TeamPatch, error) {
	var patch tuiclient.TeamPatch
	if err := checkUUID("team_id", in.TeamID); err != nil {
		return patch, err
	}
	if in.Name == "" && in.Aliases == nil {
		return patch, errors.New("set name, aliases or both")
	}
	if in.Name != "" {
		name, err := checkUserText("name", in.Name)
		if err != nil {
			return patch, err
		}
		patch.Name = &name
	}
	if in.Aliases != nil {
		if len(*in.Aliases) > maxTeamAliases {
			return patch, fmt.Errorf("at most %d aliases", maxTeamAliases)
		}
		aliases := make([]string, 0, len(*in.Aliases))
		for _, raw := range *in.Aliases {
			alias, err := checkUserText("alias", raw)
			if err != nil {
				return patch, err
			}
			aliases = append(aliases, alias)
		}
		patch.Aliases = &aliases
	}
	return patch, nil
}

func (in assignTeamIn) assignment() (tuiclient.MatchTeamAssignment, error) {
	var a tuiclient.MatchTeamAssignment
	if err := checkUUID("match_id", in.MatchID); err != nil {
		return a, err
	}
	if in.Side != "ct-start" && in.Side != "t-start" {
		return a, fmt.Errorf("side must be ct-start or t-start, not %q", cleanName(in.Side))
	}
	set := 0
	for _, given := range []bool{in.TeamID != "", in.TeamName != "", in.Clear} {
		if given {
			set++
		}
	}
	if set != 1 {
		return a, errors.New("set exactly one of team_id, team_name or clear")
	}
	a.RememberAlias = in.RememberAlias
	switch {
	case in.TeamID != "":
		if err := checkUUID("team_id", in.TeamID); err != nil {
			return a, err
		}
		a.TeamID = &in.TeamID
	case in.TeamName != "":
		name, err := checkUserText("team_name", in.TeamName)
		if err != nil {
			return a, err
		}
		a.TeamName = name
	}
	return a, nil
}

// checkUserText trims a name or title the user gave and refuses empty, long or control text.
func checkUserText(field, raw string) (string, error) {
	text := strings.TrimSpace(raw)
	if text == "" {
		return "", fmt.Errorf("%s must not be empty", field)
	}
	if utf8.RuneCountInString(text) > maxTeamNameRunes {
		return "", fmt.Errorf("%s takes at most %d characters", field, maxTeamNameRunes)
	}
	if strings.IndexFunc(text, unicode.IsControl) >= 0 {
		return "", fmt.Errorf("%s must not hold control characters", field)
	}
	return text, nil
}

func teamOf(t tuiclient.LibraryTeam) teamOut {
	return teamOut{ID: t.ID, Name: cleanName(t.Name), Aliases: cleanNames(t.Aliases), Matches: t.MatchCount}
}

func sidesOf(m tuiclient.LibraryMatch) []sideOut {
	out := make([]sideOut, 0, len(m.Teams))
	for _, t := range m.Teams {
		out = append(out, sideOut{Key: t.Key, StartSide: t.StartSide, ClanName: cleanName(t.ClanName), TeamID: t.TeamID,
			TeamName: cleanName(t.TeamName), Assignment: t.Assignment})
	}
	return out
}
