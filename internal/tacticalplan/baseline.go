package tacticalplan

import (
	"math"
	"slices"
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

// PlayerLine is one player's rounds, matches and reference values in a sample.
type PlayerLine struct {
	Rounds  int
	Matches int
	Values  map[string]float64
}

// Player returns the line of one player, or false when the sample lacks them.
func (s *BaselineSample) Player(steamID string) (PlayerLine, bool) {
	t, ok := s.players[steamID]
	if !ok {
		return PlayerLine{}, false
	}
	return PlayerLine{Rounds: t.rounds, Matches: t.matches, Values: t.values()}, true
}

// BaselineValue is the mean of one metric over the players that have it, and
// how far players spread around it.
type BaselineValue struct {
	Mean    float64 `json:"mean"`
	Players int     `json:"players"`
	// Stddev is the spread of the players' values as measured.
	Stddev float64 `json:"stddev"`
	// PriorStddev is the spread of the players' true values: Stddev without
	// the sampling noise of their rounds. Shrinkage pulls a small sample
	// toward Mean by this much.
	PriorStddev float64 `json:"prior_stddev"`
	// RoundStddev is one round's spread for a per-round rate, the noise a
	// player's own average carries; zero for the percentages.
	RoundStddev float64 `json:"round_stddev,omitempty"`
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

// ReferenceSpread is measured once over pro demos and lent to every baseline:
// the round spread of each rate, and the prior spread for a baseline with
// too few players to measure its own.
type ReferenceSpread struct {
	RoundStddev map[string]float64
	PriorStddev map[string]float64
}

// minSpreadPlayers is how many players a baseline needs to measure its own spread.
const minSpreadPlayers = 5

// percentDenominator is the count a percentage metric is a share of.
func (t playerTotals) percentDenominator(metric string) (int, bool) {
	switch metric {
	case MetricKAST:
		return t.rounds, true
	case MetricOpeningWin:
		return t.box.OpeningKills + t.box.OpeningDeaths, true
	case MetricTrade:
		return t.deaths, true
	}
	return 0, false
}

// samplingVariance is the noise in the player's value of a metric whose
// population mean is mean: binomial for a percentage, the round spread over
// the rounds for a rate.
func (t playerTotals) samplingVariance(metric string, mean, roundStddev float64) float64 {
	if n, ok := t.percentDenominator(metric); ok {
		p := mean / 100
		return 1e4 * p * (1 - p) / float64(max(n, 1))
	}
	n := t.rounds
	if metric == MetricUtilityDamage || metric == MetricFlashAssists {
		n = t.box.UtilityRounds
	}
	return roundStddev * roundStddev / float64(max(n, 1))
}

// Baseline averages each metric over the qualifying players and measures
// their spread, the method-of-moments prior of empirical Bayes shrinkage.
func (s *BaselineSample) Baseline(minRounds int, spread ReferenceSpread) Baseline {
	out := Baseline{MinRounds: minRounds, Rounds: s.Rounds, Matches: s.Matches, Metrics: map[string]BaselineValue{}}
	type point struct {
		value float64
		t     *playerTotals
	}
	points := map[string][]point{}
	for _, t := range s.players {
		if t.rounds < minRounds {
			continue
		}
		out.Players++
		for metric, v := range t.values() {
			points[metric] = append(points[metric], point{v, t})
		}
	}
	for _, metric := range ReferenceMetrics {
		ps := points[metric]
		v := BaselineValue{Players: len(ps), RoundStddev: spread.RoundStddev[metric], PriorStddev: spread.PriorStddev[metric]}
		if v.Players == 0 {
			out.Metrics[metric] = v
			continue
		}
		var sum float64
		for _, p := range ps {
			sum += p.value
		}
		mean := sum / float64(len(ps))
		v.Mean = round4(mean)
		if len(ps) >= 2 {
			var dev, noise float64
			for _, p := range ps {
				dev += (p.value - mean) * (p.value - mean)
				noise += p.t.samplingVariance(metric, mean, v.RoundStddev)
			}
			observed := dev / float64(len(ps)-1)
			v.Stddev = round4(math.Sqrt(observed))
			if len(ps) >= minSpreadPlayers {
				// Half the variance stays: with few rounds per player the noise term
				// can swallow the spread and shrink everyone onto the mean.
				v.PriorStddev = round4(math.Sqrt(max(observed-noise/float64(len(ps)), observed/2)))
			}
		}
		if v.RoundStddev > 0 {
			v.RoundStddev = round4(v.RoundStddev)
		}
		out.Metrics[metric] = v
	}
	return out
}

// LeaderboardRow is one player's place on a metric.
type LeaderboardRow struct {
	Rank      int    `json:"rank"`
	SteamID64 string `json:"steamid64"`
	Name      string `json:"name"`
	// Team is set on the bundled pro rows, which have no library page.
	Team    string  `json:"team,omitempty"`
	Rounds  int     `json:"rounds"`
	Matches int     `json:"matches"`
	Value   float64 `json:"value"`
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
	return rankRows(rows, metric, limit)
}
