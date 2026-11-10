// Package replayvideo encodes the 2D Replay's JPEG frames as an H.264 MP4 with FFmpeg, with the
// team voice cut to each segment's demo time by the sync offset and mixed into one AAC track.
package replayvideo

import (
	"errors"
	"fmt"
	"math"
	"slices"
)

const (
	// MaxSeconds bounds one video's length, so the frames the Replay holds fit in memory.
	MaxSeconds = 120
	MinSide    = 64
	MaxSide    = 2560
	sampleRate = 48000
)

// FrameRates and Speeds are the values a plan takes.
var (
	FrameRates = []int{24, 30, 60}
	Speeds     = []float64{0.25, 0.5, 1, 2, 4}
)

// Track is one stored voice recording and its length.
type Track struct {
	Path       string
	DurationMS int64
}

// Range is a stretch of the demo, in ticks.
type Range struct {
	FromTick int
	ToTick   int
}

// Segment is one stretch of the video: its frame count and, for the voice, the demo ticks it shows.
type Segment struct {
	Frames int
	Voice  *Range
}

// Sync places the recording on the demo: audio time 0 plays at demo time OffsetMS.
type Sync struct {
	Tickrate float64
	OffsetMS int64
}

// Cut is the part of one track a segment plays: SeekMS into the file, for LengthMS.
type Cut struct {
	Track    int
	SeekMS   int64
	LengthMS int64
}

// SegmentVoice is what a segment hears: LeadMS of silence in demo time, then the cuts together.
type SegmentVoice struct {
	LeadMS int64
	Cuts   []Cut
}

// CutVoice finds what each track recorded during r. A track that ends before r, or a recording
// that starts after it, leaves no cut.
func CutVoice(r Range, sync Sync, tracks []Track) SegmentVoice {
	if sync.Tickrate <= 0 || r.ToTick <= r.FromTick {
		return SegmentVoice{}
	}
	from := demoMS(r.FromTick, sync.Tickrate) - sync.OffsetMS
	to := demoMS(r.ToTick, sync.Tickrate) - sync.OffsetMS
	seek := max(0, from)
	out := SegmentVoice{LeadMS: seek - from}
	for i, t := range tracks {
		end := min(to, t.DurationMS)
		if end > seek {
			out.Cuts = append(out.Cuts, Cut{Track: i, SeekMS: seek, LengthMS: end - seek})
		}
	}
	if len(out.Cuts) == 0 {
		out.LeadMS = 0
	}
	return out
}

func demoMS(tick int, tickrate float64) int64 {
	return int64(math.Round(float64(tick) * 1000 / tickrate))
}

// Plan is one video: the frames' rate and size, the playback speed, its segments in order and,
// with Tracks, the team voice placed by Sync.
type Plan struct {
	FPS      int
	Width    int
	Height   int
	Speed    float64
	Segments []Segment
	Sync     Sync
	Tracks   []Track
	Out      string
	// GraphFile holds the filter graph when it is too long for the command line.
	GraphFile string
}

// Frames is how many frames the plan's segments take.
func (p Plan) Frames() int {
	n := 0
	for _, s := range p.Segments {
		n += s.Frames
	}
	return n
}

// Seconds is how long the video plays.
func (p Plan) Seconds() float64 {
	if p.FPS <= 0 {
		return 0
	}
	return float64(p.Frames()) / float64(p.FPS)
}

// Validate checks what FFmpeg cannot be asked for: H.264 in 4:2:0 needs even sides.
func (p Plan) Validate() error {
	if !slices.Contains(FrameRates, p.FPS) {
		return errors.New("fps must be 24, 30 or 60")
	}
	if !slices.Contains(Speeds, p.Speed) {
		return errors.New("speed must be 0.25, 0.5, 1, 2 or 4")
	}
	if p.Width < MinSide || p.Height < MinSide || p.Width > MaxSide || p.Height > MaxSide || p.Width%2 != 0 || p.Height%2 != 0 {
		return fmt.Errorf("width and height must be even, from %d to %d", MinSide, MaxSide)
	}
	if len(p.Segments) == 0 {
		return errors.New("the video needs a segment")
	}
	for _, s := range p.Segments {
		if s.Frames < 1 {
			return errors.New("every segment needs a frame")
		}
	}
	if p.Frames() > MaxSeconds*p.FPS {
		return fmt.Errorf("the video takes at most %d s", MaxSeconds)
	}
	return nil
}

// Voices are the segments' cuts; nil when the plan has no voice or none of it falls in the video.
func (p Plan) Voices() []SegmentVoice {
	if len(p.Tracks) == 0 {
		return nil
	}
	out := make([]SegmentVoice, len(p.Segments))
	heard := false
	for i, s := range p.Segments {
		if s.Voice != nil {
			out[i] = CutVoice(*s.Voice, p.Sync, p.Tracks)
			heard = heard || len(out[i].Cuts) > 0
		}
	}
	if !heard {
		return nil
	}
	return out
}
