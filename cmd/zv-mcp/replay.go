package main

import (
	"context"
	"fmt"
	"net/url"
	"slices"
	"strconv"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/rechedev9/cliphub/internal/tacticalplan"
)

// replayForms are the four ways the Replay reads its rounds, as Studio names
// them in the URL.
var replayForms = []string{"repeticion", "calor", "recorridos", "granadas"}

type replayRoundsIn struct {
	MatchID      string   `json:"match_id" jsonschema:"match id from list_matches"`
	TeamKey      string   `json:"team_key,omitempty" jsonschema:"follow one team across the side swap: ct-start or t-start, from get_match"`
	Side         string   `json:"side,omitempty" jsonschema:"only rounds seen from CT or T"`
	Result       string   `json:"result,omitempty" jsonschema:"won or lost, from the team's or the side's view; needs team_key or side"`
	Buy          []string `json:"buy,omitempty" jsonschema:"own buys: pistol, eco, semi, force, full"`
	OpponentBuy  []string `json:"opponent_buy,omitempty" jsonschema:"the other side's buys, same values"`
	Site         []string `json:"site,omitempty" jsonschema:"where the round went: A, B, mid or none"`
	AttackShape  []string `json:"attack_shape,omitempty" jsonschema:"T round shapes, such as execute, default, split or fast"`
	DefenseShape []string `json:"defense_shape,omitempty" jsonschema:"CT round shapes, as get_round reports them"`
	Tag          []string `json:"tag,omitempty" jsonschema:"round tags, such as postplant, pistol, ace or retake_won"`
	Players      []string `json:"players,omitempty" jsonschema:"SteamID64s: only rounds these players were in"`
	RoundFrom    int      `json:"round_from,omitempty"`
	RoundTo      int      `json:"round_to,omitempty"`
	Phase        string   `json:"phase,omitempty" jsonschema:"regulation or overtime"`
	Form         string   `json:"form,omitempty" jsonschema:"which form studio_path opens: repeticion (default), calor, recorridos or granadas"`
}

type replayRoundOut struct {
	Ref          string `json:"ref" jsonschema:"match_id/round, as get_round takes it"`
	Round        int    `json:"round"`
	Winner       string `json:"winner" jsonschema:"CT or T"`
	CTBuy        string `json:"ct_buy"`
	TBuy         string `json:"t_buy"`
	Site         string `json:"site,omitempty"`
	AttackShape  string `json:"attack_shape,omitempty"`
	DefenseShape string `json:"defense_shape,omitempty"`
}

type replayRoundsOut struct {
	MatchID    string           `json:"match_id"`
	Map        string           `json:"map"`
	Rounds     []replayRoundOut `json:"rounds" jsonschema:"the rounds the scope keeps, exactly the ones the Replay's scope bar shows"`
	Kept       int              `json:"kept"`
	Total      int              `json:"total"`
	StudioPath string           `json:"studio_path" jsonschema:"the path that opens the Replay in Studio with this scope and form"`
}

