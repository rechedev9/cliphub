package tacticalplan

import "testing"

// baselineMatches are the two metrics matches with a box score: a1 opens the
// round against b0, b0's death is traded and a0 survives.
func baselineMatches() []MetricsMatch {
	matches := metricsMatches()
	for i := range matches {
		players := matches[i].Document.Rounds[0].Players
		players[0].Survived = true
		players[1].Damage, players[1].OpeningKill = 100, true
		players[2].OpeningDeath, players[2].Traded = true, true
		players[3].Damage = 50
	}
	return matches
}

func TestBaselineAveragesPlayersReadOneMatchAtATime(t *testing.T) {
	var sample BaselineSample
	for _, m := range baselineMatches() {
		sample.Add(Metrics([]MetricsMatch{m}, MetricsOptions{}))
	}
	b := sample.Baseline(2)
	if b.Players != 4 || b.Rounds != 2 || b.Matches != 2 {
		t.Fatalf("baseline sample = %d players, %d rounds, %d matches", b.Players, b.Rounds, b.Matches)
	}
	want := map[string]BaselineValue{
		MetricKPR:           {Mean: 0.75, Players: 4},
		MetricDPR:           {Mean: 0.75, Players: 4},
		MetricADR:           {Mean: 37.5, Players: 4},
		MetricKAST:          {Mean: 100, Players: 4},
		MetricOpeningWin:    {Mean: 50, Players: 2},
		MetricTrade:         {Mean: 66.6667, Players: 3},
		MetricUtilityDamage: {Mean: 7.5, Players: 4},
		MetricFlashAssists:  {Mean: 0.25, Players: 4},
	}
	for metric, w := range want {
		if got := b.Metrics[metric]; !near(got.Mean, w.Mean) || got.Players != w.Players {
			t.Errorf("%s = %+v, want %+v", metric, got, w)
		}
	}
	if len(b.Metrics) != len(ReferenceMetrics) {
		t.Errorf("baseline has %d metrics, want every reference metric", len(b.Metrics))
	}

	var whole BaselineSample
	whole.Add(Metrics(baselineMatches(), MetricsOptions{}))
	if got, w := whole.Baseline(2).Metrics[MetricWPA], b.Metrics[MetricWPA]; !near(got.Mean, w.Mean) {
		t.Errorf("wpa per round = %v match by match, %v at once", w.Mean, got.Mean)
	}
	if sample.Baseline(3).Players != 0 {
		t.Error("players under the round minimum must not count")
	}
}

func TestLeaderboardRanksBestFirst(t *testing.T) {
	var sample BaselineSample
	sample.Add(Metrics(baselineMatches(), MetricsOptions{}))

	dpr := sample.Leaderboard(MetricDPR, 1, 0)
	if len(dpr) != 4 || dpr[0].SteamID64 != "10" || dpr[0].Value != 0 || dpr[0].Rank != 1 {
		t.Fatalf("dpr board = %+v, want a0 first with no deaths", dpr)
	}
	kpr := sample.Leaderboard(MetricKPR, 1, 2)
	if len(kpr) != 2 || kpr[0].SteamID64 != "10" || kpr[1].SteamID64 != "11" || kpr[1].Rank != 2 || kpr[0].Matches != 2 {
		t.Fatalf("kpr board = %+v, want the two top fraggers in SteamID order", kpr)
	}
	if got := sample.Leaderboard(MetricOpeningWin, 1, 0); len(got) != 2 || got[0].Name != "a1" {
		t.Fatalf("opening board = %+v, want only the players with an opening duel", got)
	}
	if got := sample.Leaderboard("rating", 1, 0); len(got) != 0 {
		t.Fatalf("unknown metric = %+v", got)
	}
}
