package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/editor"
	"github.com/rechedev9/cliphub/internal/job"
	"github.com/rechedev9/cliphub/internal/recording"
	"github.com/rechedev9/cliphub/internal/renderplan"
	"github.com/rechedev9/cliphub/internal/rules"
)

func TestPublishBoardTreatsWarningsAsInformational(t *testing.T) {
	tests := []struct {
		name         string
		readyState   bool
		result       editor.Result
		wantWarnings []string
	}{
		{
			name: "legacy result without state",
			result: editor.Result{
				Preset:   editor.PresetViral60Clean,
				Warnings: []string{"freeze at 00:12"},
				Shorts: []editor.ShortResult{{
					SegmentID:    "seg-001",
					OutputFormat: editor.OutputFormatShort9x16,
					PublishArtifact: recording.RecordingArtifact{
						Path:      "seg-001.mp4",
						SizeBytes: 10,
						Width:     1080,
						Height:    1920,
					},
				}},
			},
			wantWarnings: []string{"freeze at 00:12"},
		},
		{
			name:       "ready state with nested artifact warning",
			readyState: true,
			result: editor.Result{
				Preset: editor.PresetViral60Clean,
				Shorts: []editor.ShortResult{{
					SegmentID:    "seg-001",
					OutputFormat: editor.OutputFormatShort9x16,
					PublishArtifact: recording.RecordingArtifact{
						Path:      "seg-001.mp4",
						SizeBytes: 10,
						Width:     720,
						Height:    1280,
					},
				}},
			},
			wantWarnings: []string{"quality seg-001: unexpected_output_resolution"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := newFakeRepo()
			store := newFakeStorage()
			j := job.Job{ID: uuid.New(), Status: job.StatusRecorded, Rules: rules.Default()}
			repo.jobs[j.ID] = j
			h := NewHandlers(repo, store, &fakeQueue{})
			variant := editor.PresetViral60Clean
			loadout, err := renderplan.LoadoutForVariant(variant)
			if err != nil {
				t.Fatal(err)
			}
			state, err := renderplan.NewRenderVariantStateForLoadout(renderplan.NewRenderVariantStateForLoadoutOptions{
				JobID:   j.ID,
				Loadout: loadout,
				Status:  renderplan.RenderVariantStatusReady,
			})
			if err != nil {
				t.Fatal(err)
			}
			if tc.readyState {
				state, err = renderplan.NewRenderVariantStateForLoadout(renderplan.NewRenderVariantStateForLoadoutOptions{
					JobID:      j.ID,
					Loadout:    loadout,
					Status:     renderplan.RenderVariantStatusReady,
					RevisionID: uuid.New(),
				})
				if err != nil {
					t.Fatal(err)
				}
				if err := h.writeRenderVariantState(state); err != nil {
					t.Fatal(err)
				}
			}
			putAssistantJSON(t, store, state.RenderResultKey, tc.result)
			putReadyPublishArtifacts(t, store, state, "seg-001")

			router := chi.NewRouter()
			router.Get("/api/jobs/{id}/renders/{variant}/publish", h.GetRenderPublishBoard)
			req := httptest.NewRequest(http.MethodGet, "/api/jobs/"+j.ID.String()+"/renders/"+variant+"/publish", nil)
			rw := httptest.NewRecorder()
			router.ServeHTTP(rw, req)
			if rw.Code != http.StatusOK {
				t.Fatalf("publish status = %d, want 200; body=%s", rw.Code, rw.Body.String())
			}
			var board struct {
				Status      string   `json:"status"`
				RenderReady bool     `json:"render_ready"`
				Warnings    []string `json:"warnings"`
			}
			if err := json.Unmarshal(rw.Body.Bytes(), &board); err != nil {
				t.Fatal(err)
			}
			if board.Status != "ready" || !board.RenderReady || !slices.Equal(board.Warnings, tc.wantWarnings) {
				t.Fatalf("publish board = %#v, want ready with warnings %v", board, tc.wantWarnings)
			}
			materialized, exists, err := h.readRenderVariantState(j.ID, variant)
			if err != nil {
				t.Fatal(err)
			}
			if !tc.readyState {
				if exists {
					t.Fatalf("legacy result without state grew a state document: %#v", materialized)
				}
				return
			}
			if !exists ||
				materialized.Status != renderplan.RenderVariantStatusReady ||
				!slices.Equal(materialized.Warnings, tc.wantWarnings) {
				t.Fatalf("materialized state = (%#v, %v), want ready with synced warnings", materialized, exists)
			}
		})
	}
}

