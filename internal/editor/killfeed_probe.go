package editor

import (
	"bufio"
	"bytes"
	"fmt"
	"image"
	"image/png"
	"io"
	"os/exec"
	"strings"
	"sync"
)

const (
	// killfeedSampleDelaySeconds is the first probe offset after the
	// tick-derived kill time. CS2 usually has the death notice drawn by then.
	killfeedSampleDelaySeconds = 0.35
	// killfeedSampleMaxOffsetSeconds is the latest in-source offset after the
	// tick-derived kill time that refine still probes. Real HLAE captures
	// often paint the local-player highlight ~0.6–0.8s later than the demo
	// tick maps to (tickrate math vs when mirv/CS2 present the notice), so a
	// single 0.35s sample drops every overlay on those reels.
	killfeedSampleMaxOffsetSeconds = 1.80
	// killfeedSampleStepSeconds is the stride of the late-notice scan window.
	killfeedSampleStepSeconds = 0.20
	// killfeedHighlightMargin pads the detected highlight box. The two-pass
	// detection already includes the anti-aliased border ring, so anything
	// wider drags mismatched source background into the overlay.
	killfeedHighlightMargin = 1
	// killfeedMinHighlightPixels filters scene noise: a real highlight border
	// contributes a few hundred saturated red pixels.
	killfeedMinHighlightPixels = 60
	// killfeedBorderSearchRadius bounds the second, looser pass that picks up
	// the border's anti-aliased edge around the saturated-red box without
	// letting distant dim-red scene pixels stretch the crop.
	killfeedBorderSearchRadius = 6
	// killfeedOverlayScale keeps the on-screen overlay size consistent with
	// the historical 360px-crop / 430px-overlay default.
	killfeedOverlayScale = 430.0 / 360.0
	// killfeedMaxHighlightHeightDiv caps a notice ring's height at frame
	// height / 12: a kill notice bar is ~37px tall at 1080p, so /12 (90px)
	// leaves headroom for higher resolutions while rejecting tall scene
	// geometry like a red wall or container.
	killfeedMaxHighlightHeightDiv = 12
	// killfeedMinHighlightAspect is the minimum width/height ratio of a notice
	// ring: kill notices are wide and short, so a qualifying component must be
	// at least twice as wide as it is tall.
	killfeedMinHighlightAspect = 2
	// killfeedMaxHighlightFill caps the fraction of a component's bounding box
	// its pixels may fill: a 2px border ring fills ~13% of its bbox, while
	// solid scene red fills ~100%, so anything over half is rejected as a blob.
	killfeedMaxHighlightFill = 0.5
	// killfeedSampleMaxCount bounds the total number of frames probed per
	// killfeed effect (forward window plus any backward fallback samples),
	// matching the historical up-to-8-samples budget.
	killfeedSampleMaxCount = 8
)

// killfeedProbeResult is the outcome of measuring one effect: whether to keep
// it (mutated, if kept) and at most one warning to surface.
type killfeedProbeResult struct {
	keep    bool
	effect  Effect
	warning string
}

// refineKillfeedEffects replaces the static killfeed crop defaults with a
// per-kill measurement: it samples source frames in a short window after each
// kill (tick-derived time can lag the painted death notice), finds the red
// highlight box CS2 draws around the recording player's own kill notice, and
// crops exactly that entry. On success it retimes AtSeconds so the FFmpeg
// freeze uses the same winning frame. Probe errors on one sample continue the
// window; exhausting the window with no highlight drops generated
// ("edit-request") overlays and keeps script-authored crops with a warning.
//
// Each killfeed effect is measured in its own goroutine, bounded by the same
// sem/WaitGroup pattern shortPackRenderer.render uses: a kill's own sample
// window is walked in series (the first hit wins, so within-effect
// parallelism buys nothing), but a short with several kills probes them
// concurrently. Effects are read into local copies before any goroutine
// starts, and results land in a slice indexed by the effect's original
// position, so the final filter-and-merge stays single-threaded and the
// returned warnings keep the original effect order regardless of which
// goroutine finished first.
func refineKillfeedEffects(short *ShortEdit, probe func(input string, atSeconds float64) (image.Image, error)) []string {
	if probe == nil {
		return nil
	}
	results := make([]killfeedProbeResult, len(short.Effects))
	jobs := normalizeRenderJobs(0)
	sem := make(chan struct{}, jobs)
	var wg sync.WaitGroup
	for i, effect := range short.Effects {
		if effect.Type != EffectKillfeed {
			results[i] = killfeedProbeResult{keep: true, effect: effect}
			continue
		}
		wg.Add(1)
		sem <- struct{}{}
		go func(i int, effect Effect) {
			defer wg.Done()
			defer func() { <-sem }()
			results[i] = probeKillfeedEffect(short, effect, probe)
		}(i, effect)
	}
	wg.Wait()

	var warnings []string
	kept := short.Effects[:0]
	for _, result := range results {
		if result.warning != "" {
			warnings = append(warnings, result.warning)
		}
		if result.keep {
			kept = append(kept, result.effect)
		}
	}
	short.Effects = kept
	return warnings
}

