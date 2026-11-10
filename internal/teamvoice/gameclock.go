package teamvoice

import (
	"fmt"
	"math"
	"slices"
	"time"

	"github.com/rechedev9/cliphub/internal/library"
)

// GameClockInput is a plugin recording's wall-clock start and the moments CS2 said each round went live.
type GameClockInput struct {
	StartedAt time.Time
	Rounds    []library.RecordingRound
	Timeline  Timeline
}

const (
	// A round agrees with the offset when its own estimate falls this close to it.
	gameClockToleranceMS = 1500
	// How far CS2's round numbers may run from the demo's: a demo that starts a round late, or a knife round.
	gameClockMaxShift  = 3
	minGameClockRounds = 2
)

// GameClock places a recording by when each round's freeze time ended: on CS2's clock while recording,
// and in the demo. It needs two rounds that agree and most of the rounds seen; else ok is false.
func GameClock(in GameClockInput) (library.VoiceSync, bool) {
	if len(in.Rounds) < minGameClockRounds || len(in.Timeline.Rounds) == 0 {
		return library.VoiceSync{}, false
	}
	freezeEnd := map[int]int64{}
	for _, r := range in.Timeline.Rounds {
		freezeEnd[r.Number] = r.FreezeEndMS
	}
	type fit struct {
		shift, agree int
		offsetMS     int64
	}
	var best fit
	for _, shift := range shiftsByDistance(gameClockMaxShift) {
		var offsets []int64
		for _, mark := range in.Rounds {
			demoMS, ok := freezeEnd[mark.Round+shift]
			if !ok {
				continue
			}
			offsets = append(offsets, demoMS-mark.LiveAt.Sub(in.StartedAt).Milliseconds())
		}
		if len(offsets) == 0 {
			continue
		}
		median := medianOf(offsets)
		var sum int64
		agree := 0
		for _, o := range offsets {
			if abs64(o-median) <= gameClockToleranceMS {
				sum += o
				agree++
			}
		}
		if agree > best.agree {
			best = fit{shift: shift, agree: agree, offsetMS: int64(math.Round(float64(sum)/float64(agree)/10) * 10)}
		}
	}
	if best.agree < minGameClockRounds || float64(best.agree) < 0.6*float64(len(in.Rounds)) || abs64(best.offsetMS) > library.MaxVoiceOffsetMS {
		return library.VoiceSync{}, false
	}
	confidence := math.Round(float64(best.agree)/float64(len(in.Rounds))*100) / 100
	return library.VoiceSync{State: library.VoiceAuto, Method: library.VoiceMethodGameClock, OffsetMS: best.offsetMS, AutoOffsetMS: best.offsetMS,
		Confidence: confidence, Note: fmt.Sprintf("Ajustada con la hora de las rondas de CS2: %d de %d rondas cuadran.", best.agree, len(in.Rounds))}, true
}

// shiftsByDistance is 0, 1, -1, 2, -2, ...: on a tie the nearer numbering wins.
func shiftsByDistance(n int) []int {
	out := []int{0}
	for d := 1; d <= n; d++ {
		out = append(out, d, -d)
	}
	return out
}

func medianOf(xs []int64) int64 {
	sorted := slices.Clone(xs)
	slices.Sort(sorted)
	return sorted[len(sorted)/2]
}

func abs64(x int64) int64 {
	if x < 0 {
		return -x
	}
	return x
}
