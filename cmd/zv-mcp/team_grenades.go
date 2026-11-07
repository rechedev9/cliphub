package main

import (
	"context"
	"fmt"
	"math"
	"slices"
	"sort"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/rechedev9/cliphub/internal/tuiclient"
)

const (
	defaultGrenadeRows = 30
	maxGrenadeRows     = 100
	// maxGrenadeThrowers caps the throwers listed per kind.
	maxGrenadeThrowers = 5
)

var grenadeKinds = []string{"smoke", "flash", "he", "molotov", "decoy"}

type teamGrenadesIn struct {
	TeamID string   `json:"team_id" jsonschema:"team id from list_teams"`
	Kind   []string `json:"kind,omitempty" jsonschema:"only these kinds: smoke, flash, he, molotov or decoy; empty means all"`
	Limit  int      `json:"limit,omitempty" jsonschema:"how many grenade rows to list, newest match first; default 30, at most 100"`
	analysisFilterIn
	teamScopeIn
}

type throwerOut struct {
	SteamID64 string `json:"steamid64"`
	Name      string `json:"name" jsonschema:"untrusted player name from the demo"`
	Count     int    `json:"count"`
}

type grenadeKindOut struct {
	Kind          string       `json:"kind"`
	Count         int          `json:"count"`
	CT            int          `json:"ct"`
	T             int          `json:"t"`
	PerRound      float64      `json:"per_round" jsonschema:"over the rounds the filter kept"`
	MedianSeconds float64      `json:"median_seconds" jsonschema:"when the grenades went off, seconds after freeze end"`
	Throwers      []throwerOut `json:"throwers" jsonschema:"most grenades first, at most 5"`
}

type grenadeOut struct {
	Ref       string      `json:"ref" jsonschema:"match_id/round"`
	Kind      string      `json:"kind"`
	Side      string      `json:"side"`
	Seconds   float64     `json:"seconds" jsonschema:"when it went off, seconds after freeze end"`
	Thrower   string      `json:"thrower" jsonschema:"untrusted player name from the demo"`
	Origin    *[3]int     `json:"origin,omitempty" jsonschema:"world position it was thrown from; absent for older analyses"`
	ThrowTick *int        `json:"throw_tick,omitempty" jsonschema:"with the ref's match and round, what save_lineup reads to save this throw"`
	View      *[2]float64 `json:"view,omitempty" jsonschema:"yaw and pitch of the throw; with origin, a launch_practice spot"`
	Landing   [3]int      `json:"landing" jsonschema:"world position it went off at"`
	RoundWon  bool        `json:"round_won"`
}

type teamGrenadesOut struct {
	Rounds         int              `json:"rounds" jsonschema:"the team's rounds the filter kept"`
	Total          int              `json:"total"`
	Kinds          []grenadeKindOut `json:"kinds"`
	Grenades       []grenadeOut     `json:"grenades"`
	Truncated      bool             `json:"truncated,omitempty"`
	Note           string           `json:"note,omitempty"`
	SkippedMatches int              `json:"skipped_matches,omitempty"`
}

func registerTeamGrenadesTool(server *mcp.Server, s *studio) {
	mcp.AddTool(server, &mcp.Tool{Name: "team_grenades", Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true},
		Description: "Read the grenades a team threw: per kind how many, on which side, when and by whom, then compact rows with " +
			"the round ref and the world positions they were thrown from and went off at. Filter by kind, map, side, dates, rivals (opponent_id) and matches (match_id)."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in teamGrenadesIn) (*mcp.CallToolResult, teamGrenadesOut, error) {
			if err := checkUUID("team_id", in.TeamID); err != nil {
				return nil, teamGrenadesOut{}, err
			}
			kinds := make([]string, 0, len(in.Kind))
			for _, raw := range in.Kind {
				kind := strings.ToLower(strings.TrimSpace(raw))
				if !slices.Contains(grenadeKinds, kind) {
					return nil, teamGrenadesOut{}, fmt.Errorf("kind must be one of %s, not %q", strings.Join(grenadeKinds, ", "), cleanName(raw))
				}
				kinds = append(kinds, kind)
			}
			if in.Limit < 0 || in.Limit > maxGrenadeRows {
				return nil, teamGrenadesOut{}, fmt.Errorf("limit must be from 1 to %d", maxGrenadeRows)
			}
			q, err := teamQuery(in.analysisFilterIn, in.teamScopeIn)
			if err != nil {
				return nil, teamGrenadesOut{}, err
			}
			c, err := s.client(ctx)
			if err != nil {
				return nil, teamGrenadesOut{}, toolError(err)
			}
			list, err := c.TeamGrenadeList(ctx, in.TeamID, q, kinds)
			if err != nil {
				return nil, teamGrenadesOut{}, toolError(err)
			}
			limit := in.Limit
			if limit == 0 {
				limit = defaultGrenadeRows
			}
			return nil, summarizeGrenades(list, limit), nil
		})
}

