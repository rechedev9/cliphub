package main

import (
	"context"
	"fmt"
	"math"
	"slices"
	"sort"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/rechedev9/cliphub/internal/tacticalplan"
	"github.com/rechedev9/cliphub/internal/tuiclient"
)

const (
	defaultGrenadeRows = 30
	maxGrenadeRows     = 100
	// maxGrenadeThrowers caps the throwers listed per kind.
	maxGrenadeThrowers = 5
	maxGrenadePlayers  = 10
)

var grenadeKinds = []string{"smoke", "flash", "he", "molotov", "decoy"}

var grenadeBuys = []string{"pistol", "eco", "semi", "force", "full"}

type teamGrenadesIn struct {
	TeamID  string   `json:"team_id" jsonschema:"team id from list_teams"`
	Kind    []string `json:"kind,omitempty" jsonschema:"only these kinds: smoke, flash, he, molotov or decoy; empty means all"`
	Limit   int      `json:"limit,omitempty" jsonschema:"how many grenade rows to list, newest match first; default 30, at most 100"`
	Players []string `json:"players,omitempty" jsonschema:"only the throws of these players, SteamID64s from get_team; the rounds stay the same"`
	Rivals  bool     `json:"rivals,omitempty" jsonschema:"true reads the grenades the team's rivals threw in those rounds, as Rivales de <equipo> does"`
	Buy     string   `json:"buy,omitempty" jsonschema:"only rounds where the throwers bought this: pistol, eco, semi, force or full"`
	Spot    string   `json:"spot,omitempty" jsonschema:"a key from spots: reads that grenade in full, with what the team does after it and its rounds"`
	Invert  bool     `json:"invert,omitempty" jsonschema:"true lists the rounds without the kept grenades (without the spot's grenade when spot is set), as Invertir does"`
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
	LandPlace string      `json:"land_place,omitempty" jsonschema:"callout it went off at"`
	Spot      string      `json:"spot" jsonschema:"key of the repeated throw it belongs to"`
	RoundWon  bool        `json:"round_won"`
}

type teamGrenadesOut struct {
	Rounds         int                `json:"rounds" jsonschema:"the rounds the scope kept (side, dates, rivals, matches and buy): the denominator of every round count"`
	MatchingRounds int                `json:"matching_rounds" jsonschema:"rounds with at least one kept grenade, or without one when invert; with spot, rounds with that grenade (or without it)"`
	Total          int                `json:"total"`
	Kinds          []grenadeKindOut   `json:"kinds"`
	Clock          []clockBinOut      `json:"clock" jsonschema:"kept throws per 10 seconds of the round clock, from 1:55 to 0:00, by player; empty bins left out"`
	Spots          []grenadeSpotOut   `json:"spots" jsonschema:"the repeated throws, most rounds first, at most 10; pass a key as spot to read one"`
	Spot           *grenadeDetailOut  `json:"spot,omitempty"`
	Inverted       []invertedRoundOut `json:"inverted_rounds,omitempty" jsonschema:"with invert, the rounds without the grenade"`
	Grenades       []grenadeOut       `json:"grenades"`
	Truncated      bool               `json:"truncated,omitempty"`
	Note           string             `json:"note,omitempty"`
	SkippedMatches int                `json:"skipped_matches,omitempty"`
}

func registerTeamGrenadesTool(server *mcp.Server, s *studio) {
	mcp.AddTool(server, &mcp.Tool{Name: "team_grenades", Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true},
		Description: "Read the grenades a team threw, as the Granadas form of its Replay: per kind how many, on which side, when and by whom; " +
			"when on the round clock by player; the repeated throws (spots) with what the team does in the 20 s after each; then compact rows. " +
			"Pass a spot key to read one grenade in full: how many of the scope's rounds it was thrown in, what follows it, its rounds and a studio_path that superposes them. " +
			"Filter by kind, map, side, dates, rivals (opponent_id), matches (match_id), players, buy, rivals=true for their utility and invert for the rounds without it."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in teamGrenadesIn) (*mcp.CallToolResult, teamGrenadesOut, error) {
			read, err := in.read()
			if err != nil {
				return nil, teamGrenadesOut{}, err
			}
			c, err := s.client(ctx)
			if err != nil {
				return nil, teamGrenadesOut{}, toolError(err)
			}
			list, err := c.TeamGrenadeList(ctx, read)
			if err != nil {
				return nil, teamGrenadesOut{}, toolError(err)
			}
			kept := keepGrenades(list, in)
			if in.Spot != "" && !slices.ContainsFunc(kept.Grenades, func(g tacticalplan.TeamGrenade) bool { return g.Spot == in.Spot }) {
				return nil, teamGrenadesOut{}, fmt.Errorf("no spot %q in these grenades; spots lists the keys", cleanName(in.Spot))
			}
			limit := in.Limit
			if limit == 0 {
				limit = defaultGrenadeRows
			}
			out := summarizeGrenades(kept, limit)
			out.SkippedMatches = len(list.Skipped)
			readSpots(&out, kept, in, limit)
			return nil, out, nil
		})
}

