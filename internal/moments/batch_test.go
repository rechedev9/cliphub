package moments

import (
	"slices"
	"testing"
)

func play(id string, round, tick int, score float64, kills int) Moment {
	return Moment{SegmentID: id, Round: round, TickStart: tick, Score: score, Events: Events{Kills: kills}}
}

func segmentIDs(ms []Moment) []string {
	out := make([]string, 0, len(ms))
	for _, m := range ms {
		out = append(out, m.SegmentID)
	}
	return out
}

func TestPickBatchKeepsBestPlayPerRoundOverTheBar(t *testing.T) {
	ranked := []Moment{
		play("seg-005", 9, 9000, 0.81, 4),
		play("seg-002", 3, 3000, 0.7, 3),
		play("seg-003", 3, 3500, 0.65, 2),
		play("seg-004", 6, 6000, 0.62, 2),
		play("seg-001", 1, 1000, 0.43, 1),
	}
	got := PickBatch(ranked, BatchOptions{MinScore: 0.6, Limit: 5})
	if want := []string{"seg-002", "seg-004", "seg-005"}; !slices.Equal(segmentIDs(got), want) {
		t.Fatalf("picked = %v, want %v in match order", segmentIDs(got), want)
	}
}

func TestPickBatchLimitKeepsTheHighestScores(t *testing.T) {
	ranked := []Moment{
		play("seg-001", 1, 1000, 0.7, 3),
		play("seg-002", 2, 2000, 0.9, 5),
		play("seg-003", 3, 3000, 0.65, 2),
		play("seg-004", 4, 4000, 0.8, 4),
	}
	got := PickBatch(ranked, BatchOptions{MinScore: 0.6, Limit: 2})
	if want := []string{"seg-002", "seg-004"}; !slices.Equal(segmentIDs(got), want) {
		t.Fatalf("picked = %v, want %v", segmentIDs(got), want)
	}
}

func TestPickBatchSkipsDeathsAndPlaysWithoutKills(t *testing.T) {
	death := play("seg-001", 1, 1000, 0.9, 1)
	death.Death = &Death{Tick: 1100}
	lineup := play("seg-002", 2, 2000, 0.9, 0)
	tie := []Moment{play("seg-004", 3, 3100, 0.7, 2), play("seg-003", 3, 3000, 0.7, 3)}
	got := PickBatch(append([]Moment{death, lineup}, tie...), BatchOptions{MinScore: 0.6, Limit: 5})
	if want := []string{"seg-003"}; !slices.Equal(segmentIDs(got), want) {
		t.Fatalf("picked = %v, want %v (more kills break a score tie)", segmentIDs(got), want)
	}
}

func TestBatchOptionsNormalize(t *testing.T) {
	got, err := BatchOptions{}.Normalize()
	if err != nil || got.MinScore != DefaultBatchMinScore || got.Limit != DefaultBatchLimit {
		t.Fatalf("defaults = %+v, %v", got, err)
	}
	for _, bad := range []BatchOptions{{MinScore: 1.2}, {MinScore: -0.1}, {Limit: MaxBatchLimit + 1}, {Limit: -1}} {
		if _, err := bad.Normalize(); err == nil {
			t.Errorf("%+v was accepted", bad)
		}
	}
}
