package tuiclient

import (
	"context"
	"net/http"

	"github.com/rechedev9/cliphub/internal/library"
)

type PinnedPlayer = library.PinnedPlayer

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