// probeKillfeedEffect walks the sample window for a single killfeed effect
// and reports whether to keep it (with the measured crop, if any) and a
// warning to surface. It only reads short (Input/Parts/DurationSeconds),
// never mutates it, so it is safe to call concurrently across effects.
func probeKillfeedEffect(short *ShortEdit, effect Effect, probe func(input string, atSeconds float64) (image.Image, error)) killfeedProbeResult {
	input, samples := killfeedSampleTimes(short, effect)
	if input == "" || len(samples) == 0 {
		return killfeedProbeResult{
			keep:    true,
			effect:  effect,
			warning: fmt.Sprintf("killfeed crop at %.2fs: no source input to probe; keeping default crop", effect.StartSeconds),
		}
	}

	var (
		lastErr     error
		sawEmpty    bool
		found       bool
		foundSample float64
		rect        image.Rectangle
	)
	for _, sampleAt := range samples {
		frame, err := probe(input, sampleAt)
		if err != nil {
			lastErr = err
			continue
		}
		r, ok := detectKillfeedHighlight(frame)
		if !ok {
			sawEmpty = true
			continue
		}
		rect = r
		foundSample = sampleAt
		found = true
		break
	}
	if !found {
		switch {
		case lastErr != nil && !sawEmpty:
			// Every sample failed to extract a frame: keep defaults.
			return killfeedProbeResult{
				keep:    true,
				effect:  effect,
				warning: fmt.Sprintf("killfeed crop at %.2fs: %v; keeping default crop", effect.StartSeconds, lastErr),
			}
		case effect.Source == "edit-request":
			return killfeedProbeResult{
				keep:    false,
				warning: fmt.Sprintf("killfeed crop at %.2fs: no highlighted kill notice detected in %s; dropping overlay", effect.StartSeconds, input),
			}
		default:
			return killfeedProbeResult{
				keep:    true,
				effect:  effect,
				warning: fmt.Sprintf("killfeed crop at %.2fs: no highlighted kill notice detected in %s; keeping default crop", effect.StartSeconds, input),
			}
		}
	}

	effect.CropX = rect.Min.X
	effect.CropY = rect.Min.Y
	effect.CropWidth = rect.Dx()
	effect.CropHeight = rect.Dy()
	effect.Width = int(float64(rect.Dx())*killfeedOverlayScale + 0.5)
	// Retime so killfeedSamplePart (FFmpeg freeze) resolves to foundSample:
	// sample = AtSeconds - timelineStart + delay  =>  At = sample + timelineStart - delay.
	effect.AtSeconds = killfeedAtSecondsForSample(short, effect, foundSample)
	return killfeedProbeResult{keep: true, effect: effect}
}

