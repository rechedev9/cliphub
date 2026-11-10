package main

import (
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/rechedev9/cliphub/internal/tacticalplan"
	"github.com/rechedev9/cliphub/internal/tuiclient"
)

// periodShortcuts are the Periodo shortcuts of the team scope bar, in days.
var periodShortcuts = []int{3, 7, 14, 30, 90, 180, 365}

// teamRoundScopeIn is the rest of the team scope bar (match count, period, opponent level,
// whose matches, round criteria), so every team tool reads the bar's rounds.
type teamRoundScopeIn struct {
	MaxMatches        int      `json:"max_matches,omitempty" jsonschema:"only the team's most recent N matches the lineup played, 1 to 50, as Partidas: últimas 3, 5, 10, 20 or 50"`
	LastDays          int      `json:"last_days,omitempty" jsonschema:"only matches of the last N days, as the Periodo shortcuts: 3, 7, 14, 30, 90, 180 or 365; do not combine with from"`
	OpponentTop       int      `json:"opponent_top,omitempty" jsonschema:"only matches against a rival in the HLTV top N: 5, 15, 30, 100 or 200, by the rival's rank today; a rival without a rank (set with update_team) cannot be rated and comes back in skipped as opponent_unranked"`
	Whose             string   `json:"whose,omitempty" jsonschema:"own: only matches the user's own team played (scrims and official matches, marked with update_team own); others: only matches without them (scouting)"`
	Buy               []string `json:"buy,omitempty" jsonschema:"the team's buys: pistol, eco, semi, force, full; half buys are semi and force"`
	OpponentBuy       []string `json:"opponent_buy,omitempty" jsonschema:"the rival's buys, same values; full with full is a full buy matchup, pistol with pistol the pistol rounds"`
	AWP               string   `json:"awp,omitempty" jsonschema:"with: rounds in which a player of the team bought or used an AWP; without: rounds with none"`
	OpponentAWP       string   `json:"opponent_awp,omitempty" jsonschema:"with or without, for the rival's AWP"`
	Advantage         []string `json:"advantage,omitempty" jsonschema:"numeric advantage at some moment of the round, the team's players alive first: 4v5, 5v4, 2v1..."`
	RoundFrom         int      `json:"round_from,omitempty" jsonschema:"first round number to keep, a manual range when the buy classification mislabels rounds"`
	RoundTo           int      `json:"round_to,omitempty" jsonschema:"last round number to keep"`
	Site              []string `json:"site,omitempty" jsonschema:"bombsite the round went to: A, B, mid or none (Sitio atacado)"`
	Outcome           string   `json:"outcome,omitempty" jsonschema:"win or loss, from the team's view (Resultado)"`
	PostPlant         bool     `json:"post_plant,omitempty" jsonschema:"only rounds with the bomb planted (Post-plant)"`
	Phase             string   `json:"phase,omitempty" jsonschema:"regulation or overtime (Prórroga)"`
	SecondRound       bool     `json:"second_round,omitempty" jsonschema:"only the round after each regulation half's pistol round (Segunda ronda)"`
	EndReason         []string `json:"end_reason,omitempty" jsonschema:"how the round ended (Motivo de victoria; pair with outcome): elimination, bomb_exploded, bomb_defused or time_expired"`
	Weapon            string   `json:"weapon,omitempty" jsonschema:"the team's weapons: scout (a player killed or hit with a Scout), no_scout (none did), hero_ak (one player killed or hit with an AK-47 while every teammate carried under $1000 of equipment) or hero_awp (one player held an AWP while every teammate carried under $1000)"`
	OpponentWeapon    string   `json:"opponent_weapon,omitempty" jsonschema:"the rival's weapons, same values"`
	EquipFrom         int      `json:"equip_from,omitempty" jsonschema:"custom buy range: the team's equipment value per player at the economy sample, at least this many dollars"`
	EquipTo           int      `json:"equip_to,omitempty" jsonschema:"custom buy range: at most this many dollars per player"`
	OpponentEquipFrom int      `json:"opponent_equip_from,omitempty" jsonschema:"the rival's equipment value per player, at least"`
	OpponentEquipTo   int      `json:"opponent_equip_to,omitempty" jsonschema:"the rival's equipment value per player, at most"`
	Tactic            string   `json:"tactic,omitempty" jsonschema:"a Libreta play id from list_plays (Táctica): only the rounds that play was read from, its example rounds"`
}

