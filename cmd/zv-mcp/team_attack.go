package main

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/rechedev9/cliphub/internal/placename"
	"github.com/rechedev9/cliphub/internal/tuiclient"
)

// Spanish names of the attack keys, for the playlist titles Studio shows.
var (
	bandLabels  = map[string]string{"pistol": "pistola", "eco": "eco", "half": "media compra", "full": "compra completa"}
	siteLabels  = map[string]string{"A": "sitio A", "B": "sitio B", "both": "los dos sitios", "none": "sin sitio"}
	tempoLabels = map[string]string{"rush": "rush", "set_strat": "estrategia preparada", "mid_round": "toma a mitad de ronda", "no_site": "sin sitio"}
)

// attackCellOut is one site or tempo key of a row: how often and how often won.
type attackCellOut struct {
	Key        string  `json:"key"`
	Rounds     int     `json:"rounds"`
	SharePct   float64 `json:"share_pct" jsonschema:"over the rounds of its row"`
	WinPct     float64 `json:"win_pct"`
	Reliable   bool    `json:"reliable" jsonschema:"false when the sample is too small to trust"`
	StudioPath string  `json:"studio_path,omitempty" jsonschema:"opens exactly these rounds in the Replay, with the site marked"`
}

type attackMatchupOut struct {
	Buy         string          `json:"buy" jsonschema:"the team's buy band: pistol, eco, half (semi or force) or full"`
	OpponentBuy string          `json:"opponent_buy"`
	Rounds      int             `json:"rounds"`
	SharePct    float64         `json:"share_pct" jsonschema:"over every T round"`
	WinPct      float64         `json:"win_pct"`
	Reliable    bool            `json:"reliable"`
	StudioPath  string          `json:"studio_path"`
	Sites       []attackCellOut `json:"sites" jsonschema:"A, B, both or none; share is over the matchup's rounds"`
	Tempo       []attackCellOut `json:"tempo" jsonschema:"rush, set_strat, mid_round or no_site; share is over the matchup's rounds"`
}

type attackZoneOut struct {
	Callout  string  `json:"callout" jsonschema:"CS2 place token"`
	Label    string  `json:"label"`
	Rounds   int     `json:"rounds"`
	SharePct float64 `json:"share_pct" jsonschema:"over the default's rounds"`
}

type attackDefaultOut struct {
	Formation  string          `json:"formation" jsonschema:"players towards A, mid and towards B at the read second, such as 2-1-2"`
	Rounds     int             `json:"rounds"`
	SharePct   float64         `json:"share_pct" jsonschema:"usage: over the T rounds with positions"`
	WinPct     float64         `json:"win_pct"`
	Reliable   bool            `json:"reliable"`
	Zones      []attackZoneOut `json:"zones" jsonschema:"map control zones held in these rounds"`
	StudioPath string          `json:"studio_path" jsonschema:"opens these rounds in the Replay from 1:55 to 1:00 with the zones marked"`
}

type attackExecuteOut struct {
	Buy            string  `json:"buy"`
	OpponentBuy    string  `json:"opponent_buy"`
	Site           string  `json:"site" jsonschema:"A, B or both (a split)"`
	Route          string  `json:"route" jsonschema:"the callout the first players on site came from; empty when unread"`
	RouteLabel     string  `json:"route_label"`
	Rounds         int     `json:"rounds"`
	SharePct       float64 `json:"share_pct" jsonschema:"over the matchup's rounds"`
	WinPct         float64 `json:"win_pct"`
	RushPct        float64 `json:"rush_pct" jsonschema:"rounds of it that were rushes; the rest are set strats"`
	PlantPct       float64 `json:"plant_pct"`
	ContactSeconds float64 `json:"contact_seconds" jsonschema:"median site contact, seconds after freeze end"`
	StudioPath     string  `json:"studio_path" jsonschema:"opens these rounds in the Replay with the route and the site marked"`
}