// killfeedSampleTimes returns the source path and the in-source timestamps to
// probe for a killfeed effect, in preference order. The first time is the
// legacy kill+0.35s sample; later times walk a fixed forward window so
// late-painted death notices still measure. Times are clamped to the owning
// part (or short) duration.
//
// A kill with little post-roll (typically the last kill in a clip) has that
// entire forward window clamp to the same last-available frame, collapsing
// the window to a single sample; if the notice is not painted yet on that one
// frame, refineKillfeedEffects has nothing else to try and drops the overlay.
// When that happens, this also probes backward from the last available frame
// (still no earlier than the kill itself) so the sample set spans several
// distinct frames instead of repeating one. Backward samples are appended
// after the forward ones, so the existing late-offset preference order (and
// therefore the result for every clip where the forward window already fits)
// is unchanged.
func killfeedSampleTimes(short *ShortEdit, effect Effect) (string, []float64) {
	partIndex, first := killfeedSamplePart(short, effect)
	input := short.Input
	maxDuration := short.DurationSeconds
	killInSource := first - killfeedSampleDelaySeconds
	if partIndex >= 0 {
		part := short.Parts[partIndex]
		input = part.Input
		if part.DurationSeconds > 0 {
			maxDuration = part.DurationSeconds
		}
		at := effect.AtSeconds
		if at == 0 {
			at = effect.StartSeconds
		}
		killInSource = at - part.TimelineStartSeconds
	}
	if input == "" {
		return "", nil
	}
	if killInSource < 0 {
		killInSource = 0
	}

	var times []float64
	appendUnique := func(at float64) {
		if maxDuration > 0 && at > maxDuration {
			at = maxDuration
		}
		if at < 0 {
			at = 0
		}
		if len(times) > 0 && at <= times[len(times)-1]+1e-9 {
			return
		}
		times = append(times, at)
	}
	for offset := killfeedSampleDelaySeconds; offset <= killfeedSampleMaxOffsetSeconds+1e-9; offset += killfeedSampleStepSeconds {
		appendUnique(killInSource + offset)
	}
	if len(times) == 0 {
		appendUnique(first)
	}
	if maxDuration > 0 && len(times) <= 1 {
		times = appendBackwardKillfeedSamples(times, killInSource)
	}
	return input, times
}

// appendBackwardKillfeedSamples extends a forward sample window that
// collapsed to a single frame (short post-roll clamped every forward offset
// to the clip's last frame) by walking backward from that frame in
// killfeedSampleStepSeconds strides, never going earlier than killInSource
// (the highlight cannot appear before the kill itself) or below zero, up to
// killfeedSampleMaxCount total samples.
func appendBackwardKillfeedSamples(times []float64, killInSource float64) []float64 {
	last := times[len(times)-1]
	floor := killInSource
	if floor < 0 {
		floor = 0
	}
	for offset := killfeedSampleStepSeconds; len(times) < killfeedSampleMaxCount; offset += killfeedSampleStepSeconds {
		at := last - offset
		if at < floor-1e-9 {
			break
		}
		if at < 0 {
			at = 0
		}
		times = append(times, at)
	}
	return times
}

// killfeedAtSecondsForSample chooses AtSeconds so killfeedSamplePart returns sample.
func killfeedAtSecondsForSample(short *ShortEdit, effect Effect, sample float64) float64 {
	partIndex, _ := killfeedSamplePart(short, effect)
	if partIndex < 0 {
		return sample - killfeedSampleDelaySeconds
	}
	return sample + short.Parts[partIndex].TimelineStartSeconds - killfeedSampleDelaySeconds
}

// killfeedSamplePart resolves the part index (-1 for single-clip shorts) and
// the in-part timestamp of the frame that represents a killfeed effect. The
// probe measures this exact frame and the render freezes it, so both must
// resolve identically. After refineKillfeedEffects succeeds, AtSeconds is
// retimed so this helper yields the winning late-notice sample.
func killfeedSamplePart(short *ShortEdit, effect Effect) (int, float64) {
	at := effect.AtSeconds
	if at == 0 {
		at = effect.StartSeconds
	}
	if len(short.Parts) == 0 {
		sample := at + killfeedSampleDelaySeconds
		if sample < 0 {
			sample = 0
		}
		return -1, sample
	}
	partIndex := compilationPartIndexAt(short.Parts, effect)
	part := short.Parts[partIndex]
	sample := at - part.TimelineStartSeconds + killfeedSampleDelaySeconds
	if part.DurationSeconds > 0 && sample > part.DurationSeconds {
		sample = part.DurationSeconds
	}
	if sample < 0 {
		sample = 0
	}
	return partIndex, sample
}

