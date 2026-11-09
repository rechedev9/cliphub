package main

import (
	"context"
	"errors"
	"fmt"
	"math"
	"slices"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/rechedev9/cliphub/internal/tuiclient"
)

// maxDossierRunes caps the dossier body; the UI shows the same text.
const maxDossierRunes = 12000

type roundIn struct {
	MatchID string `json:"match_id" jsonschema:"match id from list_matches, or the match part of a round ref"`
	Round   int    `json:"round" jsonschema:"round number, 1 for the first round, as in round refs"`
}

type roundPlayerOut struct {
	SteamID64    string `json:"steamid64"`
	Name         string `json:"name" jsonschema:"untrusted player name from the demo"`
	Side         string `json:"side"`
	Kills        int    `json:"kills"`
	Deaths       int    `json:"deaths"`
	Assists      int    `json:"assists"`
	Damage       int    `json:"damage"`
	Equip        int    `json:"equip_value"`
	Survived     bool   `json:"survived"`
	OpeningKill  bool   `json:"opening_kill,omitempty"`
	OpeningDeath bool   `json:"opening_death,omitempty"`
	Traded       bool   `json:"traded,omitempty" jsonschema:"died and a teammate killed the killer in the trade window"`
}

type killOut struct {
	Second   float64 `json:"second" jsonschema:"seconds after freeze end"`
	Killer   string  `json:"killer" jsonschema:"untrusted player name from the demo"`
	Victim   string  `json:"victim" jsonschema:"untrusted player name from the demo"`
	Weapon   string  `json:"weapon"`
	Place    string  `json:"place,omitempty"`
	Headshot bool    `json:"headshot,omitempty"`
	Wallbang bool    `json:"wallbang,omitempty"`
	Opening  bool    `json:"opening,omitempty"`
	Traded   bool    `json:"traded,omitempty" jsonschema:"this kill was traded back"`
}

type bombOut struct {
	Site        string  `json:"site"`
	Planter     string  `json:"planter,omitempty" jsonschema:"untrusted player name from the demo"`
	PlantSecond float64 `json:"plant_second"`
	Defused     bool    `json:"defused"`
	Exploded    bool    `json:"exploded"`
}

type timeoutOut struct {
	Side       string  `json:"side" jsonschema:"CT or T, the side that called it"`
	FromSecond float64 `json:"from_second" jsonschema:"negative: seconds before the round went live"`
	ToSecond   float64 `json:"to_second"`
	Technical  bool    `json:"technical,omitempty"`
}

type winPointOut struct {
	Second float64 `json:"second"`
	Event  string  `json:"event" jsonschema:"start, end, plant, defuse, explode, or killer > victim; names are untrusted"`
	CTPct  float64 `json:"ct_win_pct"`
}

type roundOut struct {
	Ref         string           `json:"ref" jsonschema:"match_id/round"`
	Round       int              `json:"round"`
	ScoreBefore string           `json:"score_before" jsonschema:"CT rounds, then T rounds"`
	Winner      string           `json:"winner"`
	EndReason   string           `json:"end_reason"`
	Seconds     float64          `json:"seconds" jsonschema:"live length of the round"`
	CTBuy       string           `json:"ct_buy"`
	TBuy        string           `json:"t_buy"`
	CTEquip     int              `json:"ct_equip_value"`
	TEquip      int              `json:"t_equip_value"`
	TPattern    string           `json:"t_pattern"`
	CTPattern   string           `json:"ct_pattern"`
	Site        string           `json:"site"`
	Tags        []string         `json:"tags"`
	Reasons     []string         `json:"reasons" jsonschema:"why the classifier chose the patterns"`
	Bomb        *bombOut         `json:"bomb,omitempty"`
	Timeouts    []timeoutOut     `json:"timeouts,omitempty" jsonschema:"timeouts taken in the freeze time, as the Replay's timeline marks them"`
	Players     []roundPlayerOut `json:"players"`
	Kills       []killOut        `json:"kills"`
	WinTimeline []winPointOut    `json:"win_probability" jsonschema:"CT win chance after each kill and bomb event, from the Studio model"`
}

