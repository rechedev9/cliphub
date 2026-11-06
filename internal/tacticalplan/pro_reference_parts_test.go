package tacticalplan

import "testing"

func part(rounds, first int) ProDemoSummary {
	d := syntheticDemo("de_ancient", rounds, 0)
	d.FirstRound, d.LastRound = first, first+rounds-1
	return d
}

func TestMergeProDemoPartsJoinsARestartedMapOnce(t *testing.T) {
	// monte vs legacy: 3 rounds, then a restart at round 4 for the other 21.
	merged, dropped := MergeProDemoParts([]ProDemoSummary{part(21, 4), part(3, 1)})
	if dropped != 0 || merged.Match.Rounds != 24 || merged.FirstRound != 1 || merged.LastRound != 24 {
		t.Fatalf("merged = %d rounds %d..%d, dropped %d", merged.Match.Rounds, merged.FirstRound, merged.LastRound, dropped)
	}
	p0 := merged.Players[0]
	if p0.Rounds != 24 || p0.Kills != 20 || p0.RoundSums[MetricKPR].N != 24 {
		t.Fatalf("p0 = %d rounds, %d kills, %d round values", p0.Rounds, p0.Kills, p0.RoundSums[MetricKPR].N)
	}
	ref, err := BuildProReference(ProReferenceBuild{ID: "x", Description: "uno", Demos: []ProDemoSummary{merged}})
	if err != nil || ref.Baseline.Matches != 1 || ref.Players[0].Matches != 1 {
		t.Fatalf("a joined map must count as one map: %+v, %v", ref.Baseline, err)
	}
}

func TestMergeProDemoPartsCountsAReplayedRoundOnce(t *testing.T) {
	// legacy vs mibr: rounds 1..3, the server goes down in 3, the restart plays 3..21.
	first := part(3, 1)
	for _, line := range first.Players {
		first.LastRoundPlayers = append(first.LastRoundPlayers, ProDemoLine{
			SteamID64: line.SteamID64, Rounds: 1, Kills: 1, Box: BoxScore{Damage: 100, UtilityRounds: 1},
			RoundSums: map[string]RoundSums{MetricKPR: {N: 1, Sum: 1, SumSq: 1}},
		})
	}
	merged, dropped := MergeProDemoParts([]ProDemoSummary{first, part(19, 3)})
	p0 := merged.Players[0]
	if dropped != 0 || merged.Match.Rounds != 21 || p0.Rounds != 21 || p0.Kills != 19 || p0.Box.Damage != 2*1500-100 || p0.RoundSums[MetricKPR].N != 21 {
		t.Fatalf("merged = %d rounds; p0 %d rounds, %d kills, %d damage, %d values", merged.Match.Rounds, p0.Rounds, p0.Kills, p0.Box.Damage, p0.RoundSums[MetricKPR].N)
	}
	if first.Players[0].Rounds != 3 {
		t.Fatal("merging must not change the parts it read")
	}
}

func TestMergeProDemoPartsKeepsTheLongerOfAReplay(t *testing.T) {
	// The server restored round 3 from a backup and the second demo replays it.
	merged, dropped := MergeProDemoParts([]ProDemoSummary{part(3, 1), part(20, 3)})
	if dropped != 1 || merged.Match.Rounds != 20 || merged.FirstRound != 3 {
		t.Fatalf("merged = %d rounds from %d, dropped %d", merged.Match.Rounds, merged.FirstRound, dropped)
	}
	noRange := part(5, 0)
	noRange.FirstRound, noRange.LastRound = 0, 0
	merged, dropped = MergeProDemoParts([]ProDemoSummary{noRange, part(19, 6)})
	if dropped != 1 || merged.Match.Rounds != 19 {
		t.Fatalf("a part without a range must not add rounds: %d rounds, dropped %d", merged.Match.Rounds, dropped)
	}
}
