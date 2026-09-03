package reconcile

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/rechedev9/cliphub/internal/obs"
	"github.com/rechedev9/cliphub/internal/storage"
	"github.com/rechedev9/cliphub/internal/store"
)

// Result reports each durable lifecycle repaired before
// the orchestrator begins accepting requests.
type Result struct {
	DemoJobs           int
	DemoRenders        int
	GenerateRuns       int
	StreamJobs         int
	StreamRenderStates int
	StreamAcquisitions []uuid.UUID
	EditorRenders      int
}

func (r Result) Total() int {
	return r.DemoJobs + r.DemoRenders + r.GenerateRuns + r.StreamJobs + r.StreamRenderStates + len(r.StreamAcquisitions) + r.EditorRenders
}

// InterruptedWork repairs every process-local lifecycle it can, then
// returns all unrecoverable repository/storage errors together. Callers must
// not serve traffic when err is non-nil: doing so would expose active durable
// state without any queue owner capable of advancing it.
func InterruptedWork(
	ctx context.Context,
	jobs store.JobRepository,
	streams store.StreamJobRepository,
	editorProjects store.EditorProjectRepository,
	files storage.Storage,
	rec *obs.Recorder,
) (Result, error) {
	var result Result
	// Every supported main wiring constructs the stream and editor repositories
	// before startup reconciliation. A nil value therefore means the internal
	// wiring invariant was broken; fail before any partial repair instead of
	// panicking mid-sweep.
	if streams == nil {
		return result, fmt.Errorf("stream repository is required for startup reconciliation")
	}
	if editorProjects == nil {
		return result, fmt.Errorf("editor project repository is required for startup reconciliation")
	}
	var errs []error

	var err error
	result.DemoJobs, err = sweepInterruptedJobs(ctx, jobs, rec)
	if err != nil {
		errs = append(errs, fmt.Errorf("demo jobs: %w", err))
	}
	result.DemoRenders, err = sweepInterruptedDemoRenderStates(ctx, jobs, files, rec)
	if err != nil {
		errs = append(errs, fmt.Errorf("demo render states: %w", err))
	}
	result.GenerateRuns, err = sweepInterruptedGenerateRuns(ctx, jobs, files, rec)
	if err != nil {
		errs = append(errs, fmt.Errorf("generate runs: %w", err))
	}

	// Inspect render states before failing parent stream jobs. The detailed
	// result distinguishes completed durable renders from interrupted variants,
	// so parent repair cannot overwrite completion and observability is emitted
	// once per interrupted render state.
	streamRenderStates, streamRenderErr := reconcileInterruptedStreamRenderStates(ctx, streams, files, rec)
	result.StreamRenderStates = streamRenderStates.Reconciled
	err = streamRenderErr
	if err != nil {
		errs = append(errs, fmt.Errorf("stream render states: %w", err))
	}
	result.StreamAcquisitions, err = listInterruptedStreamAcquisitions(ctx, streams)
	if err != nil {
		errs = append(errs, fmt.Errorf("stream acquisitions: %w", err))
	}
	result.StreamJobs, err = sweepInterruptedStreamJobsAfterRenderStates(ctx, streams, rec, streamRenderStates)
	if err != nil {
		errs = append(errs, fmt.Errorf("stream jobs: %w", err))
	}
	result.EditorRenders, err = sweepInterruptedEditorRenders(ctx, editorProjects, files, rec)
	if err != nil {
		errs = append(errs, fmt.Errorf("editor renders: %w", err))
	}

	return result, errors.Join(errs...)
}
