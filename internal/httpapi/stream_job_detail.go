package httpapi

import (
	"net/http"

	"github.com/rechedev9/cliphub/internal/streamclips"
	"github.com/rechedev9/cliphub/internal/vodfetch"
)

// WithStreamAcquireProgress shares the acquire worker's live download progress
// so GET /api/stream-jobs/{id} can report it while the job is acquiring.
func WithStreamAcquireProgress(progress *streamclips.AcquireProgress) Option {
	return func(h *Handlers) { h.acquireProgress = progress }
}

type streamJobDetail struct {
	streamclips.Job
	RenderedOutputs            []streamRenderedOutput `json:"rendered_outputs"`
	RenderedOutputsUnavailable bool                   `json:"rendered_outputs_unavailable,omitempty"`
	// DownloadProgress is set only while an acquiring job has reported progress.
	DownloadProgress *vodfetch.Progress `json:"download_progress,omitempty"`
}

func (h *Handlers) GetStreamJob(w http.ResponseWriter, r *http.Request) {
	j, ok := h.loadStreamJob(w, r)
	if !ok {
		return
	}
	j.FailureCode = jobFailureCode(j.FailureReason, j.FailureCode)
	outputs, unavailable := h.streamJobRenderedOutputs(j, true)
	detail := streamJobDetail{Job: j, RenderedOutputs: outputs, RenderedOutputsUnavailable: unavailable}
	if j.Status == streamclips.StatusAcquiring {
		if p, ok := h.acquireProgress.Get(j.ID); ok {
			detail.DownloadProgress = &p
		}
	}
	writeJSON(w, http.StatusOK, detail)
}
