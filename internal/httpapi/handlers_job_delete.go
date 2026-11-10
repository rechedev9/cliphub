package httpapi

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/rechedev9/cliphub/internal/artifacts"
	"github.com/rechedev9/cliphub/internal/generateintent"
	"github.com/rechedev9/cliphub/internal/job"
)

// jobIsInFlight reports whether a stage is actively working on the job's files
// or processes, so deleting now would race that work. queued is included: a
// parse/scan task may be about to run against the stored demo.
func jobIsInFlight(s job.Status) bool {
	switch s {
	case job.StatusQueued, job.StatusScanning, job.StatusParsing, job.StatusRecording, job.StatusComposing:
		return true
	default:
		return false
	}
}

// DeleteJob handles DELETE /api/jobs/{id}: it removes a job together with its
// artifact tree (jobs/<id>) and its stored demo copy (demos/<id>.dem) so the
// user can clear a demo from the library and reclaim disk space. Settled jobs
// (scanned, parsed, recorded, composed, done, failed) delete; a job with work
// in flight is refused with 409 until it settles. Render variants and guided
// generate runs have their own lifecycle that never moves job.Status, so the
// delete also holds the render-state lock and the generate intent lock and
// refuses while either reports active work: otherwise a running render worker
// keeps writing into a tree that no longer exists. The job row is removed last
// so a failed artifact delete leaves the row in place to retry. Idempotent —
// a repeat delete after success returns 404.
func (h *Handlers) DeleteJob(w http.ResponseWriter, r *http.Request) {
	// Metadata only: the in-flight guard reads the status and everything
	// deleted is addressed by job id, so the plan blob is never needed.
	j, ok := h.loadJobMeta(w, r)
	if !ok {
		return
	}
	if jobIsInFlight(j.Status) {
		writeError(w, http.StatusConflict, fmt.Sprintf("job is %s; wait for it to settle before deleting", j.Status))
		return
	}
	deleter, ok := h.storage.(jobArtifactDeleter)
	if !ok {
		writeError(w, http.StatusNotImplemented, "storage backend does not support delete")
		return
	}
	// The per-job intent lock is what keeps a render from being admitted for
	// this job while its tree goes away (StartRenderVariant admits inside the
	// same WhileIdle). The process-wide renderStateMu is deliberately not
	// held: the tree can be gigabytes of captures and every other job's
	// render poll would stall behind the RemoveAll. The idle read is the same
	// lock-free read StartGenerate does under this lock.
	err := h.generateIntents.WhileIdle(j.ID, func() error {
		if err := h.requireGenerateRenderIdle(j.ID); err != nil {
			return err
		}
		if err := deleter.DeleteTree(artifacts.JobPrefix(j.ID)); err != nil {
			return fmt.Errorf("delete job artifacts: %w", err)
		}
		if err := deleter.Delete(fmt.Sprintf("demos/%s.dem", j.ID)); err != nil {
			return fmt.Errorf("delete job demo: %w", err)
		}
		if err := h.repo.Delete(r.Context(), j.ID); err != nil {
			return fmt.Errorf("delete job: %w", err)
		}
		h.rosterCache.evict(j.ID)
		return nil
	})
	switch {
	case err == nil:
		w.WriteHeader(http.StatusNoContent)
	case errors.Is(err, generateintent.ErrActiveRun), errors.Is(err, errGenerateRenderActive):
		writeCodedError(w, http.StatusConflict, generateWorkActive, "job has an active render or generate run; wait for it to settle before deleting")
	default:
		internalError(w, "delete job", err)
	}
}
