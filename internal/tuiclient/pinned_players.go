package tuiclient

import (
	"context"
	"net/http"
)

// PinnedPlayer is a player the Ranking keeps on top of its table.
type PinnedPlayer struct {
	SteamID64 string `json:"steamid64"`
	Name      string `json:"name"`
}

type pinnedPlayersBody struct {
	Players []PinnedPlayer `json:"players"`
}

const pinnedPlayersPath = "/api/library/pinned-players"

// GetPinnedPlayers reads the Ranking's pinned players in table order.
func (c *Client) GetPinnedPlayers(ctx context.Context) ([]PinnedPlayer, error) {
	var out pinnedPlayersBody
	err := c.getJSON(ctx, pinnedPlayersPath, &out)
	return out.Players, err
}

// SetPinnedPlayers replaces the whole pinned list and returns what Studio kept.
func (c *Client) SetPinnedPlayers(ctx context.Context, players []PinnedPlayer) ([]PinnedPlayer, error) {
	var out pinnedPlayersBody
	err := c.doJSON(ctx, http.MethodPut, pinnedPlayersPath, pinnedPlayersBody{Players: players}, &out)
	return out.Players, err
}
