package recording

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/rechedev9/cliphub/internal/recapplan"
)

// ValidateFullDemoRoundFrames checks the frames needed by a requested window
// against the immutable capture's original start. A narrower request can reuse
// a broader clip even when an unused part of its original tail is short.
// The caller supplies an end already bounded by the original POV attestation.
func (r RecordingResult) ValidateFullDemoRoundFrames(id string, startTick, endTick int) error {
	fail := func(detail string) error {
		return fmt.Errorf("full_demo_capture_incomplete: %s: %s", id, detail)
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
	if clip.FrameCount < offset+frames {
		return fail(fmt.Sprintf("clip has %d frames; approved window needs %d (offset=%d, length=%d); recapture this round", clip.FrameCount, offset+frames, offset, frames))
	}
	return nil
}

// ValidateFullDemoFrames expects an effective document whose safe tail trims
// have already been resolved from the capture evidence. It never changes that
// document, padding frames or shortening an approved window to fit a bad clip.
func (r RecordingResult) ValidateFullDemoFrames(effective recapplan.Document) error {
	var failures []error
	for _, round := range effective.Rounds {
		if err := r.ValidateFullDemoRoundFrames(round.ID, round.RequestedStartTick, round.EffectiveEndTick); err != nil {
			failures = append(failures, err)
		}
	}
	return errors.Join(failures...)
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
