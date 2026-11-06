package tacticalplan

import "testing"

func TestMetricsSplitsEachPlayerByTheirTeamBuy(t *testing.T) {
	matches := metricsMatches()
	matches[0].Document.Rounds[0].Economy = Economy{CTBuy: BuyFull, TBuy: BuyForce}
	r := Metrics(matches, MetricsOptions{})

	ct := playerBy(t, r, "11").Buys
	if len(ct) != 2 || ct[0].Buy != BuyFull || ct[1].Buy != BuyUnknown {
		t.Fatalf("CT buys = %+v, want full then unknown", ct)
	}
	if got := ct[0]; got.Rounds != 1 || got.Won != 1 || got.Kills != 1 || got.Deaths != 1 {
		t.Fatalf("CT full line = %+v, want one won round with one kill and one death", got)
	}

	tSide := playerBy(t, r, "21").Buys
	if got := tSide[0]; got.Buy != BuyForce || got.Rounds != 1 || got.Won != 0 || got.Kills != 1 || got.Deaths != 1 {
		t.Fatalf("T force line = %+v, want one lost round with one kill and one death", got)
	}
	total := 0
	for _, line := range tSide {
		total += line.Rounds
	}
	if p := playerBy(t, r, "21"); total != p.Rounds {
		t.Fatalf("buy rounds add to %d, want the player's %d rounds", total, p.Rounds)
	}
}
