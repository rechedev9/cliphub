package reconcile

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/rechedev9/cliphub/internal/obs"
	"github.com/rechedev9/cliphub/internal/storage"
	"github.com/rechedev9/cliphub/internal/store"
)

func sweepInterruptedStreamJobs(ctx context.Context, repo streamInterruptSweeper, rec *obs.Recorder) (int, error) {
	return sweepInterruptedStreamJobsAfterRenderStates(ctx, repo, rec, streamRenderSweepResult{auditComplete: true})
}

func sweepInterruptedStreamRenderStates(ctx context.Context, repo streamInterruptSweeper, store storage.Storage, rec *obs.Recorder) (int, error) {
	result, err := reconcileInterruptedStreamRenderStates(ctx, repo, store, rec)
	return result.Reconciled, err
}

// sweepInterruptedDemoRenderStatesFromRepo and sweepInterruptedGenerateRunsFromRepo
// list the jobs themselves. Production shares one listing across both sweeps
// (InterruptedWork owns it); these keep the per-sweep tests focused on the
// sweep instead of the listing.
func sweepInterruptedDemoRenderStatesFromRepo(ctx context.Context, repo interruptSweeper, store storage.Storage, rec *obs.Recorder) (int, error) {
	jobs, err := listAllDemoJobs(ctx, repo)
	if err != nil {
		return 0, err
	}
	return sweepInterruptedDemoRenderStates(ctx, store, rec, jobs)
}

func sweepInterruptedGenerateRunsFromRepo(ctx context.Context, repo interruptSweeper, store storage.Storage, rec *obs.Recorder) (int, error) {
	jobs, err := listAllDemoJobs(ctx, repo)
	if err != nil {
		return 0, err
	}
	return sweepInterruptedGenerateRuns(ctx, repo, store, rec, jobs)
}

func newTestSQLiteRepo(t *testing.T) *store.SQLiteJobRepository {
	t.Helper()
	repo, err := store.NewSQLiteJobRepository(filepath.Join(t.TempDir(), "jobs.db"))
	if err != nil {
		t.Fatalf("newSQLiteJobRepository: %v", err)
	}
	t.Cleanup(func() { _ = repo.Close() })
	return repo
}

func newTestSQLiteStreamRepo(t *testing.T) *store.SQLiteStreamJobRepository {
	t.Helper()
	jobRepo := newTestSQLiteRepo(t)
	streamRepo, err := store.NewSQLiteStreamJobRepository(jobRepo.DB())
	if err != nil {
		t.Fatalf("newSQLiteStreamJobRepository: %v", err)
	}
	return streamRepo
}