type attackSpotOut struct {
	Callout        string  `json:"callout" jsonschema:"CS2 place token"`
	Label          string  `json:"label"`
	Rounds         int     `json:"rounds"`
	SharePct       float64 `json:"share_pct" jsonschema:"over the player's rounds in the window"`
	ArrivalSeconds float64 `json:"arrival_seconds" jsonschema:"median second of the round the player first stood there"`
	Floor          string  `json:"floor,omitempty"`
	StudioPath     string  `json:"studio_path" jsonschema:"opens these rounds in the Replay with the player in focus, the window as tramo and the spot marked"`
}

type attackWindowOut struct {
	Window    string          `json:"window" jsonschema:"a span of the round clock, which counts down from 1:55"`
	Tramo     string          `json:"tramo" jsonschema:"the same span in seconds after freeze end, as the Replay's tramo"`
	Rounds    int             `json:"rounds"`
	StaticPct float64         `json:"static_pct" jsonschema:"rounds the player held still in the window"`
	SlowPct   float64         `json:"slow_pct" jsonschema:"rounds the player walked"`
	FastPct   float64         `json:"fast_pct" jsonschema:"rounds the player ran"`
	Spots     []attackSpotOut `json:"spots"`
}

type attackPlayerOut struct {
	SteamID64 string            `json:"steamid64"`
	Name      string            `json:"name" jsonschema:"untrusted player name from the demo"`
	Role      string            `json:"role"`
	Rounds    int               `json:"rounds"`
	Windows   []attackWindowOut `json:"windows"`
}

type prepWarningOut struct {
	Kind    string `json:"kind" jsonschema:"stale, few_rounds or roster_changed"`
	Message string `json:"message"`
}

type teamAttackOut struct {
	Team           string             `json:"team" jsonschema:"untrusted team name"`
	Map            string             `json:"map"`
	Matches        int                `json:"matches"`
	Rounds         int                `json:"rounds" jsonschema:"T rounds read"`
	FramedRounds   int                `json:"framed_rounds" jsonschema:"T rounds with positions; defaults and players read only these"`
	LastPlayedAt   string             `json:"last_played_at,omitempty"`
	DaysSinceLast  int                `json:"days_since_last_match"`
	Warnings       []prepWarningOut   `json:"warnings" jsonschema:"what to distrust in this sample; repeat them in a report"`
	Sites          []attackCellOut    `json:"sites" jsonschema:"where the T rounds went: A, B, both or none"`
	Tempo          []attackCellOut    `json:"tempo" jsonschema:"how they got there: rush, set_strat (execute or fake), mid_round or no_site"`
	Matchups       []attackMatchupOut `json:"matchups" jsonschema:"the team's buy band against the rival's, each crossed with sites and tempo"`
	Defaults       []attackDefaultOut `json:"defaults" jsonschema:"preferred defaults, most used first"`
	Executes       []attackExecuteOut `json:"executes" jsonschema:"preferred executes per buy matchup, most used first, at most 5 each"`
	Players        []attackPlayerOut  `json:"players" jsonschema:"each T player's spots and pace per window"`
	StudioPath     string             `json:"studio_path" jsonschema:"the team's Preparar tab on this map"`
	SkippedMatches int                `json:"skipped_matches,omitempty"`
}

// attackQuery validates a one-map prep request.
func (in teamPrepIn) attackQuery() (tuiclient.AnalysisQuery, error) {
	q, err := teamReportIn{TeamID: in.TeamID, analysisFilterIn: analysisFilterIn{Map: in.Map, From: in.From, To: in.To},
		teamScopeIn: in.teamScopeIn}.check(false)
	if err == nil && q.Map == "" {
		err = errors.New("map is required, such as de_mirage")
	}
	return q, err
}

