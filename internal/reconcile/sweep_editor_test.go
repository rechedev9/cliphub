package reconcile

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/storage"
	"github.com/rechedev9/cliphub/internal/store"
	"github.com/rechedev9/cliphub/internal/timelineplan"
)

type editorSweeperRepo interface {
	editorInterruptSweeper
	Create(context.Context, *timelineplan.Project) error
	Get(context.Context, uuid.UUID) (timelineplan.Project, error)
}

func newEditorSweeperRepos(t *testing.T) map[string]editorSweeperRepo {
	t.Helper()
	sqliteProjects, err := store.NewSQLiteEditorProjectRepository(newTestSQLiteRepo(t).DB())
	if err != nil {
		t.Fatalf("store.NewSQLiteEditorProjectRepository: %v", err)
	}
	return map[string]editorSweeperRepo{
		"memory": store.NewMemoryEditorProjectRepository(),
		"sqlite": sqliteProjects,
	}
}

func seedEditorProject(t *testing.T, repo editorSweeperRepo, status timelineplan.Status) timelineplan.Project {
	t.Helper()
	p := &timelineplan.Project{Title: string(status), Status: timelineplan.StatusDraft}
	if err := repo.Create(context.Background(), p); err != nil {
		t.Fatalf("Create(%s): %v", status, err)
	}
	if status != timelineplan.StatusDraft {
		if err := repo.UpdateStatus(context.Background(), p.ID, status, ""); err != nil {
			t.Fatalf("UpdateStatus(%s): %v", status, err)
		}
		p.Status = status
	}
	return *p
}

func TestSweepInterruptedEditorRendersFailsOnlyRenderingProjects(t *testing.T) {
	for name, repo := range newEditorSweeperRepos(t) {
		t.Run(name, func(t *testing.T) {
			store, err := storage.NewLocal(t.TempDir())
			if err != nil {
				t.Fatalf("NewLocal: %v", err)
			}
			cases := []struct {
				status     timelineplan.Status
				wantStatus timelineplan.Status
			}{
				{status: timelineplan.StatusDraft, wantStatus: timelineplan.StatusDraft},
				{status: timelineplan.StatusRendering, wantStatus: timelineplan.StatusFailed},
				{status: timelineplan.StatusRendered, wantStatus: timelineplan.StatusRendered},
				{status: timelineplan.StatusFailed, wantStatus: timelineplan.StatusFailed},
			}
			seeded := make([]timelineplan.Project, 0, len(cases))
			for _, tc := range cases {
				seeded = append(seeded, seedEditorProject(t, repo, tc.status))
			}

			swept, err := sweepInterruptedEditorRenders(context.Background(), repo, store, nil)
			if err != nil {
				t.Fatalf("sweepInterruptedEditorRenders: %v", err)
			}
			if swept != 1 {
				t.Fatalf("swept = %d, want 1", swept)
			}
			for i, tc := range cases {
				got, err := repo.Get(context.Background(), seeded[i].ID)
				if err != nil {
					t.Fatalf("Get(%s): %v", tc.status, err)
				}
				if got.Status != tc.wantStatus {
					t.Errorf("%s: status = %s, want %s", tc.status, got.Status, tc.wantStatus)
				}
				if tc.status == timelineplan.StatusRendering && got.FailureReason != interruptedEditorRenderReason {
					t.Errorf("failure reason = %q, want %q", got.FailureReason, interruptedEditorRenderReason)
				}
				var state timelineplan.RenderState
				found, readErr := readSweepJSON(store, timelineplan.RenderStateKey(seeded[i].ID), &state)
				if readErr != nil {
					t.Fatalf("read render state: %v", readErr)
				}
				if found != (tc.status == timelineplan.StatusRendering) {
					t.Errorf("%s: render state written = %v", tc.status, found)
				}
				if found && (state.Status != timelineplan.StatusFailed || state.Error != interruptedEditorRenderReason) {
					t.Errorf("render state = %+v, want failed with interruption reason", state)
				}
			}
		})
	}
}

func TestSweepInterruptedEditorRendersKeepsPublishedRevisionKeys(t *testing.T) {
	repo := store.NewMemoryEditorProjectRepository()
	store, err := storage.NewLocal(t.TempDir())
	if err != nil {
		t.Fatalf("NewLocal: %v", err)
	}
	p := seedEditorProject(t, repo, timelineplan.StatusRendering)
	previous := timelineplan.RenderState{
		ProjectID:   p.ID,
		AttemptID:   uuid.New(),
		Status:      timelineplan.StatusRendering,
		Fingerprint: "fp-1",
		VideoKey:    "editor-jobs/x/renders/revisions/r1/final.mp4",
		CoverKey:    "editor-jobs/x/renders/revisions/r1/cover.jpg",
		ResultKey:   "editor-jobs/x/renders/revisions/r1/render-result.json",
		UpdatedAt:   time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC),
	}
	if err := writeSweepJSON(store, timelineplan.RenderStateKey(p.ID), previous); err != nil {
		t.Fatalf("seed render state: %v", err)
	}

	if _, err := sweepInterruptedEditorRenders(context.Background(), repo, store, nil); err != nil {
		t.Fatalf("sweepInterruptedEditorRenders: %v", err)
	}
	var got timelineplan.RenderState
	if _, err := readSweepJSON(store, timelineplan.RenderStateKey(p.ID), &got); err != nil {
		t.Fatalf("read render state: %v", err)
	}
	if got.Status != timelineplan.StatusFailed || got.Error != interruptedEditorRenderReason {
		t.Fatalf("state = %+v, want failed with interruption reason", got)
	}
	if got.VideoKey != previous.VideoKey || got.CoverKey != previous.CoverKey || got.ResultKey != previous.ResultKey || got.Fingerprint != previous.Fingerprint {
		t.Fatalf("published revision keys not preserved: %+v", got)
	}
	if !got.UpdatedAt.After(previous.UpdatedAt) {
		t.Fatalf("UpdatedAt = %s, want after %s", got.UpdatedAt, previous.UpdatedAt)
	}
}
