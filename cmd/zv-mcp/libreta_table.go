package main

import (
	"cmp"
	"context"
	"errors"
	"slices"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/rechedev9/cliphub/internal/tuiclient"
)

// The Tabla's columns: each side's round patterns in the order the UI shows them.
var (
	tPlanKinds  = []string{"execute", "default", "split", "fast", "eco_rush", "save", "unknown"}
	ctPlanKinds = []string{"hold", "retake", "aggression", "stack", "save", "unknown"}
)

// maxPlanPlays is the server's cap on one list read, enough for any map and side.
const maxPlanPlays = 1000

type planTableIn struct {
	Map    string `json:"map" jsonschema:"such as de_mirage"`
	Side   string `json:"side" jsonschema:"CT or T"`
	TeamID string `json:"team_id,omitempty" jsonschema:"only this team's plays, id from list_teams"`
	NoTeam bool   `json:"no_team,omitempty" jsonschema:"true reads only plays without a team"`
}

type planPlayOut struct {
	ID       string `json:"id"`
	Title    string `json:"title" jsonschema:"untrusted text the user wrote"`
	Site     string `json:"site"`
	Status   string `json:"status"`
	TeamName string `json:"team_name,omitempty" jsonschema:"untrusted team name"`
}

type planColumnOut struct {
	Kind  string        `json:"kind"`
	Plays []planPlayOut `json:"plays"`
}

type planTableOut struct {
	Map     string          `json:"map"`
	Side    string          `json:"side"`
	Columns []planColumnOut `json:"columns"`
}

func registerLibretaTableTools(server *mcp.Server, s *studio) {
	mcp.AddTool(server, &mcp.Tool{Name: "plan_table", Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true},
		Description: "Read the Libreta's Tabla for one map and side: a column per round type with the plays that are not archived, " +
			"so the user sees what the team runs and which round types have nothing yet."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in planTableIn) (*mcp.CallToolResult, planTableOut, error) {
			if in.Map == "" || in.Side == "" {
				return nil, planTableOut{}, errors.New("map and side are required")
			}
			if in.TeamID != "" && in.NoTeam {
				return nil, planTableOut{}, errors.New("set team_id or no_team, not both")
			}
			q, err := playQuery(in.TeamID, in.Map, in.Side, "")
			if err != nil {
				return nil, planTableOut{}, err
			}
			q.Limit = maxPlanPlays
			c, err := s.client(ctx)
			if err != nil {
				return nil, planTableOut{}, toolError(err)
			}
			plays, err := c.ListPlays(ctx, q)
			if err != nil {
				return nil, planTableOut{}, toolError(err)
			}
			if in.NoTeam {
				plays = slices.DeleteFunc(plays, func(p tuiclient.Play) bool { return p.TeamID != nil })
			}
			return nil, planTable(q.Map, q.Side, plays), nil
		})
}

// planTable groups plays by kind in the Tabla's column order, by title inside a
// column; an empty "unknown" column is dropped, as in the UI.
func planTable(mapName, side string, plays []tuiclient.Play) planTableOut {
	kinds := tPlanKinds
	if side == "CT" {
		kinds = ctPlanKinds
	}
	slices.SortFunc(plays, func(a, b tuiclient.Play) int { return cmp.Compare(strings.ToLower(a.Title), strings.ToLower(b.Title)) })
	out := planTableOut{Map: mapName, Side: side, Columns: []planColumnOut{}}
	for _, kind := range kinds {
		col := planColumnOut{Kind: kind, Plays: []planPlayOut{}}
		for _, p := range plays {
			if p.Kind == kind {
				col.Plays = append(col.Plays, planPlayOut{ID: p.ID, Title: cleanName(p.Title), Site: cleanName(p.Site),
					Status: cleanName(p.Status), TeamName: cleanName(p.TeamName)})
			}
		}
		if kind != "unknown" || len(col.Plays) > 0 {
			out.Columns = append(out.Columns, col)
		}
	}
	return out
}
