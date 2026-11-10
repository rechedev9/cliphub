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
// Only a section's top row carries a studio_path; the rest list example refs.
type attackCellOut struct {
	Key      string   `json:"key"`
	Rounds   int      `json:"rounds"`
	SharePct float64  `json:"share_pct" jsonschema:"over the rounds of its row"`
	WinPct   float64  `json:"win_pct"`
	Reliable bool     `json:"reliable" jsonschema:"false when the sample is too small to trust"`
	Examples []string `json:"examples,omitempty" jsonschema:"up to 5 round refs (match_id/round); matchup cells leave them out"`
	linkOut
}

type attackMatchupOut struct {
	Buy         string          `json:"buy" jsonschema:"the team's buy band: pistol, eco, half (semi or force) or full"`
	OpponentBuy string          `json:"opponent_buy"`
	Rounds      int             `json:"rounds"`
	SharePct    float64         `json:"share_pct" jsonschema:"over every T round"`
	WinPct      float64         `json:"win_pct"`
	Reliable    bool            `json:"reliable"`
	Examples    []string        `json:"examples"`
	Sites       []attackCellOut `json:"sites" jsonschema:"A, B, both or none; share is over the matchup's rounds"`
	Tempo       []attackCellOut `json:"tempo" jsonschema:"rush, set_strat, mid_round or no_site; share is over the matchup's rounds"`
	linkOut
}

type attackZoneOut struct {
	Callout  string  `json:"callout" jsonschema:"CS2 place token"`
	Label    string  `json:"label"`
	Rounds   int     `json:"rounds"`
	SharePct float64 `json:"share_pct" jsonschema:"over the default's rounds"`
}

type attackDefaultOut struct {
	Formation string          `json:"formation" jsonschema:"players towards A, mid and towards B at the read second, such as 2-1-2"`
	Rounds    int             `json:"rounds"`
	SharePct  float64         `json:"share_pct" jsonschema:"usage: over the T rounds with positions"`
	WinPct    float64         `json:"win_pct"`
	Reliable  bool            `json:"reliable"`
	Zones     []attackZoneOut `json:"zones" jsonschema:"map control zones held in these rounds"`
	Examples  []string        `json:"examples"`
	linkOut
}

type attackExecuteOut struct {
	Buy            string   `json:"buy"`
	OpponentBuy    string   `json:"opponent_buy"`
	Site           string   `json:"site" jsonschema:"A, B or both (both sites before the plant)"`
	Route          string   `json:"route" jsonschema:"the callout the first players on site came from; empty when unread"`
	RouteLabel     string   `json:"route_label"`
	Rounds         int      `json:"rounds"`
	SharePct       float64  `json:"share_pct" jsonschema:"over the matchup's rounds with positions"`
	WinPct         float64  `json:"win_pct"`
	RushPct        float64  `json:"rush_pct" jsonschema:"rounds of it that were rushes; the rest are set strats"`
	PlantPct       float64  `json:"plant_pct"`
	ContactSeconds *float64 `json:"contact_seconds" jsonschema:"median site contact, seconds after freeze end; null when no round read it"`
	Examples       []string `json:"examples"`
	linkOut
}

type attackSpotOut struct {
	Callout        string   `json:"callout" jsonschema:"CS2 place token"`
	Label          string   `json:"label"`
	Rounds         int      `json:"rounds"`
	SharePct       float64  `json:"share_pct" jsonschema:"over the player's rounds in the window"`
	ArrivalSeconds float64  `json:"arrival_seconds" jsonschema:"median second of the round the player first stood there"`
	Floor          string   `json:"floor,omitempty"`
	Examples       []string `json:"examples"`
	linkOut
}

