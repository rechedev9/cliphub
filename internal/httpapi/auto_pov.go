package httpapi

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/rechedev9/cliphub/internal/moments"
)

const autoPOVSinglePlayerCode = "auto_pov_single_player"

// GetAutoPOV handles GET /api/jobs/{id}/auto-pov?target_seconds=&extra_seconds=: the plays and players
// a multi-POV summary shows, picked by moments.Direct. It scores the kill plan afresh, like the Auto-Shorts batch.
func (h *Handlers) GetAutoPOV(w http.ResponseWriter, r *http.Request) {
	opts, err := parseDirectorOptions(r)
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
	if !j.IsMultiPOV() {
		writeCodedError(w, http.StatusConflict, autoPOVSinglePlayerCode, "Auto-POV picks between several players; this job follows one")
		return
	}
	writeJSON(w, http.StatusOK, moments.Direct(*j.KillPlan, opts))
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
