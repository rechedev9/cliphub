package main

import (
	"context"
	"errors"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/rechedev9/cliphub/internal/tuiclient"
)

const maxProTeamBatches = 60

type importProTeamIn struct {
	TeamID int `json:"team_id" jsonschema:"HLTV team id from the teams of list_pro_matches"`
}

type importProTeamOut struct {
	TeamID     string `json:"team_id,omitempty" jsonschema:"the library team the maps went to, for get_team and the team tools; missing when no map could be read"`
	Name       string `json:"name"`
	Days       int    `json:"days" jsonschema:"the catalog period read, in days"`
	Total      int    `json:"total" jsonschema:"the team's maps in the catalog over that period"`
	Held       int    `json:"held" jsonschema:"how many of them the library holds now; a map the user deleted or one the server sent unreadable stays out"`
	Imported   int    `json:"imported" jsonschema:"maps this call brought in"`
	StudioPath string `json:"studio_path,omitempty" jsonschema:"the path that opens the team in the Buscador de patrones in Studio"`
}

type proTeamImporter interface {
	ImportProTeam(ctx context.Context, teamID int) (tuiclient.ProTeamImport, error)
}

func importWholeProTeam(ctx context.Context, c proTeamImporter, teamID int) (importProTeamOut, error) {
	out := importProTeamOut{}
	left := -1
	for range maxProTeamBatches {
		res, err := c.ImportProTeam(ctx, teamID)
		if err != nil {
			return out, err
		}
		out.Name, out.Days, out.Total, out.Held = res.Name, res.Days, res.Total, res.Held
		out.Imported += res.Imported
		if res.TeamID != nil {
			out.TeamID = res.TeamID.String()
			out.StudioPath = "/patrones/" + out.TeamID
		}
		if res.Remaining == 0 || (left >= 0 && res.Remaining >= left) {
			return out, nil
		}
		left = res.Remaining
	}
	return out, nil
}

func registerProTeamTools(server *mcp.Server, s *studio) {
	mcp.AddTool(server, &mcp.Tool{Name: "import_pro_team", Annotations: &mcp.ToolAnnotations{IdempotentHint: true},
		Description: "Import every map of an HLTV team that the Partidas pro catalog holds (its whole period, 120 days), as the Buscador de patrones does when it opens the team: " +
			"Studio downloads each analysis the library lacks (never the demo), so the pattern, grenade and position tools search the team's public maps together with the user's own demos. " +
			"Maps already in the library are skipped, a map the user deleted stays out, and calling it again brings only the new ones."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in importProTeamIn) (*mcp.CallToolResult, importProTeamOut, error) {
			if in.TeamID <= 0 {
				return nil, importProTeamOut{}, errors.New("team_id must be an HLTV team id from list_pro_matches")
			}
			c, err := s.client(ctx)
			if err != nil {
				return nil, importProTeamOut{}, toolError(err)
			}
			out, err := importWholeProTeam(ctx, c, in.TeamID)
			if err != nil {
				return nil, importProTeamOut{}, proMatchesError(err)
			}
			return nil, out, nil
		})
}
