package main

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	maxSearchRunes   = 200
	maxSearchPerKind = 25
)

type searchIn struct {
	Query   string `json:"query" jsonschema:"words to look for, such as humo ventana mirage; a map name keeps that map, a grenade word keeps those lineups"`
	PerKind int    `json:"per_kind,omitempty" jsonschema:"results of each kind, default 8, at most 25"`
}

type searchResultOut struct {
	Kind     string `json:"kind" jsonschema:"play, lineup, call, team, player or match"`
	ID       string `json:"id" jsonschema:"the id for get_play, get_lineup, update_call, get_team, a player's SteamID64 or get_match"`
	Title    string `json:"title" jsonschema:"untrusted text"`
	Map      string `json:"map,omitempty"`
	Side     string `json:"side,omitempty"`
	Status   string `json:"status,omitempty"`
	Detail   string `json:"detail,omitempty" jsonschema:"a play's kind, a lineup's grenade, a call's meaning, a team's or player's match count, a match's date"`
	TeamName string `json:"team_name,omitempty"`
}

type searchOut struct {
	Map     string            `json:"map,omitempty" jsonschema:"the map the query named"`
	Grenade string            `json:"grenade,omitempty" jsonschema:"the grenade the query named"`
	Results []searchResultOut `json:"results"`
}

func registerSearchTool(server *mcp.Server, s *studio) {
	mcp.AddTool(server, &mcp.Tool{Name: "search_studio", Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true},
		Description: "Search the library as Ctrl+K does in Studio: plays, lineups, calls, teams, players and matches holding every word, " +
			"accents and & ignored. A map name in the query keeps that map's records; a grenade word (humo, flash, molotov...) keeps those lineups."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in searchIn) (*mcp.CallToolResult, searchOut, error) {
			q := strings.TrimSpace(in.Query)
			switch {
			case q == "":
				return nil, searchOut{}, errors.New("query must not be empty")
			case utf8.RuneCountInString(q) > maxSearchRunes || strings.ContainsFunc(q, unicode.IsControl):
				return nil, searchOut{}, fmt.Errorf("query takes at most %d characters of plain text", maxSearchRunes)
			case in.PerKind < 0 || in.PerKind > maxSearchPerKind:
				return nil, searchOut{}, fmt.Errorf("per_kind must be from 1 to %d", maxSearchPerKind)
			}
			c, err := s.client(ctx)
			if err != nil {
				return nil, searchOut{}, toolError(err)
			}
			res, err := c.Search(ctx, q, in.PerKind)
			if err != nil {
				return nil, searchOut{}, toolError(err)
			}
			out := searchOut{Map: cleanName(res.Map), Grenade: cleanName(res.Grenade), Results: []searchResultOut{}}
			for _, r := range res.Results {
				out.Results = append(out.Results, searchResultOut{Kind: cleanName(r.Kind), ID: cleanName(r.ID), Title: cleanName(r.Title),
					Map: cleanName(r.Map), Side: cleanName(r.Side), Status: cleanName(r.Status), Detail: cleanText(r.Detail, maxErrorRunes),
					TeamName: cleanName(r.TeamName)})
			}
			return nil, out, nil
		})
}