type attackWindowOut struct {
	Window    string          `json:"window" jsonschema:"a span of the round clock, which counts down from 1:55"`
	Tramo     string          `json:"tramo" jsonschema:"the same span in seconds after freeze end, as the Replay's tramo"`
	Rounds    int             `json:"rounds"`
	StaticPct float64         `json:"static_pct" jsonschema:"rounds the player held still in the window"`
	SlowPct   float64         `json:"slow_pct" jsonschema:"rounds the player walked"`
	FastPct   float64         `json:"fast_pct" jsonschema:"rounds the player ran"`
	Spots     []attackSpotOut `json:"spots" jsonschema:"top 3, each with up to 3 example refs; the first spot of the first window carries the player's studio_path"`
}

type attackPlayerOut struct {
	SteamID64 string            `json:"steamid64"`
	Name      string            `json:"name" jsonschema:"untrusted player name from the demo"`
	Role      string            `json:"role"`
	Rounds    int               `json:"rounds"`
	Windows   []attackWindowOut `json:"windows"`
}

type prepWarningOut struct {
	Kind    string `json:"kind" jsonschema:"stale, few_rounds, roster_changed or capped"`
	Message string `json:"message"`
}

// attackSectionOut opens every round of one section in the Replay.
type attackSectionOut struct {
	Rounds int `json:"rounds"`
	linkOut
}