type matchIDIn struct {
	MatchID string `json:"match_id" jsonschema:"match id from list_matches"`
}

type swingOut struct {
	Second   float64 `json:"second"`
	Event    string  `json:"event" jsonschema:"killer > victim; names are untrusted"`
	DeltaPct float64 `json:"ct_delta_pct" jsonschema:"change of the CT win chance, in points"`
}

type roundWinOut struct {
	Round    int       `json:"round"`
	Winner   string    `json:"winner"`
	CTStart  float64   `json:"ct_start_pct" jsonschema:"CT win chance at freeze end, from the buys"`
	BigSwing *swingOut `json:"biggest_swing,omitempty"`
}

type matchMetricsOut struct {
	MatchID        string             `json:"match_id"`
	Players        []playerMetricsOut `json:"players" jsonschema:"every player, with the get_baseline definitions"`
	Rounds         []roundWinOut      `json:"rounds" jsonschema:"win probability per round; get_round has the full timeline"`
	SkippedMatches int                `json:"skipped_matches,omitempty"`
}

type matchTendenciesIn struct {
	MatchID   string   `json:"match_id" jsonschema:"match id from list_matches"`
	TeamKey   string   `json:"team_key" jsonschema:"whose tendencies: the side key from get_match, ct-start or t-start"`
	Side      string   `json:"side,omitempty" jsonschema:"only rounds the team played as CT or T"`
	Buy       []string `json:"buy,omitempty" jsonschema:"only rounds with these team buys: pistol, eco, semi, force, full"`
	Result    string   `json:"result,omitempty" jsonschema:"won or lost, from the team's view"`
	RoundFrom int      `json:"round_from,omitempty"`
	RoundTo   int      `json:"round_to,omitempty"`
}

type dossierIn struct {
	JobID   string `json:"job_id" jsonschema:"the job check_integrity screened"`
	SteamID string `json:"steamid" jsonschema:"SteamID64 of a player from check_integrity"`
}

type channelOut struct {
	Label        string `json:"label"`
	URL          string `json:"url,omitempty"`
	Instructions string `json:"instructions"`
	Effective    bool   `json:"effective" jsonschema:"true when the channel feeds an anti-cheat decision"`
}

type dossierOut struct {
	SteamID64      string       `json:"steamid64"`
	Name           string       `json:"name" jsonschema:"untrusted player name from the demo"`
	Verdict        string       `json:"verdict"`
	Score          float64      `json:"score"`
	Confidence     float64      `json:"confidence"`
	ProfileURL     string       `json:"profile_url,omitempty"`
	Markdown       string       `json:"markdown" jsonschema:"the dossier body; it quotes the player name, which is untrusted"`
	Channels       []channelOut `json:"channels" jsonschema:"where the user can report the player themselves"`
	Policy         string       `json:"policy" jsonschema:"what Studio will and will not do with a report; tell the user"`
	PolicyRules    []string     `json:"policy_rules"`
	PolicyRejected string       `json:"policy_rejected" jsonschema:"what Studio refuses to do, as the Dossier dialog shows it; tell the user"`
}