func registerTeamAttackTool(server *mcp.Server, s *studio) {
	mcp.AddTool(server, &mcp.Tool{Name: "team_attack", Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true},
		Description: "Prepare for a team's T side on one map, as Equipos > Preparar > Ataque shows it: where each buy matchup goes " +
			"(A, B, both or no site, crossed with pistol, eco, half and full buys on both teams), how fast (rush, set strat, mid round), " +
			"the preferred defaults with usage and win rate, the preferred executes per buy with their entry route, and each T player's " +
			"spots and pace per tramo of the round clock. Every row's studio_path opens exactly its rounds in the 2D Replay, " +
			"with the player in focus, the tramo and the zones marked. warnings say when the sample is old, thin or crosses a lineup change. map is required."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in teamPrepIn) (*mcp.CallToolResult, teamAttackOut, error) {
			q, err := in.attackQuery()
			if err != nil {
				return nil, teamAttackOut{}, err
			}
			c, err := s.client(ctx)
			if err != nil {
				return nil, teamAttackOut{}, toolError(err)
			}
			attack, err := c.TeamAttackReport(ctx, in.TeamID, q)
			if err != nil {
				return nil, teamAttackOut{}, toolError(err)
			}
			out := summarizeAttack(attack)
			out.StudioPath = preparePath(in.TeamID, q)
			return nil, out, nil
		})
}

func summarizeAttack(a tuiclient.TeamAttack) teamAttackOut {
	team := cleanName(a.TeamName)
	title := func(parts ...string) string {
		return strings.Join(append([]string{team, cleanName(a.Map)}, parts...), " · ")
	}
	cell := func(c tuiclient.AttackCell, link findingLink) attackCellOut {
		link.refs = c.Refs
		return attackCellOut{Key: cleanName(c.Key), Rounds: c.Rounds, SharePct: c.Share.Pct, WinPct: c.WinRate.Pct,
			Reliable: c.WinRate.Reliable, StudioPath: link.path()}
	}
	out := teamAttackOut{
		Team: team, Map: cleanName(a.Map), Matches: a.Matches, Rounds: a.RoundCount, FramedRounds: a.Framed,
		LastPlayedAt: cleanName(a.LastPlayedAt), DaysSinceLast: a.DaysSince, Warnings: warningsOut(a.Warnings),
		Sites: []attackCellOut{}, Tempo: []attackCellOut{}, Matchups: []attackMatchupOut{}, Defaults: []attackDefaultOut{},
		Executes: []attackExecuteOut{}, Players: []attackPlayerOut{}, SkippedMatches: len(a.Skipped),
	}
	for _, c := range a.Sites {
		out.Sites = append(out.Sites, cell(c, findingLink{title: title(siteLabels[c.Key]), zones: siteZone(c.Key)}))
	}
	for _, c := range a.Tempo {
		out.Tempo = append(out.Tempo, cell(c, findingLink{title: title(tempoLabels[c.Key])}))
	}
	for _, m := range a.Matchups {
		vs := bandLabels[m.Buy] + " contra " + bandLabels[m.OpponentBuy]
		mo := attackMatchupOut{Buy: cleanName(m.Buy), OpponentBuy: cleanName(m.OpponentBuy), Rounds: m.Rounds, SharePct: m.Share.Pct,
			WinPct: m.WinRate.Pct, Reliable: m.WinRate.Reliable, StudioPath: findingLink{title: title(vs), refs: m.Refs}.path(),
			Sites: []attackCellOut{}, Tempo: []attackCellOut{}}
		for _, c := range m.Sites {
			if c.Rounds > 0 {
				mo.Sites = append(mo.Sites, cell(c, findingLink{title: title(vs, siteLabels[c.Key]), zones: siteZone(c.Key)}))
			}
		}
		for _, c := range m.Tempo {
			if c.Rounds > 0 {
				mo.Tempo = append(mo.Tempo, cell(c, findingLink{title: title(vs, tempoLabels[c.Key])}))
			}
		}
		out.Matchups = append(out.Matchups, mo)
	}
	for _, d := range a.Defaults {
		do := attackDefaultOut{Formation: cleanName(d.Formation), Rounds: d.Rounds, SharePct: d.Share.Pct, WinPct: d.WinRate.Pct,
			Reliable: d.WinRate.Reliable, Zones: []attackZoneOut{}}
		zones := []string{}
		for _, z := range d.Zones {
			do.Zones = append(do.Zones, attackZoneOut{Callout: cleanName(z.Zone), Label: cleanName(z.Label), Rounds: z.Rounds, SharePct: z.Share.Pct})
			zones = append(zones, z.Zone)
		}
		do.StudioPath = findingLink{title: title("default " + d.Formation), refs: d.Refs, tramo: windowTramo(0, 55), zones: zones}.path()
		out.Defaults = append(out.Defaults, do)
	}
	for _, e := range a.Executes {
		zones := siteZone(e.Site)
		if e.Route != "" {
			zones = append([]string{e.Route}, zones...)
		}
		name := bandLabels[e.Buy] + " contra " + bandLabels[e.OpponentBuy]
		where := siteLabels[e.Site]
		if e.Route != "" {
			where += " por " + placename.Label(e.Route)
		}
		out.Executes = append(out.Executes, attackExecuteOut{
			Buy: cleanName(e.Buy), OpponentBuy: cleanName(e.OpponentBuy), Site: cleanName(e.Site), Route: cleanName(e.Route),
			RouteLabel: cleanName(e.RouteLabel), Rounds: e.Rounds, SharePct: e.Share.Pct, WinPct: e.WinRate.Pct, RushPct: e.Rushes.Pct,
			PlantPct: e.Plants.Pct, ContactSeconds: e.ContactSeconds, StudioPath: findingLink{title: title(name, where), refs: e.Refs, zones: zones}.path(),
		})
	}
	for _, p := range a.Players {
		out.Players = append(out.Players, attackPlayerOf(p, title))
	}
	return out
}

