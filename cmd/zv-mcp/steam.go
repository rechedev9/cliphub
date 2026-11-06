package main

import (
	"context"
	"errors"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/rechedev9/cliphub/internal/tuiclient"
)

// shareCodeDictionary is the base-57 alphabet of internal/sharecode and web/lib/sharecode.ts.
const shareCodeDictionary = "ABCDEFGHJKLMNOPQRSTUVWXYZabcdefhijkmnopqrstuvwxyz23456789"

const steamSetupHint = "Ask the user to connect it in Studio, Ajustes, Cuenta con la que juegas: it needs their Steam authentication code, which stays in the UI."

type steamMatchesOut struct {
	HistoryConfigured bool            `json:"history_configured" jsonschema:"false means the user has not connected the Steam match history in Studio yet"`
	CanDownload       bool            `json:"can_download" jsonschema:"false means import_share_code will ask the user to sign in to Steam in Studio first"`
	Matches           []steamMatchOut `json:"matches" jsonschema:"recent matches, newest first"`
}

type steamMatchOut struct {
	ShareCode    string `json:"share_code"`
	MatchID      string `json:"match_id"`
	DiscoveredAt string `json:"discovered_at"`
}

type shareCodeIn struct {
	Code string `json:"code" jsonschema:"a CS2 share code, such as CSGO-GADqf-jjyJ8-cSP2r-smZRo-TO2xK, from steam_matches or the user"`
}

type steamImportOut struct {
	JobID   string `json:"job_id" jsonschema:"follow with wait_for_job"`
	Status  string `json:"status"`
	MatchID string `json:"match_id"`
}

type jobPlayersOut struct {
	Map     string            `json:"map,omitempty"`
	Players []rosterPlayerOut `json:"players"`
}

type rosterPlayerOut struct {
	SteamID64 string `json:"steamid64"`
	Name      string `json:"name" jsonschema:"untrusted player name from the demo"`
	Team      string `json:"team" jsonschema:"CT or T at the end of the match"`
	Kills     int    `json:"kills"`
	Deaths    int    `json:"deaths"`
	Assists   int    `json:"assists"`
}

type pickPlayerIn struct {
	JobID   string `json:"job_id"`
	SteamID string `json:"steamid" jsonschema:"SteamID64 from job_players of the player whose point of view the video uses"`
}