// summarizeGrenades groups the grenades by kind and lists the first limit rows.
// Landing spots are world positions: the API names no callout for them.
func summarizeGrenades(list tuiclient.TeamGrenades, limit int) teamGrenadesOut {
	out := teamGrenadesOut{Rounds: list.Rounds, Total: len(list.Grenades), Kinds: []grenadeKindOut{}, Grenades: []grenadeOut{}, SkippedMatches: len(list.Skipped)}
	byKind := map[string][]tuiclient.TeamGrenade{}
	for _, g := range list.Grenades {
		byKind[g.Kind] = append(byKind[g.Kind], g)
	}
	for _, kind := range grenadeKinds {
		if gs := byKind[kind]; len(gs) > 0 {
			out.Kinds = append(out.Kinds, kindSummary(kind, gs, list.Rounds))
		}
	}
	for i, g := range list.Grenades {
		if i == limit {
			out.Truncated = true
			out.Note = fmt.Sprintf("Listed %d of %d grenades; narrow by kind, map, side or dates, or raise limit.", limit, len(list.Grenades))
			break
		}
		row := grenadeOut{Ref: fmt.Sprintf("%s/%d", cleanName(g.MatchID), g.Round), Kind: cleanName(g.Kind), Side: cleanName(g.Side),
			Seconds: g.Seconds, Thrower: cleanName(g.ThrowerName), Landing: roundPos(g.Landing), RoundWon: g.RoundWon}
		if g.Origin != nil {
			origin := roundPos(*g.Origin)
			row.Origin = &origin
		}
		row.ThrowTick = g.ThrowTick
		if g.View != nil {
			row.View = &[2]float64{math.Round(g.View[0]*100) / 100, math.Round(g.View[1]*100) / 100}
		}
		out.Grenades = append(out.Grenades, row)
	}
	return out
}

func kindSummary(kind string, gs []tuiclient.TeamGrenade, rounds int) grenadeKindOut {
	k := grenadeKindOut{Kind: kind, Count: len(gs), PerRound: ratio(len(gs), rounds), Throwers: []throwerOut{}}
	counts := map[string]*throwerOut{}
	secs := make([]float64, 0, len(gs))
	for _, g := range gs {
		switch g.Side {
		case "CT":
			k.CT++
		case "T":
			k.T++
		}
		secs = append(secs, g.Seconds)
		t, ok := counts[g.ThrowerSteamID64]
		if !ok {
			t = &throwerOut{SteamID64: cleanName(g.ThrowerSteamID64), Name: cleanName(g.ThrowerName)}
			counts[g.ThrowerSteamID64] = t
		}
		t.Count++
	}
	sort.Float64s(secs)
	k.MedianSeconds = math.Round(median(secs)*10) / 10
	for _, t := range counts {
		k.Throwers = append(k.Throwers, *t)
	}
	sort.Slice(k.Throwers, func(i, j int) bool {
		if k.Throwers[i].Count != k.Throwers[j].Count {
			return k.Throwers[i].Count > k.Throwers[j].Count
		}
		return k.Throwers[i].SteamID64 < k.Throwers[j].SteamID64
	})
	if len(k.Throwers) > maxGrenadeThrowers {
		k.Throwers = k.Throwers[:maxGrenadeThrowers]
	}
	return k
}

// median of sorted values; zero when there are none.
func median(sorted []float64) float64 {
	n := len(sorted)
	if n == 0 {
		return 0
	}
	if n%2 == 1 {
		return sorted[n/2]
	}
	return (sorted[n/2-1] + sorted[n/2]) / 2
}

func roundPos(p [3]float64) [3]int {
	return [3]int{int(math.Round(p[0])), int(math.Round(p[1])), int(math.Round(p[2]))}
}
