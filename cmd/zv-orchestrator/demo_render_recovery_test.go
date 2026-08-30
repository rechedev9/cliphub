package main

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/hibiken/asynq"

	"github.com/rechedev9/cliphub/internal/editor"
	"github.com/rechedev9/cliphub/internal/job"
	"github.com/rechedev9/cliphub/internal/renderplan"
	"github.com/rechedev9/cliphub/internal/storage"
	"github.com/rechedev9/cliphub/internal/tasks"
)

type recordingRenderQueue struct {
	tasks []*asynq.Task
	err   error
}

func (q *recordingRenderQueue) Enqueue(task *asynq.Task, _ ...asynq.Option) (*asynq.TaskInfo, error) {
	if q.err != nil {
		return nil, q.err
	}
	q.tasks = append(q.tasks, task)
	return &asynq.TaskInfo{ID: "recovered"}, nil
}

func TestRecoverFullDemoRenderResumesWorkerAfterRestart(t *testing.T) {
	ctx := context.Background()
	repo := newMemoryJobRepository()
	store, err := storage.NewLocal(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	j := seedJob(t, repo, job.StatusRecorded)
	loadout, err := renderplan.LoadoutForVariant(editor.PresetGameplayPOV60)
	if err != nil {
		t.Fatal(err)
	}
	state, err := renderplan.NewRenderVariantStateForLoadout(renderplan.NewRenderVariantStateForLoadoutOptions{
		JobID:   j.ID,
		Loadout: loadout,
		Status:  renderplan.RenderVariantStatusRendering,
	})
	if err != nil {
		t.Fatal(err)
	}
	key, err := renderplan.RenderVariantStateKey(j.ID, editor.PresetGameplayPOV60)
	if err != nil {
		t.Fatal(err)
	}
	putSweepFixture(t, store, key, state)

	swept, err := sweepInterruptedDemoRenderStates(ctx, repo, store, nil)
	if err != nil {
		t.Fatalf("sweep: %v", err)
	}
	if swept.Failed != 0 || len(swept.Recoverable) != 1 {
		t.Fatalf("sweep = failed %d recoverable %d, want 0/1", swept.Failed, len(swept.Recoverable))
	}
	if swept.Recoverable[0].Variant != editor.PresetGameplayPOV60 {
		t.Fatalf("recoverable variant = %q, want %q", swept.Recoverable[0].Variant, editor.PresetGameplayPOV60)
	}
	var afterSweep renderplan.RenderVariantState
	readSweepFixture(t, store, key, &afterSweep)
	if afterSweep.Status != renderplan.RenderVariantStatusRendering {
		t.Fatalf("sweep rewrote status to %q, want rendering until recover", afterSweep.Status)
	}

	queue := &recordingRenderQueue{}
	if err := recoverDemoRenders(ctx, swept.Recoverable, true, store, queue, nil); err != nil {
		t.Fatalf("recoverDemoRenders: %v", err)
	}
	if len(queue.tasks) != 1 || queue.tasks[0].Type() != tasks.TypeRenderVariant {
		t.Fatalf("enqueued = %#v, want one %s", queue.tasks, tasks.TypeRenderVariant)
	}
	var payload tasks.RenderVariantPayload
	if err := json.Unmarshal(queue.tasks[0].Payload(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.JobID != j.ID || payload.Variant != editor.PresetGameplayPOV60 {
		t.Fatalf("payload = %+v, want job %s variant %s", payload, j.ID, editor.PresetGameplayPOV60)
	}
	if !payload.Edit.MatchRecap || payload.Edit.Format != renderplan.FormatLandscape16x9 {
		t.Fatalf("recovered edit = %+v, want Full Demo recap", payload.Edit)
	}
	var afterRecover renderplan.RenderVariantState
	readSweepFixture(t, store, key, &afterRecover)
	if afterRecover.Status != renderplan.RenderVariantStatusQueued {
		t.Fatalf("status after recover = %q, want queued for the resumed worker", afterRecover.Status)
	}
}

func TestRecoverFullDemoRenderFailsVisiblyWhenEnqueueImpossible(t *testing.T) {
	ctx := context.Background()
	repo := newMemoryJobRepository()
	store, err := storage.NewLocal(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	j := seedJob(t, repo, job.StatusRecorded)
	loadout, err := renderplan.LoadoutForVariant(editor.PresetGameplayPOV60)
	if err != nil {
		t.Fatal(err)
	}
	state, err := renderplan.NewRenderVariantStateForLoadout(renderplan.NewRenderVariantStateForLoadoutOptions{
		JobID:   j.ID,
		Loadout: loadout,
		Status:  renderplan.RenderVariantStatusRendering,
	})
	if err != nil {
		t.Fatal(err)
	}
	key, err := renderplan.RenderVariantStateKey(j.ID, editor.PresetGameplayPOV60)
	if err != nil {
		t.Fatal(err)
	}
	putSweepFixture(t, store, key, state)
	swept, err := sweepInterruptedDemoRenderStates(ctx, repo, store, nil)
	if err != nil {
		t.Fatal(err)
	}

	if err := recoverDemoRenders(ctx, swept.Recoverable, false, store, nil, nil); err != nil {
		t.Fatalf("recover without worker: %v", err)
	}
	var failed renderplan.RenderVariantState
	readSweepFixture(t, store, key, &failed)
	if failed.Status != renderplan.RenderVariantStatusFailed || failed.Error != demoRenderRecoveryDisabledReason {
		t.Fatalf("disabled recover = status %q error %q, want failed/%q", failed.Status, failed.Error, demoRenderRecoveryDisabledReason)
	}

	putSweepFixture(t, store, key, state)
	swept, err = sweepInterruptedDemoRenderStates(ctx, repo, store, nil)
	if err != nil {
		t.Fatal(err)
	}
	queue := &recordingRenderQueue{err: errInlineQueueFull}
	if err := recoverDemoRenders(ctx, swept.Recoverable, true, store, queue, nil); err != nil {
		t.Fatalf("recover with full queue: %v", err)
	}
	readSweepFixture(t, store, key, &failed)
	if failed.Status != renderplan.RenderVariantStatusFailed {
		t.Fatalf("full-queue recover = %q, want failed", failed.Status)
	}
	if failed.Error == "" {
		t.Fatal("full-queue recover left an empty error")
	}
}
