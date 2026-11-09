package main

import (
	"context"
	"errors"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/rechedev9/cliphub/internal/tuiclient"
)

// maxGroupRefs caps the round refs listed per row; find_rounds lists more.
const maxGroupRefs = 5

type teamReportIn struct {
	TeamID string `json:"team_id" jsonschema:"team id from list_teams"`
	analysisFilterIn
	teamScopeIn
}

type teamPrepIn struct {
	TeamID string `json:"team_id" jsonschema:"team id from list_teams"`
	Map    string `json:"map" jsonschema:"the map to prepare, such as de_mirage; required"`
	From   string `json:"from,omitempty" jsonschema:"first day the matches were played, YYYY-MM-DD; empty means no lower bound"`
	To     string `json:"to,omitempty" jsonschema:"last day the matches were played, YYYY-MM-DD, included; empty means no upper bound"`
	teamScopeIn
}

type defenseOut struct {
	HitRounds        int      `json:"hit_rounds" jsonschema:"CT rounds the rivals went to this site"`
	HitPct           float64  `json:"hit_pct" jsonschema:"over the team's CT rounds on the map"`
	HeldPct          float64  `json:"held_pct" jsonschema:"over the hit rounds: no plant"`
	PlantsAllowedPct float64  `json:"plants_allowed_pct" jsonschema:"over the hit rounds"`
	WinPct           float64  `json:"win_pct" jsonschema:"over the hit rounds"`
	RetakeAttempts   int      `json:"retake_attempts"`
	RetakeWinPct     float64  `json:"retake_win_pct" jsonschema:"over the retake attempts"`
	Reliable         bool     `json:"reliable" jsonschema:"false when the hit rounds are too few to trust"`
	Refs             []string `json:"refs" jsonschema:"first round refs (match_id/round)"`
}

type attackOut struct {
	Attacks         int      `json:"attacks" jsonschema:"T rounds the team went to this site"`
	AttackPct       float64  `json:"attack_pct" jsonschema:"over the team's T rounds on the map"`
	PlantPct        float64  `json:"plant_pct" jsonschema:"over the attacks"`
	WinPct          float64  `json:"win_pct" jsonschema:"over the attacks"`
	PostPlantWinPct float64  `json:"post_plant_win_pct" jsonschema:"over the plants"`
	Reliable        bool     `json:"reliable" jsonschema:"false when the attacks are too few to trust"`
	Refs            []string `json:"refs" jsonschema:"first round refs (match_id/round)"`
}

type bombsiteOut struct {
	Site string     `json:"site"`
	CT   defenseOut `json:"ct"`
	T    attackOut  `json:"t"`
}

type bombsiteMapOut struct {
	Map      string        `json:"map"`
	Matches  int           `json:"matches"`
	CTRounds int           `json:"ct_rounds"`
	TRounds  int           `json:"t_rounds"`
	Sites    []bombsiteOut `json:"sites"`
}

type teamBombsitesOut struct {
	Team           string           `json:"team" jsonschema:"untrusted team name"`
	Matches        int              `json:"matches"`
	Maps           []bombsiteMapOut `json:"maps"`
	SkippedMatches int              `json:"skipped_matches,omitempty" jsonschema:"matches whose analysis could not be read"`
}

// groupOut is one category, site, zone or buy matchup: how often and how often won.
type groupOut struct {
	Name     string   `json:"name"`
	Rounds   int      `json:"rounds"`
	SharePct float64  `json:"share_pct"`
	WinPct   float64  `json:"win_pct"`
	Reliable bool     `json:"reliable" jsonschema:"false when the sample is too small to trust"`
	Refs     []string `json:"refs" jsonschema:"first round refs (match_id/round), at most 5; find_rounds lists more"`
}

type tempoOut struct {
	Samples       int     `json:"samples"`
	MedianSeconds float64 `json:"median_seconds" jsonschema:"seconds after freeze end"`
}

