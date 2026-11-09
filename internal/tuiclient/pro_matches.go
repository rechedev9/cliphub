package tuiclient

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/community"
	"github.com/rechedev9/cliphub/internal/library"
)

// ProMatchTeam is one team of a pro match; Followed is true when Studio imports its new matches.
type ProMatchTeam struct {
	community.ProTeam
	Followed bool `json:"followed"`
}

// ProMatchMap is one analysed map of a pro match; LibraryMatchID is set once it is imported.
type ProMatchMap struct {
	community.ProMapFile
	LibraryMatchID *uuid.UUID `json:"library_match_id,omitempty"`
	RadarMayBeOff  bool       `json:"radar_may_be_off"`
}

// ProMatch is one HLTV match of the community catalog.
type ProMatch struct {
	ID       int            `json:"id"`
	Event    string         `json:"event"`
	PlayedAt time.Time      `json:"played_at"`
	Teams    []ProMatchTeam `json:"teams"`
	Maps     []ProMatchMap  `json:"maps"`
}

// ProMatches is one page of the catalog. Status is ok, disabled (no community
// server), missing (the server has no catalog yet) or unavailable.
type ProMatches struct {
	Status    string              `json:"status"`
	UpdatedAt *time.Time          `json:"updated_at,omitempty"`
	Days      int                 `json:"days"`
	Page      int                 `json:"page"`
	Pages     int                 `json:"pages"`
	Total     int                 `json:"total"`
	Matches   []ProMatch          `json:"matches"`
	Facets    community.ProFacets `json:"facets"`
	Follows   []library.ProFollow `json:"follows"`
}

// ProImport is the library match a pro map became.
type ProImport struct {
	MatchID       uuid.UUID `json:"match_id"`
	DemoID        uuid.UUID `json:"demo_id"`
	Created       bool      `json:"created"`
	StudioPath    string    `json:"studio_path"`
	RadarMayBeOff bool      `json:"radar_may_be_off"`
}

// ListProMatches reads one page of the pro match catalog.
func (c *Client) ListProMatches(ctx context.Context, q community.ProMatchQuery) (ProMatches, error) {
	path := "/api/library/pro-matches"
	if v := q.Values(); len(v) > 0 {
		path += "?" + v.Encode()
	}
	var out ProMatches
	err := c.getJSON(ctx, path, &out)
	return out, err
}

// ImportProMatch stores one pro map as a library match, or returns the one stored before.
func (c *Client) ImportProMatch(ctx context.Context, matchID int, entry string) (ProImport, error) {
	var out ProImport
	err := c.doJSON(ctx, http.MethodPost, "/api/library/pro-matches/"+strconv.Itoa(matchID)+"/"+url.PathEscape(entry)+"/import", nil, &out)
	return out, err
}

// ListProFollows lists the HLTV teams Studio follows.
func (c *Client) ListProFollows(ctx context.Context) ([]library.ProFollow, error) {
	var out struct {
		Follows []library.ProFollow `json:"follows"`
	}
	err := c.getJSON(ctx, "/api/library/pro-follows", &out)
	return out.Follows, err
}

// FollowProTeam starts importing an HLTV team's pro matches.
func (c *Client) FollowProTeam(ctx context.Context, teamID int, name string) (library.ProFollow, error) {
	var out library.ProFollow
	err := c.doJSON(ctx, http.MethodPost, "/api/library/pro-follows", map[string]any{"team_id": teamID, "name": name}, &out)
	return out, err
}

// UnfollowProTeam stops importing a team's new pro matches.
func (c *Client) UnfollowProTeam(ctx context.Context, teamID int) error {
	return c.doJSON(ctx, http.MethodDelete, "/api/library/pro-follows/"+strconv.Itoa(teamID), nil, nil)
}
