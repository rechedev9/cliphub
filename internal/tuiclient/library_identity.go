package tuiclient

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// RosterEntry is one player who played for a team, by date.
type RosterEntry struct {
	SteamID64     string    `json:"steamid64"`
	Nick          string    `json:"nick"`
	FirstPlayedAt time.Time `json:"first_played_at"`
	LastPlayedAt  time.Time `json:"last_played_at"`
	Matches       int       `json:"matches"`
}

// LibraryTeamDetail is GET /api/library/teams/{id}: the team with its roster.
type LibraryTeamDetail struct {
	LibraryTeam
	Roster []RosterEntry `json:"roster"`
}

// PlayerNick is a name a player used, with the dates it was first and last seen.
type PlayerNick struct {
	Nick        string    `json:"nick"`
	FirstSeenAt time.Time `json:"first_seen_at"`
	LastSeenAt  time.Time `json:"last_seen_at"`
}

// PlayerTeam is a team a player played for, by date.
type PlayerTeam struct {
	TeamID        string    `json:"team_id"`
	TeamName      string    `json:"team_name"`
	FirstPlayedAt time.Time `json:"first_played_at"`
	LastPlayedAt  time.Time `json:"last_played_at"`
	Matches       int       `json:"matches"`
}

// LibraryPlayer is a player of the library; Nicks and Teams are present on single reads.
type LibraryPlayer struct {
	SteamID64      string       `json:"steamid64"`
	Nick           string       `json:"nick"`
	Nicks          []PlayerNick `json:"nicks"`
	FaceitPlayerID string       `json:"faceit_player_id"`
	FaceitNickname string       `json:"faceit_nickname"`
	MatchCount     int          `json:"match_count"`
	Teams          []PlayerTeam `json:"teams"`
}

// PlayerSearch filters GET /api/library/players; zero fields are unset.
type PlayerSearch struct {
	Query  string
	TeamID string
	Limit  int
}

// ListLibraryPlayers lists the library's players whose nick matches the query.
func (c *Client) ListLibraryPlayers(ctx context.Context, q PlayerSearch) ([]LibraryPlayer, error) {
	params := map[string]string{"q": q.Query, "team_id": q.TeamID}
	if q.Limit > 0 {
		params["limit"] = strconv.Itoa(q.Limit)
	}
	var out struct {
		Players []LibraryPlayer `json:"players"`
	}
	err := c.getJSON(ctx, "/api/library/players"+query(params), &out)
	return out.Players, err
}

// GetLibraryPlayer returns one player with the nick history and the teams by date.
func (c *Client) GetLibraryPlayer(ctx context.Context, steamID string) (LibraryPlayer, error) {
	var out LibraryPlayer
	err := c.getJSON(ctx, "/api/library/players/"+url.PathEscape(steamID), &out)
	return out, err
}

// GetLibraryTeam returns one team with its roster.
func (c *Client) GetLibraryTeam(ctx context.Context, id string) (LibraryTeamDetail, error) {
	var out LibraryTeamDetail
	err := c.getJSON(ctx, "/api/library/teams/"+url.PathEscape(id), &out)
	return out, err
}

// TeamPatch renames a team or replaces its aliases; nil fields stay unchanged.
type TeamPatch struct {
	Name    *string   `json:"name,omitempty"`
	Aliases *[]string `json:"aliases,omitempty"`
}

// UpdateLibraryTeam applies a rename or a new alias set.
func (c *Client) UpdateLibraryTeam(ctx context.Context, id string, patch TeamPatch) (LibraryTeam, error) {
	var out LibraryTeam
	err := c.doJSON(ctx, http.MethodPatch, "/api/library/teams/"+url.PathEscape(id), patch, &out)
	return out, err
}

// MergeLibraryTeam folds the team id into intoID and returns the merged team.
func (c *Client) MergeLibraryTeam(ctx context.Context, id, intoID string) (LibraryTeam, error) {
	var out LibraryTeam
	err := c.doJSON(ctx, http.MethodPost, "/api/library/teams/"+url.PathEscape(id)+"/merge", map[string]string{"into_team_id": intoID}, &out)
	return out, err
}

// MatchTeamAssignment picks the team of one side: TeamID, else TeamName (found
// by name or alias, created when none has it); neither clears the side.
type MatchTeamAssignment struct {
	TeamID        *string `json:"team_id"`
	TeamName      string  `json:"team_name,omitempty"`
	RememberAlias bool    `json:"remember_alias"`
}

// AssignMatchTeam sets the team of one side of a match; key is ct-start or t-start.
func (c *Client) AssignMatchTeam(ctx context.Context, matchID, key string, a MatchTeamAssignment) (LibraryMatch, error) {
	var out LibraryMatch
	err := c.doJSON(ctx, http.MethodPut, "/api/library/matches/"+url.PathEscape(matchID)+"/teams/"+url.PathEscape(key), a, &out)
	return out, err
}