// teamOnly reports a key the per-player round reads cannot apply.
func (in teamRoundScopeIn) teamOnly() bool {
	return in.MaxMatches != 0 || in.OpponentTop != 0 || in.Whose != "" || in.AWP != "" || in.OpponentAWP != "" || len(in.Advantage) > 0 ||
		len(in.Site) > 0 || in.Outcome != "" || in.PostPlant || in.Phase != "" || in.SecondRound || len(in.EndReason) > 0 ||
		in.Weapon != "" || in.OpponentWeapon != "" || in.EquipFrom != 0 || in.EquipTo != 0 || in.OpponentEquipFrom != 0 ||
		in.OpponentEquipTo != 0 || in.Tactic != ""
}

// values writes the scope as the query the team endpoints read.
func (in teamRoundScopeIn) values() url.Values {
	v := url.Values{}
	for key, n := range map[string]int{"max_matches": in.MaxMatches, "opponent_top": in.OpponentTop, "round_from": in.RoundFrom, "round_to": in.RoundTo,
		"equip_from": in.EquipFrom, "equip_to": in.EquipTo, "opponent_equip_from": in.OpponentEquipFrom, "opponent_equip_to": in.OpponentEquipTo} {
		if n != 0 {
			v.Set(key, strconv.Itoa(n))
		}
	}
	for key, raw := range map[string]string{"whose": in.Whose, "awp": in.AWP, "opponent_awp": in.OpponentAWP, "outcome": in.Outcome, "phase": in.Phase,
		"weapon": in.Weapon, "opponent_weapon": in.OpponentWeapon, "tactic": in.Tactic} {
		if raw = strings.ToLower(strings.TrimSpace(raw)); raw != "" {
			v.Set(key, raw)
		}
	}
	for key, list := range map[string][]string{"buy": in.Buy, "opponent_buy": in.OpponentBuy, "advantage": in.Advantage, "site": in.Site, "end_reason": in.EndReason} {
		for _, item := range list {
			v.Add(key, strings.ToLower(strings.TrimSpace(item)))
		}
	}
	if in.PostPlant {
		v.Set("tag", "postplant")
	}
	if in.SecondRound {
		v.Set("second_round", "true")
	}
	return v
}

// apply validates the scope as the server does and adds it to a team query.
func (in teamRoundScopeIn) apply(q tuiclient.AnalysisQuery) (tuiclient.AnalysisQuery, error) {
	if in.MaxMatches < 0 || in.MaxMatches > maxPlayerMetricsMatches {
		return q, fmt.Errorf("max_matches must be from 1 to %d", maxPlayerMetricsMatches)
	}
	if _, err := tacticalplan.ParseOpponentTop(in.OpponentTop); err != nil {
		return q, err
	}
	v := in.values()
	if _, err := tacticalplan.ParseWhose(v.Get("whose")); err != nil {
		return q, err
	}
	if in.Tactic != "" {
		if err := checkUUID("tactic", in.Tactic); err != nil {
			return q, err
		}
	}
	if _, err := tacticalplan.FilterFromValues(v); err != nil {
		return q, errors.New(strings.TrimPrefix(err.Error(), "filter: "))
	}
	if in.LastDays != 0 {
		if !containsInt(periodShortcuts, in.LastDays) {
			return q, fmt.Errorf("last_days must be one of %v", periodShortcuts)
		}
		if !q.From.IsZero() {
			return q, errors.New("set last_days or from, not both")
		}
		q.From = time.Now().UTC().AddDate(0, 0, -in.LastDays)
	}
	q.Scope = v
	return q, nil
}

func containsInt(list []int, v int) bool {
	for _, item := range list {
		if item == v {
			return true
		}
	}
	return false
}
