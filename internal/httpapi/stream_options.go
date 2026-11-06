package httpapi

import "github.com/rechedev9/cliphub/internal/streamclips"

func WithStreamRepository(repo StreamJobRepository) Option {
	return func(h *Handlers) {
		h.streamRepo = repo
	}
}

// WithStreamJobLocks shares per-job render admission/finalization locks with
// the stream worker. The local orchestrator must pass the same instance to
// both owners; tests and handler-only deployments receive a private instance.
func WithStreamJobLocks(locks *streamclips.JobLocks) Option {
	return func(h *Handlers) {
		if locks != nil {
			h.streamJobLocks = locks
		}
	}
}

func WithStreamProber(prober streamclips.Prober) Option {
	return func(h *Handlers) {
		h.streamProber = prober
	}
}

// WithStreamRenderProgress shares the stream render worker's live encode
// progress so the render state and the job list can report it.
func WithStreamRenderProgress(board *streamclips.RenderProgressBoard) Option {
	return func(h *Handlers) { h.streamRenderBoard = board }
}
