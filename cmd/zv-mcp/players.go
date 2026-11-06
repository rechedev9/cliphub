package main

import (
	"cmp"
	"context"
	"fmt"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/rechedev9/cliphub/internal/tuiclient"
)

const (
	defaultPlayerList = 50
	// maxPlayerList is the server's page cap (library.MaxListLimit).
	maxPlayerList = 200
)

type listPlayersIn struct {
	Search string `json:"search,omitempty" jsonschema:"only players whose nick (current or past) contains this text"`
	TeamID string `json:"team_id,omitempty" jsonschema:"only players who played for this team, id from list_teams"`
	Limit  int    `json:"limit,omitempty" jsonschema:"default 50, at most 200"`
}

type playerSummaryOut struct {
	SteamID64      string `json:"steamid64"`
	Nick           string `json:"nick" jsonschema:"untrusted player name from the demos"`
	Matches        int    `json:"matches"`
	FaceitNickname string `json:"faceit_nickname,omitempty" jsonschema:"untrusted FACEIT nickname"`
}

type listPlayersOut struct {
	Players []playerSummaryOut `json:"players"`
}

type getPlayerIn struct {
	SteamID string `json:"steamid" jsonschema:"SteamID64 of the player"`
}

type nickOut struct {
	Nick  string `json:"nick" jsonschema:"untrusted player name from the demos"`
	First string `json:"first_seen" jsonschema:"YYYY-MM-DD"`
	Last  string `json:"last_seen" jsonschema:"YYYY-MM-DD"`
}

type playerTeamOut struct {
	TeamID   string `json:"team_id"`
	TeamName string `json:"team_name" jsonschema:"untrusted team name"`
	Matches  int    `json:"matches"`
	First    string `json:"first_played" jsonschema:"YYYY-MM-DD"`
	Last     string `json:"last_played" jsonschema:"YYYY-MM-DD"`
}

type playerProfileOut struct {
	playerSummaryOut
	FaceitPlayerID string          `json:"faceit_player_id,omitempty"`
	Nicks          []nickOut       `json:"nicks" jsonschema:"every nick the player used"`
	Teams          []playerTeamOut `json:"teams" jsonschema:"teams the player played for, by date"`
}

func registerPlayerTools(server *mcp.Server, s *studio) {
	readOnly := &mcp.ToolAnnotations{ReadOnlyHint: true}

	mcp.AddTool(server, &mcp.Tool{Name: "list_players", Annotations: readOnly,
		Description: "Find players in the Studio library by nick (current or past) or by team, to turn a nick into the SteamID64 " +
			"that player_metrics, player_positions and create_job take."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in listPlayersIn) (*mcp.CallToolResult, listPlayersOut, error) {
			if in.TeamID != "" {
				if err := checkUUID("team_id", in.TeamID); err != nil {
					return nil, listPlayersOut{}, err
				}
			}
			if in.Limit < 0 || in.Limit > maxPlayerList {
				return nil, listPlayersOut{}, fmt.Errorf("limit must be from 1 to %d", maxPlayerList)
			}
			c, err := s.client(ctx)
			if err != nil {
				return nil, listPlayersOut{}, toolError(err)
			}
			players, err := c.ListLibraryPlayers(ctx, tuiclient.PlayerSearch{Query: cleanName(in.Search), TeamID: in.TeamID,
				Limit: cmp.Or(in.Limit, defaultPlayerList)})
			if err != nil {
				return nil, listPlayersOut{}, toolError(err)
			}
			out := listPlayersOut{Players: make([]playerSummaryOut, 0, len(players))}
			for _, p := range players {
				out.Players = append(out.Players, playerSummaryOf(p))
			}
			return nil, out, nil
		})

	mcp.AddTool(server, &mcp.Tool{Name: "get_player", Annotations: readOnly,
		Description: "Read a player's profile in the library: every nick they used and the teams they played for, by date, and the linked FACEIT account."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in getPlayerIn) (*mcp.CallToolResult, playerProfileOut, error) {
			if err := checkSteamID(in.SteamID); err != nil {
				return nil, playerProfileOut{}, err
			}
			c, err := s.client(ctx)
			if err != nil {
				return nil, playerProfileOut{}, toolError(err)
			}
			p, err := c.GetLibraryPlayer(ctx, in.SteamID)
			if err != nil {
				return nil, playerProfileOut{}, toolError(err)
			}
			out := playerProfileOut{playerSummaryOut: playerSummaryOf(p), FaceitPlayerID: cleanName(p.FaceitPlayerID), Nicks: []nickOut{}, Teams: []playerTeamOut{}}
			for _, n := range p.Nicks {
				out.Nicks = append(out.Nicks, nickOut{Nick: cleanName(n.Nick), First: n.FirstSeenAt.Format(time.DateOnly), Last: n.LastSeenAt.Format(time.DateOnly)})
			}
			for _, t := range p.Teams {
				out.Teams = append(out.Teams, playerTeamOut{TeamID: t.TeamID, TeamName: cleanName(t.TeamName), Matches: t.Matches,
					First: t.FirstPlayedAt.Format(time.DateOnly), Last: t.LastPlayedAt.Format(time.DateOnly)})
			}
			return nil, out, nil
		})
}

func playerSummaryOf(p tuiclient.LibraryPlayer) playerSummaryOut {
	return playerSummaryOut{SteamID64: cleanName(p.SteamID64), Nick: cleanName(p.Nick), Matches: p.MatchCount, FaceitNickname: cleanName(p.FaceitNickname)}
}