// detectKillfeedHighlight finds the red border CS2 draws around the local
// player's kill notices in the top-right region. The first pass is
// shape-aware: it groups strict saturated-red pixels into connected
// components and keeps only those shaped like a notice highlight ring - wide,
// short, and mostly hollow. This rejects red scene geometry (a wall or
// container) that passes the same color threshold but forms a tall, solid
// blob, which previously got unioned into the crop. A second, looser pass
// within a few pixels of the qualifying rings picks up the border's
// anti-aliased edge. Returns the padded bounding box, or ok=false when no
// component looks like a notice.
func detectKillfeedHighlight(frame image.Image) (image.Rectangle, bool) {
	bounds := frame.Bounds()
	scanRegion := image.Rect(
		bounds.Min.X+bounds.Dx()*3/5,
		bounds.Min.Y,
		bounds.Max.X,
		bounds.Min.Y+bounds.Dy()*3/10,
	)
	maxHeight := bounds.Dy() / killfeedMaxHighlightHeightDiv
	core := image.Rectangle{}
	qualified := 0
	for _, comp := range redComponents(frame, scanRegion, 150, 55) {
		if isNoticeRing(comp, maxHeight) {
			if qualified == 0 {
				core = comp.bounds
			} else {
				core = core.Union(comp.bounds)
			}
			qualified++
		}
	}
	if qualified == 0 {
		return image.Rectangle{}, false
	}
	edgeRegion := core.Inset(-killfeedBorderSearchRadius).Intersect(bounds)
	edge, edgeCount := redPixelBounds(frame, edgeRegion, 120, 70)
	if edgeCount > 0 {
		core = core.Union(edge)
	}
	return core.Inset(-killfeedHighlightMargin).Intersect(bounds), true
}

// redComponent is a connected group of strict-red pixels: its bounding box and
// pixel count, used to tell a thin notice ring from a solid scene blob.
type redComponent struct {
	bounds image.Rectangle
	count  int
}

// isNoticeRing reports whether a red component is shaped like a CS2 kill-notice
// highlight border rather than solid scene geometry: enough pixels to clear
// noise, short (a notice bar), wide (notices are wide and short), and mostly
// hollow (a 2px ring barely fills its bounding box, a solid red wall fills it
// completely).
func isNoticeRing(comp redComponent, maxHeight int) bool {
	if comp.count < killfeedMinHighlightPixels {
		return false
	}
	w, h := comp.bounds.Dx(), comp.bounds.Dy()
	if h == 0 || w == 0 || h > maxHeight {
		return false
	}
	if w < killfeedMinHighlightAspect*h {
		return false
	}
	fill := float64(comp.count) / float64(w*h)
	return fill <= killfeedMaxHighlightFill
}

// redComponents groups the strict-red pixels of region (same threshold as
// redPixelBounds) into 8-connected components via BFS over a bool grid indexed
// relative to region. Region is small (~768x324 at 1080p), so a dense grid is
// cheap and simple.
func redComponents(frame image.Image, region image.Rectangle, minRed, maxGreenBlue uint32) []redComponent {
	w, h := region.Dx(), region.Dy()
	if w <= 0 || h <= 0 {
		return nil
	}
	red := make([]bool, w*h)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			r, g, b, _ := frame.At(region.Min.X+x, region.Min.Y+y).RGBA()
			if r>>8 > minRed && g>>8 < maxGreenBlue && b>>8 < maxGreenBlue {
				red[y*w+x] = true
			}
		}
	}
	visited := make([]bool, w*h)
	var comps []redComponent
	queue := make([]int, 0, 64)
	for start := 0; start < w*h; start++ {
		if !red[start] || visited[start] {
			continue
		}
		visited[start] = true
		queue = queue[:0]
		queue = append(queue, start)
		sx, sy := start%w, start/w
		minX, minY, maxX, maxY := sx, sy, sx, sy
		count := 0
		for len(queue) > 0 {
			idx := queue[len(queue)-1]
			queue = queue[:len(queue)-1]
			count++
			cx, cy := idx%w, idx/w
			if cx < minX {
				minX = cx
			}
			if cx > maxX {
				maxX = cx
			}
			if cy < minY {
				minY = cy
			}
			if cy > maxY {
				maxY = cy
			}
			for dy := -1; dy <= 1; dy++ {
				for dx := -1; dx <= 1; dx++ {
					if dx == 0 && dy == 0 {
						continue
					}
					nx, ny := cx+dx, cy+dy
					if nx < 0 || nx >= w || ny < 0 || ny >= h {
						continue
					}
					nidx := ny*w + nx
					if red[nidx] && !visited[nidx] {
						visited[nidx] = true
						queue = append(queue, nidx)
					}
				}
			}
		}
		comps = append(comps, redComponent{
			bounds: image.Rect(
				region.Min.X+minX, region.Min.Y+minY,
				region.Min.X+maxX+1, region.Min.Y+maxY+1,
			),
			count: count,
		})
	}
	return comps
}

