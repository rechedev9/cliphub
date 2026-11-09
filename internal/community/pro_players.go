package community

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"github.com/rechedev9/cliphub/internal/tacticalplan"
)

// ProPlayersDefaultLimit is how many players a board lists unless asked for fewer or more.
const ProPlayersDefaultLimit = 100

// maxProPlayersBytes bounds a board: 500 rows of a dozen numbers.
const maxProPlayersBytes = 4 << 20

// ProPlayersTops are the opponent levels the server offers: the rival is ranked
// at most this high in the HLTV world ranking.
var ProPlayersTops = []int{5, 15, 30, 50}

// ProPlayersQuery selects the board of GET /v1/pro-players; zero fields are unset.
type ProPlayersQuery struct {
	Map  string
	Side string
	Role string
	// MapRole keeps the players who hold this callout (or "allround") on Side; it needs Map and Side.
	MapRole string
	// Event keeps the maps of one event.
	Event string
	// Days keeps the maps played within this many days.
	Days int
	// Top keeps the lines against a rival ranked this high or better.
	Top int
	// TeamTop keeps the players whose own team is ranked this high or better.
	TeamTop   int
	Metric    string
	MinRounds int
	Limit     int
}

// ProPlayersBoard is the answer: the pro players of the server's maps, ranked.
type ProPlayersBoard struct {
	Map            string `json:"map"`
	Side           string `json:"side,omitempty"`
	Role           string `json:"role,omitempty"`
	MapRole        string `json:"map_role,omitempty"`
	Event          string `json:"event,omitempty"`
	TeamTop        int    `json:"team_top,omitempty"`
	Days           int    `json:"days,omitempty"`
	Top            int    `json:"top,omitempty"`
	Metric         string `json:"metric"`
	HigherIsBetter bool   `json:"higher_is_better"`
	MinRounds      int    `json:"min_rounds"`
	Maps           int    `json:"maps"`
	Rounds         int    `json:"rounds"`
	BuiltAt        string `json:"built_at,omitempty"`
	// Events are the events of the maps in the period, to offer in a filter.
	Events []string `json:"events"`
	// MapRoles are the map roles players have on Side, when Map and Side are set.
	MapRoles []tacticalplan.MapRoleOption  `json:"map_roles"`
	Players  []tacticalplan.LeaderboardRow `json:"players"`
}

// Values encodes the query for the server.
func (q ProPlayersQuery) Values() url.Values {
	v := url.Values{}
	set := func(key, value string) {
		if value != "" {
			v.Set(key, value)
		}
	}
	setInt := func(key string, n int) {
		if n > 0 {
			v.Set(key, strconv.Itoa(n))
		}
	}
	set("map", q.Map)
	set("side", q.Side)
	set("role", q.Role)
	set("map_role", q.MapRole)
	set("event", q.Event)
	set("metric", q.Metric)
	setInt("days", q.Days)
	setInt("top", q.Top)
	setInt("team_top", q.TeamTop)
	setInt("min_rounds", q.MinRounds)
	setInt("limit", q.Limit)
	return v
}

// ProPlayers asks the server for the board of pro players.
func (c *Client) ProPlayers(ctx context.Context, q ProPlayersQuery) (ProPlayersBoard, error) {
	path := "/v1/pro-players"
	if enc := q.Values().Encode(); enc != "" {
		path += "?" + enc
	}
	req, err := c.request(ctx, http.MethodGet, path, nil)
	if err != nil {
		return ProPlayersBoard{}, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return ProPlayersBoard{}, fmt.Errorf("community: pro players: %w", err)
	}
	defer resp.Body.Close()
	// A server that does not have the route yet has no pro players: an empty board.
	if resp.StatusCode == http.StatusNotFound {
		return ProPlayersBoard{Map: q.Map, Metric: q.Metric, Events: []string{}, MapRoles: []tacticalplan.MapRoleOption{}, Players: []tacticalplan.LeaderboardRow{}}, nil
	}
	body, err := readBody(resp, maxProPlayersBytes)
	if err != nil {
		return ProPlayersBoard{}, err
	}
	var board ProPlayersBoard
	if err := json.Unmarshal(body, &board); err != nil {
		return ProPlayersBoard{}, fmt.Errorf("community: decode pro players: %w", err)
	}
	return board, nil
}