// The startup pass settles legacy render state so the request path is a read:
// a stored review_required state is promoted to ready on the first run, and
// the second run finds nothing to write.
func TestMaterializeRenderVariantStatesPromotesLegacyReviewOnceAtStartup(t *testing.T) {
	repo := newFakeRepo()
	store := newFakeStorage()
	j := job.Job{ID: uuid.New(), Status: job.StatusRecorded, Rules: rules.Default()}
	repo.jobs[j.ID] = j
	untouched := job.Job{ID: uuid.New(), Status: job.StatusParsed, Rules: rules.Default()}
	repo.jobs[untouched.ID] = untouched
	h := NewHandlers(repo, store, &fakeQueue{})
	variant := editor.PresetViral60Clean
	loadout, err := renderplan.LoadoutForVariant(variant)
	if err != nil {
		t.Fatal(err)
	}
	legacy, err := renderplan.NewRenderVariantStateForLoadout(renderplan.NewRenderVariantStateForLoadoutOptions{
		JobID:    j.ID,
		Loadout:  loadout,
		Status:   renderplan.RenderVariantStatusReview,
		Warnings: []string{"freeze at 00:12"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := h.writeRenderVariantState(legacy); err != nil {
		t.Fatal(err)
	}
	putAssistantJSON(t, store, legacy.RenderResultKey, editor.Result{
		Preset:   variant,
		Warnings: []string{"freeze at 00:12"},
		Shorts: []editor.ShortResult{{
			SegmentID:       "seg-001",
			OutputFormat:    editor.OutputFormatShort9x16,
			PublishArtifact: recording.RecordingArtifact{Path: "seg-001.mp4", SizeBytes: 10, Width: 1080, Height: 1920},
		}},
	})
	putReadyPublishArtifacts(t, store, legacy, "seg-001")

	jobs := []job.Job{j, untouched}
	migrated, err := h.MaterializeRenderVariantStates(context.Background(), jobs)
	if err != nil {
		t.Fatalf("MaterializeRenderVariantStates: %v", err)
	}
	if migrated != 1 {
		t.Fatalf("migrated = %d, want 1", migrated)
	}
	state, exists, err := h.readRenderVariantState(j.ID, variant)
	if err != nil || !exists {
		t.Fatalf("render state after startup pass: exists=%v err=%v", exists, err)
	}
	if state.Status != renderplan.RenderVariantStatusReady || !slices.Equal(state.Warnings, []string{"freeze at 00:12"}) {
		t.Fatalf("state = %#v, want ready with the legacy warning", state)
	}
	if _, exists, err := h.readRenderVariantState(untouched.ID, variant); err != nil || exists {
		t.Fatalf("job without a render grew a state file: exists=%v err=%v", exists, err)
	}

	writesBefore := len(store.puts)
	migrated, err = h.MaterializeRenderVariantStates(context.Background(), jobs)
	if err != nil || migrated != 0 {
		t.Fatalf("second pass migrated = %d err = %v, want 0 and nil", migrated, err)
	}
	if len(store.puts) != writesBefore {
		t.Fatalf("second pass wrote %d new artifacts; startup materialization must be idempotent", len(store.puts)-writesBefore)
	}
}

func putReadyPublishArtifacts(
	t *testing.T,
	store *fakeStorage,
	state renderplan.RenderVariantState,
	segmentID string,
) {
	t.Helper()
	for _, kind := range []renderplan.RenderVariantArtifactKind{
		renderplan.RenderVariantArtifactVideo,
		renderplan.RenderVariantArtifactCaption,
	} {
		ref, err := renderplan.NewRenderVariantArtifactRefForState(state, kind, segmentID)
		if err != nil {
			t.Fatal(err)
		}
		if err := store.Put(ref.Key, strings.NewReader("artifact")); err != nil {
			t.Fatal(err)
		}
	}
}
