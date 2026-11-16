package moments

import (
	"fmt"
	"sort"
)

// Auto-Shorts limits: the default score bar, how many Shorts one batch makes and the hard cap.
const (
	DefaultBatchMinScore = 0.6
	DefaultBatchLimit    = 5
	MaxBatchLimit        = 10
)

// BatchOptions chooses the plays of an Auto-Shorts batch. A zero field keeps its default.
type BatchOptions struct {
	MinScore float64
	Limit    int
}

// Normalize fills the defaults and rejects a bar outside 0..1 or a limit outside 1..MaxBatchLimit.
func (o BatchOptions) Normalize() (BatchOptions, error) {
	if o.MinScore == 0 {
		o.MinScore = DefaultBatchMinScore
	}
	if o.Limit == 0 {
		o.Limit = DefaultBatchLimit
	}
	if o.MinScore < 0 || o.MinScore > 1 {
		return o, fmt.Errorf("min_score must be between 0 and 1")
	}
	if o.Limit < 1 || o.Limit > MaxBatchLimit {
		return o, fmt.Errorf("limit must be between 1 and %d", MaxBatchLimit)
	}
	return o, nil
}

// PickBatch keeps the best kill play of each round that reaches opts.MinScore,
// then the opts.Limit best of those, returned in match order. One play per round
// also keeps the plays of a multi-POV job from overlapping in time.
func PickBatch(ms []Moment, opts BatchOptions) []Moment {
	best := map[int]Moment{}
	for _, m := range ms {
		if m.Death != nil || m.Events.Kills == 0 || m.Score < opts.MinScore {
			continue
		}
		if cur, ok := best[m.Round]; !ok || better(m, cur) {
			best[m.Round] = m
		}
	}
	picked := make([]Moment, 0, len(best))
	for _, m := range best {
		picked = append(picked, m)
	}
	sort.Slice(picked, func(i, j int) bool { return better(picked[i], picked[j]) })
	if len(picked) > opts.Limit {
		picked = picked[:opts.Limit]
	}
	sort.Slice(picked, func(i, j int) bool {
		if picked[i].TickStart != picked[j].TickStart {
			return picked[i].TickStart < picked[j].TickStart
		}
		return picked[i].SegmentID < picked[j].SegmentID
	})
	return picked
}

// better orders plays like Rank: score, then kills, then segment ID for a stable tie.
func better(a, b Moment) bool {
	if a.Score != b.Score {
		return a.Score > b.Score
	}
	if a.Events.Kills != b.Events.Kills {
		return a.Events.Kills > b.Events.Kills
	}
	return a.SegmentID < b.SegmentID
}
