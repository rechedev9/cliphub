package tacticalplan

import (
	"math"

	"github.com/rechedev9/cliphub/internal/radarmap"
)

// These mirror web/lib/replay/floor-assign.ts (FLOOR_BAND, STILL_TICKS, STILL_RISE), so a heatmap
// and the Replay agree on the floor of every spot: change them together.
const (
	// floorBand is how far past the altitude cut a player must get before the floor changes:
	// a ramp or a stair crosses the cut slowly, jump by jump.
	floorBand = 48
	// floorStillTicks of standing still within floorStillRise drop the band: a camper takes
	// the floor of the cut, so the band never leaves them on the wrong block.
	floorStillTicks = 128
	floorStillRise  = 6
)

// assignFloor is the floor at height z, keeping previous while z stays inside the dead band
// around the cut; an empty previous takes the hard cut.
func assignFloor(cal radarmap.Calibration, z float64, previous string) string {
	if cal.LowerAltitudeMax == nil {
		return radarmap.LevelDefault
	}
	cut := *cal.LowerAltitudeMax
	switch previous {
	case "":
		return cal.Level(z)
	case radarmap.LevelLower:
		if z > cut+floorBand {
			return radarmap.LevelDefault
		}
		return radarmap.LevelLower
	default:
		if z <= cut-floorBand {
			return radarmap.LevelLower
		}
		return radarmap.LevelDefault
	}
}

// floorTimeline is the floor of each sample of each frame of one round, settled by walking the
// frames in order once, as createFloorTimeline does in the web.
type floorTimeline [][]string

// slotTrack is one player's height and life at each frame of a round.
type slotTrack struct {
	z       []float64
	present []bool
	living  []bool
}

func newFloorTimeline(cal radarmap.Calibration, frames []Frame) floorTimeline {
	out := make(floorTimeline, len(frames))
	tracks := map[uint8]*slotTrack{}
	for i, frame := range frames {
		out[i] = make([]string, len(frame.Samples))
		for _, s := range frame.Samples {
			t, ok := tracks[s.Slot]
			if !ok {
				t = &slotTrack{z: make([]float64, len(frames)), present: make([]bool, len(frames)), living: make([]bool, len(frames))}
				tracks[s.Slot] = t
			}
			t.z[i], t.present[i], t.living[i] = s.Z, true, s.Flags.Has(FlagAlive)
		}
	}
	levels := make(map[uint8][]string, len(tracks))
	for slot, t := range tracks {
		still := stillFrames(frames, t)
		row := make([]string, len(frames))
		previous := ""
		for i := range frames {
			if !t.present[i] {
				continue
			}
			level := assignFloor(cal, t.z[i], previous)
			if still[i] {
				level = assignFloor(cal, t.z[i], "")
			}
			row[i], previous = level, level
		}
		levels[slot] = row
	}
	for i, frame := range frames {
		for j, s := range frame.Samples {
			out[i][j] = levels[s.Slot][i]
		}
	}
	return out
}

// stillFrames marks the frames in which a living player stands still: their height stays within
// floorStillRise for at least floorStillTicks. A frame without the player, or with them dead, ends a run.
func stillFrames(frames []Frame, t *slotTrack) []bool {
	n := len(frames)
	still := make([]bool, n)
	for start := 0; start < n; {
		if !t.living[start] {
			start++
			continue
		}
		low, high, end := t.z[start], t.z[start], start
		for end+1 < n && t.living[end+1] && math.Max(high, t.z[end+1])-math.Min(low, t.z[end+1]) <= floorStillRise {
			end++
			low, high = math.Min(low, t.z[end]), math.Max(high, t.z[end])
		}
		if frames[end].Tick-frames[start].Tick >= floorStillTicks {
			for i := start; i <= end; i++ {
				still[i] = true
			}
		}
		start = end + 1
	}
	return still
}

// at is the floor of sample j of frame i.
func (t floorTimeline) at(frame, sample int) string { return t[frame][sample] }