type teamAttackOut struct {
	Team           string             `json:"team" jsonschema:"untrusted team name"`
	Map            string             `json:"map"`
	Matches        int                `json:"matches"`
	Rounds         int                `json:"rounds" jsonschema:"T rounds read"`
	FramedRounds   int                `json:"framed_rounds" jsonschema:"T rounds with positions; defaults, executes and players read only these"`
	LastPlayedAt   string             `json:"last_played_at,omitempty"`
	DaysSinceLast  int                `json:"days_since_last_match"`
	Warnings       []prepWarningOut   `json:"warnings" jsonschema:"what to distrust in this sample; repeat them in a report"`
	AllRounds      attackSectionOut   `json:"all_rounds" jsonschema:"every T round: the section link of sites, tempo, matchups and players"`
	Sites          []attackCellOut    `json:"sites" jsonschema:"where the T rounds went: A, B, both or none"`
	Tempo          []attackCellOut    `json:"tempo" jsonschema:"how they got there: rush, set_strat (execute or fake), mid_round or no_site"`
	Matchups       []attackMatchupOut `json:"matchups" jsonschema:"the team's buy band against the rival's, each crossed with sites and tempo"`
	DefaultRounds  attackSectionOut   `json:"default_rounds" jsonschema:"every round a default was read from"`
	Defaults       []attackDefaultOut `json:"defaults" jsonschema:"preferred defaults, most used first"`
	ExecuteRounds  attackSectionOut   `json:"execute_rounds" jsonschema:"every round an execute was read from"`
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
		Description: "Prepare for a team's T side on one map, as Equipos > Preparar (Ataque en T) and Informes > Ataque show it: where each buy matchup goes " +
			"(A, B, both or no site, crossed with pistol, eco, half and full buys on both teams), how fast (rush, set strat, mid round), " +
			"the preferred defaults with usage and win rate, the preferred executes per buy with their entry route, and each T player's " +
			"spots and pace per tramo of the round clock. Each row gives its rounds and up to 5 example refs; all_rounds, default_rounds, " +
			"execute_rounds and the top row of each list carry a studio_path that opens exactly those rounds in the 2D Replay (rounds_linked when cut at 200), " +
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

// topRow is the index of the row with the most rounds, the only one of its list that carries a link.
func topRow(n int, rounds func(int) int) int {
	best := -1
	for i := range n {
		if rounds(i) > 0 && (best < 0 || rounds(i) > rounds(best)) {
			best = i
		}
	}
	return best
}

// section links every round of a list's rows.
func section(link findingLink, refs ...[]tuiclient.Evidence) attackSectionOut {
	for _, r := range refs {
		link.refs = append(link.refs, r...)
	}
	return attackSectionOut{Rounds: len(link.roundKeys()), linkOut: link.out()}
}

func summarizeAttack(a tuiclient.TeamAttack) teamAttackOut {
	team := cleanName(a.TeamName)
	title := func(parts ...string) string {
		return strings.Join(append([]string{team, cleanName(a.Map)}, parts...), " · ")
	}
	cells := func(list []tuiclient.AttackCell, link func(tuiclient.AttackCell) findingLink) []attackCellOut {
		out := []attackCellOut{}
		top := topRow(len(list), func(i int) int { return list[i].Rounds })
		for i, c := range list {
			row := attackCellOut{Key: cleanName(c.Key), Rounds: c.Rounds, SharePct: c.Share.Pct, WinPct: c.WinRate.Pct,
				Reliable: c.WinRate.Reliable, Examples: exampleRefs(c.Refs)}
			if i == top {
				l := link(c)
				l.refs = c.Refs
				row.linkOut = l.out()
			}
			out = append(out, row)
		}
		return out
	}
	siteLink := func(prefix ...string) func(tuiclient.AttackCell) findingLink {
		return func(c tuiclient.AttackCell) findingLink {
			return findingLink{title: title(append(prefix, siteLabels[c.Key])...), zones: siteZone(c.Key)}
		}
	}
	tempoLink := func(prefix ...string) func(tuiclient.AttackCell) findingLink {
		return func(c tuiclient.AttackCell) findingLink {
			return findingLink{title: title(append(prefix, tempoLabels[c.Key])...)}
		}
	}
	out := teamAttackOut{
		Team: team, Map: cleanName(a.Map), Matches: a.Matches, Rounds: a.RoundCount, FramedRounds: a.Framed,
		LastPlayedAt: cleanName(a.LastPlayedAt), DaysSinceLast: a.DaysSince, Warnings: warningsOut(a.Warnings),
		Matchups: []attackMatchupOut{}, Defaults: []attackDefaultOut{}, Executes: []attackExecuteOut{}, Players: []attackPlayerOut{},
		SkippedMatches: len(a.Skipped),
	}
	all := [][]tuiclient.Evidence{}
	for _, c := range a.Sites {
		all = append(all, c.Refs)
	}
	out.AllRounds = section(findingLink{title: title("rondas de T")}, all...)
	out.Sites = cells(a.Sites, siteLink())
	out.Tempo = cells(a.Tempo, tempoLink())
	topMatchup := topRow(len(a.Matchups), func(i int) int { return a.Matchups[i].Rounds })
	for i, m := range a.Matchups {
		vs := bandLabels[m.Buy] + " contra " + bandLabels[m.OpponentBuy]
		mo := attackMatchupOut{Buy: cleanName(m.Buy), OpponentBuy: cleanName(m.OpponentBuy), Rounds: m.Rounds, SharePct: m.Share.Pct,
			WinPct: m.WinRate.Pct, Reliable: m.WinRate.Reliable, Examples: exampleRefs(m.Refs)}
		if i == topMatchup {
			mo.linkOut = findingLink{title: title(vs), refs: m.Refs}.out()
		}
		// A matchup's cells give counts only: the matchup's examples and link cover their rounds.
		mo.Sites, mo.Tempo = nonEmptyCells(m.Sites), nonEmptyCells(m.Tempo)
		out.Matchups = append(out.Matchups, mo)
	}
	defaults := [][]tuiclient.Evidence{}
	for i, d := range a.Defaults {
		do := attackDefaultOut{Formation: cleanName(d.Formation), Rounds: d.Rounds, SharePct: d.Share.Pct, WinPct: d.WinRate.Pct,
			Reliable: d.WinRate.Reliable, Zones: []attackZoneOut{}, Examples: exampleRefs(d.Refs)}
		zones := []string{}
		for _, z := range d.Zones {
			do.Zones = append(do.Zones, attackZoneOut{Callout: cleanName(z.Zone), Label: cleanName(z.Label), Rounds: z.Rounds, SharePct: z.Share.Pct})
			zones = append(zones, z.Zone)
		}
		if i == 0 {
			do.linkOut = findingLink{title: title("default " + d.Formation), refs: d.Refs, tramo: windowTramo(0, 55), zones: zones}.out()
		}
		defaults = append(defaults, d.Refs)
		out.Defaults = append(out.Defaults, do)
	}
	out.DefaultRounds = section(findingLink{title: title("defaults"), tramo: windowTramo(0, 55)}, defaults...)
	executes := [][]tuiclient.Evidence{}
	topExecute := topRow(len(a.Executes), func(i int) int { return a.Executes[i].Rounds })
	for i, e := range a.Executes {
		row := attackExecuteOut{
			Buy: cleanName(e.Buy), OpponentBuy: cleanName(e.OpponentBuy), Site: cleanName(e.Site), Route: cleanName(e.Route),
			RouteLabel: cleanName(e.RouteLabel), Rounds: e.Rounds, SharePct: e.Share.Pct, WinPct: e.WinRate.Pct, RushPct: e.Rushes.Pct,
			PlantPct: e.Plants.Pct, ContactSeconds: e.ContactSeconds, Examples: exampleRefs(e.Refs),
		}
		if i == topExecute {
			zones := siteZone(e.Site)
			where := siteLabels[e.Site]
			if e.Route != "" {
				zones = append([]string{e.Route}, zones...)
				where += " por " + placename.Label(e.Route)
			}
			row.linkOut = findingLink{title: title(bandLabels[e.Buy]+" contra "+bandLabels[e.OpponentBuy], where), refs: e.Refs, zones: zones}.out()
		}
		executes = append(executes, e.Refs)
		out.Executes = append(out.Executes, row)
	}
	out.ExecuteRounds = section(findingLink{title: title("ejecuciones")}, executes...)
	for _, p := range a.Players {
		out.Players = append(out.Players, attackPlayerOf(p, title))
	}
	return out
}

func nonEmptyCells(list []tuiclient.AttackCell) []attackCellOut {
	out := []attackCellOut{}
	for _, c := range list {
		if c.Rounds > 0 {
			out = append(out, attackCellOut{Key: cleanName(c.Key), Rounds: c.Rounds, SharePct: c.Share.Pct, WinPct: c.WinRate.Pct,
				Reliable: c.WinRate.Reliable})
		}
	}
	return out
}

// Each window lists its top spots with fewer examples: five players by three windows add up.
const (
	maxAttackSpots  = 3
	maxSpotExamples = 3
)

// attackPlayerOf lists a player's spots with examples; their most held spot of the first window carries the link.
func attackPlayerOf(p tuiclient.AttackPlayer, title func(parts ...string) string) attackPlayerOut {
	po := attackPlayerOut{SteamID64: cleanName(p.SteamID64), Name: cleanName(p.Name), Role: cleanName(p.Role), Rounds: p.Rounds, Windows: []attackWindowOut{}}
	linked := false
	for _, w := range p.Windows {
		tramo := windowTramo(w.FromSeconds, w.ToSeconds)
		wo := attackWindowOut{Window: windowName(w.FromSeconds, w.ToSeconds), Tramo: tramo, Rounds: w.Rounds,
			StaticPct: w.Pace.Static.Pct, SlowPct: w.Pace.Slow.Pct, FastPct: w.Pace.Fast.Pct, Spots: []attackSpotOut{}}
		for i, sp := range w.Spots {
			if i == maxAttackSpots {
				break
			}
			label := placename.Label(sp.Callout)
			row := attackSpotOut{Callout: cleanName(sp.Callout), Label: cleanName(label), Rounds: sp.Share.Count,
				SharePct: sp.Share.Pct, ArrivalSeconds: sp.ArrivalSeconds, Floor: cleanName(sp.Floor), Examples: examplesOf(sp.Refs, maxSpotExamples)}
			if !linked {
				linked = true
				row.linkOut = findingLink{title: title(cleanName(p.Name), label), refs: sp.Refs, focus: []string{p.SteamID64}, tramo: tramo, zones: []string{sp.Callout}}.out()
			}
			wo.Spots = append(wo.Spots, row)
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
		case "capped":
			msg = fmt.Sprintf("Read the newest %d of %d matches: narrow the dates or rivals to read the rest.", w.Matches, w.Available)
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
