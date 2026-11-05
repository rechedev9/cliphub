package tacticalplan

import (
	"slices"
	"sort"
)

// Reference metrics a player is compared on. Rates are per round; the _pct
// metrics are percentages from 0 to 100.
const (
	MetricKPR           = "kpr"
	MetricDPR           = "dpr"
	MetricADR           = "adr"
	MetricKAST          = "kast"
	MetricOpeningWin    = "opening_win_pct"
	MetricTrade         = "trade_pct"
	MetricUtilityDamage = "utility_damage_per_round"
	MetricFlashAssists  = "flash_assists_per_round"
	MetricWPA           = "wpa_per_round"
)

// ReferenceMetrics lists the reference metrics in report order.
var ReferenceMetrics = []string{
	MetricKPR, MetricDPR, MetricADR, MetricKAST, MetricOpeningWin, MetricTrade,
	MetricUtilityDamage, MetricFlashAssists, MetricWPA,
}

// BaselineMinRounds is the rounds a player needs to count in a baseline.
const BaselineMinRounds = 50

// LowerIsBetter reports whether a smaller value ranks a player higher.
func LowerIsBetter(metric string) bool {
	return metric == MetricDPR
}

// BoxScore is the per-round box score the metrics events do not carry.
type BoxScore struct {
	Damage  int `json:"damage"`
	Assists int `json:"assists"`
	// KASTRounds are the rounds with a kill, an assist, survival or a traded death.
	KASTRounds    int `json:"kast_rounds"`
	OpeningKills  int `json:"opening_kills"`
	OpeningDeaths int `json:"opening_deaths"`
	// UtilityRounds are the rounds of matches scanned with utility effects,
	// the denominator of the utility rates.
	UtilityRounds int `json:"utility_rounds"`
}

func addBoxRound(b *BoxScore, pr PlayerRound, utility bool) {
	b.Damage += pr.Damage
	b.Assists += pr.Assists
	if pr.Kills > 0 || pr.Assists > 0 || pr.Survived || pr.Traded {
		b.KASTRounds++
	}
	if pr.OpeningKill {
		b.OpeningKills++
	}
	if pr.OpeningDeath {
		b.OpeningDeaths++
	}
	if utility {
		b.UtilityRounds++
	}
}

func (b *BoxScore) add(o BoxScore) {
	b.Damage += o.Damage
	b.Assists += o.Assists
	b.KASTRounds += o.KASTRounds
	b.OpeningKills += o.OpeningKills
	b.OpeningDeaths += o.OpeningDeaths
	b.UtilityRounds += o.UtilityRounds
}

// playerTotals is the additive part of a player's metrics: the totals of
// several reports add up to the totals of their union.
type playerTotals struct {
	steamID       string
	name          string
	matches       int
	rounds        int
	kills         int
	deaths        int
	tradedDeaths  int
	flashAssists  int
	utilityDamage int
	wpa           float64
	box           BoxScore
}

func (t *playerTotals) add(p PlayerMetrics) {
	t.matches += p.Matches
	t.rounds += p.Rounds
	t.kills += p.Kills
	t.deaths += p.Deaths
	t.tradedDeaths += p.Trades.TradedDeaths.Count
	t.flashAssists += p.Utility.Flash.Assists.Count
	t.utilityDamage += p.Utility.HE.EnemyDamage + p.Utility.Molotov.EnemyDamage
	t.wpa += p.Swing.WPA
	t.box.add(p.Box)
}

// values is the player's value of each reference metric. A metric without a
// denominator (no deaths, no opening duel, no utility scan) is left out.
func (t playerTotals) values() map[string]float64 {
	out := map[string]float64{}
	if t.rounds == 0 {
		return out
	}
	rounds := float64(t.rounds)
	out[MetricKPR] = round4(float64(t.kills) / rounds)
	out[MetricDPR] = round4(float64(t.deaths) / rounds)
	out[MetricADR] = round4(float64(t.box.Damage) / rounds)
	out[MetricKAST] = round4(100 * float64(t.box.KASTRounds) / rounds)
	out[MetricWPA] = round4(t.wpa / rounds)
	if duels := t.box.OpeningKills + t.box.OpeningDeaths; duels > 0 {
		out[MetricOpeningWin] = round4(100 * float64(t.box.OpeningKills) / float64(duels))
	}
	if t.deaths > 0 {
		out[MetricTrade] = round4(100 * float64(t.tradedDeaths) / float64(t.deaths))
	}
	if u := t.box.UtilityRounds; u > 0 {
		out[MetricUtilityDamage] = round4(float64(t.utilityDamage) / float64(u))
		out[MetricFlashAssists] = round4(float64(t.flashAssists) / float64(u))
	}
	return out
}

