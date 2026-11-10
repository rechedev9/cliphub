package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/rechedev9/cliphub/internal/tuiclient"
)

// maxPinnedPlayers is the server's cap (internal/library/pinned_players.go).
const maxPinnedPlayers = 100

type pinnedPlayerIn struct {
	SteamID64 string `json:"steamid64" jsonschema:"17 digit SteamID64, from list_players or a leaderboard row"`
	Name      string `json:"name,omitempty" jsonschema:"the nick shown while the player is not in the table"`
}

type setPinnedPlayersIn struct {
	Players []pinnedPlayerIn `json:"players" jsonschema:"the whole pinned list in table order, up to 100; [] clears it"`
}

type pinnedPlayerOut struct {
	SteamID64 string `json:"steamid64"`
	Name      string `json:"name" jsonschema:"untrusted nick"`
}

type pinnedPlayersOut struct {
	Players []pinnedPlayerOut `json:"players" jsonschema:"in table order, shown above the other rows of the Ranking"`
}

func registerPinnedPlayerTools(server *mcp.Server, s *studio) {
	mcp.AddTool(server, &mcp.Tool{Name: "list_pinned_players", Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true},
		Description: "Read the players pinned in the Ranking (Fijados): the ones it keeps on top of the table through every filter."},
		func(ctx context.Context, _ *mcp.CallToolRequest, _ empty) (*mcp.CallToolResult, pinnedPlayersOut, error) {
			c, err := s.client(ctx)
			if err != nil {
				return nil, pinnedPlayersOut{}, toolError(err)
			}
			players, err := c.GetPinnedPlayers(ctx)
			if err != nil {
				return nil, pinnedPlayersOut{}, toolError(err)
			}
			return nil, pinnedOf(players), nil
		})

	mcp.AddTool(server, &mcp.Tool{Name: "set_pinned_players", Annotations: &mcp.ToolAnnotations{IdempotentHint: true},
		Description: "Pin players in the Ranking, as the pin button does. It replaces the whole list, so start from list_pinned_players to add or remove one."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in setPinnedPlayersIn) (*mcp.CallToolResult, pinnedPlayersOut, error) {
			if len(in.Players) > maxPinnedPlayers {
				return nil, pinnedPlayersOut{}, fmt.Errorf("players takes at most %d players", maxPinnedPlayers)
			}
			players := make([]tuiclient.PinnedPlayer, 0, len(in.Players))
			for _, p := range in.Players {
				if err := checkSteamID(p.SteamID64); err != nil {
					return nil, pinnedPlayersOut{}, err
				}
				players = append(players, tuiclient.PinnedPlayer{SteamID64: p.SteamID64, Name: strings.TrimSpace(p.Name)})
			}
			c, err := s.client(ctx)
			if err != nil {
				return nil, pinnedPlayersOut{}, toolError(err)
			}
			saved, err := c.SetPinnedPlayers(ctx, players)
			if err != nil {
				return nil, pinnedPlayersOut{}, toolError(err)
			}
			return nil, pinnedOf(saved), nil
		})
}

func pinnedOf(players []tuiclient.PinnedPlayer) pinnedPlayersOut {
	out := pinnedPlayersOut{Players: make([]pinnedPlayerOut, 0, len(players))}
	for _, p := range players {
		out.Players = append(out.Players, pinnedPlayerOut{SteamID64: p.SteamID64, Name: cleanName(p.Name)})
	}
	return out
}
