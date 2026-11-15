package httpapi

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/rechedev9/cliphub/internal/moments"
)

// shortsBatchResponse is the Auto-Shorts choice: the plays POST /generate then
// records in one capture and renders as separate Shorts (edit.separate_shorts).
type shortsBatchResponse struct {
	MinScore   float64          `json:"min_score"`
	Limit      int              `json:"limit"`
	SegmentIDs []string         `json:"segment_ids"`
	Moments    []moments.Moment `json:"moments"`
}

// GetShortsBatch handles GET /api/jobs/{id}/shorts-batch?min_score=&limit=. It
// scores the kill plan afresh, so a moments file stored by an older build cannot skew the bar.
func (h *Handlers) GetShortsBatch(w http.ResponseWriter, r *http.Request) {
	opts, err := parseBatchOptions(r)
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
	picked := moments.PickBatch(moments.Rank(*j.KillPlan), opts)
	out := shortsBatchResponse{MinScore: opts.MinScore, Limit: opts.Limit, SegmentIDs: make([]string, 0, len(picked)), Moments: picked}
	for _, m := range picked {
		out.SegmentIDs = append(out.SegmentIDs, m.SegmentID)
	}
	writeJSON(w, http.StatusOK, out)
}

func parseBatchOptions(r *http.Request) (moments.BatchOptions, error) {
	var opts moments.BatchOptions
	q := r.URL.Query()
	if raw := q.Get("min_score"); raw != "" {
		v, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			return opts, fmt.Errorf("min_score must be a number")
		}
		opts.MinScore = v
	}
	if raw := q.Get("limit"); raw != "" {
		v, err := strconv.Atoi(raw)
		if err != nil {
			return opts, fmt.Errorf("limit must be an integer")
		}
		opts.Limit = v
	}
	return opts.Normalize()
}
