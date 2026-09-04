package httpapi

import (
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/editor"
	"github.com/rechedev9/cliphub/internal/job"
	"github.com/rechedev9/cliphub/internal/recording"
	"github.com/rechedev9/cliphub/internal/renderplan"
	"github.com/rechedev9/cliphub/internal/rules"
)

// readWhileLocked calls readOrMaterializeRenderVariantState with renderStateMu
// held by the test. A settled document must come back without the lock; an
// unsettled one must block until the lock is released.
func readWhileLocked(t *testing.T, h *Handlers, id uuid.UUID, variant string) (result *renderplan.RenderVariantState, blocked bool) {
	t.Helper()
	type outcome struct {
		state *renderplan.RenderVariantState
		err   error
	}
	done := make(chan outcome, 1)
	h.renderStateMu.Lock()
	go func() {
		state, _, err := h.readOrMaterializeRenderVariantState(id, variant)
		done <- outcome{state, err}
	}()
	select {
	case out := <-done:
		h.renderStateMu.Unlock()
		if out.err != nil {
			t.Fatalf("read while locked: %v", out.err)
		}
		return out.state, false
	case <-time.After(300 * time.Millisecond):
		h.renderStateMu.Unlock()
		out := <-done
		if out.err != nil {
			t.Fatalf("read after unlock: %v", out.err)
		}
		return out.state, true
	}
}

func seedReadyRender(t *testing.T, h *Handlers, store *fakeStorage, id uuid.UUID, warnings []string) renderplan.RenderVariantState {
	t.Helper()
	loadout, err := renderplan.LoadoutForVariant(editor.PresetViral60Clean)
	if err != nil {
		t.Fatal(err)
	}
	state, err := renderplan.NewRenderVariantStateForLoadout(renderplan.NewRenderVariantStateForLoadoutOptions{
		JobID:   id,
		Loadout: loadout,
		Status:  renderplan.RenderVariantStatusReady,
	})
	if err != nil {
		t.Fatal(err)
	}
	putAssistantJSON(t, store, state.RenderResultKey, editor.Result{
		Preset:   editor.PresetViral60Clean,
		Warnings: warnings,
		Shorts: []editor.ShortResult{{
			SegmentID:       "seg-001",
			OutputFormat:    editor.OutputFormatShort9x16,
			PublishArtifact: recording.RecordingArtifact{Path: "seg-001.mp4", SizeBytes: 10, Width: 1080, Height: 1920},
		}},
	})
	putReadyPublishArtifacts(t, store, state, "seg-001")
	if err := h.writeRenderVariantState(state); err != nil {
		t.Fatal(err)
	}
	return state
}

func TestRenderVariantReadServesSettledStateWithoutTheLock(t *testing.T) {
	repo := newFakeRepo()
	store := newFakeStorage()
	j := job.Job{ID: uuid.New(), Status: job.StatusRecorded, Rules: rules.Default()}
	repo.jobs[j.ID] = j
	h := NewHandlers(repo, store, &fakeQueue{})
	seeded := seedReadyRender(t, h, store, j.ID, nil)

	writesBefore := len(store.puts)
	state, blocked := readWhileLocked(t, h, j.ID, editor.PresetViral60Clean)
	if blocked {
		t.Fatal("a settled ready state must be served without renderStateMu")
	}
	if state.Status != renderplan.RenderVariantStatusReady || state.ArtifactPrefix != seeded.ArtifactPrefix {
		t.Fatalf("state = %#v, want the stored ready state", state)
	}
	if len(store.puts) != writesBefore {
		t.Fatalf("settled read wrote %d artifacts; the fast path must not write", len(store.puts)-writesBefore)
	}
}

func TestRenderVariantReadStillMigratesUnsettledStateUnderTheLock(t *testing.T) {
	repo := newFakeRepo()
	store := newFakeStorage()
	j := job.Job{ID: uuid.New(), Status: job.StatusRecorded, Rules: rules.Default()}
	repo.jobs[j.ID] = j
	h := NewHandlers(repo, store, &fakeQueue{})
	// The result carries a warning the stored ready state never recorded: the
	// locked path must rewrite it as review_required, exactly as before.
	seedReadyRender(t, h, store, j.ID, []string{"freeze at 00:12"})

	state, blocked := readWhileLocked(t, h, j.ID, editor.PresetViral60Clean)
	if !blocked {
		t.Fatal("an unsettled ready state must wait for renderStateMu before migrating")
	}
	if state.Status != renderplan.RenderVariantStatusReview || !slices.Equal(state.Warnings, []string{"freeze at 00:12"}) {
		t.Fatalf("state = %#v, want review with the result's warning", state)
	}
	// Once migrated the document is settled and the next read is lock-free.
	if _, blocked := readWhileLocked(t, h, j.ID, editor.PresetViral60Clean); blocked {
		t.Fatal("a migrated review state must be served without the lock on the next read")
	}
}
