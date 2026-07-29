package httpapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/rechedev9/fragforge/internal/editor"
	"github.com/rechedev9/fragforge/internal/job"
	"github.com/rechedev9/fragforge/internal/recording"
	"github.com/rechedev9/fragforge/internal/renderplan"
	"github.com/rechedev9/fragforge/internal/rules"
)

func TestReviewReplacementQueueFailureRestoresExactReview(t *testing.T) {
	tests := []struct {
		name       string
		queueError error
		discard    bool
		wantStart  int
	}{
		{
			name:       "rejected before admission",
			queueError: errors.New("inline queue is full"),
			wantStart:  http.StatusInternalServerError,
		},
		{
			name:      "discarded after admission",
			discard:   true,
			wantStart: http.StatusAccepted,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := newFakeRepo()
			store := newFakeStorage()
			countingStore := &renderReviewCountingStorage{fakeStorage: store}
			queue := &fakeQueue{err: tc.queueError}
			j := job.Job{ID: uuid.New(), Status: job.StatusRecorded, Rules: rules.Default()}
			repo.jobs[j.ID] = j
			h := NewHandlers(repo, countingStore, queue)
			variant := editor.PresetViral60Clean
			loadout, err := renderplan.LoadoutForVariant(variant)
			if err != nil {
				t.Fatal(err)
			}
			review, err := renderplan.NewRenderVariantStateForLoadout(renderplan.NewRenderVariantStateForLoadoutOptions{
				JobID:      j.ID,
				Loadout:    loadout,
				Status:     renderplan.RenderVariantStatusReview,
				Warnings:   []string{"freeze at 00:12"},
				RevisionID: uuid.New(),
			})
			if err != nil {
				t.Fatal(err)
			}
			if err := h.writeRenderVariantState(review); err != nil {
				t.Fatal(err)
			}
			putAssistantJSON(t, store, review.EditDocumentKey, renderplan.EditDocument{
				SchemaVersion: renderplan.EditDocumentSchemaVersion,
				Edit:          renderplan.DefaultEditRequest(),
				Music:         &renderplan.MusicSnapshot{},
			})
			putAssistantJSON(t, store, review.RenderResultKey, editor.Result{
				Preset:   variant,
				Warnings: append([]string(nil), review.Warnings...),
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
			})
			putReadyPublishArtifacts(t, store, review, "seg-001")
			countingStore.putKeys = nil

			router := chi.NewRouter()
			router.Post("/api/jobs/{id}/renders/{variant}", h.StartRenderVariant)
			router.Post("/api/jobs/{id}/renders/{variant}/review", h.ResolveRenderReview)
			router.Get("/api/jobs/{id}/renders/{variant}/publish", h.GetRenderPublishBoard)
			renderPath := "/api/jobs/" + j.ID.String() + "/renders/" + variant
			startBody := fmt.Sprintf(
				`{"expected_artifact_prefix":%q,"expected_warnings":["freeze at 00:12"],"edit":{"transition":"whip"}}`,
				review.ArtifactPrefix,
			)
			req := httptest.NewRequest(http.MethodPost, renderPath, strings.NewReader(startBody))
			req.Header.Set("Content-Type", "application/json")
			rw := httptest.NewRecorder()
			router.ServeHTTP(rw, req)
			if rw.Code != tc.wantStart {
				t.Fatalf("start status = %d, want %d; body=%s", rw.Code, tc.wantStart, rw.Body.String())
			}
			if tc.queueError != nil && len(countingStore.putKeys) != 0 {
				t.Fatalf("pre-admission rejection wrote %v, want no compensating Put", countingStore.putKeys)
			}
			if tc.discard {
				if len(queue.transitions) != 1 {
					t.Fatalf("queue transitions = %d, want 1", len(queue.transitions))
				}
				countingStore.putKeys = nil
				if err := queue.transitions[0](errors.New("inline queue task discarded during shutdown")); err != nil {
					t.Fatalf("discard transition error = %v", err)
				}
				if len(countingStore.putKeys) != 1 {
					t.Fatalf("discard compensation writes = %v, want one exact review restore", countingStore.putKeys)
				}
			}

			restored, exists, err := h.readRenderVariantState(j.ID, variant)
			if err != nil || !exists {
				t.Fatalf("restored state = (%#v, %v, %v)", restored, exists, err)
			}
			if !reflect.DeepEqual(*restored, review) {
				t.Fatalf("restored state = %#v, want exact prior review %#v", *restored, review)
			}

			resolveBody := fmt.Sprintf(
				`{"note":"intentional hold","expected_artifact_prefix":%q,"expected_warnings":["freeze at 00:12"]}`,
				review.ArtifactPrefix,
			)
			req = httptest.NewRequest(http.MethodPost, renderPath+"/review", strings.NewReader(resolveBody))
			req.Header.Set("Content-Type", "application/json")
			rw = httptest.NewRecorder()
			router.ServeHTTP(rw, req)
			if rw.Code != http.StatusOK {
				t.Fatalf("review status = %d, want 200; body=%s", rw.Code, rw.Body.String())
			}

			req = httptest.NewRequest(http.MethodGet, renderPath+"/publish", nil)
			rw = httptest.NewRecorder()
			router.ServeHTTP(rw, req)
			if rw.Code != http.StatusOK ||
				!strings.Contains(rw.Body.String(), `"status":"ready"`) ||
				!strings.Contains(rw.Body.String(), `"render_ready":true`) {
				t.Fatalf("publish board after restored review = %d %s, want ready", rw.Code, rw.Body.String())
			}
		})
	}
}