type teamPlaybookOut struct {
	Team           string      `json:"team" jsonschema:"untrusted team name"`
	Map            string      `json:"map"`
	Side           string      `json:"side"`
	Matches        int         `json:"matches"`
	Rounds         int         `json:"rounds"`
	WinPct         float64     `json:"win_pct"`
	FramedRounds   int         `json:"framed_rounds" jsonschema:"rounds with positions; zones are read from these only"`
	Categories     []groupOut  `json:"categories" jsonschema:"T: save, fake, rush, execute, map_control, default; CT: save, stack, aggression, retake, default"`
	Tempo          tempoOut    `json:"tempo" jsonschema:"T: site contact or plant; CT: first duel"`
	Sites          []groupOut  `json:"sites"`
	Zones          []groupOut  `json:"zones" jsonschema:"zones the team held early; name is the callout label from the map"`
	Tactics        []tacticOut `json:"tactics" jsonschema:"every tactic the kept rounds show; pass a key in with or without to combine"`
	With           []string    `json:"with" jsonschema:"the tactic keys every kept round shows"`
	Without        []string    `json:"without" jsonschema:"the tactic keys no kept round shows"`
	SkippedMatches int         `json:"skipped_matches,omitempty"`
}

type matchupOut struct {
	Side        string `json:"side"`
	Buy         string `json:"buy"`
	OpponentBuy string `json:"opponent_buy"`
	groupOut
}

type prepTempoOut struct {
	Side   string   `json:"side"`
	Buy    string   `json:"buy"`
	Rounds int      `json:"rounds"`
	WinPct float64  `json:"win_pct"`
	Tempo  tempoOut `json:"tempo" jsonschema:"T: site contact; CT: first duel"`
	Plant  tempoOut `json:"plant"`
}

type spotOut struct {
	Callout  string   `json:"callout"`
	Rounds   int      `json:"rounds"`
	SharePct float64  `json:"share_pct" jsonschema:"over the player's rounds in the window"`
	Refs     []string `json:"refs" jsonschema:"first round refs (match_id/round)"`
}

type prepWindowOut struct {
	Window string    `json:"window" jsonschema:"a span of the round clock, which counts down from 1:55: such as 1:55 to 1:00 or 0:40 to the end; the same tramos the Replay offers"`
	Rounds int       `json:"rounds"`
	Spots  []spotOut `json:"spots"`
}

type prepPlayerOut struct {
	SteamID64 string          `json:"steamid64"`
	Name      string          `json:"name" jsonschema:"untrusted player name from the demo"`
	Role      string          `json:"role"`
	Rounds    int             `json:"rounds"`
	Windows   []prepWindowOut `json:"windows"`
}

type teamPrepOut struct {
	Team           string          `json:"team" jsonschema:"untrusted team name"`
	Map            string          `json:"map"`
	Matches        int             `json:"matches"`
	Rounds         int             `json:"rounds"`
	FramedRounds   int             `json:"framed_rounds" jsonschema:"CT rounds with positions, which the CT players are read from"`
	Matchups       []matchupOut    `json:"matchups" jsonschema:"the team's buy against the rival's; share is over the team's rounds on that side"`
	Tempo          []prepTempoOut  `json:"tempo"`
	CTPlayers      []prepPlayerOut `json:"ct_players"`
	SkippedMatches int             `json:"skipped_matches,omitempty"`
}