// read validates the input into the orchestrator's query.
func (in teamGrenadesIn) read() (tuiclient.TeamGrenadeQuery, error) {
	if err := checkUUID("team_id", in.TeamID); err != nil {
		return tuiclient.TeamGrenadeQuery{}, err
	}
	kinds := make([]string, 0, len(in.Kind))
	for _, raw := range in.Kind {
		kind := strings.ToLower(strings.TrimSpace(raw))
		if !slices.Contains(grenadeKinds, kind) {
			return tuiclient.TeamGrenadeQuery{}, fmt.Errorf("kind must be one of %s, not %q", strings.Join(grenadeKinds, ", "), cleanName(raw))
		}
		kinds = append(kinds, kind)
	}
	if in.Limit < 0 || in.Limit > maxGrenadeRows {
		return tuiclient.TeamGrenadeQuery{}, fmt.Errorf("limit must be from 1 to %d", maxGrenadeRows)
	}
	if in.Buy != "" && !slices.Contains(grenadeBuys, in.Buy) {
		return tuiclient.TeamGrenadeQuery{}, fmt.Errorf("buy must be one of %s", strings.Join(grenadeBuys, ", "))
	}
	if len(in.Players) > maxGrenadePlayers {
		return tuiclient.TeamGrenadeQuery{}, fmt.Errorf("players takes at most %d SteamID64s", maxGrenadePlayers)
	}
	for _, p := range in.Players {
		if !steamIDPattern.MatchString(p) {
			return tuiclient.TeamGrenadeQuery{}, fmt.Errorf("players: %q is not a SteamID64", cleanName(p))
		}
	}
	q, err := teamQuery(in.analysisFilterIn, in.teamScopeIn)
	if err != nil {
		return tuiclient.TeamGrenadeQuery{}, err
	}
	return tuiclient.TeamGrenadeQuery{TeamID: in.TeamID, Query: q, Kinds: kinds, Rivals: in.Rivals}, nil
}

// keepGrenades applies the buy (to the rounds) and the players (to the throws).
func keepGrenades(list tuiclient.TeamGrenades, in teamGrenadesIn) tacticalplan.TeamGrenades {
	out := tacticalplan.TeamGrenades{Grenades: []tacticalplan.TeamGrenade{}, RoundList: []tacticalplan.GrenadeRound{}}
	kept := map[string]bool{}
	for _, r := range list.RoundList {
		if in.Buy == "" || string(r.Buy) == in.Buy {
			out.RoundList = append(out.RoundList, r)
			kept[roundRef(r.MatchID.String(), r.Round)] = true
		}
	}
	out.Rounds = len(out.RoundList)
	if len(list.RoundList) == 0 && in.Buy == "" {
		out.Rounds = list.Rounds
	}
	for _, g := range list.Grenades {
		if in.Buy != "" && !kept[roundRef(g.MatchID.String(), g.Round)] {
			continue
		}
		if len(in.Players) > 0 && !slices.Contains(in.Players, g.ThrowerSteamID64) {
			continue
		}
		out.Grenades = append(out.Grenades, g)
	}
	return out
}

func roundRef(matchID string, round int) string { return fmt.Sprintf("%s/%d", matchID, round) }

// summarizeGrenades groups the grenades by kind and lists the first limit rows.
func summarizeGrenades(list tacticalplan.TeamGrenades, limit int) teamGrenadesOut {
	out := teamGrenadesOut{Rounds: list.Rounds, Total: len(list.Grenades), Kinds: []grenadeKindOut{}, Grenades: []grenadeOut{}}
	byKind := map[string][]tacticalplan.TeamGrenade{}
	for _, g := range list.Grenades {
		byKind[string(g.Kind)] = append(byKind[string(g.Kind)], g)
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
		row := grenadeOut{Ref: roundRef(g.MatchID.String(), g.Round), Kind: cleanName(string(g.Kind)), Side: cleanName(string(g.Side)),
			Seconds: g.Seconds, Thrower: cleanName(g.ThrowerName), Landing: roundPos(g.Landing), LandPlace: cleanName(g.LandPlace),
			Spot: cleanName(g.Spot), RoundWon: g.RoundWon}
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

func kindSummary(kind string, gs []tacticalplan.TeamGrenade, rounds int) grenadeKindOut {
	k := grenadeKindOut{Kind: kind, Count: len(gs), PerRound: ratio(len(gs), rounds), Throwers: []throwerOut{}}
	counts := map[string]*throwerOut{}
	secs := make([]float64, 0, len(gs))
	for _, g := range gs {
		switch g.Side {
		case tacticalplan.SideCT:
			k.CT++
		case tacticalplan.SideT:
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
