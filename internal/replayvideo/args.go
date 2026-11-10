package replayvideo

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// localProtocols keeps FFmpeg on local files and the frames pipe.
const localProtocols = "file,pipe"

// inlineGraphLimit is the longest filter graph kept on the command line; Windows caps the whole line at 32,767 characters.
var inlineGraphLimit = 4096

// Command is how FFmpeg runs a plan. Graph, when set, is written to the plan's GraphFile first.
type Command struct {
	Args  []string
	Graph string
	Voice bool
}

// Args is the FFmpeg command for a plan: JPEG frames from stdin, each voice track once as an
// input after it, and an MP4 at Out.
func Args(p Plan) (Command, error) {
	if err := p.Validate(); err != nil {
		return Command{}, err
	}
	fps := strconv.Itoa(p.FPS)
	cmd := Command{Args: []string{"-hide_banner", "-loglevel", "error", "-y", "-protocol_whitelist", localProtocols,
		"-f", "image2pipe", "-framerate", fps, "-c:v", "mjpeg", "-i", "pipe:0"}}
	voices := p.Voices()
	if voices == nil {
		cmd.Args = append(cmd.Args, "-map", "0:v")
	} else {
		cmd.Voice = true
		inputs, graph := voiceGraph(p, voices)
		for _, track := range inputs {
			cmd.Args = append(cmd.Args, "-i", p.Tracks[track].Path)
		}
		if len(graph) > inlineGraphLimit && p.GraphFile != "" {
			cmd.Graph = graph
			cmd.Args = append(cmd.Args, "-/filter_complex", p.GraphFile)
		} else {
			cmd.Args = append(cmd.Args, "-filter_complex", graph)
		}
		cmd.Args = append(cmd.Args, "-map", "0:v", "-map", "[aout]")
	}
	cmd.Args = append(cmd.Args, "-c:v", "libx264", "-preset", "veryfast", "-crf", "18", "-pix_fmt", "yuv420p", "-r", fps)
	if cmd.Voice {
		cmd.Args = append(cmd.Args, "-c:a", "aac", "-b:a", "160k", "-ar", strconv.Itoa(sampleRate), "-ac", "2")
	}
	cmd.Args = append(cmd.Args, "-t", strconv.FormatFloat(p.Seconds(), 'f', 3, 64), "-movflags", "+faststart", "-f", "mp4", p.Out)
	return cmd, nil
}

// voiceGraph splits each heard track into its cuts, mixes each segment's cuts and joins the
// segments. inputs are the tracks in input order, from input 1.
func voiceGraph(p Plan, voices []SegmentVoice) (inputs []int, graph string) {
	uses := make([][]int, len(p.Tracks))
	for j, v := range voices {
		for _, c := range v.Cuts {
			uses[c.Track] = append(uses[c.Track], j)
		}
	}
	var parts []string
	for track, segs := range uses {
		if len(segs) == 0 {
			continue
		}
		inputs = append(inputs, track)
		src := []string{fmt.Sprintf("[%d:a]", len(inputs))}
		if len(segs) > 1 {
			src = src[:0]
			for i := range segs {
				src = append(src, fmt.Sprintf("[t%ds%d]", track, i))
			}
			parts = append(parts, fmt.Sprintf("[%d:a]asplit=%d%s", len(inputs), len(segs), strings.Join(src, "")))
		}
		for i, j := range segs {
			for _, c := range voices[j].Cuts {
				if c.Track == track {
					parts = append(parts, fmt.Sprintf("%satrim=start=%s:duration=%s,asetpts=PTS-STARTPTS[c%dt%d]", src[i], msSeconds(c.SeekMS), msSeconds(c.LengthMS), j, track))
				}
			}
		}
	}
	for j, v := range voices {
		labels := make([]string, 0, len(v.Cuts))
		for _, c := range v.Cuts {
			labels = append(labels, fmt.Sprintf("[c%dt%d]", j, c.Track))
		}
		out := fmt.Sprintf("a%d", j)
		if len(voices) == 1 {
			out = "aout"
		}
		seconds := float64(p.Segments[j].Frames) / float64(p.FPS)
		parts = append(parts, segmentGraph(segmentAudio{inputs: labels, lead: v.LeadMS, speed: p.Speed, seconds: seconds, out: out}))
	}
	if len(voices) > 1 {
		var concat strings.Builder
		for j := range voices {
			fmt.Fprintf(&concat, "[a%d]", j)
		}
		fmt.Fprintf(&concat, "concat=n=%d:v=0:a=1[aout]", len(voices))
		parts = append(parts, concat.String())
	}
	return inputs, strings.Join(parts, ";")
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
	chain := append([]string{fmt.Sprintf("%samix=inputs=%d:duration=longest:normalize=0", strings.Join(s.inputs, ""), len(s.inputs))}, tempo(s.speed)...)
	if lead := int64(math.Round(float64(s.lead) / s.speed)); lead > 0 {
		chain = append(chain, fmt.Sprintf("adelay=%d:all=1", lead))
	}
	return strings.Join(append(chain, tail), ",")
}

// tempo plays the voice at the video's speed; one atempo goes no slower than 0.5, so 0.25 takes two.
func tempo(speed float64) []string {
	switch {
	case speed == 1:
		return nil
	case speed < 0.5:
		return []string{"atempo=0.5", "atempo=" + strconv.FormatFloat(speed/0.5, 'f', -1, 64)}
	default:
		return []string{"atempo=" + strconv.FormatFloat(speed, 'f', -1, 64)}
	}
}

func msSeconds(ms int64) string {
	return strconv.FormatFloat(float64(ms)/1000, 'f', 3, 64)
}
