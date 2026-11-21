package main

import (
	"context"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/rechedev9/cliphub/internal/tacticalplan"
)

type libraryNormsIn struct {
	Map  string `json:"map,omitempty" jsonschema:"such as de_mirage; with a map the answer also has the playbook categories per side"`
	From string `json:"from,omitempty" jsonschema:"first day the matches were played, YYYY-MM-DD"`
	To   string `json:"to,omitempty" jsonschema:"last day the matches were played, YYYY-MM-DD, included"`
}

type libraryNormsOut struct {
	Map     string `json:"map,omitempty"`
	Skipped int    `json:"skipped,omitempty" jsonschema:"matches Studio could not read and left out"`
	tacticalplan.Norms
}

func registerLibraryNormsTool(server *mcp.Server, s *studio) {
	mcp.AddTool(server, &mcp.Tool{Name: "library_norms", Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true},
		Description: "The library's reference rates, pooled over both teams of every match on a map and dates: bombsite attack, plant and hold rates, " +
			"buy matchup win rates and, with a map, each playbook category's share and win rate per side. " +
			"team_bombsites, team_playbook and team_prep report one team; compare them with this to say what is above or below the usual, " +
			"as the ▲▼ marks of those screens do (frente a los equipos de la biblioteca)."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in libraryNormsIn) (*mcp.CallToolResult, libraryNormsOut, error) {
			if err := checkOptionalFilter("map", in.Map, mapNamePattern); err != nil {
				return nil, libraryNormsOut{}, err
			}
			from, to, err := parseDateRange(in.From, in.To)
			if err != nil {
				return nil, libraryNormsOut{}, err
			}
			if !to.IsZero() {
				to = to.Add(-time.Second)
			}
			c, err := s.client(ctx)
			if err != nil {
				return nil, libraryNormsOut{}, toolError(err)
			}
			norms, err := c.LibraryNorms(ctx, strings.ToLower(in.Map), from, to)
			if err != nil {
				return nil, libraryNormsOut{}, toolError(err)
			}
			return nil, libraryNormsOut{Map: cleanName(norms.Map), Skipped: norms.Skipped, Norms: norms.Norms}, nil
		})
}
