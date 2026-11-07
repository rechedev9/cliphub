package main

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/rechedev9/cliphub/internal/tuiclient"
)

// The server's caps (internal/library/roles.go).
const (
	maxRoleSlots     = 5
	maxRoleNameRunes = 40
	maxRoleTaskRunes = 200
	maxRoleCallout   = 64
)

type roleIn struct {
	Name      string `json:"name" jsonschema:"the role, such as Entry, AWP or Ancla B"`
	SteamID64 string `json:"steamid64,omitempty" jsonschema:"the player who plays it, from get_team's roster"`
	Callout   string `json:"callout,omitempty" jsonschema:"where they play, a CS2 place name such as BombsiteB"`
	Task      string `json:"task,omitempty" jsonschema:"what they do, in a sentence"`
}

type roleOut struct {
	Name      string `json:"name" jsonschema:"untrusted text the user wrote"`
	SteamID64 string `json:"steamid64,omitempty"`
	Callout   string `json:"callout,omitempty"`
	Task      string `json:"task,omitempty" jsonschema:"untrusted text the user wrote"`
}

type teamRolesIn struct {
	TeamID string `json:"team_id" jsonschema:"team id from list_teams"`
	Map    string `json:"map" jsonschema:"such as de_mirage"`
}

type setTeamRolesIn struct {
	teamRolesIn
	T  []roleIn `json:"t" jsonschema:"up to 5 T roles, the whole side; [] clears it"`
	CT []roleIn `json:"ct" jsonschema:"up to 5 CT roles, the whole side; [] clears it"`
}

type roleSuggestionOut struct {
	SteamID64 string `json:"steamid64"`
	Player    string `json:"player"`
	Role      string `json:"role" jsonschema:"awp, entry, support, rifler or unknown on T; awp, anchor or rotator on CT"`
	Callout   string `json:"callout,omitempty" jsonschema:"CT only: where the player sets up most"`
	Rounds    int    `json:"rounds"`
}

type teamRolesOut struct {
	TeamName    string              `json:"team_name"`
	Map         string              `json:"map"`
	T           []roleOut           `json:"t" jsonschema:"the saved T sheet"`
	CT          []roleOut           `json:"ct" jsonschema:"the saved CT sheet"`
	SuggestedT  []roleSuggestionOut `json:"suggested_t" jsonschema:"read from the team's matches on the map"`
	SuggestedCT []roleSuggestionOut `json:"suggested_ct"`
	Matches     int                 `json:"matches" jsonschema:"matches the suggestion read"`
}

type savedRolesOut struct {
	Map string    `json:"map"`
	T   []roleOut `json:"t"`
	CT  []roleOut `json:"ct"`
}

func registerRoleTools(server *mcp.Server, s *studio) {
	mcp.AddTool(server, &mcp.Tool{Name: "get_team_roles", Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true},
		Description: "Read a team's role sheet on one map (the Libreta's Roles tab): who plays what on T and CT, and Studio's suggestion from the team's matches there."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in teamRolesIn) (*mcp.CallToolResult, teamRolesOut, error) {
			if err := in.check(); err != nil {
				return nil, teamRolesOut{}, err
			}
			c, err := s.client(ctx)
			if err != nil {
				return nil, teamRolesOut{}, toolError(err)
			}
			roles, err := c.GetTeamRoles(ctx, in.TeamID, strings.ToLower(in.Map))
			if err != nil {
				return nil, teamRolesOut{}, toolError(err)
			}
			return nil, teamRolesOut{TeamName: cleanName(roles.TeamName), Map: cleanName(roles.Sheet.Map), T: rolesOf(roles.Sheet.T), CT: rolesOf(roles.Sheet.CT),
				SuggestedT: suggestionsOf(roles.Suggested.T), SuggestedCT: suggestionsOf(roles.Suggested.CT), Matches: roles.Matches}, nil
		})

	mcp.AddTool(server, &mcp.Tool{Name: "set_team_roles", Annotations: &mcp.ToolAnnotations{IdempotentHint: true},
		Description: "Save a team's role sheet on one map, both sides at once, as the Roles tab does. It replaces the sheet, so start from get_team_roles."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in setTeamRolesIn) (*mcp.CallToolResult, savedRolesOut, error) {
			if err := in.check(); err != nil {
				return nil, savedRolesOut{}, err
			}
			t, err := checkRoles("t", in.T)
			if err != nil {
				return nil, savedRolesOut{}, err
			}
			ct, err := checkRoles("ct", in.CT)
			if err != nil {
				return nil, savedRolesOut{}, err
			}
			c, err := s.client(ctx)
			if err != nil {
				return nil, savedRolesOut{}, toolError(err)
			}
			saved, err := c.SetTeamRoles(ctx, in.TeamID, tuiclient.TeamRoleSheet{Map: strings.ToLower(in.Map), T: t, CT: ct})
			if err != nil {
				return nil, savedRolesOut{}, toolError(err)
			}
			return nil, savedRolesOut{Map: cleanName(saved.Map), T: rolesOf(saved.T), CT: rolesOf(saved.CT)}, nil
		})
}

