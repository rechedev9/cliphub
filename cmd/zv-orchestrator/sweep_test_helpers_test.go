package main

import (
	"context"

	"github.com/rechedev9/cliphub/internal/obs"
	"github.com/rechedev9/cliphub/internal/storage"
)

func sweepInterruptedStreamJobs(ctx context.Context, repo streamInterruptSweeper, rec *obs.Recorder) (int, error) {
	return sweepInterruptedStreamJobsAfterRenderStates(ctx, repo, rec, streamRenderSweepResult{auditComplete: true})
}

func sweepInterruptedStreamRenderStates(ctx context.Context, repo streamInterruptSweeper, store storage.Storage, rec *obs.Recorder) (int, error) {
	result, err := reconcileInterruptedStreamRenderStates(ctx, repo, store, rec)
	return result.Reconciled, err
}
