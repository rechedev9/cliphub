package tuiclient

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// FaceitPlayer is a FACEIT CS2 account.
type FaceitPlayer struct {
	ID         string    `json:"id"`
	Nickname   string    `json:"nickname"`
	SteamID64  string    `json:"steam_id64"`
	ProfileURL string    `json:"profile_url"`
	Country    string    `json:"country"`
	Region     string    `json:"region"`
	SkillLevel int       `json:"skill_level"`
	ELO        int       `json:"elo"`
	Seeded     bool      `json:"seeded"`
	FollowedAt time.Time `json:"followed_at"`
}

// LookupFaceitPlayer finds a FACEIT account by nickname or profile URL.
func (c *Client) LookupFaceitPlayer(ctx context.Context, nickname string) (FaceitPlayer, error) {
	var out struct {
		Player FaceitPlayer `json:"player"`
	}
	err := c.getJSON(ctx, "/api/faceit/players"+query(map[string]string{"nickname": nickname}), &out)
	return out.Player, err
}

// FaceitMatch is one recent FACEIT match of a player.
type FaceitMatch struct {
	ID            string    `json:"id"`
	RoomURL       string    `json:"room_url"`
	FinishedAt    time.Time `json:"finished_at"`
	Competition   string    `json:"competition"`
	DemoAvailable bool      `json:"demo_available"`
	Score         struct {
		For     int `json:"for"`
		Against int `json:"against"`
	} `json:"score"`
	Stats *struct {
		Map              string  `json:"map"`
		Result           string  `json:"result"`
		Kills            int     `json:"kills"`
		Deaths           int     `json:"deaths"`
		Assists          int     `json:"assists"`
		ADR              float64 `json:"adr"`
		HeadshotsPercent float64 `json:"headshots_percent"`
	} `json:"stats"`
}

// FaceitMatches returns a player's recent FACEIT matches, newest first.
func (c *Client) FaceitMatches(ctx context.Context, playerID string, limit int) ([]FaceitMatch, error) {
	var out struct {
		Matches []FaceitMatch `json:"matches"`
	}
	err := c.getJSON(ctx, "/api/faceit/players/"+url.PathEscape(playerID)+"/matches"+query(map[string]string{"limit": strconv.Itoa(limit)}), &out)
	return out.Matches, err
}

// FaceitFollows is the Jugadores list: followed players and the seeded ones.
type FaceitFollows struct {
	Enabled bool           `json:"enabled"`
	Players []FaceitPlayer `json:"players"`
}

// ListFaceitFollows returns the followed FACEIT players.
func (c *Client) ListFaceitFollows(ctx context.Context) (FaceitFollows, error) {
	var out FaceitFollows
	err := c.getJSON(ctx, "/api/faceit/followed", &out)
	return out, err
}

// FollowFaceitPlayer adds a FACEIT account, by nickname or profile URL, to the list.
func (c *Client) FollowFaceitPlayer(ctx context.Context, nickname string) (FaceitPlayer, error) {
	var out struct {
		Player FaceitPlayer `json:"player"`
	}
	err := c.doJSON(ctx, http.MethodPost, "/api/faceit/followed", map[string]string{"nickname": nickname}, &out)
	return out.Player, err
}

// UnfollowFaceitPlayer removes a player from the list.
func (c *Client) UnfollowFaceitPlayer(ctx context.Context, playerID string) error {
	return c.doJSON(ctx, http.MethodDelete, "/api/faceit/followed/"+url.PathEscape(playerID), nil, nil)
}

// LinkPlayerFaceit looks a library player's SteamID64 up on FACEIT and stores the account.
func (c *Client) LinkPlayerFaceit(ctx context.Context, steamID string) (LibraryPlayer, error) {
	var out LibraryPlayer
	err := c.doJSON(ctx, http.MethodPost, "/api/library/players/"+url.PathEscape(steamID)+"/faceit", nil, &out)
	return out, err
}