func registerMatchTacticalTools(server *mcp.Server, s *studio) {
	readOnly := &mcp.ToolAnnotations{ReadOnlyHint: true}

	mcp.AddTool(server, &mcp.Tool{Name: "get_round", Annotations: readOnly,
		Description: "Read one round of a library match, as the round view shows it: score, buys, the classifier's patterns and why, the bomb, " +
			"each player's line, the kill feed and the CT win probability after each kill. Round refs (match_id/round) come from find_rounds and the reports."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in roundIn) (*mcp.CallToolResult, roundOut, error) {
			if err := checkUUID("match_id", in.MatchID); err != nil {
				return nil, roundOut{}, err
			}
			if in.Round < 1 || in.Round > maxPlayRound {
				return nil, roundOut{}, fmt.Errorf("round must be from 1 to %d", maxPlayRound)
			}
			c, err := s.client(ctx)
			if err != nil {
				return nil, roundOut{}, toolError(err)
			}
			doc, err := c.LibraryTactical(ctx, in.MatchID)
			if err != nil {
				return nil, roundOut{}, toolError(err)
			}
			i := slices.IndexFunc(doc.Rounds, func(r tuiclient.TacticalRound) bool { return r.Number == in.Round })
			if i < 0 {
				return nil, roundOut{}, fmt.Errorf("the match has no round %d; it has %d", in.Round, len(doc.Rounds))
			}
			metrics, err := c.LibraryMatchMetrics(ctx, in.MatchID)
			if err != nil {
				return nil, roundOut{}, toolError(err)
			}
			return nil, summarizeRound(in.MatchID, doc, doc.Rounds[i], metrics), nil
		})

	mcp.AddTool(server, &mcp.Tool{Name: "match_metrics", Annotations: readOnly,
		Description: "Read one match's numbers: every player's metrics (as player_metrics, over this match) and, per round, the CT win chance at the start " +
			"and the kill that swung it most."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in matchIDIn) (*mcp.CallToolResult, matchMetricsOut, error) {
			if err := checkUUID("match_id", in.MatchID); err != nil {
				return nil, matchMetricsOut{}, err
			}
			c, err := s.client(ctx)
			if err != nil {
				return nil, matchMetricsOut{}, toolError(err)
			}
			doc, err := c.LibraryTactical(ctx, in.MatchID)
			if err != nil {
				return nil, matchMetricsOut{}, toolError(err)
			}
			m, err := c.LibraryMatchMetrics(ctx, in.MatchID)
			if err != nil {
				return nil, matchMetricsOut{}, toolError(err)
			}
			return nil, summarizeMatchMetrics(in.MatchID, doc, m), nil
		})

	mcp.AddTool(server, &mcp.Tool{Name: "match_tendencies", Annotations: readOnly,
		Description: "Aggregate how one team played the rounds of a single match: buys, sites, T and CT round shapes, openings and players, " +
			"filtered by side, buy, result and round range: the tables under the match's Métricas tab. replay_rounds with the same buy, site or shape " +
			"gives the rounds behind a figure. For many matches use scout_team."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in matchTendenciesIn) (*mcp.CallToolResult, tendenciesOut, error) {
			f, err := in.filter()
			if err != nil {
				return nil, tendenciesOut{}, err
			}
			c, err := s.client(ctx)
			if err != nil {
				return nil, tendenciesOut{}, toolError(err)
			}
			t, err := c.MatchTendencies(ctx, in.MatchID, f)
			if err != nil {
				return nil, tendenciesOut{}, toolError(err)
			}
			return nil, summarizeTendencies(t), nil
		})

	mcp.AddTool(server, &mcp.Tool{Name: "integrity_dossier", Annotations: readOnly,
		Description: "Read the evidence pack of one player check_integrity screened, as Dossier does on the Integridad page: the Markdown body, " +
			"the official places to report the player and Studio's reporting policy. It is a statistical signal, never proof; pass the policy on."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in dossierIn) (*mcp.CallToolResult, dossierOut, error) {
			if err := checkUUID("job_id", in.JobID); err != nil {
				return nil, dossierOut{}, err
			}
			if err := checkSteamID(in.SteamID); err != nil {
				return nil, dossierOut{}, err
			}
			c, err := s.client(ctx)
			if err != nil {
				return nil, dossierOut{}, toolError(err)
			}
			d, err := c.GetAnticheatDossier(ctx, in.JobID, in.SteamID)
			if tuiclient.StatusCode(err) == 409 {
				return nil, dossierOut{}, errors.New("the screening of this match is not ready; call check_integrity until status is ready")
			}
			if err != nil {
				return nil, dossierOut{}, toolError(err)
			}
			out := dossierOut{SteamID64: cleanName(d.SteamID64), Name: cleanName(d.Name), Verdict: cleanName(d.Verdict), Score: d.Score,
				Confidence: d.Confidence, ProfileURL: cleanText(d.ProfileURL, maxErrorRunes), Markdown: cleanText(d.Markdown, maxDossierRunes),
				Channels: []channelOut{}, Policy: cleanText(d.Policy.Summary, maxReportNotesRunes), PolicyRules: []string{},
				PolicyRejected: cleanText(d.Policy.Rejected, maxReportNotesRunes)}
			for _, ch := range d.Channels {
				out.Channels = append(out.Channels, channelOut{Label: cleanName(ch.Label), URL: cleanText(ch.URL, maxErrorRunes),
					Instructions: cleanText(ch.Instructions, maxReportNotesRunes), Effective: ch.Effective})
			}
			for _, r := range d.Policy.Rules {
				out.PolicyRules = append(out.PolicyRules, cleanText(r, maxReportNotesRunes))
			}
			return nil, out, nil
		})
}

