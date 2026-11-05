package vodplays

import (
	"fmt"
	"math"
	"sort"
)

const (
	// Kills this close together are one play; a CS2 round rarely leaves more
	// than this between the kills of one multi-kill.
	playGapSeconds = 12.0
	leadInSeconds  = 6.0
	tailSeconds    = 3.0
	fastKillGap    = 2.5
)

type Play struct {
	ID           string    `json:"id"`
	Label        string    `json:"label"`
	StartSeconds float64   `json:"start_seconds"`
	EndSeconds   float64   `json:"end_seconds"`
	KillSeconds  []float64 `json:"kill_seconds"`
	Score        int       `json:"score"`
}

func (p Play) Kills() int { return len(p.KillSeconds) }

// GroupPlays turns kills into plays with a lead-in and a tail clamped to the
// video, scored 0 to 100 from the number of kills and how fast they came.
// The result is ordered by time; ids follow that order.
func GroupPlays(kills []Kill, durationSeconds float64) []Play {
	times := make([]float64, 0, len(kills))
	for _, k := range kills {
		times = append(times, k.Seconds)
	}
	sort.Float64s(times)
	var plays []Play
	for i := 0; i < len(times); {
		j := i + 1
		for j < len(times) && times[j]-times[j-1] <= playGapSeconds {
			j++
		}
		group := append([]float64(nil), times[i:j]...)
		start := math.Max(0, group[0]-leadInSeconds)
		end := group[len(group)-1] + tailSeconds
		if durationSeconds > 0 {
			end = math.Min(end, durationSeconds)
		}
		plays = append(plays, Play{
			ID:           fmt.Sprintf("play-%03d", len(plays)+1),
			Label:        playLabel(len(group)),
			StartSeconds: round3(start),
			EndSeconds:   round3(end),
			KillSeconds:  group,
			Score:        playScore(group),
		})
		i = j
	}
	return plays
}

// BestPlays keeps the plays with at least minKills, highest score first,
// then earliest; limit <= 0 keeps them all.
func BestPlays(plays []Play, minKills, limit int) []Play {
	var kept []Play
	for _, p := range plays {
		if p.Kills() >= minKills {
			kept = append(kept, p)
		}
	}
	sort.SliceStable(kept, func(a, b int) bool {
		if kept[a].Score != kept[b].Score {
			return kept[a].Score > kept[b].Score
		}
		return kept[a].StartSeconds < kept[b].StartSeconds
	})
	if limit > 0 && len(kept) > limit {
		kept = kept[:limit]
	}
	return kept
}

func playScore(kills []float64) int {
	base := []int{0, 20, 45, 65, 82, 100}
	score := base[min(len(kills), len(base)-1)]
	for i := 1; i < len(kills); i++ {
		if kills[i]-kills[i-1] <= fastKillGap {
			score += 5
		}
	}
	return min(score, 100)
}

func playLabel(kills int) string {
	switch kills {
	case 1:
		return "1 kill"
	case 2:
		return "Doble kill"
	case 3:
		return "Triple kill"
	case 4:
		return "4 kills"
	default:
		return "ACE"
	}
}

func round3(v float64) float64 {
	return math.Round(v*1000) / 1000
}