// BaselineSample adds up player metrics report by report, so a whole library
// can be read one match at a time.
type BaselineSample struct {
	Matches int
	Rounds  int
	players map[string]*playerTotals
}

// Add folds one metrics report into the sample.
func (s *BaselineSample) Add(r MetricsReport) {
	if s.players == nil {
		s.players = map[string]*playerTotals{}
	}
	s.Matches += r.Matches
	s.Rounds += r.Rounds
	for _, p := range r.Players {
		t := s.players[p.SteamID64]
		if t == nil {
			t = &playerTotals{steamID: p.SteamID64, name: p.Name}
			s.players[p.SteamID64] = t
		}
		t.add(p)
	}
}

// BaselineValue is the mean of one metric over the players that have it.
type BaselineValue struct {
	Mean    float64 `json:"mean"`
	Players int     `json:"players"`
}

// Baseline is the per-player average of each reference metric over the
// players with at least MinRounds rounds.
type Baseline struct {
	MinRounds int `json:"min_rounds"`
	Players   int `json:"players"`
	// Rounds and Matches count everything read, qualifying players or not.
	Rounds  int                      `json:"rounds"`
	Matches int                      `json:"matches"`
	Metrics map[string]BaselineValue `json:"metrics"`
}

// Baseline averages each metric over the qualifying players.
func (s *BaselineSample) Baseline(minRounds int) Baseline {
	out := Baseline{MinRounds: minRounds, Rounds: s.Rounds, Matches: s.Matches, Metrics: map[string]BaselineValue{}}
	sums := map[string]float64{}
	counts := map[string]int{}
	for _, t := range s.players {
		if t.rounds < minRounds {
			continue
		}
		out.Players++
		for metric, v := range t.values() {
			sums[metric] += v
			counts[metric]++
		}
	}
	for _, metric := range ReferenceMetrics {
		v := BaselineValue{Players: counts[metric]}
		if v.Players > 0 {
			v.Mean = round4(sums[metric] / float64(v.Players))
		}
		out.Metrics[metric] = v
	}
	return out
}

// LeaderboardRow is one player's place on a metric.
type LeaderboardRow struct {
	Rank      int     `json:"rank"`
	SteamID64 string  `json:"steamid64"`
	Name      string  `json:"name"`
	Rounds    int     `json:"rounds"`
	Matches   int     `json:"matches"`
	Value     float64 `json:"value"`
}

// Leaderboard ranks the players with at least minRounds rounds on one metric,
// best first, and keeps the first limit. A player without the metric is left out.
func (s *BaselineSample) Leaderboard(metric string, minRounds, limit int) []LeaderboardRow {
	rows := []LeaderboardRow{}
	if !slices.Contains(ReferenceMetrics, metric) {
		return rows
	}
	for _, t := range s.players {
		v, ok := t.values()[metric]
		if t.rounds < minRounds || !ok {
			continue
		}
		rows = append(rows, LeaderboardRow{SteamID64: t.steamID, Name: t.name, Rounds: t.rounds, Matches: t.matches, Value: v})
	}
	lower := LowerIsBetter(metric)
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Value != rows[j].Value {
			return (rows[i].Value < rows[j].Value) == lower
		}
		if rows[i].Rounds != rows[j].Rounds {
			return rows[i].Rounds > rows[j].Rounds
		}
		return rows[i].SteamID64 < rows[j].SteamID64
	})
	if limit > 0 && len(rows) > limit {
		rows = rows[:limit]
	}
	for i := range rows {
		rows[i].Rank = i + 1
	}
	return rows
}
