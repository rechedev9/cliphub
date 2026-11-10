package tacticalplan

import (
	"math"
	"strings"
	"testing"
)

func TestDefaultProReferenceCoversEveryMetricAndMap(t *testing.T) {
	ref := DefaultProReference()
	if !strings.Contains(ref.Description, "Iberian Soul") || len(ref.Matches) != 4 {
		t.Fatalf("reference = %q over %d matches", ref.Description, len(ref.Matches))
	}
	for _, metric := range []string{MetricKPR, MetricADR, MetricWPA} {
		if ref.Baseline.Metrics[metric].RoundStddev <= 0 {
			t.Errorf("%s has no round spread", metric)
		}
	}
	if ref.Baseline.Metrics[MetricKAST].RoundStddev != 0 {
		t.Error("a percentage must not carry a round spread")
	}
	if b, rows, used := ref.For("de_nuke"); used != "de_nuke" || b.Players < ProReferenceMinPlayers || len(rows) != b.Players {
		t.Fatalf("nuke = %d players, %d rows, map %q", b.Players, len(rows), used)
	}
	if _, rows, used := ref.For("de_mirage"); used != "" || len(rows) != len(ref.Players) {
		t.Fatalf("a map without an entry must fall back to every map, got %q", used)
	}
	if _, err := DecodeProReference([]byte(`{"description":"x","players":[{"steamid64":"1"}],"baseline":{"metrics":{}}}`)); err == nil {
		t.Fatal("a reference without metrics must not load")
	}
}

func TestProLeaderboardRanksAndFilters(t *testing.T) {
	rows := []ProReferenceRow{
		{SteamID64: "1", Name: "a", Rounds: 40, Values: map[string]float64{MetricDPR: 0.7}},
		{SteamID64: "2", Name: "b", Rounds: 10, Values: map[string]float64{MetricDPR: 0.1}},
		{SteamID64: "3", Name: "c", Rounds: 40, Values: map[string]float64{MetricDPR: 0.5}},
	}
	got := ProLeaderboard(rows, MetricDPR, 20, 0)
	if len(got) != 2 || got[0].Name != "c" || got[0].Rank != 1 || got[1].Name != "a" {
		t.Fatalf("dpr board = %+v, want c before a and b under the minimum", got)
	}
}

func TestBaselineSpreadRemovesSamplingNoise(t *testing.T) {
	sample := BaselineSample{players: map[string]*playerTotals{}}
	kills := []int{50, 60, 70, 80, 90, 100}
	for i, k := range kills {
		sample.players[string(rune('a'+i))] = &playerTotals{steamID: string(rune('a' + i)), rounds: 100, kills: k}
	}
	b := sample.Baseline(1, ReferenceSpread{RoundStddev: map[string]float64{MetricKPR: 0.5}})
	kpr := b.Metrics[MetricKPR]
	// Values 0.5 to 1.0: variance 0.035; each player's noise is 0.25/100.
	if !near(kpr.Mean, 0.75) || !near(kpr.Stddev, round4(math.Sqrt(0.035))) || !near(kpr.PriorStddev, round4(math.Sqrt(0.035-0.0025))) {
		t.Fatalf("kpr = %+v", kpr)
	}

	small := BaselineSample{players: map[string]*playerTotals{"a": sample.players["a"], "b": sample.players["b"]}}
	fallback := small.Baseline(1, ReferenceSpread{PriorStddev: map[string]float64{MetricKPR: 0.09}})
	if got := fallback.Metrics[MetricKPR].PriorStddev; got != 0.09 {
		t.Fatalf("two players borrow the reference prior, got %v", got)
	}
}

func TestProLeaderboardDerivesKDAndHasNoRating(t *testing.T) {
	rows := []ProReferenceRow{{SteamID64: "1", Name: "a", Rounds: 40, Values: map[string]float64{MetricKPR: 0.8, MetricDPR: 0.5}}}
	got := ProLeaderboard(rows, MetricKD, 20, 0)
	if len(got) != 1 || got[0].Value != 1.6 || got[0].Metrics[MetricKD] != 1.6 {
		t.Fatalf("kd board = %+v, want kills per round over deaths per round", got)
	}
	if got := ProLeaderboard(rows, MetricRating, 20, 0); len(got) != 0 {
		t.Fatalf("rating board = %+v, want none: the reference keeps no assists", got)
	}
}
