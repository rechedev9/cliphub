package httpapi

import (
	"errors"
	"net/http"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/job"
	"github.com/rechedev9/cliphub/internal/obs"
)

// userReportCategories is the closed set a user can report about a finished
// or failed job. No free text crosses this endpoint, so the report can go to
// remote diagnostics without another privacy review.
var userReportCategories = map[string]bool{
	"black_video":   true,
	"wrong_overlay": true,
	"audio":         true,
	"cuts":          true,
	"other":         true,
}

// userReportInterval bounds a job to one report per minute, so a repeated
// click cannot page the developer more than once.
const userReportInterval = 60 * time.Second

// userReportLimiter remembers when each job was last reported. Its zero value
// is ready to use.
type userReportLimiter struct {
	mu   sync.Mutex
	last map[uuid.UUID]time.Time
	now  func() time.Time
}

// allow records a report for id unless one was accepted less than
// userReportInterval ago. Expired entries are dropped on every call.
func (l *userReportLimiter) allow(id uuid.UUID) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	if l.now != nil {
		now = l.now()
	}
	for reported, at := range l.last {
		if now.Sub(at) >= userReportInterval {
			delete(l.last, reported)
		}
	}
	if _, recent := l.last[id]; recent {
		return false
	}
	if l.last == nil {
		l.last = map[uuid.UUID]time.Time{}
	}
	l.last[id] = now
	return true
}

type userReportRequest struct {
	Category string `json:"category"`
}

// ReportJob handles POST /api/jobs/{id}/report: the user flags a problem with
// a job's video. It emits a user.report trace tied to the job so alerting can
// join it with the job's attempts.
func (h *Handlers) ReportJob(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid job id")
		return
	}
	var req userReportRequest
	if err := decodeSingleJSONBody(w, r, &req, true); err != nil || !userReportCategories[req.Category] {
		writeError(w, http.StatusBadRequest, "category must be one of black_video, wrong_overlay, audio, cuts, other")
		return
	}
	if _, err := h.repo.GetMeta(r.Context(), id); err != nil {
		if errors.Is(err, job.ErrNotFound) {
			writeError(w, http.StatusNotFound, "job not found")
			return
		}
		internalError(w, "report job", err)
		return
	}
	if !h.userReports.allow(id) {
		writeError(w, http.StatusTooManyRequests, "this job was already reported in the last minute")
		return
	}
	ctx := obs.WithTrace(r.Context(), obs.TraceContext{JobID: id.String()})
	obs.EmitTrace(ctx, obs.TraceEntry{Event: "user.report", Level: "warn", Message: "category=" + req.Category})
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "reported"})
}
