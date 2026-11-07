package tuiclient

import (
	"context"
	"net/http"
	"net/url"
)

// RoleSlot is one player's job in a play or on a team's role sheet.
type RoleSlot struct {
	Name      string `json:"name"`
	SteamID64 string `json:"steamid64,omitempty"`
	Callout   string `json:"callout,omitempty"`
	Task      string `json:"task,omitempty"`
}

// TeamRoleSheet is who does what for a team on one map, per side.
type TeamRoleSheet struct {
	Map string     `json:"map"`
	T   []RoleSlot `json:"t"`
	CT  []RoleSlot `json:"ct"`
}

// RoleSuggestion is a role Studio read from the team's matches on the map.
type RoleSuggestion struct {
	SteamID64 string `json:"steamid64"`
	Player    string `json:"player"`
	Role      string `json:"role"`
	Callout   string `json:"callout,omitempty"`
	Rounds    int    `json:"rounds"`
}

// TeamRoles is the saved sheet of a map and a suggestion for each side.
type TeamRoles struct {
	TeamName  string        `json:"team_name"`
	Sheet     TeamRoleSheet `json:"sheet"`
	Suggested struct {
		T  []RoleSuggestion `json:"t"`
		CT []RoleSuggestion `json:"ct"`
	} `json:"suggested"`
	Matches int `json:"matches"`
}

func teamRolesPath(teamID string) string { return "/api/library/teams/" + url.PathEscape(teamID) + "/roles" }

// GetTeamRoles reads a team's role sheet on a map with Studio's suggestion.
func (c *Client) GetTeamRoles(ctx context.Context, teamID, mapName string) (TeamRoles, error) {
	var out TeamRoles
	err := c.getJSON(ctx, teamRolesPath(teamID)+query(map[string]string{"map": mapName}), &out)
	return out, err
}

// SetTeamRoles replaces a team's role sheet on its map.
func (c *Client) SetTeamRoles(ctx context.Context, teamID string, sheet TeamRoleSheet) (TeamRoleSheet, error) {
	var out TeamRoleSheet
	err := c.doJSON(ctx, http.MethodPut, teamRolesPath(teamID), sheet, &out)
	return out, err
}
