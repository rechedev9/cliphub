package tacticalplan

import (
	"fmt"
	"math"
	"testing"
)

// syntheticDemo is one map of ten players whose every value differs, so each
// metric has a spread and every player appears in each rate's round sums.
func syntheticDemo(mapName string, rounds, offset int) ProDemoSummary {
	d := ProDemoSummary{Match: ProReferenceMatch{Map: mapName, Teams: []string{"A", "B"}, Rounds: rounds}}
	for i := range 10 {
		k := i + offset
		sums := map[string]RoundSums{}
		for _, metric := range proRateMetrics {
			sums[metric] = RoundSums{N: rounds, Sum: float64(k), SumSq: float64(k*k) + 2}
		}
		d.Players = append(d.Players, ProDemoLine{
			SteamID64: fmt.Sprint(76561190000000000 + i), Name: fmt.Sprint("p", i), Team: "A", Rounds: rounds,
			Kills: 10 + k, Deaths: 12 + i%3, TradedDeaths: 2 + i%2, FlashAssists: i % 3, UtilityDamage: 40 + 5*k, WPA: float64(k) / 20,
			Box:       BoxScore{Damage: 1500 + 40*k, KASTRounds: rounds - 5 + i%4, OpeningKills: 1 + i%3, OpeningDeaths: 2, UtilityRounds: rounds},
			RoundSums: sums,
		})
	}
	return d
}

func TestBuildProReferenceMergesDemosLikeTheBundledOne(t *testing.T) {
	demos := []ProDemoSummary{syntheticDemo("de_nuke", 20, 0), syntheticDemo("de_nuke", 22, 3), syntheticDemo("de_inferno", 24, 1)}
	ref, err := BuildProReference(ProReferenceBuild{ID: "x", Description: "tres mapas", UpdatedAt: "2026-11-06T00:00:00Z", Demos: demos})
	if err != nil {
		t.Fatal(err)
	}
	if len(ref.Matches) != 3 || ref.Baseline.Rounds != 66 || ref.Baseline.Matches != 3 || ref.Baseline.Players != 10 {
		t.Fatalf("baseline = %+v over %d matches", ref.Baseline, len(ref.Matches))
	}
	nuke, ok := ref.Maps["de_nuke"]
	if !ok || nuke.Baseline.Rounds != 42 || nuke.Players[0].Matches != 2 || nuke.Players[0].Rounds != 42 {
		t.Fatalf("nuke = %+v", nuke.Baseline)
	}
	// p0 over both nuke demos: 10+0 and 10+3 kills in 42 rounds.
	if got := nuke.Players[0].Values[MetricKPR]; !near(got, round4(23.0/42)) {
		t.Fatalf("p0 nuke kpr = %v", got)
	}
	// Every player-demo adds 2 to the sum of squares over its mean per round,
	// pooled per player: three demos of n rounds each pool exactly.
	var dev float64
	var df int
	for i := range 10 {
		n := 66
		sum := float64(i + (i + 3) + (i + 1))
		sumSq := float64(i*i+(i+3)*(i+3)+(i+1)*(i+1)) + 6
		dev += sumSq - sum*sum/float64(n)
		df += n - 1
	}
	if got, want := ref.Baseline.Metrics[MetricKPR].RoundStddev, round4(math.Sqrt(dev/float64(df))); !near(got, want) {
		t.Fatalf("kpr round spread = %v, want %v", got, want)
	}
	if ref.Players[0].Team != "A" || ref.UpdatedAt == "" {
		t.Fatalf("rows lost team or date: %+v", ref.Players[0])
	}
}

func TestBuildProReferenceRefusesAnEmptySample(t *testing.T) {
	if _, err := BuildProReference(ProReferenceBuild{ID: "x", Description: "nada"}); err == nil {
		t.Fatal("a reference without demos must not build")
	}
}
