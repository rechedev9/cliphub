package main

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/rechedev9/cliphub/internal/tacticalplan"
	"github.com/rechedev9/cliphub/internal/tuiclient"
)

// comboIn is a playbook's tactic combination.
type comboIn struct {
	With    []string `json:"with,omitempty" jsonschema:"tactic keys from team_playbook (such as zone:Middle or utility:smoke@Window); keep the rounds that show all of them"`
	Without []string `json:"without,omitempty" jsonschema:"tactic keys from team_playbook; drop the rounds that show any of them"`
}

type teamPlaybookIn struct {
	teamReportIn
	comboIn
}

type savePlaybookComboIn struct {
	teamPlaybookIn
	Title string `json:"title" jsonschema:"the play's title in the stratbook"`
}

// tacticOut is one tactic of a playbook and the kept rounds that show it.
type tacticOut struct {
	Key  string `json:"key" jsonschema:"pass it in with or without"`
	Kind string `json:"kind" jsonschema:"category, site, buy, opponent_buy, opening, opening_result, awp, zone or utility"`
	groupOut
}

// read validates the team, filter and combination and reads the playbook.
func (in teamPlaybookIn) read(ctx context.Context, s *studio) (tuiclient.TeamPlaybook, error) {
	q, err := in.check(true)
	if err != nil {
		return tuiclient.TeamPlaybook{}, err
	}
	if err := tacticalplan.CheckTacticKeys("with", in.With); err != nil {
		return tuiclient.TeamPlaybook{}, err
	}
	if err := tacticalplan.CheckTacticKeys("without", in.Without); err != nil {
		return tuiclient.TeamPlaybook{}, err
	}
	q.With, q.Without = in.With, in.Without
	c, err := s.client(ctx)
	if err != nil {
		return tuiclient.TeamPlaybook{}, toolError(err)
	}
	pb, err := c.TeamPlaybookReport(ctx, in.TeamID, q)
	if err != nil {
		return tuiclient.TeamPlaybook{}, toolError(err)
	}
	return pb, nil
}

// tacticName is a tactic as the assistant reads it, such as "smoke at Window".
func tacticName(t tuiclient.PlaybookTactic) string {
	place := t.Label
	if place == "" {
		place = t.Value
	}
	switch t.Kind {
	case tacticalplan.TacticUtility:
		return t.Grenade + " at " + place
	case tacticalplan.TacticOpening, tacticalplan.TacticAWP, tacticalplan.TacticZone:
		return t.Kind + " at " + place
	default:
		return t.Kind + " " + t.Value
	}
}

func tacticsOf(pb tuiclient.TeamPlaybook, scope titleScope) (tactics []tacticOut, with, without []string) {
	tactics = []tacticOut{}
	for _, t := range pb.Tactics {
		name := tacticName(t)
		tactics = append(tactics, tacticOut{Key: cleanName(t.Key), Kind: cleanName(t.Kind),
			groupOut: groupOf(groupSpec{title: scope.of(tacticLabel(t)), name: name, group: t.ReportGroup})})
	}
	return tactics, cleanKeys(pb.Combo.With), cleanKeys(pb.Combo.Without)
}

func cleanKeys(keys []string) []string {
	out := []string{}
	for _, key := range keys {
		out = append(out, cleanName(key))
	}
	return out
}

// mostCommon is the value most rounds share, the first on a tie, or fallback.
func mostCommon(values []string, fallback string) string {
	counts := map[string]int{}
	best, bestCount := fallback, 0
	for _, v := range values {
		if v == "" {
			continue
		}
		counts[v]++
		if counts[v] > bestCount {
			best, bestCount = v, counts[v]
		}
	}
	return best
}

// comboPlay is the stratbook play of a narrowed playbook, as Guardar en el
// stratbook builds it: the common pattern and site, the combination as notes.
func comboPlay(pb tuiclient.TeamPlaybook, teamID, title string) tuiclient.Play {
	patterns, sites := []string{}, []string{}
	won := 0
	examples := []tuiclient.Evidence{}
	for _, r := range pb.Rounds {
		patterns, sites = append(patterns, r.Pattern), append(sites, r.Site)
		if r.Won {
			won++
		}
		if len(examples) < maxPlayExamples {
			examples = append(examples, tuiclient.Evidence{MatchID: r.MatchID, Round: r.Round, Tick: r.Tick})
		}
	}
	fallback := "default"
	if pb.Side == string(tacticalplan.SideCT) {
		fallback = "hold"
	}
	parts := []string{}
	if len(pb.Combo.With) > 0 {
		parts = append(parts, "con "+strings.Join(pb.Combo.With, " + "))
	}
	if len(pb.Combo.Without) > 0 {
		parts = append(parts, "sin "+strings.Join(pb.Combo.Without, ", "))
	}
	pct := int(math.Round(100 * float64(won) / float64(max(len(pb.Rounds), 1))))
	rounds := fmt.Sprintf("%d rondas", len(pb.Rounds))
	if len(pb.Rounds) == 1 {
		rounds = "1 ronda"
	}
	return tuiclient.Play{
		Title: title, TeamID: &teamID, Map: pb.Map, Side: pb.Side, Origin: "playbook",
		Kind: mostCommon(patterns, fallback), Site: mostCommon(sites, "none"),
		Grenades: []tuiclient.PlayGrenade{}, Positions: []tuiclient.PlayPosition{}, Examples: examples,
		Notes: fmt.Sprintf("Combinación del playbook: %s. Gana el %d %% de %s.", strings.Join(parts, "; "), pct, rounds),
	}
}

func registerPlaybookComboTools(server *mcp.Server, s *studio) {
	mcp.AddTool(server, &mcp.Tool{Name: "save_playbook_combo", Annotations: &mcp.ToolAnnotations{},
		Description: "Save a team_playbook tactic combination to the Libreta as a play, as Guardar en la Libreta does in Combina tácticas: " +
			"the kept rounds' common pattern and site, the combination in the notes and up to 50 kept rounds as examples. " +
			"Pass the same team, map, side, filter, with and without as the team_playbook call."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in savePlaybookComboIn) (*mcp.CallToolResult, playDetailOut, error) {
			title, err := checkUserText("title", in.Title)
			if err != nil {
				return nil, playDetailOut{}, err
			}
			if len(in.With) == 0 && len(in.Without) == 0 {
				return nil, playDetailOut{}, errors.New("pass at least one tactic key in with or without")
			}
			pb, err := in.read(ctx, s)
			if err != nil {
				return nil, playDetailOut{}, err
			}
			if len(pb.Rounds) == 0 {
				return nil, playDetailOut{}, errors.New("no round shows this combination; nothing to save")
			}
			c, err := s.client(ctx)
			if err != nil {
				return nil, playDetailOut{}, toolError(err)
			}
			p, err := c.CreatePlay(ctx, comboPlay(pb, in.TeamID, title))
			if err != nil {
				return nil, playDetailOut{}, toolError(err)
			}
			return nil, playDetailOf(p), nil
		})
}
