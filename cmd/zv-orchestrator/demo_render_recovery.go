package main

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/hibiken/asynq"

	"github.com/rechedev9/cliphub/internal/editor"
	"github.com/rechedev9/cliphub/internal/obs"
	"github.com/rechedev9/cliphub/internal/renderplan"
	"github.com/rechedev9/cliphub/internal/storage"
	"github.com/rechedev9/cliphub/internal/tasks"
)

const demoRenderRecoveryDisabledReason = "interrupted: the orchestrator restarted before render completed and the render worker is disabled"

type demoRenderEnqueuer interface {
	Enqueue(task *asynq.Task, opts ...asynq.Option) (*asynq.TaskInfo, error)
}

// recoverDemoRenders re-enqueues every valid queued/rendering variant left by
// the previous process. Capture segments stay on disk; the worker restarts the
// encode. If the worker is off or enqueue fails, the variant is failed so
// Biblioteca cannot sit on EDITANDO with no work.
func recoverDemoRenders(
	ctx context.Context,
	items []demoRenderRecovery,
	workerEnabled bool,
	store storage.Storage,
	queue demoRenderEnqueuer,
	rec *obs.Recorder,
) error {
	var errs []error
	for _, item := range items {
		if err := ctx.Err(); err != nil {
			return errors.Join(append(errs, err)...)
		}
		if !workerEnabled || queue == nil {
			if err := failRecoveredDemoRender(store, item, demoRenderRecoveryDisabledReason, rec); err != nil {
				errs = append(errs, err)
			}
			continue
		}
		if err := enqueueRecoveredDemoRender(store, queue, item, rec); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func enqueueRecoveredDemoRender(store storage.Storage, queue demoRenderEnqueuer, item demoRenderRecovery, rec *obs.Recorder) error {
	state, ok, err := readRecoveredDemoRender(store, item)
	if err != nil {
		return fmt.Errorf("read recovered demo render %s %s: %w", item.JobID, item.Variant, err)
	}
	if !ok {
		return failRecoveredDemoRender(store, item, interruptedDemoRenderReason, rec)
	}
	edit, musicKey, musicVolume, gameVolume := resumeDemoRenderIntent(store, state)
	task, err := tasks.NewRenderVariantTask(item.JobID, item.Variant, musicKey, musicVolume, gameVolume, edit)
	if err != nil {
		if failErr := failRecoveredDemoRender(store, item, interruptedDemoRenderReason+": "+err.Error(), rec); failErr != nil {
			return errors.Join(failErr, err)
		}
		return nil
	}
	if err := writeRecoveredDemoRenderStatus(store, state, renderplan.RenderVariantStatusQueued, ""); err != nil {
		return fmt.Errorf("queue recovered demo render %s %s: %w", item.JobID, item.Variant, err)
	}
	if _, err := queue.Enqueue(task, asynq.MaxRetry(0)); err != nil {
		if failErr := failRecoveredDemoRender(store, item, "enqueue recovered render: "+err.Error(), rec); failErr != nil {
			return errors.Join(failErr, err)
		}
		return nil
	}
	return nil
}

func resumeDemoRenderIntent(store storage.Storage, state renderplan.RenderVariantState) (renderplan.EditRequest, string, float64, *float64) {
	if state.Variant == editor.PresetGameplayPOV60 {
		return renderplan.RecapEditRequest(), "", 0, nil
	}
	if state.EditDocumentKey == "" {
		return renderplan.DefaultEditRequest(), "", 0, nil
	}
	var document renderplan.EditDocument
	found, err := readSweepJSON(store, state.EditDocumentKey, &document)
	if err != nil || !found {
		return renderplan.DefaultEditRequest(), "", 0, nil
	}
	musicKey := ""
	musicVolume := 0.0
	var gameVolume *float64
	if document.Music != nil {
		musicKey = document.Music.Key
		musicVolume = document.Music.Volume
		gameVolume = document.Music.GameVolume
	}
	return document.Edit, musicKey, musicVolume, gameVolume
}

func failRecoveredDemoRender(store storage.Storage, item demoRenderRecovery, reason string, rec *obs.Recorder) error {
	state, ok, err := readRecoveredDemoRender(store, item)
	if err != nil {
		return fmt.Errorf("read demo render %s %s to fail: %w", item.JobID, item.Variant, err)
	}
	if !ok {
		loadout, loadoutErr := renderplan.LoadoutForVariant(item.Variant)
		if loadoutErr != nil {
			return fmt.Errorf("loadout for failed demo render %s %s: %w", item.JobID, item.Variant, loadoutErr)
		}
		built, buildErr := renderplan.NewRenderVariantStateForLoadout(renderplan.NewRenderVariantStateForLoadoutOptions{
			JobID:   item.JobID,
			Loadout: loadout,
			Status:  renderplan.RenderVariantStatusFailed,
			Error:   reason,
			Now:     time.Now().UTC(),
		})
		if buildErr != nil {
			return fmt.Errorf("build failed demo render %s %s: %w", item.JobID, item.Variant, buildErr)
		}
		state = built
	} else {
		state.Status = renderplan.RenderVariantStatusFailed
		state.Error = reason
		state.UpdatedAt = time.Now().UTC()
	}
	key, err := renderplan.RenderVariantStateKey(item.JobID, item.Variant)
	if err != nil {
		return fmt.Errorf("resolve failed demo render %s %s: %w", item.JobID, item.Variant, err)
	}
	if err := writeSweepJSON(store, key, state); err != nil {
		return fmt.Errorf("write failed demo render %s: %w", key, err)
	}
	recordInterruptedRender(rec, item.Demo, item.Target, reason)
	return nil
}

func writeRecoveredDemoRenderStatus(store storage.Storage, state renderplan.RenderVariantState, status, message string) error {
	state.Status = status
	state.Error = message
	state.UpdatedAt = time.Now().UTC()
	key, err := renderplan.RenderVariantStateKey(state.JobID, state.Variant)
	if err != nil {
		return err
	}
	return writeSweepJSON(store, key, state)
}

func readRecoveredDemoRender(store storage.Storage, item demoRenderRecovery) (renderplan.RenderVariantState, bool, error) {
	key, err := renderplan.RenderVariantStateKey(item.JobID, item.Variant)
	if err != nil {
		return renderplan.RenderVariantState{}, false, err
	}
	var state renderplan.RenderVariantState
	found, err := readSweepJSON(store, key, &state)
	if err != nil || !found {
		return renderplan.RenderVariantState{}, false, err
	}
	if !validDemoRenderStateIdentity(state, item.JobID, item.Variant) {
		return renderplan.RenderVariantState{}, false, nil
	}
	return state, true, nil
}
