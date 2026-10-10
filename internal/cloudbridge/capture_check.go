package cloudbridge

import (
	"fmt"
	"log"
	"strconv"
	"strings"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/recording"
)

// blackCaptureBitsPerPixel is the floor under which a whole capture is taken
// for the black capture of 2026-09-23 (docs/incidents.md): HLAE drew only the
// HUD, every exit code was fine, and the 1080p60 video averaged 2.4 Mb/s,
// which is 0.019 bits per pixel, where a normal take has about 0.3 (40 Mb/s).
// The floor is twice the incident. The lowest of 70 real takes measured on
// healthy captures (Shorts and Full Demo, NVENC) is 0.149, and the same take
// encoded with the software encoder is 0.136: more than three times the
// floor either way, so quiet footage stays well clear of it.
const blackCaptureBitsPerPixel = 0.04

const (
	captureRoleRaw     = "raw"
	captureRoleSegment = "segment"
	captureTypeVideo   = "video"
)

// captureMeasure adds up the video the recorder probed after a capture.
type captureMeasure struct {
	role    string
	takes   int
	bits    float64
	pixels  float64
	seconds float64
}

func (m captureMeasure) bitsPerPixel() float64 {
	return m.bits / m.pixels
}

func (m captureMeasure) looksBlack() bool {
	return m.bitsPerPixel() < blackCaptureBitsPerPixel
}

// detail is what the operator reads as the reason of the automatic pause, so
// the first 300 characters must say what happened and what to do.
func (m captureMeasure) detail() string {
	rate := ""
	if m.seconds > 0 {
		rate = fmt.Sprintf("%.1f Mb/s, ", m.bits/m.seconds/1e6)
	}
	return fmt.Sprintf(
		"black capture suspected: %d %s takes average %s%.3f bits per pixel, under the floor of %.2f (a normal capture has about 0.3, or 40 Mb/s at 1080p60). "+
			"HLAE is probably not drawing the game on this CS2 build. Capture one Short by hand and look at the video before resuming.",
		m.takes, m.role, rate, m.bitsPerPixel(), blackCaptureBitsPerPixel,
	)
}

// measureCapture reads the takes HLAE wrote, or the segment clips cut from
// them when no take was probed. ok is false when nothing can be measured.
func measureCapture(artifacts []recording.RecordingArtifact) (captureMeasure, bool) {
	for _, role := range []string{captureRoleRaw, captureRoleSegment} {
		measure := captureMeasure{role: role}
		for _, artifact := range artifacts {
			if artifact.Type != captureTypeVideo || artifact.Role != role {
				continue
			}
			frames := videoFrames(artifact)
			if artifact.SizeBytes <= 0 || artifact.Width <= 0 || artifact.Height <= 0 || frames <= 0 {
				continue
			}
			measure.takes++
			measure.bits += float64(artifact.SizeBytes) * 8
			measure.pixels += float64(artifact.Width) * float64(artifact.Height) * frames
			measure.seconds += artifact.DurationSeconds
		}
		if measure.takes > 0 {
			return measure, true
		}
	}
	return captureMeasure{}, false
}

// videoFrames is the probed frame count, or duration times frame rate.
func videoFrames(artifact recording.RecordingArtifact) float64 {
	if artifact.FrameCount > 0 {
		return float64(artifact.FrameCount)
	}
	numerator, denominator, found := strings.Cut(artifact.FrameRate, "/")
	rate, err := strconv.ParseFloat(numerator, 64)
	if err != nil || rate <= 0 {
		return 0
	}
	if found {
		divisor, err := strconv.ParseFloat(denominator, 64)
		if err != nil || divisor <= 0 {
			return 0
		}
		rate /= divisor
	}
	return artifact.DurationSeconds * rate
}

// checkCapture keeps a capture that does not show the game from being
// delivered. Exit codes cannot tell: the render of a black capture succeeds.
// It judges the capture as a whole, so one quiet window never trips it, and
// it reports the machine fault that pauses the worker, because every job
// after this one would come out the same.
func (r *runner) checkCapture(localID string) *jobFailure {
	jobID, err := uuid.Parse(localID)
	if err != nil {
		return failureOf(codeInternal, "invalid local job id "+localID)
	}
	var result recording.RecordingResult
	if err := r.readJSON(recording.ResultArtifactKey(jobID), &result); err != nil {
		log.Printf("cloudbridge: capture of local job %s was not checked for black video: %v", localID, err)
		return nil
	}
	measure, ok := measureCapture(result.Artifacts)
	if !ok {
		log.Printf("cloudbridge: capture of local job %s was not checked for black video: the recorder probed no video", localID)
		return nil
	}
	if !measure.looksBlack() {
		return nil
	}
	return failureOf(codeCaptureIncompatible, measure.detail())
}