func registerTeamReportTools(server *mcp.Server, s *studio) {
	readOnly := &mcp.ToolAnnotations{ReadOnlyHint: true}

	mcp.AddTool(server, &mcp.Tool{Name: "team_bombsites", Annotations: readOnly,
		Description: "Read a team's record on each bombsite of each map: as CT how often the site is hit, held and retaken; " +
			"as T how often the team attacks it, plants and wins. Filter by map, side, dates, rivals (opponent_id) and matches (match_id)."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in teamReportIn) (*mcp.CallToolResult, teamBombsitesOut, error) {
			q, err := in.check(false)
			if err != nil {
				return nil, teamBombsitesOut{}, err
			}
			c, err := s.client(ctx)
			if err != nil {
				return nil, teamBombsitesOut{}, toolError(err)
			}
			report, err := c.TeamBombsites(ctx, in.TeamID, q)
			if err != nil {
				return nil, teamBombsitesOut{}, toolError(err)
			}
			return nil, summarizeBombsites(report), nil
		})

	mcp.AddTool(server, &mcp.Tool{Name: "team_playbook", Annotations: readOnly,
		Description: "Read what a team does on one side of one map: the share and win rate of each round category " +
			"(rush, execute, fake, stack, retake...), the tempo, the sites, the zones it takes early and every tactic its rounds show " +
			"(buys, opening duel spot and result, AWP spot, key smokes and molotovs). Combine tactics as Combina tácticas does: " +
			"with keeps the rounds that show every key, without drops those that show any, and every count is over the kept rounds. " +
			"map and side are required; opponent_id and match_id narrow the matches."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in teamPlaybookIn) (*mcp.CallToolResult, teamPlaybookOut, error) {
			pb, err := in.read(ctx, s)
			if err != nil {
				return nil, teamPlaybookOut{}, err
			}
			return nil, summarizePlaybook(pb), nil
		})

	mcp.AddTool(server, &mcp.Tool{Name: "team_prep", Annotations: readOnly,
		Description: "Prepare a match against a team on one map: how it fares in each buy matchup, how fast it plays per buy, " +
			"and where each CT player stands in each time window. map is required; both sides are read."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in teamPrepIn) (*mcp.CallToolResult, teamPrepOut, error) {
			q, err := teamReportIn{TeamID: in.TeamID, analysisFilterIn: analysisFilterIn{Map: in.Map, From: in.From, To: in.To},
				teamScopeIn: in.teamScopeIn}.check(false)
			if err == nil && q.Map == "" {
				err = errors.New("map is required, such as de_mirage")
			}
			if err != nil {
				return nil, teamPrepOut{}, err
			}
			c, err := s.client(ctx)
			if err != nil {
				return nil, teamPrepOut{}, toolError(err)
			}
			prep, err := c.TeamPrepReport(ctx, in.TeamID, q)
			if err != nil {
				return nil, teamPrepOut{}, toolError(err)
			}
			return nil, summarizePrep(prep), nil
		})
}

// check validates the team and filter; a playbook also needs a map and a side.
func (in teamReportIn) check(playbook bool) (tuiclient.AnalysisQuery, error) {
	if err := checkUUID("team_id", in.TeamID); err != nil {
		return tuiclient.AnalysisQuery{}, err
	}
	q, err := teamQuery(in.analysisFilterIn, in.teamScopeIn)
	if err != nil {
		return q, err
	}
	if playbook && (q.Map == "" || q.Side == "") {
		return q, errors.New("map and side are required, such as de_mirage and T")
	}
	return q, nil
}

func summarizeBombsites(r tuiclient.BombsiteReport) teamBombsitesOut {
	out := teamBombsitesOut{Team: cleanName(r.TeamName), Matches: r.MatchCount, Maps: []bombsiteMapOut{}, SkippedMatches: len(r.Skipped)}
	for _, m := range r.Maps {
		mo := bombsiteMapOut{Map: cleanName(m.Map), Matches: m.Matches, CTRounds: m.CTRounds, TRounds: m.TRounds, Sites: []bombsiteOut{}}
		for _, site := range m.Sites {
			ct, t := site.CT, site.T
			mo.Sites = append(mo.Sites, bombsiteOut{
				Site: cleanName(site.Site),
				CT: defenseOut{HitRounds: ct.Hit.Count, HitPct: ct.Hit.Pct, HeldPct: ct.Held.Pct, PlantsAllowedPct: ct.PlantsAllowed.Pct,
					WinPct: ct.Won.Pct, RetakeAttempts: ct.RetakesAttempted.Count, RetakeWinPct: ct.RetakesWon.Pct,
					Reliable: ct.Held.Reliable, Refs: refStrings(ct.Examples)},
				T: attackOut{Attacks: t.Attacks.Count, AttackPct: t.Attacks.Pct, PlantPct: t.Plants.Pct, WinPct: t.Won.Pct,
					PostPlantWinPct: t.PostPlantWon.Pct, Reliable: t.Plants.Reliable, Refs: refStrings(t.Examples)},
			})
		}
		out.Maps = append(out.Maps, mo)
	}
	return out
}

