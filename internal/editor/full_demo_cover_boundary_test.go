package editor

import (
	"strconv"
	"testing"
)

func TestFullDemoCoverSeekDoesNotRoundPastApprovedFrame(t *testing.T) {
	for _, frame := range []int{120, 121, 241, 361} {
		timestamp := float64(frame) / 60
		short := ShortEdit{CoverTimeSeconds: timestamp, FullDemo: &FullDemoRenderEvidence{}}
		args := BuildCoverFFmpegCommand("ffmpeg", short)
		for i, arg := range args {
			if arg != "-ss" {
				continue
			}
			seek, err := strconv.ParseFloat(args[i+1], 64)
			if err != nil || seek > timestamp || timestamp-seek > 0.0000011 {
				t.Fatalf("frame %d: unsafe seek %s for %v", frame, args[i+1], timestamp)
			}
		}
	}
}