// redPixelBounds returns the bounding box and count of pixels within region
// whose 8-bit color exceeds minRed and stays below maxGreenBlue on the other
// channels.
func redPixelBounds(frame image.Image, region image.Rectangle, minRed, maxGreenBlue uint32) (image.Rectangle, int) {
	found := image.Rectangle{}
	count := 0
	for y := region.Min.Y; y < region.Max.Y; y++ {
		for x := region.Min.X; x < region.Max.X; x++ {
			r, g, b, _ := frame.At(x, y).RGBA()
			if r>>8 > minRed && g>>8 < maxGreenBlue && b>>8 < maxGreenBlue {
				pixel := image.Rect(x, y, x+1, y+1)
				if count == 0 {
					found = pixel
				} else {
					found = found.Union(pixel)
				}
				count++
			}
		}
	}
	return found, count
}

// ffmpegFrameProbe extracts a single source frame as PNG via FFmpeg for
// killfeed crop measurement. Up to killfeedSampleMaxCount frames are probed
// per killfeed effect and a compilation carries one effect per kill, so this
// runs dozens of times per render; the PNG is streamed over the process stdout
// pipe instead of round-tripping a full-frame temp file. It is the same PNG
// encoder feeding the same PNG decoder, so the measured pixels are unchanged.
func ffmpegFrameProbe(ffmpegPath string) func(input string, atSeconds float64) (image.Image, error) {
	return func(input string, atSeconds float64) (image.Image, error) {
		frame, err := decodeKillfeedFrameCommand(killfeedFrameProbeCommand(ffmpegPath, input, atSeconds))
		if err != nil {
			return nil, fmt.Errorf("extract frame from %s at %.3fs: %w", input, atSeconds, err)
		}
		return frame, nil
	}
}

// killfeedFrameProbeCommand builds the single-frame PNG probe argv. The frame
// goes to stdout through the image2pipe muxer rather than to a file, so no
// temp path appears in the command.
func killfeedFrameProbeCommand(ffmpegPath, input string, atSeconds float64) []string {
	if ffmpegPath == "" {
		ffmpegPath = "ffmpeg"
	}
	return []string{
		ffmpegPath,
		"-y", "-v", "error",
		"-ss", fmt.Sprintf("%.3f", atSeconds),
		"-i", input,
		"-frames:v", "1",
		"-f", "image2pipe",
		"-c:v", "png",
		"pipe:1",
	}
}

// decodeKillfeedFrameCommand runs a frame-probe command and decodes the PNG
// straight off its stdout pipe. Stdout is read to completion (png.Decode stops
// at IEND) before Wait so the child never blocks on a full pipe, and stderr is
// buffered so a probe failure still reports what FFmpeg said.
func decodeKillfeedFrameCommand(command []string) (image.Image, error) {
	if len(command) == 0 || command[0] == "" {
		return nil, fmt.Errorf("killfeed frame probe command is empty")
	}
	// #nosec G204 -- ffmpegPath and input are local pipeline configuration, not untrusted input.
	cmd := exec.Command(command[0], command[1:]...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("open killfeed frame pipe: %w", err)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start killfeed frame probe: %w", err)
	}
	frame, decodeErr := png.Decode(bufio.NewReader(stdout))
	_, _ = io.Copy(io.Discard, stdout)
	if err := cmd.Wait(); err != nil {
		if msg := strings.TrimSpace(stderr.String()); msg != "" {
			return nil, fmt.Errorf("run killfeed frame probe: %w: %s", err, msg)
		}
		return nil, fmt.Errorf("run killfeed frame probe: %w", err)
	}
	if decodeErr != nil {
		return nil, fmt.Errorf("decode killfeed frame: %w", decodeErr)
	}
	return frame, nil
}
