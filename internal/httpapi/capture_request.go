package httpapi

import (
	"net/http"
	"slices"

	"github.com/rechedev9/cliphub/internal/editor"
	"github.com/rechedev9/cliphub/internal/job"
	"github.com/rechedev9/cliphub/internal/recapplan"
	"github.com/rechedev9/cliphub/internal/renderplan"
)

func applyCaptureOverrides(hudMode string, edit renderplan.EditRequest) (string, bool) {
	if edit.CaptureHUD != "" {
		hudMode = edit.CaptureHUD
	}
	if edit.NativeHUD {
		hudMode = "gameplay"
	}
	return hudMode, edit.MatchRecap
}

func (h *Handlers) requireRecapPlan(w http.ResponseWriter, j job.Job) bool {
	if rejectMultiPOVFullDemo(w, j) {
		return false
	}
	plan, found, err := recapplan.Load(h.storage, j.ID)
	if err != nil {
		internalError(w, "load recap plan", err)
		return false
	}
	if !found {
		writeError(w, http.StatusConflict, "recap plan not ready")
		return false
	}
	if len(plan.Segments) == 0 {
		writeError(w, http.StatusConflict, "recap plan has no rounds")
		return false
	}
	return true
}

// validatePlaySelection checks the plays a capture records and its pre-roll, which recuts the plays of one player only.
func (h *Handlers) validatePlaySelection(w http.ResponseWriter, j job.Job, ids []string, edit renderplan.EditRequest) bool {
	if err := renderplan.ValidatePreRoll(edit.PreRollSeconds); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return false
	}
	if edit.PreRollSeconds != 0 && j.IsMultiPOV() {
		writeCodedError(w, http.StatusBadRequest, multiPOVSelectionCode, "the pre-roll is for a video of one player")
		return false
	}
	return h.validateSegmentSelection(w, j, ids, edit.DeathReel)
}

func sameCapture(a, b renderplan.GenerateIntent) bool {
	// Admission deduplicates the complete approved intent. A music-only change
	// may reuse capture later, but it must not silently adopt an older render.
	if a.Edit.FullDemo != nil || b.Edit.FullDemo != nil {
		return a.Edit.FullDemo != nil && b.Edit.FullDemo != nil && a.Edit.FullDemo.Document.PlanHash == b.Edit.FullDemo.Document.PlanHash && a.Variant == b.Variant
	}
	ca, okA := captureRequestFor(a)
	cb, okB := captureRequestFor(b)
	return okA && okB && ca.hudMode == cb.hudMode && ca.useRecapPlan == cb.useRecapPlan && ca.useDeathPlan == cb.useDeathPlan &&
		ca.portraitSafeKillfeed == cb.portraitSafeKillfeed && ca.victimReplay == cb.victimReplay && ca.preRollSeconds == cb.preRollSeconds &&
		slices.Equal(ca.segmentIDs, cb.segmentIDs)
}

type captureRequest struct {
	hudMode              string
	segmentIDs           []string
	portraitSafeKillfeed bool
	useRecapPlan         bool
	useDeathPlan         bool
	victimReplay         bool
	preRollSeconds       int
}

// captureRequestFor derives the capture-time choices of an intent the same way
// StartGenerate does when it builds the record task.
func captureRequestFor(intent renderplan.GenerateIntent) (captureRequest, bool) {
	preset, ok := editor.PresetByName(intent.Variant)
	if !ok {
		return captureRequest{}, false
	}
	hudMode, useRecapPlan := applyCaptureOverrides(preset.HUDMode, intent.Edit)
	segmentIDs := intent.SegmentIDs
	if useRecapPlan || len(segmentIDs) == 0 {
		segmentIDs = nil
	}
	return captureRequest{
		hudMode:              hudMode,
		segmentIDs:           segmentIDs,
		portraitSafeKillfeed: capturePortraitSafeKillfeed(preset.KillfeedSource, intent.Edit),
		useRecapPlan:         useRecapPlan,
		useDeathPlan:         !useRecapPlan && intent.Edit.DeathReel,
		victimReplay:         intent.Edit.VictimReplay && !useRecapPlan,
		preRollSeconds:       intent.Edit.PreRollSeconds,
	}, true
}