func attackPlayerOf(p tuiclient.AttackPlayer, title func(parts ...string) string) attackPlayerOut {
	po := attackPlayerOut{SteamID64: cleanName(p.SteamID64), Name: cleanName(p.Name), Role: cleanName(p.Role), Rounds: p.Rounds, Windows: []attackWindowOut{}}
	for _, w := range p.Windows {
		tramo := windowTramo(w.FromSeconds, w.ToSeconds)
		wo := attackWindowOut{Window: windowName(w.FromSeconds, w.ToSeconds), Tramo: tramo, Rounds: w.Rounds,
			StaticPct: w.Pace.Static.Pct, SlowPct: w.Pace.Slow.Pct, FastPct: w.Pace.Fast.Pct, Spots: []attackSpotOut{}}
		for _, sp := range w.Spots {
			label := placename.Label(sp.Callout)
			link := findingLink{title: title(cleanName(p.Name), label), refs: sp.Refs, focus: []string{p.SteamID64}, tramo: tramo, zones: []string{sp.Callout}}
			wo.Spots = append(wo.Spots, attackSpotOut{Callout: cleanName(sp.Callout), Label: cleanName(label), Rounds: sp.Share.Count,
				SharePct: sp.Share.Pct, ArrivalSeconds: sp.ArrivalSeconds, Floor: cleanName(sp.Floor), StudioPath: link.path()})
		}
		po.Windows = append(po.Windows, wo)
	}
	return po
}

// warningsOut says each warning in a sentence an assistant can repeat.
func warningsOut(warnings []tuiclient.PrepWarning) []prepWarningOut {
	out := []prepWarningOut{}
	for _, w := range warnings {
		var msg string
		switch w.Kind {
		case "stale":
			msg = fmt.Sprintf("The newest match of this sample is %d days old: the team may play differently now.", w.Days)
		case "few_rounds":
			msg = fmt.Sprintf("Only %d T rounds: read the shares as hints, not habits.", w.Rounds)
		case "roster_changed":
			parts := []string{}
			if len(w.Joined) > 0 {
				parts = append(parts, strings.Join(cleanNames(w.Joined), ", ")+" joined after the first match")
			}
			if len(w.Left) > 0 {
				parts = append(parts, strings.Join(cleanNames(w.Left), ", ")+" missed the latest matches")
			}
			msg = "The lineup changed inside the sample: " + strings.Join(parts, "; ") + "."
		default:
			continue
		}
		out = append(out, prepWarningOut{Kind: w.Kind, Message: msg})
	}
	return out
}
