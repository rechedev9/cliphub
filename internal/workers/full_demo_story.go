package workers

import (
	"fmt"

	"github.com/rechedev9/cliphub/internal/demooverlay"
	"github.com/rechedev9/cliphub/internal/job"
	"github.com/rechedev9/cliphub/internal/recapplan"
	"github.com/rechedev9/cliphub/internal/recording"
	"github.com/rechedev9/cliphub/internal/renderplan"
)

const (
	fullDemoStoryOverlayPolicy = "story-overlays-v1"
	fullDemoHookPolicy         = "hook-v1"
	fullDemoSecondCameraPolicy = "second-camera-v1"
)

func fullDemoHalftime(j job.Job, edit renderplan.EditRequest, doc demooverlay.Document) *demooverlay.Halftime {
	if edit.FullDemo == nil || j.KillPlan == nil || !edit.FullDemo.Document.Options.Overlays.Halftime {
		return nil
	}
	return demooverlay.BuildHalftime(doc, j.KillPlan.Rounds, edit.FullDemo.Document.Options.Overlays.TeamNames)
}

type fullDemoCaptureInput struct {
	SegmentID, ContentSHA256 string
	StartTick, EndTick       int
}

func fullDemoCameraInputs(result recording.RecordingResult, effective recapplan.Document) ([]fullDemoCaptureInput, error) {
	if effective.SecondCamera == nil {
		return nil, nil
	}
	var inputs []fullDemoCaptureInput
	for _, w := range effective.SecondCamera.Windows {
		clip, ok := recording.ResolveFullDemoCameraClip(result, w.ID)
		if !ok || len(clip.Artifact.ContentSHA256) != 64 {
			return nil, fmt.Errorf("second camera %s lacks a clip with a content SHA-256", w.ID)
		}
		inputs = append(inputs, fullDemoCaptureInput{w.ID, clip.Artifact.ContentSHA256, w.StartTick, result.FullDemoEvidence.CertifiedEnds[w.ID]})
	}
	return inputs, nil
}