func TestReviewReplacementDiscardDoesNotOverwriteAdvancedState(t *testing.T) {
	repo := newFakeRepo()
	store := newFakeStorage()
	queue := &fakeQueue{}
	j := job.Job{ID: uuid.New(), Status: job.StatusRecorded, Rules: rules.Default()}
	repo.jobs[j.ID] = j
	h := NewHandlers(repo, store, queue)
	variant := editor.PresetViral60Clean
	loadout, err := renderplan.LoadoutForVariant(variant)
	if err != nil {
		t.Fatal(err)
	}
	review, err := renderplan.NewRenderVariantStateForLoadout(renderplan.NewRenderVariantStateForLoadoutOptions{
		JobID:      j.ID,
		Loadout:    loadout,
		Status:     renderplan.RenderVariantStatusReview,
		Warnings:   []string{"freeze at 00:12"},
		RevisionID: uuid.New(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := h.writeRenderVariantState(review); err != nil {
		t.Fatal(err)
	}
	putAssistantJSON(t, store, review.EditDocumentKey, renderplan.EditDocument{
		SchemaVersion: renderplan.EditDocumentSchemaVersion,
		Edit:          renderplan.DefaultEditRequest(),
		Music:         &renderplan.MusicSnapshot{},
	})
	router := chi.NewRouter()
	router.Post("/api/jobs/{id}/renders/{variant}", h.StartRenderVariant)
	renderPath := "/api/jobs/" + j.ID.String() + "/renders/" + variant
	body := fmt.Sprintf(
		`{"expected_artifact_prefix":%q,"expected_warnings":["freeze at 00:12"],"edit":{"transition":"whip"}}`,
		review.ArtifactPrefix,
	)
	req := httptest.NewRequest(http.MethodPost, renderPath, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rw := httptest.NewRecorder()
	router.ServeHTTP(rw, req)
	if rw.Code != http.StatusAccepted || len(queue.transitions) != 1 {
		t.Fatalf("start = %d %s, transitions=%d; want accepted", rw.Code, rw.Body.String(), len(queue.transitions))
	}
	advanced, exists, err := h.readRenderVariantState(j.ID, variant)
	if err != nil || !exists {
		t.Fatalf("queued state = (%#v, %v, %v)", advanced, exists, err)
	}
	advanced.Status = renderplan.RenderVariantStatusRendering
	advanced.Error = ""
	if err := h.writeRenderVariantState(*advanced); err != nil {
		t.Fatal(err)
	}
	if err := queue.transitions[0](errors.New("late discard after worker ownership")); err != nil {
		t.Fatalf("late discard transition error = %v", err)
	}
	current, exists, err := h.readRenderVariantState(j.ID, variant)
	if err != nil || !exists || !reflect.DeepEqual(*current, *advanced) {
		t.Fatalf("late discard state = (%#v, %v, %v), want advanced state %#v", current, exists, err, *advanced)
	}
}

func TestPublishBoardFirstAccessMaterializesLegacyReviewToken(t *testing.T) {
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
			router.Post("/api/jobs/{id}/renders/{variant}/review", h.ResolveRenderReview)
			renderPath := "/api/jobs/" + j.ID.String() + "/renders/" + variant
			req := httptest.NewRequest(http.MethodGet, renderPath+"/publish", nil)
			rw := httptest.NewRecorder()
			router.ServeHTTP(rw, req)
			if rw.Code != http.StatusOK {
				t.Fatalf("publish status = %d, want 200; body=%s", rw.Code, rw.Body.String())
			}
			var board struct {
				Status                 string   `json:"status"`
				Warnings               []string `json:"warnings"`
				ExpectedArtifactPrefix string   `json:"expected_artifact_prefix"`
				ExpectedWarnings       []string `json:"expected_warnings"`
			}
			if err := json.Unmarshal(rw.Body.Bytes(), &board); err != nil {
				t.Fatal(err)
			}
			if board.Status != "review_required" ||
				board.ExpectedArtifactPrefix == "" ||
				!slices.Equal(board.Warnings, tc.wantWarnings) ||
				!slices.Equal(board.ExpectedWarnings, tc.wantWarnings) {
				t.Fatalf("publish board = %#v, want exact actionable review token", board)
			}
			materialized, exists, err := h.readRenderVariantState(j.ID, variant)
			if err != nil || !exists ||
				materialized.Status != renderplan.RenderVariantStatusReview ||
				materialized.ArtifactPrefix != board.ExpectedArtifactPrefix ||
				!slices.Equal(materialized.Warnings, board.ExpectedWarnings) {
				t.Fatalf("materialized state = (%#v, %v, %v), want board token", materialized, exists, err)
			}

			resolveBody, err := json.Marshal(map[string]any{
				"note":                     "reviewed from publish board",
				"expected_artifact_prefix": board.ExpectedArtifactPrefix,
				"expected_warnings":        board.ExpectedWarnings,
			})
			if err != nil {
				t.Fatal(err)
			}
			req = httptest.NewRequest(http.MethodPost, renderPath+"/review", bytes.NewReader(resolveBody))
			req.Header.Set("Content-Type", "application/json")
			rw = httptest.NewRecorder()
			router.ServeHTTP(rw, req)
			if rw.Code != http.StatusOK {
				t.Fatalf("review status = %d, want 200; body=%s", rw.Code, rw.Body.String())
			}

			req = httptest.NewRequest(http.MethodGet, renderPath+"/publish", nil)
			rw = httptest.NewRecorder()
			router.ServeHTTP(rw, req)
			if rw.Code != http.StatusOK ||
				!strings.Contains(rw.Body.String(), `"status":"ready"`) ||
				!strings.Contains(rw.Body.String(), `"render_ready":true`) {
				t.Fatalf("publish board after review = %d %s, want ready", rw.Code, rw.Body.String())
			}
		})
	}
}

func TestStartRenderVariantDirectPostMaterializesReviewBeforeCAS(t *testing.T) {
	const warning = "freeze at 00:12"
	for _, tc := range []struct {
		name       string
		body       func(renderplan.RenderVariantState) string
		wantStatus int
		wantQueued bool
	}{
		{
			name:       "missing review token is rejected",
			body:       func(renderplan.RenderVariantState) string { return `{}` },
			wantStatus: http.StatusConflict,
		},
		{
			name: "exact review token is accepted",
			body: func(state renderplan.RenderVariantState) string {
				return fmt.Sprintf(
					`{"expected_artifact_prefix":%q,"expected_warnings":[%q],"edit":{"transition":"whip"}}`,
					state.ArtifactPrefix,
					warning,
				)
			},
			wantStatus: http.StatusAccepted,
			wantQueued: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := newFakeRepo()
			store := newFakeStorage()
			queue := &fakeQueue{}
			j := job.Job{ID: uuid.New(), Status: job.StatusRecorded, Rules: rules.Default()}
			repo.jobs[j.ID] = j
			variant := editor.PresetViral60Clean
			loadout, err := renderplan.LoadoutForVariant(variant)
			if err != nil {
				t.Fatal(err)
			}
			legacyReady, err := renderplan.NewRenderVariantStateForLoadout(renderplan.NewRenderVariantStateForLoadoutOptions{
				JobID:   j.ID,
				Loadout: loadout,
				Status:  renderplan.RenderVariantStatusReady,
			})
			if err != nil {
				t.Fatal(err)
			}
			h := NewHandlers(repo, store, queue)
			if err := h.writeRenderVariantState(legacyReady); err != nil {
				t.Fatal(err)
			}
			putAssistantJSON(t, store, legacyReady.RenderResultKey, editor.Result{
				Preset:   variant,
				Warnings: []string{warning},
			})
			putAssistantJSON(t, store, legacyReady.EditDocumentKey, renderplan.EditDocument{
				SchemaVersion: renderplan.EditDocumentSchemaVersion,
				Edit:          renderplan.DefaultEditRequest(),
				Music:         &renderplan.MusicSnapshot{},
			})

			router := chi.NewRouter()
			router.Post("/api/jobs/{id}/renders/{variant}", h.StartRenderVariant)
			renderPath := "/api/jobs/" + j.ID.String() + "/renders/" + variant
			req := httptest.NewRequest(http.MethodPost, renderPath, strings.NewReader(tc.body(legacyReady)))
			req.Header.Set("Content-Type", "application/json")
			rw := httptest.NewRecorder()
			router.ServeHTTP(rw, req)
			if rw.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d; body=%s", rw.Code, tc.wantStatus, rw.Body.String())
			}

			state, exists, err := h.readRenderVariantState(j.ID, variant)
			if err != nil || !exists {
				t.Fatalf("state after direct POST = (%#v, %v, %v)", state, exists, err)
			}
			if tc.wantQueued {
				if state.Status != renderplan.RenderVariantStatusQueued || len(queue.enqueued) != 1 {
					t.Fatalf("accepted correction = state %#v, enqueued %d; want queued once", state, len(queue.enqueued))
				}
				return
			}
			if state.Status != renderplan.RenderVariantStatusReview ||
				state.ArtifactPrefix != legacyReady.ArtifactPrefix ||
				!slices.Equal(state.Warnings, []string{warning}) {
				t.Fatalf("rejected correction state = %#v, want exact materialized review", state)
			}
			if len(queue.enqueued) != 0 {
				t.Fatalf("rejected correction enqueued %d tasks, want 0", len(queue.enqueued))
			}
		})
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

type renderReviewCountingStorage struct {
	*fakeStorage
	putKeys []string
}

func (s *renderReviewCountingStorage) Put(key string, r io.Reader) error {
	s.putKeys = append(s.putKeys, key)
	return s.fakeStorage.Put(key, r)
}
