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

// teamRoundScopeIn is the rest of the team scope bar: the match count, the
// period shortcut, the opponent level, whose matches and the round criteria.
// Every team tool reads the same rounds as the bar with the same values.
type teamRoundScopeIn struct {
	MaxMatches  int      `json:"max_matches,omitempty" jsonschema:"only the team's most recent N matches the lineup played, 1 to 50, as Partidas: últimas 3, 5, 10, 20 or 50"`
	LastDays    int      `json:"last_days,omitempty" jsonschema:"only matches of the last N days, as the Periodo shortcuts: 3, 7, 14, 30, 90, 180 or 365; do not combine with from"`
	OpponentTop int      `json:"opponent_top,omitempty" jsonschema:"only matches against a rival in the HLTV top N: 5, 15, 30, 100 or 200; a rival without a rank (set with update_team) cannot be rated and comes back in skipped as opponent_unranked"`
	Whose       string   `json:"whose,omitempty" jsonschema:"own: only matches one of the user's own teams played (scrims and official matches, marked with update_team own); others: only matches without them (scouting)"`
	Buy         []string `json:"buy,omitempty" jsonschema:"the team's buys: pistol, eco, semi, force, full; half buys are semi and force"`
	OpponentBuy []string `json:"opponent_buy,omitempty" jsonschema:"the rival's buys, same values; full with full is a full buy matchup, pistol with pistol the pistol rounds"`
	AWP         string   `json:"awp,omitempty" jsonschema:"with: rounds in which a player of the team bought or used an AWP; without: rounds with none"`
	OpponentAWP string   `json:"opponent_awp,omitempty" jsonschema:"with or without, for the rival's AWP"`
	Advantage   []string `json:"advantage,omitempty" jsonschema:"numeric advantage at some moment of the round, the team's players alive first: 4v5, 5v4, 2v1..."`
	RoundFrom   int      `json:"round_from,omitempty" jsonschema:"first round number to keep, a manual range when the buy classification mislabels rounds"`
	RoundTo     int      `json:"round_to,omitempty" jsonschema:"last round number to keep"`
}

// values writes the scope as the query the team endpoints read.
func (in teamRoundScopeIn) values() url.Values {
	v := url.Values{}
	for key, n := range map[string]int{"max_matches": in.MaxMatches, "opponent_top": in.OpponentTop, "round_from": in.RoundFrom, "round_to": in.RoundTo} {
		if n != 0 {
			v.Set(key, strconv.Itoa(n))
		}
	}
	for key, raw := range map[string]string{"whose": in.Whose, "awp": in.AWP, "opponent_awp": in.OpponentAWP} {
		if raw = strings.ToLower(strings.TrimSpace(raw)); raw != "" {
			v.Set(key, raw)
		}
	}
	for key, list := range map[string][]string{"buy": in.Buy, "opponent_buy": in.OpponentBuy, "advantage": in.Advantage} {
		for _, item := range list {
			v.Add(key, strings.ToLower(strings.TrimSpace(item)))
		}
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
