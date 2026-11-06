package tacticalplan

import (
	"errors"
	"math"
	"sort"

	"github.com/google/uuid"
)

// proReferenceMinRounds keeps every player of one full map in a pro reference.
const proReferenceMinRounds = 15

// proRateMetrics are the per-round rates whose one-round spread a reference measures.
var proRateMetrics = []string{MetricKPR, MetricDPR, MetricADR, MetricUtilityDamage, MetricFlashAssists, MetricWPA}

// RoundSums are the count, sum and sum of squares of one player's per-round
// values of a rate, enough to pool the round spread across demos exactly.
type RoundSums struct {
	N     int     `json:"n"`
	Sum   float64 `json:"sum"`
	SumSq float64 `json:"sum_sq"`
}

func (s *RoundSums) add(o RoundSums) {
	s.N += o.N
	s.Sum += o.Sum
	s.SumSq += o.SumSq
}

// ProDemoLine is one player's additive totals over one demo, so a reference
// can merge many demos without reading them again.
type ProDemoLine struct {
	SteamID64     string               `json:"steamid64"`
	Name          string               `json:"name"`
	Team          string               `json:"team"`
	Rounds        int                  `json:"rounds"`
	Kills         int                  `json:"kills"`
	Deaths        int                  `json:"deaths"`
	TradedDeaths  int                  `json:"traded_deaths"`
	FlashAssists  int                  `json:"flash_assists"`
	UtilityDamage int                  `json:"utility_damage"`
	WPA           float64              `json:"wpa"`
	Box           BoxScore             `json:"box"`
	RoundSums     map[string]RoundSums `json:"round_sums"`
}

// ProDemoSummary is what a pro reference keeps of one demo; FirstRound and
// LastRound are game round numbers, so restart parts of one map can be joined.
type ProDemoSummary struct {
	Match      ProReferenceMatch `json:"match"`
	FirstRound int               `json:"first_round,omitempty"`
	LastRound  int               `json:"last_round,omitempty"`
	Players    []ProDemoLine     `json:"players"`
}

// SummarizeProDemo reduces a scanned demo to its players' additive totals and
// the per-round sums of the rates.
func SummarizeProDemo(doc Document) ProDemoSummary {
	id := uuid.UUID{1}
	teamName := map[string]string{}
	names := []string{}
	for _, team := range doc.Teams {
		names = append(names, team.Name)
		teamName[team.Key] = team.Name
	}
	team := map[string]string{}
	for _, p := range doc.Players {
		team[p.SteamID64] = teamName[p.TeamKey]
	}
	out := ProDemoSummary{Match: ProReferenceMatch{Map: doc.Demo.Map, Teams: names, Rounds: len(doc.Rounds)}}
	if n := len(doc.Rounds); n > 0 {
		out.FirstRound = gameRound(doc.Rounds[0])
		out.LastRound = gameRound(doc.Rounds[n-1])
	}
	index := map[string]int{}
	report := Metrics([]MetricsMatch{{MatchID: id, Document: doc}}, MetricsOptions{})
	for _, p := range report.Players {
		var t playerTotals
		t.add(p)
		index[p.SteamID64] = len(out.Players)
		out.Players = append(out.Players, ProDemoLine{
			SteamID64: p.SteamID64, Name: p.Name, Team: team[p.SteamID64], Rounds: t.rounds, Kills: t.kills, Deaths: t.deaths,
			TradedDeaths: t.tradedDeaths, FlashAssists: t.flashAssists, UtilityDamage: t.utilityDamage, WPA: t.wpa, Box: t.box,
			RoundSums: map[string]RoundSums{},
		})
	}
	for i := range doc.Rounds {
		one := doc
		one.Rounds = doc.Rounds[i : i+1]
		for _, p := range Metrics([]MetricsMatch{{MatchID: id, Document: one}}, MetricsOptions{}).Players {
			at, ok := index[p.SteamID64]
			if !ok {
				continue
			}
			var t playerTotals
			t.add(p)
			values := t.values()
			for _, metric := range proRateMetrics {
				if v, ok := values[metric]; ok {
					s := out.Players[at].RoundSums[metric]
					s.add(RoundSums{N: 1, Sum: v, SumSq: v * v})
					out.Players[at].RoundSums[metric] = s
				}
			}
		}
	}
	return out
}

