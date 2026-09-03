package reconcile

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/obs"
	"github.com/rechedev9/cliphub/internal/storage"
	"github.com/rechedev9/cliphub/internal/timelineplan"
)

const interruptedEditorRenderReason = "interrupted: the orchestrator restarted before the editor render completed"

// editorInterruptSweeper is the uncapped repository surface the startup sweep
// needs to find editor projects whose render died with the previous process.
type editorInterruptSweeper interface {
	ListByStatus(context.Context, timelineplan.Status) ([]timelineplan.Project, error)
	UpdateStatus(context.Context, uuid.UUID, timelineplan.Status, string) error
}

// sweepInterruptedEditorRenders fails every editor project left in
// `rendering`: the timeline render task lived only in the in-process queue, so
// nothing can finish it after a restart and StartEditorRender would answer 409
// forever. It mirrors the worker's failure path: the row goes to failed and
// the render state file keeps the previously published revision keys so the
// last good MP4 stays downloadable.
func sweepInterruptedEditorRenders(ctx context.Context, repo editorInterruptSweeper, store storage.Storage, rec *obs.Recorder) (int, error) {
	projects, err := repo.ListByStatus(ctx, timelineplan.StatusRendering)
	if err != nil {
		return 0, fmt.Errorf("list rendering editor projects: %w", err)
	}
	swept := 0
	var errs []error
	for _, p := range projects {
		if err := ctx.Err(); err != nil {
			errs = append(errs, err)
			break
		}
		if err := repo.UpdateStatus(ctx, p.ID, timelineplan.StatusFailed, interruptedEditorRenderReason); err != nil {
			errs = append(errs, fmt.Errorf("fail interrupted editor project %s: %w", p.ID, err))
			continue
		}
		failed := timelineplan.RenderState{
			ProjectID: p.ID,
			Status:    timelineplan.StatusFailed,
			Error:     interruptedEditorRenderReason,
			UpdatedAt: time.Now().UTC(),
		}
		var previous timelineplan.RenderState
		key := timelineplan.RenderStateKey(p.ID)
		if found, readErr := readSweepJSON(store, key, &previous); found && readErr == nil {
			failed.Fingerprint = previous.Fingerprint
			failed.VideoKey = previous.VideoKey
			failed.CoverKey = previous.CoverKey
			failed.ResultKey = previous.ResultKey
		}
		if err := writeSweepJSON(store, key, failed); err != nil {
			errs = append(errs, fmt.Errorf("write failed editor render state for project %s: %w", p.ID, err))
			continue
		}
		swept++
		if rec != nil {
			_ = rec.RecordError(obs.Event{
				JobID:   p.ID.String(),
				Stage:   obs.StageEditor,
				Class:   interruptedClass,
				Message: interruptedEditorRenderReason,
			})
		}
	}
	return swept, errors.Join(errs...)
}