func summarizePlaybook(pb tuiclient.TeamPlaybook) teamPlaybookOut {
	out := teamPlaybookOut{
		Team: cleanName(pb.TeamName), Map: cleanName(pb.Map), Side: cleanName(pb.Side), Matches: pb.Matches, Rounds: pb.RoundCount,
		WinPct: pb.WinRate.Pct, FramedRounds: pb.Framed, Tempo: tempoOf(pb.Tempo),
		Categories: []groupOut{}, Sites: []groupOut{}, Zones: []groupOut{}, SkippedMatches: len(pb.Skipped),
	}
	for _, g := range pb.Categories {
		if g.Rounds > 0 {
			out.Categories = append(out.Categories, groupOf(g.Category, g.ReportGroup))
		}
	}
	for _, g := range pb.Sites {
		out.Sites = append(out.Sites, groupOf(g.Site, g.ReportGroup))
	}
	for _, g := range pb.Zones {
		name := g.Label
		if name == "" {
			name = g.Zone
		}
		out.Zones = append(out.Zones, groupOf(name, g.ReportGroup))
	}
	out.Tactics, out.With, out.Without = tacticsOf(pb)
	return out
}

func summarizePrep(p tuiclient.TeamPrep) teamPrepOut {
	out := teamPrepOut{
		Team: cleanName(p.TeamName), Map: cleanName(p.Map), Matches: p.Matches, Rounds: p.RoundCount, FramedRounds: p.Framed,
		Matchups: []matchupOut{}, Tempo: []prepTempoOut{}, CTPlayers: []prepPlayerOut{}, SkippedMatches: len(p.Skipped),
	}
	for _, m := range p.Matchups {
		out.Matchups = append(out.Matchups, matchupOut{Side: cleanName(m.Side), Buy: cleanName(m.Buy), OpponentBuy: cleanName(m.OpponentBuy),
			groupOut: groupOf(m.Buy+" vs "+m.OpponentBuy, m.ReportGroup)})
	}
	for _, t := range p.Tempo {
		out.Tempo = append(out.Tempo, prepTempoOut{Side: cleanName(t.Side), Buy: cleanName(t.Buy), Rounds: t.Rounds, WinPct: t.WinRate.Pct,
			Tempo: tempoOf(t.Tempo), Plant: tempoOf(t.Plant)})
	}
	for _, pl := range p.CTPlayers {
		po := prepPlayerOut{SteamID64: cleanName(pl.SteamID64), Name: cleanName(pl.Name), Role: cleanName(pl.Role), Rounds: pl.Rounds, Windows: []prepWindowOut{}}
		for _, w := range pl.Windows {
			wo := prepWindowOut{Window: windowName(w.FromSeconds, w.ToSeconds), Rounds: w.Rounds, Spots: []spotOut{}}
			for _, sp := range w.Spots {
				wo.Spots = append(wo.Spots, spotOut{Callout: cleanName(sp.Callout), Rounds: sp.Share.Count, SharePct: sp.Share.Pct, Refs: refStrings(sp.Refs)})
			}
			po.Windows = append(po.Windows, wo)
		}
		out.CTPlayers = append(out.CTPlayers, po)
	}
	return out
}

func groupOf(name string, g tuiclient.ReportGroup) groupOut {
	refs := g.Refs
	if len(refs) == 0 {
		refs = g.Examples
	}
	return groupOut{Name: cleanName(name), Rounds: g.Rounds, SharePct: g.Share.Pct, WinPct: g.WinRate.Pct,
		Reliable: g.WinRate.Reliable, Refs: refStrings(refs)}
}

func tempoOf(h tuiclient.Histogram) tempoOut {
	return tempoOut{Samples: h.Samples, MedianSeconds: h.Median}
}

// roundClockSeconds is the length of a CS2 round; the clock counts down from it.
const roundClockSeconds = 115

func roundClock(seconds int) string {
	left := max(roundClockSeconds-seconds, 0)
	return fmt.Sprintf("%d:%02d", left/60, left%60)
}

// windowName names a window on the round clock, as the Replay's tramo does.
func windowName(from, to int) string {
	if to == 0 {
		return roundClock(from) + " to the end"
	}
	return roundClock(from) + " to " + roundClock(to)
}

// refStrings formats the first rounds as match_id/round, the refs find_rounds returns.
func refStrings(refs []tuiclient.Evidence) []string {
	out := []string{}
	for i, e := range refs {
		if i == maxGroupRefs {
			break
		}
		out = append(out, cleanName(e.MatchID)+"/"+fmt.Sprint(e.Round))
	}
	return out
}