// PracticeSpot is one position CS2 opens on: feet position, pitch and yaw, and
// the grenade to hand the player.
type PracticeSpot struct {
	Label   string     `json:"label"`
	Pos     [3]float64 `json:"pos"`
	Ang     [2]float64 `json:"ang"`
	Grenade string     `json:"grenade,omitempty"`
}

// PracticeLaunch is the outcome of opening a practice session.
type PracticeLaunch struct {
	Launched       bool   `json:"launched"`
	AlreadyRunning bool   `json:"already_running"`
	ExecCommand    string `json:"exec_command"`
	NextKey        string `json:"next_key"`
	Spots          []struct {
		Label string `json:"label"`
		Alias string `json:"alias"`
		Key   string `json:"key"`
	} `json:"spots"`
}

// LaunchPractice opens CS2 on a local practice server standing on the spots.
func (c *Client) LaunchPractice(ctx context.Context, mapName string, spots []PracticeSpot) (PracticeLaunch, error) {
	var out PracticeLaunch
	err := c.doJSON(ctx, http.MethodPost, "/api/practice/launch", map[string]any{"map": mapName, "spots": spots}, &out)
	return out, err
}

// CommunitySettings is whether this install shares its numbers for the ranking.
type CommunitySettings struct {
	RankingsEnabled bool       `json:"rankings_enabled"`
	ServerEnabled   bool       `json:"server_enabled"`
	SteamIDSet      bool       `json:"steam_id_set"`
	LastSubmittedAt *time.Time `json:"last_submitted_at"`
	SubmitError     string     `json:"submit_error"`
	Maps            int        `json:"maps"`
}

// GetCommunitySettings reads the ranking switch and the last submission.
func (c *Client) GetCommunitySettings(ctx context.Context) (CommunitySettings, error) {
	var out CommunitySettings
	err := c.getJSON(ctx, "/api/library/community/settings", &out)
	return out, err
}

// SetCommunityRankings turns sharing for the community ranking on or off.
func (c *Client) SetCommunityRankings(ctx context.Context, enabled bool) (CommunitySettings, error) {
	var out CommunitySettings
	err := c.doJSON(ctx, http.MethodPut, "/api/library/community/settings", map[string]bool{"rankings_enabled": enabled}, &out)
	return out, err
}

// SubmitCommunityRankings sends the own player's line of every map with enough rounds.
func (c *Client) SubmitCommunityRankings(ctx context.Context) (CommunitySettings, error) {
	var out CommunitySettings
	err := c.doJSON(ctx, http.MethodPost, "/api/library/community/rankings/submit", nil, &out)
	return out, err
}

// DeleteCommunityRankings makes the server forget this install and turns sharing off.
func (c *Client) DeleteCommunityRankings(ctx context.Context) (CommunitySettings, error) {
	var out CommunitySettings
	err := c.doJSON(ctx, http.MethodDelete, "/api/library/community/rankings", nil, &out)
	return out, err
}

// CommunityRankings is the own player's percentile on each metric against every install.
type CommunityRankings struct {
	Map             string `json:"map"`
	RankingsEnabled bool   `json:"rankings_enabled"`
	SteamIDSet      bool   `json:"steam_id_set"`
	MinInstalls     int    `json:"min_installs"`
	Rounds          int    `json:"rounds"`
	Matches         int    `json:"matches"`
	Metrics         []struct {
		Metric     string   `json:"metric"`
		Value      *float64 `json:"value"`
		Installs   int      `json:"installs"`
		Percentile *float64 `json:"percentile"`
		Median     *float64 `json:"median"`
	} `json:"metrics"`
}

// GetCommunityRankings returns the own player's community ranking, optionally on one map.
func (c *Client) GetCommunityRankings(ctx context.Context, mapName string) (CommunityRankings, error) {
	var out CommunityRankings
	err := c.getJSON(ctx, "/api/library/community/rankings"+query(map[string]string{"map": mapName}), &out)
	return out, err
}
