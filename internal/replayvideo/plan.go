// Package replayvideo encodes the 2D Replay's JPEG frames as an H.264 MP4 with FFmpeg, with the
// team voice cut to each segment's demo time by the sync offset and mixed into one AAC track.
package replayvideo

import (
	"errors"
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"
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
	Speeds     = []float64{0.5, 1, 2, 4}
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
		return errors.New("speed must be 0.5, 1, 2 or 4")
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

// localProtocols keeps FFmpeg on local files and the frames pipe.
const localProtocols = "file,pipe"

// Args is the FFmpeg command line for a plan: JPEG frames from stdin, the voice cuts as inputs
// after it, and an MP4 at Out. voice says whether the video carries an audio track.
func Args(p Plan) (args []string, voice bool, err error) {
	if err := p.Validate(); err != nil {
		return nil, false, err
	}
	fps := strconv.Itoa(p.FPS)
	args = []string{"-hide_banner", "-loglevel", "error", "-y", "-protocol_whitelist", localProtocols,
		"-f", "image2pipe", "-framerate", fps, "-c:v", "mjpeg", "-i", "pipe:0"}
	voices := p.Voices()
	if voices != nil {
		var graph []string
		input := 1
		for i, v := range voices {
			labels := make([]string, 0, len(v.Cuts))
			for _, c := range v.Cuts {
				args = append(args, "-ss", msSeconds(c.SeekMS), "-t", msSeconds(c.LengthMS), "-i", p.Tracks[c.Track].Path)
				labels = append(labels, fmt.Sprintf("[%d:a]", input))
				input++
			}
			seconds := float64(p.Segments[i].Frames) / float64(p.FPS)
			graph = append(graph, segmentGraph(segmentAudio{inputs: labels, lead: v.LeadMS, speed: p.Speed, seconds: seconds, out: fmt.Sprintf("a%d", i)}))
		}
		if len(voices) == 1 {
			graph[0] = strings.TrimSuffix(graph[0], "[a0]") + "[aout]"
		} else {
			var concat strings.Builder
			for i := range voices {
				fmt.Fprintf(&concat, "[a%d]", i)
			}
			fmt.Fprintf(&concat, "concat=n=%d:v=0:a=1[aout]", len(voices))
			graph = append(graph, concat.String())
		}
		args = append(args, "-filter_complex", strings.Join(graph, ";"), "-map", "0:v", "-map", "[aout]")
	} else {
		args = append(args, "-map", "0:v")
	}
	args = append(args, "-c:v", "libx264", "-preset", "veryfast", "-crf", "18", "-pix_fmt", "yuv420p", "-r", fps)
	if voices != nil {
		args = append(args, "-c:a", "aac", "-b:a", "160k", "-ar", strconv.Itoa(sampleRate), "-ac", "2")
	}
	args = append(args, "-t", strconv.FormatFloat(p.Seconds(), 'f', 3, 64), "-movflags", "+faststart", "-f", "mp4", p.Out)
	return args, voices != nil, nil
}

type segmentAudio struct {
	inputs  []string
	lead    int64
	speed   float64
	seconds float64
	out     string
}

// segmentGraph is one segment's audio: its cuts mixed, played at the video's speed, after the
// lead of silence, then padded or trimmed to the segment's length so the segments line up.
func segmentGraph(s segmentAudio) string {
	length := strconv.FormatFloat(s.seconds, 'f', 3, 64)
	tail := fmt.Sprintf("aresample=%d,aformat=sample_fmts=fltp:channel_layouts=stereo,apad,atrim=duration=%s[%s]", sampleRate, length, s.out)
	if len(s.inputs) == 0 {
		return fmt.Sprintf("anullsrc=r=%d:cl=stereo,%s", sampleRate, tail)
	}
	chain := []string{fmt.Sprintf("%samix=inputs=%d:duration=longest:normalize=0", strings.Join(s.inputs, ""), len(s.inputs))}
	if s.speed != 1 {
		chain = append(chain, "atempo="+strconv.FormatFloat(s.speed, 'f', -1, 64))
	}
	if lead := int64(math.Round(float64(s.lead) / s.speed)); lead > 0 {
		chain = append(chain, fmt.Sprintf("adelay=%d:all=1", lead))
	}
	return strings.Join(append(chain, tail), ",")
}

func msSeconds(ms int64) string {
	return strconv.FormatFloat(float64(ms)/1000, 'f', 3, 64)
}