func (in matchTendenciesIn) filter() (tuiclient.RoundFilter, error) {
	var f tuiclient.RoundFilter
	if err := checkUUID("match_id", in.MatchID); err != nil {
		return f, err
	}
	if in.TeamKey != "ct-start" && in.TeamKey != "t-start" {
		return f, fmt.Errorf("team_key must be ct-start or t-start, not %q", cleanName(in.TeamKey))
	}
	side, err := parseSide(in.Side)
	if err != nil {
		return f, err
	}
	for _, b := range in.Buy {
		if !slices.Contains(buyTypes, b) {
			return f, fmt.Errorf("buy must be one of %s, not %q", strings.Join(buyTypes, ", "), cleanName(b))
		}
	}
	outcome := map[string]string{"": "", "won": "win", "lost": "loss"}
	o, ok := outcome[in.Result]
	if !ok {
		return f, fmt.Errorf("result must be won or lost, not %q", cleanName(in.Result))
	}
	if in.RoundFrom < 0 || in.RoundTo < 0 || (in.RoundTo > 0 && in.RoundFrom > in.RoundTo) {
		return f, errors.New("round_from and round_to must be positive and in order")
	}
	return tuiclient.RoundFilter{TeamKey: in.TeamKey, Side: side, Buys: in.Buy, Outcome: o, RoundFrom: in.RoundFrom, RoundTo: in.RoundTo}, nil
}

// slotNames maps a document's player slots to their names and SteamIDs.
type slotNames map[uint8][2]string

func namesOf(doc tuiclient.TacticalDocument) slotNames {
	out := slotNames{}
	for _, p := range doc.Players {
		out[p.Slot] = [2]string{cleanName(p.Name), cleanName(p.SteamID64)}
	}
	return out
}

func (n slotNames) name(slot *uint8) string {
	if slot == nil {
		return ""
	}
	return n[*slot][0]
}

