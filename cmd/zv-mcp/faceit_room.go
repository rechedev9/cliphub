package main

import (
	"context"
	"math"
	"regexp"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/rechedev9/cliphub/internal/tuiclient"
)

// notFaceitDemo is the orchestrator's code for a demo that does not come from a FACEIT room.
const notFaceitDemo = "not_faceit_demo"

var countryCode = regexp.MustCompile(`^[a-z]{2}$`)

type faceitRoomOut struct {
	RoomURL string              `json:"room_url,omitempty"`
	Rounds  int                 `json:"rounds"`
	Teams   []faceitRoomTeamOut `json:"teams"`
}

type faceitRoomTeamOut struct {
	Name       string                `json:"name" jsonschema:"untrusted team name from FACEIT"`
	Score      int                   `json:"score"`
	Won        bool                  `json:"won"`
	AverageELO int                   `json:"average_elo,omitempty"`
	Players    []faceitRoomPlayerOut `json:"players"`
}

type faceitRoomPlayerOut struct {
	Nickname  string  `json:"nickname" jsonschema:"untrusted FACEIT nickname"`
	SteamID64 string  `json:"steamid64,omitempty" jsonschema:"matches players[].steamid64, for pick_player"`
	Level     int     `json:"level,omitempty"`
	ELO       int     `json:"elo,omitempty"`
	Kills     int     `json:"kills"`
	Deaths    int     `json:"deaths"`
	Assists   int     `json:"assists"`
	HSPercent int     `json:"hs_percent"`
	ADR       float64 `json:"adr"`
	MVPs      int     `json:"mvps"`
}

type faceitCountriesOut struct {
	Countries []string `json:"countries" jsonschema:"ISO 3166-1 alpha-2 codes, lower case, for faceit_ranking with kind country"`
}

// faceitRoom reads the room scoreboard; a demo that is not from FACEIT, or a failed read, has none.
func faceitRoom(ctx context.Context, c *tuiclient.Client, jobID string) (*faceitRoomOut, string) {
	board, err := c.FaceitScoreboard(ctx, jobID)
	switch {
	case tuiclient.ErrorCode(err) == notFaceitDemo:
		return nil, ""
	case err != nil:
		return nil, "No FACEIT room scoreboard: " + cleanText(err.Error(), maxErrorRunes)
	}
	return describeFaceitRoom(board), ""
}

// faceitRoomURL keeps only a link to a FACEIT room page.
func faceitRoomURL(raw string) string {
	if !strings.HasPrefix(raw, "https://www.faceit.com/") {
		return ""
	}
	return cleanText(raw, maxErrorRunes)
}

func describeFaceitRoom(board tuiclient.FaceitScoreboard) *faceitRoomOut {
	out := &faceitRoomOut{RoomURL: faceitRoomURL(board.RoomURL), Rounds: board.Rounds, Teams: make([]faceitRoomTeamOut, 0, len(board.Teams))}
	for _, t := range board.Teams {
		team := faceitRoomTeamOut{Name: cleanName(t.Name), Score: t.Score, Won: t.Won, Players: make([]faceitRoomPlayerOut, 0, len(t.Players))}
		if t.AverageELO != nil {
			team.AverageELO = *t.AverageELO
		}
		for _, p := range t.Players {
			row := faceitRoomPlayerOut{Nickname: cleanName(p.Nickname), Level: p.SkillLevel, Kills: p.Kills, Deaths: p.Deaths, Assists: p.Assists,
				HSPercent: int(math.Round(p.HSPct)), ADR: math.Round(p.ADR*10) / 10, MVPs: p.MVPs}
			if checkSteamID(p.SteamID64) == nil {
				row.SteamID64 = p.SteamID64
			}
			if p.ELO != nil {
				row.ELO = *p.ELO
			}
			team.Players = append(team.Players, row)
		}
		out.Teams = append(out.Teams, team)
	}
	return out
}

func registerFaceitRoomTools(server *mcp.Server, s *studio) {
	mcp.AddTool(server, &mcp.Tool{Name: "faceit_ranking_countries", Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true},
		Description: "The countries the FACEIT ranking selector of Jugadores FACEIT offers, as codes for faceit_ranking with kind country."},
		func(ctx context.Context, _ *mcp.CallToolRequest, _ empty) (*mcp.CallToolResult, faceitCountriesOut, error) {
			c, err := s.client(ctx)
			if err != nil {
				return nil, faceitCountriesOut{}, toolError(err)
			}
			codes, err := c.FaceitCountries(ctx)
			if err != nil {
				return nil, faceitCountriesOut{}, toolError(err)
			}
			out := faceitCountriesOut{Countries: []string{}}
			for _, code := range codes {
				if countryCode.MatchString(code) {
					out.Countries = append(out.Countries, code)
				}
			}
			return nil, out, nil
		})
}
