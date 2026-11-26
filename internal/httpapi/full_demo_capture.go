package httpapi

import (
	"net/http"
	"slices"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/recapplan"
	"github.com/rechedev9/cliphub/internal/recording"
)

// fullDemoCapture tells the producer whether a plan only renders again or
// must open CS2 first, and for which source rounds.
type fullDemoCapture struct {
	Needed         bool        `json:"needed"`
	MissingRounds  []int       `json:"missing_rounds"`
	MissingCameras []string    `json:"missing_cameras,omitempty"`
	OmittedCameras []string    `json:"omitted_cameras,omitempty"`
	Space          *spaceCheck `json:"space,omitempty"`
}

const fullDemoCameraCaptureSeconds = recapplan.SecondCameraLeadSeconds*2 + recapplan.SecondCameraTailSeconds

// CheckFullDemoCapture handles POST /api/jobs/{id}/full-demo/capture-check: it
// plans the options like PlanFullDemo but saves nothing.
func (h *Handlers) CheckFullDemoCapture(w http.ResponseWriter, r *http.Request) {
	j, ok := h.loadJob(w, r)
	if !ok {
		return
	}
	if !canGenerateFromStatus(j.Status) || j.KillPlan == nil {
		writeCodedError(w, http.StatusConflict, recapplan.ErrFactsInsufficient, "Parse the selected player before planning Full Demo")
		return
	}
	if rejectMultiPOVFullDemo(w, j) {
		return
	}
	var req struct {
		Options *recapplan.Options `json:"options"`
	}
	if err := decodeSingleJSONBody(w, r, &req, true); err != nil || req.Options == nil {
		writeCodedError(w, http.StatusBadRequest, "invalid_full_demo_options", "A complete Full Demo options document is required")
		return
	}
	var capture fullDemoCapture
	err := h.generateIntents.WhileIdle(j.ID, func() error {
		if err := h.requireGenerateRenderIdle(j.ID); err != nil {
			return err
		}
		d, err := h.planFullDemo(r.Context(), j, *req.Options)
		if err != nil {
			return err
		}
		capture, err = h.fullDemoCaptureFor(j.ID, d)
		return err
	})
	if err != nil {
		rejectFullDemo(w, err)
		return
	}
	writeJSON(w, http.StatusOK, capture)
}

func (h *Handlers) fullDemoCaptureFor(id uuid.UUID, d recapplan.Document) (fullDemoCapture, error) {
	missing, err := recording.MissingFullDemoRounds(h.storage, id, d)
	if err != nil {
		return fullDemoCapture{}, err
	}
	var work spaceEstimate
	for _, round := range d.Rounds {
		seconds := float64(round.CaptureEndTick-round.CaptureStartTick) / float64(max(d.Clock.TickRate, 1))
		work.RenderSeconds += seconds
		if slices.Contains(missing, round.Number) {
			work.CaptureSeconds += seconds
		}
	}
	missingCameras, omittedCameras, err := recording.MissingFullDemoCameras(h.storage, id, d)
	if err != nil {
		return fullDemoCapture{}, err
	}
	work.CaptureSeconds += float64(len(missingCameras) * fullDemoCameraCaptureSeconds)
	return fullDemoCapture{Needed: len(missing) > 0 || len(missingCameras) > 0, MissingRounds: missing, MissingCameras: missingCameras, OmittedCameras: omittedCameras, Space: checkSpace(work, h.dataDiskFree)}, nil
}