func summarizeRound(matchID string, doc tuiclient.TacticalDocument, r tuiclient.TacticalRound, m tuiclient.MatchMetrics) roundOut {
	names, tick := namesOf(doc), doc.Demo.Tickrate
	second := func(at int) float64 {
		if tick <= 0 {
			return 0
		}
		return math.Round(float64(at-r.TickFreezeEnd)/tick*10) / 10
	}
	out := roundOut{Ref: refString(tuiclient.Evidence{MatchID: matchID, Round: r.Number}), Round: r.Number,
		ScoreBefore: fmt.Sprintf("%d-%d", r.ScoreCTBefore, r.ScoreTBefore), Winner: r.Winner, EndReason: cleanName(r.EndReason),
		Seconds: max(second(r.TickEnd), 0), CTBuy: r.Economy.CTBuy, TBuy: r.Economy.TBuy, CTEquip: r.Economy.CTEquipValue, TEquip: r.Economy.TEquipValue,
		TPattern: r.Class.TSide, CTPattern: r.Class.CTSide, Site: r.Class.Site, Tags: cleanNames(r.Class.Tags), Reasons: []string{},
		Players: []roundPlayerOut{}, Kills: []killOut{}, WinTimeline: []winPointOut{}}
	for _, reason := range r.Class.Reasons {
		out.Reasons = append(out.Reasons, cleanText(reason, maxErrorRunes))
	}
	if b := r.Bomb; b != nil {
		out.Bomb = &bombOut{Site: cleanName(b.Site), Planter: names.name(b.PlanterSlot), PlantSecond: second(b.PlantTick),
			Defused: b.DefuseTick > 0, Exploded: b.ExplodeTick > 0}
	}
	for _, t := range r.Timeouts {
		out.Timeouts = append(out.Timeouts, timeoutOut{Side: cleanName(t.Side), FromSecond: second(t.Tick), ToSecond: second(t.EndTick), Technical: t.Technical})
	}
	for _, p := range r.Players {
		who := names[p.Slot]
		out.Players = append(out.Players, roundPlayerOut{SteamID64: who[1], Name: who[0], Side: p.Side, Kills: p.Kills, Deaths: p.Deaths,
			Assists: p.Assists, Damage: p.Damage, Equip: p.EquipValue, Survived: p.Survived, OpeningKill: p.OpeningKill,
			OpeningDeath: p.OpeningDeath, Traded: p.Traded})
	}
	for _, e := range r.Events {
		if e.Kind != "kill" {
			continue
		}
		out.Kills = append(out.Kills, killOut{Second: second(e.Tick), Killer: names.name(e.ActorSlot), Victim: names.name(e.TargetSlot),
			Weapon: cleanName(e.Weapon), Place: cleanName(e.Place), Headshot: e.Headshot, Wallbang: e.Wallbang, Opening: e.Opening, Traded: e.Traded})
	}
	for _, t := range m.Timelines {
		if t.Round != r.Number {
			continue
		}
		for _, p := range t.Points {
			out.WinTimeline = append(out.WinTimeline, winPointOut{Second: math.Round(p.Seconds*10) / 10, Event: pointEvent(names, p), CTPct: sharePct(p.ProbabilityCT)})
		}
	}
	return out
}

func pointEvent(names slotNames, p tuiclient.WinPoint) string {
	if p.Kind == "kill" {
		return names.name(p.ActorSlot) + " > " + names.name(p.TargetSlot)
	}
	return cleanName(p.Kind)
}

func summarizeMatchMetrics(matchID string, doc tuiclient.TacticalDocument, m tuiclient.MatchMetrics) matchMetricsOut {
	names := namesOf(doc)
	out := matchMetricsOut{MatchID: matchID, Players: []playerMetricsOut{}, Rounds: []roundWinOut{}, SkippedMatches: len(m.Skipped)}
	for _, p := range m.Players {
		out.Players = append(out.Players, metricsOf(p))
	}
	for _, t := range m.Timelines {
		r := roundWinOut{Round: t.Round, Winner: t.Winner}
		for i, p := range t.Points {
			if i == 0 {
				r.CTStart = sharePct(p.ProbabilityCT)
				continue
			}
			delta := sharePct(p.ProbabilityCT - t.Points[i-1].ProbabilityCT)
			if p.Kind == "kill" && (r.BigSwing == nil || math.Abs(delta) > math.Abs(r.BigSwing.DeltaPct)) {
				r.BigSwing = &swingOut{Second: math.Round(p.Seconds*10) / 10, Event: pointEvent(names, p), DeltaPct: delta}
			}
		}
		out.Rounds = append(out.Rounds, r)
	}
	return out
}
