package httpapi

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/rechedev9/cliphub/internal/moments"
	"github.com/rechedev9/cliphub/internal/parser"
	"github.com/rechedev9/cliphub/internal/recapplan"
	"github.com/rechedev9/cliphub/internal/renderplan"
)

// GetAutoPOV handles GET /api/jobs/{id}/auto-pov?target_seconds=&extra_seconds=&pre_roll_seconds=: the plays (and
// players, in a multi-POV summary) picked by moments.Direct. It scores the kill plan afresh, like the Auto-Shorts
// batch, with each play recut to the pre-roll of the video.
func (h *Handlers) GetAutoPOV(w http.ResponseWriter, r *http.Request) {
	opts, err := parseDirectorOptions(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	preRoll, err := parsePreRollQuery(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	j, ok := h.loadJob(w, r)
	if !ok {
		return
	}
	if j.KillPlan == nil {
		writeError(w, http.StatusConflict, fmt.Sprintf("job moments are not ready (status=%s)", j.Status))
		return
	}
	if preRoll != 0 && j.IsMultiPOV() {
		writeCodedError(w, http.StatusBadRequest, multiPOVSelectionCode, "the pre-roll is for a video of one player")
		return
	}
	plan := *j.KillPlan
	if preRoll != 0 {
		facts, _, err := recapplan.LoadFacts(h.storage, j.ID)
		if err != nil {
			internalError(w, "load round starts for the pre-roll", err)
			return
		}
		plan = parser.WithPreRoll(plan, preRoll, parser.FactsRoundStarts(facts))
	}
	writeJSON(w, http.StatusOK, moments.Direct(plan, opts))
}

// parsePreRollQuery reads pre_roll_seconds; absent or 3 keeps the analysed plays.
func parsePreRollQuery(r *http.Request) (int, error) {
	raw := r.URL.Query().Get("pre_roll_seconds")
	if raw == "" {
		return 0, nil
	}
	seconds, err := strconv.Atoi(raw)
	if err != nil {
		return 0, renderplan.ValidatePreRoll(-1)
	}
	if seconds == renderplan.DefaultPreRollSeconds {
		return 0, nil
	}
	return seconds, renderplan.ValidatePreRoll(seconds)
}

func parseDirectorOptions(r *http.Request) (moments.DirectorOptions, error) {
	var opts moments.DirectorOptions
	q := r.URL.Query()
	fields := []struct {
		key string
		dst *float64
	}{{"target_seconds", &opts.TargetSeconds}, {"extra_seconds", &opts.ExtraSeconds}}
	for _, f := range fields {
		raw := q.Get(f.key)
		if raw == "" {
			continue
		}
		v, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			return opts, fmt.Errorf("%s must be a number", f.key)
		}
		*f.dst = v
	}
	return opts.Normalize()
}