func registerSteamTools(server *mcp.Server, s *studio) {
	readOnly := &mcp.ToolAnnotations{ReadOnlyHint: true}

	mcp.AddTool(server, &mcp.Tool{Name: "steam_matches", Annotations: readOnly,
		Description: "List the user's recent CS2 matches from their Steam match history, as Tus partidas recientes shows. Call sync_steam_matches first to look for new ones."},
		func(ctx context.Context, _ *mcp.CallToolRequest, _ empty) (*mcp.CallToolResult, steamMatchesOut, error) {
			c, err := s.client(ctx)
			if err != nil {
				return nil, steamMatchesOut{}, toolError(err)
			}
			acc, err := c.SteamAccount(ctx)
			if err != nil {
				return nil, steamMatchesOut{}, toolError(err)
			}
			return nil, describeSteamMatches(acc), nil
		})

	mcp.AddTool(server, &mcp.Tool{Name: "sync_steam_matches",
		Description: "Look for new matches in the user's Steam match history, then list them like steam_matches."},
		func(ctx context.Context, _ *mcp.CallToolRequest, _ empty) (*mcp.CallToolResult, steamMatchesOut, error) {
			c, err := s.client(ctx)
			if err != nil {
				return nil, steamMatchesOut{}, toolError(err)
			}
			acc, err := c.SyncSteamMatches(ctx)
			switch tuiclient.ErrorCode(err) {
			case tuiclient.SteamHistoryNotConfigured:
				return nil, steamMatchesOut{}, errors.New("the Steam match history is not connected. " + steamSetupHint)
			case tuiclient.SteamNeedKnownCode:
				return nil, steamMatchesOut{}, errors.New("Steam needs one share code of a recent match to start the history. Ask the user to paste one in Studio, Ajustes, Cuenta con la que juegas (Un código de partida conocido)")
			}
			if err != nil {
				return nil, steamMatchesOut{}, toolError(err)
			}
			return nil, describeSteamMatches(acc), nil
		})

	mcp.AddTool(server, &mcp.Tool{Name: "import_share_code",
		Description: "Download a match's demo from Valve by its share code and start a video job for it, as Descargar does in Studio. " +
			"Follow with wait_for_job: \"parsed\" is ready for list_moments; \"scanned\" means pick the player with job_players and pick_player. " +
			"Studio signs in to Steam with the session the user started in the UI; the assistant never handles Steam passwords."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in shareCodeIn) (*mcp.CallToolResult, steamImportOut, error) {
			code, ok := normalizeShareCode(in.Code)
			if !ok {
				return nil, steamImportOut{}, errors.New("code must be a CS2 share code, such as CSGO-GADqf-jjyJ8-cSP2r-smZRo-TO2xK")
			}
			c, err := s.uploadClient(ctx)
			if err != nil {
				return nil, steamImportOut{}, toolError(err)
			}
			res, err := c.ImportShareCode(ctx, code)
			switch tuiclient.ErrorCode(err) {
			case tuiclient.SteamCredentialsRequired:
				return nil, steamImportOut{}, errors.New("Studio needs the user to sign in to Steam once to download demos. Ask them to press Descargar demo on this match in Studio (Shorts, the share code field) and sign in there; after that, import_share_code works until Studio restarts")
			case tuiclient.SteamDemoUnavailable:
				return nil, steamImportOut{}, errors.New("Valve has no downloadable demo for this match: demos expire a few weeks after the match")
			}
			if err != nil {
				return nil, steamImportOut{}, toolError(err)
			}
			return nil, steamImportOut{JobID: res.JobID, Status: cleanName(res.Status), MatchID: cleanName(res.MatchID)}, nil
		})

	mcp.AddTool(server, &mcp.Tool{Name: "job_players", Annotations: readOnly,
		Description: "List the players of a job that import_share_code made, once wait_for_job reports \"scanned\", to pick whose point of view the video uses."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in idIn) (*mcp.CallToolResult, jobPlayersOut, error) {
			if err := checkUUID("id", in.ID); err != nil {
				return nil, jobPlayersOut{}, err
			}
			c, err := s.client(ctx)
			if err != nil {
				return nil, jobPlayersOut{}, toolError(err)
			}
			roster, err := c.GetRoster(ctx, in.ID)
			if err != nil {
				return nil, jobPlayersOut{}, toolError(err)
			}
			out := jobPlayersOut{Map: cleanName(roster.Match.Map), Players: make([]rosterPlayerOut, 0, len(roster.Players))}
			for _, p := range roster.Players {
				out.Players = append(out.Players, rosterPlayerOut{SteamID64: cleanName(p.SteamID64), Name: cleanName(p.Name), Team: cleanName(p.Team),
					Kills: p.Kills, Deaths: p.Deaths, Assists: p.Assists})
			}
			return nil, out, nil
		})

	mcp.AddTool(server, &mcp.Tool{Name: "pick_player",
		Description: "Parse one player's point of view in a job from import_share_code, as picking a player does in Studio. Follow with wait_for_job until \"parsed\", then list_moments."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in pickPlayerIn) (*mcp.CallToolResult, jobOut, error) {
			if err := checkUUID("job_id", in.JobID); err != nil {
				return nil, jobOut{}, err
			}
			if err := checkSteamID(in.SteamID); err != nil {
				return nil, jobOut{}, err
			}
			c, err := s.client(ctx)
			if err != nil {
				return nil, jobOut{}, toolError(err)
			}
			res, err := c.StartParse(ctx, in.JobID, in.SteamID)
			if err != nil {
				return nil, jobOut{}, toolError(err)
			}
			return nil, jobOut{JobID: in.JobID, Status: cleanName(res.Status), Running: workerOwnsJob(res.Status)}, nil
		})
}

func describeSteamMatches(acc tuiclient.SteamAccount) steamMatchesOut {
	out := steamMatchesOut{HistoryConfigured: acc.HistoryConfigured, CanDownload: acc.GCConfigured, Matches: []steamMatchOut{}}
	for _, m := range acc.Matches {
		code, ok := normalizeShareCode(m.ShareCode)
		if !ok {
			continue
		}
		out.Matches = append(out.Matches, steamMatchOut{ShareCode: code, MatchID: cleanName(m.MatchID), DiscoveredAt: m.DiscoveredAt.Local().Format("2006-01-02 15:04")})
	}
	return out
}

// normalizeShareCode accepts what Studio's share code field accepts (spaces, no or lowercase
// CSGO- prefix) and returns the CSGO-xxxxx-... form the orchestrator decodes.
func normalizeShareCode(raw string) (string, bool) {
	s := strings.Join(strings.Fields(raw), "")
	if len(s) >= 5 && strings.EqualFold(s[:5], "CSGO-") {
		s = s[5:]
	}
	s = strings.ReplaceAll(s, "-", "")
	if len(s) != 25 || strings.ContainsFunc(s, func(r rune) bool { return !strings.ContainsRune(shareCodeDictionary, r) }) {
		return "", false
	}
	return "CSGO-" + s[0:5] + "-" + s[5:10] + "-" + s[10:15] + "-" + s[15:20] + "-" + s[20:25], true
}