// metrics turns the line back into the fields BaselineSample.Add reads.
func (l ProDemoLine) metrics() PlayerMetrics {
	p := PlayerMetrics{SteamID64: l.SteamID64, Name: l.Name, Matches: 1, Rounds: l.Rounds, Kills: l.Kills, Deaths: l.Deaths, Box: l.Box}
	p.Trades.TradedDeaths.Count = l.TradedDeaths
	p.Utility.Flash.Assists.Count = l.FlashAssists
	p.Utility.HE.EnemyDamage = l.UtilityDamage
	p.Swing.WPA = l.WPA
	return p
}

// ProReferenceBuild names a reference and the demos it reads.
type ProReferenceBuild struct {
	ID          string
	Description string
	UpdatedAt   string
	Demos       []ProDemoSummary
}

// BuildProReference measures a pro reference from demo summaries, the way the
// bundled reference was measured; maps with too few players get no entry.
func BuildProReference(b ProReferenceBuild) (ProReference, error) {
	var all BaselineSample
	perMap := map[string]*BaselineSample{}
	sums := map[string]map[string]RoundSums{}
	teams := map[string]string{}
	ref := ProReference{ID: b.ID, Description: b.Description, UpdatedAt: b.UpdatedAt, Maps: map[string]ProReferenceMap{}}
	for _, demo := range b.Demos {
		report := MetricsReport{Matches: 1, Rounds: demo.Match.Rounds}
		for _, line := range demo.Players {
			report.Players = append(report.Players, line.metrics())
			if line.Team != "" {
				teams[line.SteamID64] = line.Team
			}
			for metric, s := range line.RoundSums {
				if sums[metric] == nil {
					sums[metric] = map[string]RoundSums{}
				}
				merged := sums[metric][line.SteamID64]
				merged.add(s)
				sums[metric][line.SteamID64] = merged
			}
		}
		all.Add(report)
		if perMap[demo.Match.Map] == nil {
			perMap[demo.Match.Map] = &BaselineSample{}
		}
		perMap[demo.Match.Map].Add(report)
		ref.Matches = append(ref.Matches, demo.Match)
	}
	spread := ReferenceSpread{RoundStddev: map[string]float64{}}
	for _, metric := range proRateMetrics {
		spread.RoundStddev[metric] = pooledRoundStddev(sums[metric])
	}
	ref.Baseline = all.Baseline(proReferenceMinRounds, spread)
	ref.Players = referenceRows(&all, teams)
	for name, sample := range perMap {
		mb := sample.Baseline(proReferenceMinRounds, spread)
		if mb.Players >= ProReferenceMinPlayers {
			ref.Maps[name] = ProReferenceMap{Baseline: mb, Players: referenceRows(sample, teams)}
		}
	}
	if err := ref.validate(); err != nil {
		return ProReference{}, err
	}
	return ref, nil
}

// pooledRoundStddev is the spread of a round around each player's own mean.
func pooledRoundStddev(byPlayer map[string]RoundSums) float64 {
	var dev float64
	var df int
	for _, s := range byPlayer {
		if s.N < 2 {
			continue
		}
		dev += max(s.SumSq-s.Sum*s.Sum/float64(s.N), 0)
		df += s.N - 1
	}
	if df == 0 {
		return 0
	}
	return math.Sqrt(dev / float64(df))
}

// referenceRows is every player's value of every metric, in SteamID order.
func referenceRows(sample *BaselineSample, teams map[string]string) []ProReferenceRow {
	rows := map[string]*ProReferenceRow{}
	for _, metric := range ReferenceMetrics {
		for _, line := range sample.Leaderboard(metric, 1, 0) {
			row := rows[line.SteamID64]
			if row == nil {
				row = &ProReferenceRow{SteamID64: line.SteamID64, Name: line.Name, Team: teams[line.SteamID64], Rounds: line.Rounds, Matches: line.Matches, Values: map[string]float64{}}
				rows[line.SteamID64] = row
			}
			row.Values[metric] = line.Value
		}
	}
	out := make([]ProReferenceRow, 0, len(rows))
	for _, row := range rows {
		out = append(out, *row)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].SteamID64 < out[j].SteamID64 })
	return out
}

var errEmptyProReference = errors.New("pro reference has no description or no players")
