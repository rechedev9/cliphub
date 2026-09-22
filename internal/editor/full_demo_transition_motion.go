package editor

import (
	"bytes"
	"context"
	"fmt"
	"math"
	"os/exec"
	"runtime"
	"strings"
	"sync"

	"github.com/rechedev9/cliphub/internal/recapplan"
)

const transitionMotionWidth, transitionMotionHeight = 96, 54

func prepareFullDemoTransitions(ctx context.Context, short *ShortEdit) error {
	d := short.FullDemo.Effective
	short.FullDemo.Transitions = transitionDirections(d)
	o := d.RenderTransitions()
	if len(short.FullDemo.Transitions) == 0 || !o.Whip || o.Direction != "follow-motion" {
		return nil
	}
	count := len(short.FullDemo.Transitions)
	return runFullDemoTransitionPool(ctx, count, fullDemoTransitionJobs(count), func(ctx context.Context, index int) error {
		return probeFullDemoTransitionMotion(ctx, short, index)
	})
}

// fullDemoTransitionJobs bounds the motion probes. Every probe is one seek plus
// at most twelve 96x54 grayscale frames, so the stage is dominated by process
// launch and seek latency rather than by CPU. It runs before any encoder
// starts, so a small pool collapses the serial prefix without competing for the
// item or voice budgets.
func fullDemoTransitionJobs(count int) int {
	if count <= 0 {
		return 0
	}
	jobs := 4
	if cpus := runtime.NumCPU(); cpus > 0 && cpus < jobs {
		jobs = cpus
	}
	if count < jobs {
		jobs = count
	}
	return jobs
}

// runFullDemoTransitionPool probes independent boundaries in a bounded worker
// pool. Each probe owns exactly one boundary index and writes only that element,
// so the recorded evidence keeps the boundary order regardless of which probe
// finishes first.
//
// Unlike the encoder pools this one never cancels its siblings on a failure:
// the probes are tiny, and letting the scheduled ones finish makes the reported
// error always the lowest failing boundary — the exact error the former serial
// loop returned — instead of whichever cancellation happened to land first.
// Cancellation from the caller still stops scheduling and kills running probes.
func runFullDemoTransitionPool(ctx context.Context, count, jobs int, probe func(ctx context.Context, index int) error) error {
	if count <= 0 {
		return nil
	}
	if jobs < 1 {
		jobs = 1
	}
	if jobs > count {
		jobs = count
	}
	failures := make([]error, count)
	var wg sync.WaitGroup
	sem := make(chan struct{}, jobs)
scheduling:
	for index := 0; index < count; index++ {
		// Never wait on a saturated pool without watching cancellation, and
		// recheck the context after acquiring a slot so a queued probe cannot
		// start once the render was cancelled.
		select {
		case sem <- struct{}{}:
		case <-ctx.Done():
			break scheduling
		}
		if ctx.Err() != nil {
			<-sem
			break scheduling
		}
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			defer func() { <-sem }()
			failures[index] = probe(ctx, index)
		}(index)
	}
	wg.Wait()
	for _, err := range failures {
		if err != nil {
			return err
		}
	}
	return ctx.Err()
}

// probeFullDemoTransitionMotion estimates the direction of one boundary. It
// reads the approved document, the parts and the recording plan, and writes
// exactly one element of short.FullDemo.Transitions, the one it owns.
func probeFullDemoTransitionMotion(ctx context.Context, short *ShortEdit, index int) error {
	d := short.FullDemo.Effective
	b := &short.FullDemo.Transitions[index]
	item := d.Timeline[b.OutgoingIndex]
	// The intro is an uploaded clip, not a recorded demo segment. Keep its
	// deterministic direction instead of looking it up in the capture plan.
	if item.Role == "bumper" {
		return nil
	}
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
	seconds := float64(end-start) / 60
	// Central scene crop excludes the HUD. At most one second, 12 tiny
	// grayscale frames per cut; no optical-flow runtime or Python needed.
	probeCtx := fullDemoTimingScope(ctx, "transitions", index, -1, seconds)
	command := exec.CommandContext(probeCtx, short.fullDemo.ffmpeg, "-v", "error", "-ss", decimal(float64(start)/60), "-i", path,
		"-t", decimal(seconds), "-vf", "fps=12,crop=iw*0.72:ih*0.65:iw*0.14:ih*0.15,scale=96:54,format=gray",
		"-frames:v", "12", "-an", "-f", "rawvideo", "pipe:1")
	var output bytes.Buffer
	var stderr strings.Builder
	// Raw frames stay on their own pipe: only stderr is shared with the
	// diagnostic trace, so the decoded bytes reach the estimator untouched.
	command.Stdout, command.Stderr = &output, &stderr
	if err := runDiagnosticFFmpeg(probeCtx, command, "Full Demo transition motion"); err != nil {
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
