package recording

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/rechedev9/cliphub/internal/recapplan"
)

// FullDemoTailPadToleranceFrames bounds how many trailing frames of an approved
// window a segment clip may lack and still be rendered. The window length is
// TickFrames, which rounds a tick span half-up to the nearest 60 fps frame,
// while an HLAE take can stop on the frame before that boundary: a whole Full
// Demo capture was discarded over one missing 16.7 ms frame. Two frames (33 ms)
// stays below audio/video sync perceptibility. The renderer clones the clip's
// last frame exactly FullDemoTailPad.PaddedFrames times, so every item keeps
// its canonical frame count, the audio bus stays sample-exact and no offset
// can accumulate into the next round. Larger shortfalls still fail.
const FullDemoTailPadToleranceFrames int64 = 2

// FullDemoTailPad records a bounded tail shortfall of one approved round
// window. ClipFrames is the clip's frame count and WindowFrames the capture
// relative frame where the approved window ends; PaddedFrames is their
// difference, the number of cloned frames the renderer appends.
type FullDemoTailPad struct {
	SegmentID    string `json:"segment_id"`
	ClipFrames   int64  `json:"clip_frames"`
	WindowFrames int64  `json:"window_frames"`
	PaddedFrames int64  `json:"padded_frames"`
}

// Validate checks that a recorded pad is a bounded, self-consistent tail
// shortfall. Every consumer that turns it into an FFmpeg filter value relies on
// this bound.
func (p FullDemoTailPad) Validate() error {
	if p.SegmentID == "" || p.ClipFrames <= 0 || p.PaddedFrames < 1 || p.PaddedFrames > FullDemoTailPadToleranceFrames || p.WindowFrames-p.ClipFrames != p.PaddedFrames {
		return fmt.Errorf("full_demo_capture_incomplete: %s: invalid tail pad (clip=%d, window=%d, padded=%d)", p.SegmentID, p.ClipFrames, p.WindowFrames, p.PaddedFrames)
	}
	return nil
}

// ValidateFullDemoRoundFrames checks the frames needed by a requested window
// against the immutable capture's original start. A narrower request can reuse
// a broader clip even when an unused part of its original tail is short.
// The caller supplies an end already bounded by the original POV attestation.
func (r RecordingResult) ValidateFullDemoRoundFrames(id string, startTick, endTick int) error {
	_, err := r.FullDemoRoundTailPad(id, startTick, endTick)
	return err
}

// FullDemoRoundTailPad validates a requested window like
// ValidateFullDemoRoundFrames and also returns how many trailing frames the
// clip lacks (zero when it covers the whole window). A shortfall is accepted
// only at the end of the window, only up to FullDemoTailPadToleranceFrames and
// only while the window keeps at least one captured frame to clone.
func (r RecordingResult) FullDemoRoundTailPad(id string, startTick, endTick int) (FullDemoTailPad, error) {
	fail := func(detail string) (FullDemoTailPad, error) {
		return FullDemoTailPad{}, fmt.Errorf("full_demo_capture_incomplete: %s: %s", id, detail)
	}
	var capture *RecordingSegment
	for i := range r.Plan.Segments {
		if r.Plan.Segments[i].ID == id {
			capture = &r.Plan.Segments[i]
			break
		}
	}
	if capture == nil || startTick < capture.TickStart || endTick <= startTick || endTick > capture.TickEnd {
		return fail("requested frame window is outside the recorded interval")
	}
	offset, err := recapplan.TickFrames(startTick-capture.TickStart, r.Plan.Tickrate)
	if err != nil {
		return fail(err.Error())
	}
	frames, err := recapplan.TickFrames(endTick-startTick, r.Plan.Tickrate)
	if err != nil || frames < 1 {
		return fail("invalid requested frame duration")
	}
	var clip *RecordingArtifact
	for i := range r.Artifacts {
		a := &r.Artifacts[i]
		if a.SegmentID == id && isUsableSegmentClip(*a) {
			if clip != nil {
				return fail("duplicate segment clips")
			}
			clip = a
		}
	}
	if clip == nil {
		return fail("missing segment clip")
	}
	if clip.FrameCount <= 0 || !fullDemoFrameRate(clip.FrameRate) {
		return fail(fmt.Sprintf("missing valid 60 fps frame metadata (frames=%d, fps=%q)", clip.FrameCount, clip.FrameRate))
	}
	needed := offset + frames
	shortfall := needed - clip.FrameCount
	if shortfall <= 0 {
		return FullDemoTailPad{SegmentID: id}, nil
	}
	if shortfall > FullDemoTailPadToleranceFrames || shortfall >= frames {
		return fail(fmt.Sprintf("clip has %d frames; approved window needs %d (offset=%d, length=%d); recapture this round", clip.FrameCount, needed, offset, frames))
	}
	pad := FullDemoTailPad{SegmentID: id, ClipFrames: clip.FrameCount, WindowFrames: needed, PaddedFrames: shortfall}
	if err := pad.Validate(); err != nil {
		return FullDemoTailPad{}, err
	}
	return pad, nil
}

// ValidateFullDemoFrames expects an effective document whose safe tail trims
// have already been resolved from the capture evidence. It never changes that
// document or shortens an approved window to fit a bad clip; a bounded tail
// shortfall is reported by FullDemoTailPads and padded by the renderer.
func (r RecordingResult) ValidateFullDemoFrames(effective recapplan.Document) error {
	_, err := r.FullDemoTailPads(effective)
	return err
}

// FullDemoTailPads validates every effective round window and returns the
// rounds whose clip lacks a bounded number of trailing frames, in round order.
// The renderer pads exactly these frames and nothing else.
func (r RecordingResult) FullDemoTailPads(effective recapplan.Document) ([]FullDemoTailPad, error) {
	var failures []error
	var pads []FullDemoTailPad
	for _, round := range effective.Rounds {
		pad, err := r.FullDemoRoundTailPad(round.ID, round.RequestedStartTick, round.EffectiveEndTick)
		if err != nil {
			failures = append(failures, err)
			continue
		}
		if pad.PaddedFrames > 0 {
			pads = append(pads, pad)
		}
	}
	if err := errors.Join(failures...); err != nil {
		return nil, err
	}
	return pads, nil
}

func validateFullDemoCaptureFrames(plan RecordingPlan, artifacts []RecordingArtifact) error {
	result := RecordingResult{Plan: plan, Artifacts: artifacts}
	var failures []error
	for _, segment := range plan.Segments {
		if err := result.ValidateFullDemoRoundFrames(segment.ID, segment.TickStart, segment.TickEnd); err != nil {
			failures = append(failures, err)
		}
	}
	return errors.Join(failures...)
}

func fullDemoFrameRate(raw string) bool {
	parts := strings.Split(raw, "/")
	if len(parts) > 2 {
		return false
	}
	rate, err := strconv.ParseFloat(parts[0], 64)
	if err != nil {
		return false
	}
	if len(parts) == 2 {
		denominator, err := strconv.ParseFloat(parts[1], 64)
		if err != nil || denominator <= 0 {
			return false
		}
		rate /= denominator
	}
	return !math.IsNaN(rate) && !math.IsInf(rate, 0) && math.Abs(rate-recapplan.OutputFPS) < .01
}
