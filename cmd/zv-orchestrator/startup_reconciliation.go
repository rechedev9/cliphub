package main

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/rechedev9/cliphub/internal/obs"
	"github.com/rechedev9/cliphub/internal/storage"
)

// startupReconciliationResult reports each durable lifecycle repaired before
// the orchestrator begins accepting requests.
type startupReconciliationResult struct {
	DemoJobs             int
	DemoRenders          int
	DemoRenderRecoveries []demoRenderRecovery
	GenerateRuns         int
	StreamJobs           int
	StreamRenderStates   int
	StreamAcquisitions   []uuid.UUID
}

func (r startupReconciliationResult) total() int {
	return r.DemoJobs + r.DemoRenders + len(r.DemoRenderRecoveries) + r.GenerateRuns + r.StreamJobs + r.StreamRenderStates + len(r.StreamAcquisitions)
}

// reconcileInterruptedWork repairs every process-local lifecycle it can, then
// returns all unrecoverable repository/storage errors together. Callers must
// not serve traffic when err is non-nil: doing so would expose active durable
// state without any queue owner capable of advancing it.
func reconcileInterruptedWork(
	ctx context.Context,
	repo orchestratorJobRepository,
	streamRepo orchestratorStreamJobRepository,
	store storage.Storage,
	rec *obs.Recorder,
) (startupReconciliationResult, error) {
	var result startupReconciliationResult
	// Every supported main wiring constructs a stream repository before startup
	// reconciliation. A nil value therefore means the internal wiring invariant
	// was broken; fail before any partial repair instead of panicking mid-sweep.
	if streamRepo == nil {
		return result, fmt.Errorf("stream repository is required for startup reconciliation")
	}
	var errs []error

	var err error
	result.DemoJobs, err = sweepInterruptedJobs(ctx, repo, rec)
	if err != nil {
		errs = append(errs, fmt.Errorf("demo jobs: %w", err))
	}
	demoRenders, err := sweepInterruptedDemoRenderStates(ctx, repo, store, rec)
	result.DemoRenders = demoRenders.Failed
	result.DemoRenderRecoveries = demoRenders.Recoverable
	if err != nil {
		errs = append(errs, fmt.Errorf("demo render states: %w", err))
	}
	result.GenerateRuns, err = sweepInterruptedGenerateRuns(ctx, repo, store, rec)
	if err != nil {
		errs = append(errs, fmt.Errorf("generate runs: %w", err))
	}

	// Inspect render states before failing parent stream jobs. The detailed
	// result distinguishes completed durable renders from interrupted variants,
	// so parent repair cannot overwrite completion and observability is emitted
	// once per interrupted render state.
	streamRenderStates, streamRenderErr := reconcileInterruptedStreamRenderStates(ctx, streamRepo, store, rec)
	result.StreamRenderStates = streamRenderStates.Reconciled
	err = streamRenderErr
	if err != nil {
		errs = append(errs, fmt.Errorf("stream render states: %w", err))
	}
	result.StreamAcquisitions, err = listInterruptedStreamAcquisitions(ctx, streamRepo)
	if err != nil {
		errs = append(errs, fmt.Errorf("stream acquisitions: %w", err))
	}
	result.StreamJobs, err = sweepInterruptedStreamJobsAfterRenderStates(ctx, streamRepo, rec, streamRenderStates)
	if err != nil {
		errs = append(errs, fmt.Errorf("stream jobs: %w", err))
	}

	return result, errors.Join(errs...)
}