// values turns the tool input into the query the Replay keeps in its URL, the
// same one tacticalplan.FilterFromValues reads.
func (in replayRoundsIn) values(doc tacticalplan.Document) (url.Values, error) {
	v := url.Values{}
	if in.TeamKey != "" && in.TeamKey != "ct-start" && in.TeamKey != "t-start" {
		return nil, fmt.Errorf("team_key must be ct-start or t-start, not %q", cleanName(in.TeamKey))
	}
	side, err := parseSide(in.Side)
	if err != nil {
		return nil, err
	}
	outcome, ok := map[string]string{"": "", roundResultWon: "win", roundResultLost: "loss"}[in.Result]
	if !ok {
		return nil, fmt.Errorf("result must be won or lost, not %q", cleanName(in.Result))
	}
	if outcome != "" && in.TeamKey == "" && side == "" {
		return nil, fmt.Errorf("result needs team_key or side: a round is won or lost by someone")
	}
	for key, value := range map[string]string{"team": in.TeamKey, "side": side, "outcome": outcome, "phase": strings.ToLower(strings.TrimSpace(in.Phase))} {
		if value != "" {
			v.Set(key, value)
		}
	}
	for key, list := range map[string][]string{"buy": in.Buy, "opponent_buy": in.OpponentBuy, "site": in.Site, "t_pattern": in.AttackShape, "ct_pattern": in.DefenseShape, "tag": in.Tag} {
		for _, value := range list {
			v.Add(key, strings.TrimSpace(value))
		}
	}
	for _, id := range in.Players {
		if err := checkSteamID(id); err != nil {
			return nil, err
		}
		i := slices.IndexFunc(doc.Players, func(p tacticalplan.Player) bool { return p.SteamID64 == id })
		if i < 0 {
			return nil, fmt.Errorf("player %s did not play this match", id)
		}
		v.Add("slot", strconv.Itoa(int(doc.Players[i].Slot)))
	}
	if in.RoundFrom < 0 || in.RoundTo < 0 {
		return nil, fmt.Errorf("round_from and round_to must be positive")
	}
	if in.RoundFrom > 0 {
		v.Set("round_from", strconv.Itoa(in.RoundFrom))
	}
	if in.RoundTo > 0 {
		v.Set("round_to", strconv.Itoa(in.RoundTo))
	}
	return v, nil
}

func registerReplayTools(server *mcp.Server, s *studio) {
	mcp.AddTool(server, &mcp.Tool{Name: "replay_rounds", Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true},
		Description: "Read the rounds of one match that a Replay scope keeps, exactly as the scope bar of the Replay does: team, side, result, buys, site, " +
			"round shapes, tags, players, round range and phase. Every form of the Replay (Repetición, Mapa de calor, Recorridos, Granadas) reads these same rounds. " +
			"studio_path opens the Replay in Studio on that scope and form."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in replayRoundsIn) (*mcp.CallToolResult, replayRoundsOut, error) {
			if err := checkUUID("match_id", in.MatchID); err != nil {
				return nil, replayRoundsOut{}, err
			}
			form := strings.ToLower(strings.TrimSpace(in.Form))
			if form != "" && !slices.Contains(replayForms, form) {
				return nil, replayRoundsOut{}, fmt.Errorf("form must be one of %s, not %q", strings.Join(replayForms, ", "), cleanName(in.Form))
			}
			c, err := s.client(ctx)
			if err != nil {
				return nil, replayRoundsOut{}, toolError(err)
			}
			doc, err := c.LibraryTacticalPlan(ctx, in.MatchID)
			if err != nil {
				return nil, replayRoundsOut{}, toolError(err)
			}
			values, err := in.values(doc)
			if err != nil {
				return nil, replayRoundsOut{}, err
			}
			filter, err := tacticalplan.FilterFromValues(values)
			if err != nil {
				return nil, replayRoundsOut{}, err
			}
			out := replayRoundsOut{MatchID: in.MatchID, Map: cleanName(doc.Demo.Map), Rounds: []replayRoundOut{}, Total: len(doc.Rounds)}
			for _, r := range filter.Apply(doc) {
				out.Rounds = append(out.Rounds, replayRoundOut{
					Ref: fmt.Sprintf("%s/%d", in.MatchID, r.Number), Round: r.Number, Winner: string(r.Winner),
					CTBuy: string(r.Economy.CTBuy), TBuy: string(r.Economy.TBuy), Site: string(r.Class.Site),
					AttackShape: string(r.Class.TSide), DefenseShape: string(r.Class.CTSide),
				})
			}
			out.Kept = len(out.Rounds)
			values.Set("tab", "replay")
			if form != "" && form != replayForms[0] {
				values.Set("forma", form)
			}
			out.StudioPath = "/partidas/" + url.PathEscape(in.MatchID) + "?" + values.Encode()
			return nil, out, nil
		})
}