func (in teamRolesIn) check() error {
	if err := checkUUID("team_id", in.TeamID); err != nil {
		return err
	}
	if !mapNamePattern.MatchString(in.Map) {
		return errors.New("map must be a map name such as de_mirage")
	}
	return nil
}

// checkRoles trims each role and refuses what the server would.
func checkRoles(field string, in []roleIn) ([]tuiclient.RoleSlot, error) {
	if len(in) > maxRoleSlots {
		return nil, fmt.Errorf("%s takes at most %d roles", field, maxRoleSlots)
	}
	out := make([]tuiclient.RoleSlot, 0, len(in))
	for i, r := range in {
		slot := tuiclient.RoleSlot{Name: strings.TrimSpace(r.Name), SteamID64: strings.TrimSpace(r.SteamID64),
			Callout: strings.TrimSpace(r.Callout), Task: strings.TrimSpace(r.Task)}
		switch {
		case slot.Name == "" || utf8.RuneCountInString(slot.Name) > maxRoleNameRunes:
			return nil, fmt.Errorf("%s role %d: name takes 1 to %d characters", field, i+1, maxRoleNameRunes)
		case utf8.RuneCountInString(slot.Task) > maxRoleTaskRunes || utf8.RuneCountInString(slot.Callout) > maxRoleCallout:
			return nil, fmt.Errorf("%s role %d: task takes at most %d characters, callout %d", field, i+1, maxRoleTaskRunes, maxRoleCallout)
		case slot.SteamID64 != "" && !steamIDPattern.MatchString(slot.SteamID64):
			return nil, fmt.Errorf("%s role %d: steamid64 must be a SteamID64", field, i+1)
		case strings.ContainsFunc(slot.Name+slot.Callout+slot.Task, unicode.IsControl):
			return nil, fmt.Errorf("%s role %d must not hold control characters", field, i+1)
		}
		out = append(out, slot)
	}
	return out, nil
}

func rolesOf(roles []tuiclient.RoleSlot) []roleOut {
	out := make([]roleOut, 0, len(roles))
	for _, r := range roles {
		out = append(out, roleOut{Name: cleanName(r.Name), SteamID64: cleanName(r.SteamID64), Callout: cleanName(r.Callout),
			Task: cleanText(r.Task, maxRoleTaskRunes)})
	}
	return out
}

func suggestionsOf(in []tuiclient.RoleSuggestion) []roleSuggestionOut {
	out := make([]roleSuggestionOut, 0, len(in))
	for _, s := range in {
		out = append(out, roleSuggestionOut{SteamID64: cleanName(s.SteamID64), Player: cleanName(s.Player), Role: cleanName(s.Role),
			Callout: cleanName(s.Callout), Rounds: s.Rounds})
	}
	return out
}
