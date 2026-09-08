package editor

import (
	"bytes"
	"context"
	"fmt"
	"math"
	"os/exec"
	"strings"

	"github.com/rechedev9/cliphub/internal/recapplan"
)

const transitionMotionWidth, transitionMotionHeight = 96, 54

func prepareFullDemoTransitions(ctx context.Context, short *ShortEdit) error {
	d := short.FullDemo.Effective
	short.FullDemo.Transitions = transitionDirections(d)
	if len(short.FullDemo.Transitions) == 0 || !d.Options.Transitions.Whip || d.Options.Transitions.Direction != "follow-motion" {
		return nil
	}
	for i := range short.FullDemo.Transitions {
		b := &short.FullDemo.Transitions[i]
		item := d.Timeline[b.OutgoingIndex]
		var path string
		var captureStart int
		for _, part := range short.Parts {
			if part.SegmentID == item.SourceRef {
				path = part.Input
				break
			}
		}
		for _, segment := range short.fullDemo.recording.Plan.Segments {
			if segment.ID == item.SourceRef {
				captureStart = segment.TickStart
				break
			}
		}
		if path == "" {
			return fmt.Errorf("transition motion source missing for %s", item.SourceRef)
		}
		offset, err := recapplan.TickFrames(item.SourceStartTick-captureStart, d.Clock.TickRate)
		if err != nil {
			return err
		}
		end := offset + item.SourceOffsetFrames + item.EndFrame - item.StartFrame
		start := max(offset+item.SourceOffsetFrames, end-recapplan.OutputFPS)
		// Central scene crop excludes the HUD. At most one second, 12 tiny
		// grayscale frames per cut; no optical-flow runtime or Python needed.
		command := exec.CommandContext(ctx, short.fullDemo.ffmpeg, "-v", "error", "-ss", decimal(float64(start)/60), "-i", path,
			"-t", decimal(float64(end-start)/60), "-vf", "fps=12,crop=iw*0.72:ih*0.65:iw*0.14:ih*0.15,scale=96:54,format=gray",
			"-frames:v", "12", "-an", "-f", "rawvideo", "pipe:1")
		var output bytes.Buffer
		var stderr strings.Builder
		command.Stdout, command.Stderr = &output, &stderr
		if err := command.Run(); err != nil {
			return fmt.Errorf("analyze transition motion for %s: %w: %.2000s", item.SourceRef, err, stderr.String())
		}
		data := output.Bytes()
		const size = transitionMotionWidth * transitionMotionHeight
		if len(data)%size != 0 || len(data) == 0 {
			return fmt.Errorf("invalid transition motion frames for %s", item.SourceRef)
		}
		for end := len(data); end >= 2*size; end -= size {
			if direction, ok := estimateTransitionDirection(data[end-2*size:end-size], data[end-size:end]); ok {
				b.Direction, b.DirectionSource = direction, "estimated-motion"
				break
			}
		}
	}
	return nil
}

// A bounded translation search estimates screen motion, not player intent or
// optical flow. Static/ambiguous images retain the documented alternate fallback.
func estimateTransitionDirection(previous, current []byte) (string, bool) {
	const w, h = transitionMotionWidth, transitionMotionHeight
	if len(previous) != w*h || len(current) != w*h {
		return "", false
	}
	score := func(dx, dy int) float64 {
		var total, count int
		for y := 8; y < h-8; y += 2 {
			for x := 8; x < w-8; x += 2 {
				if absInt(x-w/2) < 6 && absInt(y-h/2) < 6 {
					continue
				}
				total += absInt(int(current[y*w+x]) - int(previous[(y-dy)*w+x-dx]))
				count++
			}
		}
		return float64(total) / float64(count)
	}
	base := score(0, 0)
	if base < 2 {
		return "", false
	}
	best, bestX, bestY := base, 0, 0
	for dy := -6; dy <= 6; dy++ {
		for dx := -6; dx <= 6; dx++ {
			if value := score(dx, dy); value < best {
				best, bestX, bestY = value, dx, dy
			}
		}
	}
	if (bestX == 0 && bestY == 0) || best > base*.8 {
		return "", false
	}
	if math.Abs(float64(bestX)/w) >= math.Abs(float64(bestY)/h) {
		if bestX > 0 {
			return "right", true
		}
		return "left", true
	}
	if bestY > 0 {
		return "down", true
	}
	return "up", true
}

func absInt(value int) int {
	if value < 0 {
		return -value
	}
	return value
}
