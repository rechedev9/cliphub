package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/hibiken/asynq"

	"github.com/rechedev9/cliphub/internal/artifacts"
	"github.com/rechedev9/cliphub/internal/editor"
	"github.com/rechedev9/cliphub/internal/job"
	"github.com/rechedev9/cliphub/internal/killplan"
	"github.com/rechedev9/cliphub/internal/renderplan"
	"github.com/rechedev9/cliphub/internal/rules"
	"github.com/rechedev9/cliphub/internal/tasks"
)

func generateRouter(h *Handlers) chi.Router {
	r := chi.NewRouter()
	r.Post("/api/jobs/{id}/generate", h.StartGenerate)
	return r
}

func postGenerate(t *testing.T, h *Handlers, id uuid.UUID, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/jobs/"+id.String()+"/generate", strings.NewReader(body))
	rw := httptest.NewRecorder()
	generateRouter(h).ServeHTTP(rw, req)
	return rw
}

type failingGenerateUpdateRepo struct {
	*fakeRepo
	err error
}

func (r failingGenerateUpdateRepo) UpdateStatus(context.Context, uuid.UUID, job.Status, string) error {
	return r.err
}

func TestStartGenerateEnqueuesRecordAndWritesIntent(t *testing.T) {
	repo := newFakeRepo()
	store := newFakeStorage()
	queue := &fakeQueue{}
	plan := killplan.NewPlan()
	j := job.Job{ID: uuid.New(), Status: job.StatusParsed, Rules: rules.Default(), KillPlan: &plan}
	repo.jobs[j.ID] = j
	h := NewHandlers(repo, store, queue, WithCapabilities(Capabilities{RecordEnabled: true}))

	rw := postGenerate(t, h, j.ID, `{"preset":"clean-pov-60","music":"phonk-01","edit":{"format":"short-9x16","killEffect":"velocity","transition":"whip","intro":true}}`)

	if rw.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202; body=%s", rw.Code, rw.Body.String())
	}

	// Exactly one task is enqueued: the recording. The render is chained later by
	// the record worker, not enqueued here.
	if len(queue.enqueued) != 1 {
		t.Fatalf("enqueued = %d, want 1", len(queue.enqueued))
	}
	if got := queue.enqueued[0].Type(); got != tasks.TypeRecordDemo {
		t.Fatalf("task type = %q, want %q", got, tasks.TypeRecordDemo)
	}
	var payload tasks.RecordDemoPayload
	if err := json.Unmarshal(queue.enqueued[0].Payload(), &payload); err != nil {
		t.Fatalf("unmarshal record payload: %v", err)
	}
	// clean-pov-60 records HUD-less, so it resolves to the "clean" HUD.
	if payload.HUDMode != "clean" {
		t.Fatalf("HUDMode = %q, want clean", payload.HUDMode)
	}
	if len(queue.options) != 1 || !hasAsynqOption(queue.options[0], "Unique(") {
		t.Fatalf("enqueue options = %#v, want Unique option for dedup", queue.options)
	}
	if !hasAsynqOption(queue.options[0], "MaxRetry(0)") {
		t.Fatalf("enqueue options = %#v, want MaxRetry(0) so capture never auto-retries", queue.options)
	}

	// The intent is persisted so the record worker can chain the render.
	raw, ok := store.puts[artifacts.GenerateIntentKey(j.ID)]
	if !ok {
		t.Fatalf("generate intent not written; puts=%v", keysOf(store))
	}
	var intent renderplan.GenerateIntent
	if err := json.Unmarshal(raw, &intent); err != nil {
		t.Fatalf("unmarshal intent: %v", err)
	}
	want := renderplan.GenerateIntent{
		Variant:  "clean-pov-60",
		MusicKey: "phonk-01",
		Edit: renderplan.EditRequest{
			Format:        renderplan.FormatShort9x16,
			KillEffect:    renderplan.KillEffectVelocity,
			Transition:    renderplan.TransitionWhip,
			Intro:         true,
			CoverStrategy: renderplan.CoverStrategyGenerated,
		},
	}
	if intent.ActiveRunID == uuid.Nil || intent.AcceptedAt.IsZero() {
		t.Fatalf("active generate marker = run %s accepted %s, want populated", intent.ActiveRunID, intent.AcceptedAt)
	}
	want.ActiveRunID = intent.ActiveRunID
	want.AcceptedAt = intent.AcceptedAt
	if !reflect.DeepEqual(intent, want) {
		t.Fatalf("intent = %#v, want %#v", intent, want)
	}
	taskIntent, ok, err := tasks.GenerateIntentFromTask(queue.enqueued[0])
	if err != nil || !ok {
		t.Fatalf("GenerateIntentFromTask = (%#v, %v, %v)", taskIntent, ok, err)
	}
	if !reflect.DeepEqual(taskIntent, want) {
		t.Fatalf("task intent = %#v, want %#v", taskIntent, want)
	}
}

func TestStartGenerateDuplicatePreservesAcceptedIntent(t *testing.T) {
	repo := newFakeRepo()
	store := newFakeStorage()
	queue := &fakeQueue{err: asynq.ErrDuplicateTask}
	plan := killplan.NewPlan()
	j := job.Job{ID: uuid.New(), Status: job.StatusParsed, Rules: rules.Default(), KillPlan: &plan}
	repo.jobs[j.ID] = j
	h := NewHandlers(repo, store, queue, WithCapabilities(Capabilities{RecordEnabled: true}))
	existing := renderplan.GenerateIntent{
		Variant:     editor.PresetCleanPOV60,
		MusicKey:    "first-track",
		Edit:        renderplan.DefaultEditRequest(),
		ActiveRunID: uuid.New(),
	}
	if err := h.generateIntents.Begin(j.ID, existing, nil); err != nil {
		t.Fatalf("seed generate intent: %v", err)
	}

	rw := postGenerate(t, h, j.ID, `{"preset":"clean-pov-60","music":"second-track","edit":{"intro":true}}`)
	if rw.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202; body=%s", rw.Code, rw.Body.String())
	}
	if !strings.Contains(rw.Body.String(), `"duplicate":true`) {
		t.Fatalf("body missing duplicate marker: %s", rw.Body.String())
	}
	got, ok, err := h.readGenerateIntent(j.ID)
	if err != nil || !ok {
		t.Fatalf("readGenerateIntent = (%#v, %v, %v)", got, ok, err)
	}
	if !reflect.DeepEqual(got, existing) {
		t.Fatalf("intent = %#v, want preserved %#v", got, existing)
	}
}

func TestStartGenerateEnqueueFailureDoesNotPublishIntent(t *testing.T) {
	repo := newFakeRepo()
	store := newFakeStorage()
	queue := &fakeQueue{err: errors.New("inline queue is full")}
	plan := killplan.NewPlan()
	j := job.Job{ID: uuid.New(), Status: job.StatusParsed, Rules: rules.Default(), KillPlan: &plan}
	repo.jobs[j.ID] = j
	h := NewHandlers(repo, store, queue, WithCapabilities(Capabilities{RecordEnabled: true}))

	rw := postGenerate(t, h, j.ID, `{"preset":"viral-60-clean"}`)
	if rw.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500; body=%s", rw.Code, rw.Body.String())
	}
	if _, ok := store.puts[artifacts.GenerateIntentKey(j.ID)]; ok {
		t.Fatal("intent published for rejected generate task")
	}
}

func TestStartGenerateMarksAcceptedPendingJobFailedWhenQueueDiscardsIt(t *testing.T) {
	repo := newFakeRepo()
	store := newFakeStorage()
	queue := &fakeQueue{}
	plan := killplan.NewPlan()
	j := job.Job{ID: uuid.New(), Status: job.StatusParsed, Rules: rules.Default(), KillPlan: &plan}
	repo.jobs[j.ID] = j
	h := NewHandlers(repo, store, queue, WithCapabilities(Capabilities{RecordEnabled: true}))

	rw := postGenerate(t, h, j.ID, `{"preset":"viral-60-clean"}`)
	if rw.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202; body=%s", rw.Code, rw.Body.String())
	}
	if len(queue.transitions) != 1 {
		t.Fatalf("queue transitions = %d, want 1", len(queue.transitions))
	}
	if err := queue.transitions[0](errors.New("inline queue task discarded during shutdown")); err != nil {
		t.Fatalf("discard transition error = %v", err)
	}
	got := repo.jobs[j.ID]
	if got.Status != job.StatusFailed || !strings.Contains(got.FailureReason, "discarded during shutdown") {
		t.Fatalf("job after discard = status %s, reason %q; want failed discard reason", got.Status, got.FailureReason)
	}
	intent, ok, err := h.readGenerateIntent(j.ID)
	if err != nil || !ok {
		t.Fatalf("readGenerateIntent = (%#v, %v, %v)", intent, ok, err)
	}
	if intent.ActiveRunID != uuid.Nil {
		t.Fatalf("active run after discard = %s, want cleared", intent.ActiveRunID)
	}
}

func TestStartGenerateDiscardKeepsRecoveryMarkerWhenFailureStatusDoesNotPersist(t *testing.T) {
	base := newFakeRepo()
	store := newFakeStorage()
	queue := &fakeQueue{}
	plan := killplan.NewPlan()
	j := job.Job{ID: uuid.New(), Status: job.StatusParsed, Rules: rules.Default(), KillPlan: &plan}
	base.jobs[j.ID] = j
	repo := failingGenerateUpdateRepo{fakeRepo: base, err: errors.New("sqlite write failed")}
	h := NewHandlers(repo, store, queue, WithCapabilities(Capabilities{RecordEnabled: true}))

	rw := postGenerate(t, h, j.ID, `{"preset":"viral-60-clean"}`)
	if rw.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202; body=%s", rw.Code, rw.Body.String())
	}
	if got := len(queue.transitions); got != 1 {
		t.Fatalf("transitions = %d, want 1", got)
	}
	if err := queue.transitions[0](errors.New("shutdown discard")); err == nil {
		t.Fatal("discard transition error = nil, want durable status failure")
	}
	intent, ok, err := h.readGenerateIntent(j.ID)
	if err != nil || !ok {
		t.Fatalf("readGenerateIntent = (%#v, %v, %v)", intent, ok, err)
	}
	if intent.ActiveRunID == uuid.Nil {
		t.Fatal("discard cleared ActiveRunID without persisting failed job status")
	}
	if got := base.jobs[j.ID].Status; got != job.StatusParsed {
		t.Fatalf("job status = %s, want parsed after injected write failure", got)
	}
}

func TestStartGenerateRejectsOverlappingCaptureBeforeItCanReplaceIntent(t *testing.T) {
	repo := newFakeRepo()
	store := newFakeStorage()
	queue := &fakeQueue{}
	plan := killplan.NewPlan()
	j := job.Job{ID: uuid.New(), Status: job.StatusParsed, Rules: rules.Default(), KillPlan: &plan}
	repo.jobs[j.ID] = j
	h := NewHandlers(repo, store, queue, WithCapabilities(Capabilities{RecordEnabled: true}))

	firstResponse := postGenerate(t, h, j.ID, `{"preset":"clean-pov-60","music":"first-track"}`)
	if firstResponse.Code != http.StatusAccepted {
		t.Fatalf("first status = %d, want 202; body=%s", firstResponse.Code, firstResponse.Body.String())
	}
	secondResponse := postGenerate(t, h, j.ID, `{"preset":"viral-60-clean","music":"second-track"}`)
	if secondResponse.Code != http.StatusConflict {
		t.Fatalf("second status = %d, want 409; body=%s", secondResponse.Code, secondResponse.Body.String())
	}
	if len(queue.enqueued) != 1 {
		t.Fatalf("enqueued = %d, want only the first capture", len(queue.enqueued))
	}

	first, ok, err := tasks.GenerateIntentFromTask(queue.enqueued[0])
	if err != nil || !ok {
		t.Fatalf("first task intent = (%#v, %v, %v)", first, ok, err)
	}
	current, ok, err := h.readGenerateIntent(j.ID)
	if err != nil || !ok {
		t.Fatalf("current intent = (%#v, %v, %v)", current, ok, err)
	}
	if first.MusicKey != "first-track" || current.ActiveRunID != first.ActiveRunID || current.MusicKey != first.MusicKey {
		t.Fatalf("active intent changed after rejected overlap: task=%+v current=%+v", first, current)
	}
}

func TestStartGenerateRejectsWhileRenderStateIsActive(t *testing.T) {
	repo := newFakeRepo()
	store := newFakeStorage()
	queue := &fakeQueue{}
	plan := killplan.NewPlan()
	j := job.Job{ID: uuid.New(), Status: job.StatusRecorded, Rules: rules.Default(), KillPlan: &plan}
	repo.jobs[j.ID] = j
	h := NewHandlers(repo, store, queue, WithCapabilities(Capabilities{RecordEnabled: true}))
	loadout, err := renderplan.LoadoutForVariant(editor.PresetViral60Clean)
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
	if err := h.writeRenderVariantState(state); err != nil {
		t.Fatal(err)
	}

	rw := postGenerate(t, h, j.ID, `{"preset":"viral-60-clean"}`)
	if rw.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409; body=%s", rw.Code, rw.Body.String())
	}
	var response struct {
		Code string `json:"code"`
	}
	if err := json.Unmarshal(rw.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Code != generateWorkActive {
		t.Fatalf("code = %q, want %q", response.Code, generateWorkActive)
	}
	if len(queue.enqueued) != 0 {
		t.Fatalf("enqueued = %d, want 0 while render is active", len(queue.enqueued))
	}
	if _, ok := store.puts[artifacts.GenerateIntentKey(j.ID)]; ok {
		t.Fatal("active render rejection published a generate intent")
	}
}

func TestStartGenerateKillfeedCaptureProfile(t *testing.T) {
	tests := []struct {
		name                     string
		preset                   string
		format                   string
		wantHUD                  string
		wantPortraitSafeKillfeed bool
	}{
		{name: "kill feed vertical", preset: editor.PresetViral60Clean, format: renderplan.FormatShort9x16, wantHUD: "deathnotices", wantPortraitSafeKillfeed: true},
		{name: "full HUD vertical", preset: editor.PresetFullHUD60, format: renderplan.FormatShort9x16, wantHUD: "gameplay", wantPortraitSafeKillfeed: true},
		{name: "full HUD landscape", preset: editor.PresetFullHUD60, format: renderplan.FormatLandscape16x9, wantHUD: "gameplay"},
		{name: "clean POV vertical", preset: editor.PresetCleanPOV60, format: renderplan.FormatShort9x16, wantHUD: "clean"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := newFakeRepo()
			store := newFakeStorage()
			queue := &fakeQueue{}
			plan := killplan.NewPlan()
			j := job.Job{ID: uuid.New(), Status: job.StatusParsed, Rules: rules.Default(), KillPlan: &plan}
			repo.jobs[j.ID] = j
			h := NewHandlers(repo, store, queue, WithCapabilities(Capabilities{RecordEnabled: true}))

			body := fmt.Sprintf(`{"preset":%q,"edit":{"format":%q}}`, tc.preset, tc.format)
			rw := postGenerate(t, h, j.ID, body)
			if rw.Code != http.StatusAccepted {
				t.Fatalf("status = %d, want 202; body=%s", rw.Code, rw.Body.String())
			}
			var payload tasks.RecordDemoPayload
			if err := json.Unmarshal(queue.enqueued[0].Payload(), &payload); err != nil {
				t.Fatalf("unmarshal record payload: %v", err)
			}
			if payload.HUDMode != tc.wantHUD || payload.PortraitSafeKillfeed != tc.wantPortraitSafeKillfeed {
				t.Fatalf("record payload = %#v, want HUD %q portrait-safe %t", payload, tc.wantHUD, tc.wantPortraitSafeKillfeed)
			}
		})
	}
}

func TestStartGenerateRoundTripsBookendText(t *testing.T) {
	repo := newFakeRepo()
	store := newFakeStorage()
	queue := &fakeQueue{}
	plan := killplan.NewPlan()
	j := job.Job{ID: uuid.New(), Status: job.StatusParsed, Rules: rules.Default(), KillPlan: &plan}
	repo.jobs[j.ID] = j
	h := NewHandlers(repo, store, queue, WithCapabilities(Capabilities{RecordEnabled: true}))

	rw := postGenerate(t, h, j.ID, `{"preset":"clean-pov-60","edit":{"format":"short-9x16","killEffect":"velocity","transition":"whip","intro":true,"outro":true,"hook_text":true,"kill_counter":true,"intro_text":"Watch this ace","outro_text":"follow for more"}}`)

	if rw.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202; body=%s", rw.Code, rw.Body.String())
	}
	raw, ok := store.puts[artifacts.GenerateIntentKey(j.ID)]
	if !ok {
		t.Fatalf("generate intent not written; puts=%v", keysOf(store))
	}
	var intent renderplan.GenerateIntent
	if err := json.Unmarshal(raw, &intent); err != nil {
		t.Fatalf("unmarshal intent: %v", err)
	}
	if intent.Edit.IntroText != "Watch this ace" || intent.Edit.OutroText != "follow for more" {
		t.Fatalf("edit bookend text = %q / %q, want round-tripped custom text", intent.Edit.IntroText, intent.Edit.OutroText)
	}
	if !intent.Edit.HookText || !intent.Edit.KillCounter {
		t.Fatalf("edit automatic text = hook %v / counter %v, want true / true", intent.Edit.HookText, intent.Edit.KillCounter)
	}
}

func TestStartGenerateRejectsOverlongBookendText(t *testing.T) {
	repo := newFakeRepo()
	store := newFakeStorage()
	queue := &fakeQueue{}
	plan := killplan.NewPlan()
	j := job.Job{ID: uuid.New(), Status: job.StatusParsed, Rules: rules.Default(), KillPlan: &plan}
	repo.jobs[j.ID] = j
	h := NewHandlers(repo, store, queue, WithCapabilities(Capabilities{RecordEnabled: true}))

	body := fmt.Sprintf(`{"preset":"viral-60-clean","edit":{"intro_text":"%s"}}`, strings.Repeat("a", 81))
	rw := postGenerate(t, h, j.ID, body)

	if rw.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body=%s", rw.Code, rw.Body.String())
	}
	if len(queue.enqueued) != 0 {
		t.Fatalf("enqueued = %d, want 0", len(queue.enqueued))
	}
}

func TestStartGenerateRejectsUnknownPreset(t *testing.T) {
	repo := newFakeRepo()
	store := newFakeStorage()
	queue := &fakeQueue{}
	plan := killplan.NewPlan()
	j := job.Job{ID: uuid.New(), Status: job.StatusParsed, Rules: rules.Default(), KillPlan: &plan}
	repo.jobs[j.ID] = j
	h := NewHandlers(repo, store, queue, WithCapabilities(Capabilities{RecordEnabled: true}))

	rw := postGenerate(t, h, j.ID, `{"preset":"no-such-preset"}`)

	if rw.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body=%s", rw.Code, rw.Body.String())
	}
	if len(queue.enqueued) != 0 {
		t.Fatalf("enqueued = %d, want 0", len(queue.enqueued))
	}
	if _, ok := store.puts[artifacts.GenerateIntentKey(j.ID)]; ok {
		t.Fatal("intent written for a rejected request")
	}
}

func TestStartGeneratePersistsSegmentIDsIntoIntent(t *testing.T) {
	repo := newFakeRepo()
	store := newFakeStorage()
	queue := &fakeQueue{}
	plan := killplan.NewPlan()
	plan.Segments = []killplan.Segment{{ID: "seg-001"}, {ID: "seg-002"}, {ID: "seg-003"}}
	j := job.Job{ID: uuid.New(), Status: job.StatusParsed, Rules: rules.Default(), KillPlan: &plan}
	repo.jobs[j.ID] = j
	h := NewHandlers(repo, store, queue, WithCapabilities(Capabilities{RecordEnabled: true}))

	rw := postGenerate(t, h, j.ID, `{"preset":"viral-60-clean","segment_ids":["seg-001"]}`)
	if rw.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202; body=%s", rw.Code, rw.Body.String())
	}

	intent, ok, err := h.readGenerateIntent(j.ID)
	if err != nil || !ok {
		t.Fatalf("readGenerateIntent = (%#v, %v, %v)", intent, ok, err)
	}
	if len(intent.SegmentIDs) != 1 || intent.SegmentIDs[0] != "seg-001" {
		t.Fatalf("intent.SegmentIDs = %v, want [seg-001]", intent.SegmentIDs)
	}

	// The record task carries the same selection so the record worker can
	// chain a render scoped to exactly this segment (chainRender).
	taskIntent, ok, err := tasks.GenerateIntentFromTask(queue.enqueued[0])
	if err != nil || !ok {
		t.Fatalf("GenerateIntentFromTask = (%#v, %v, %v)", taskIntent, ok, err)
	}
	if len(taskIntent.SegmentIDs) != 1 || taskIntent.SegmentIDs[0] != "seg-001" {
		t.Fatalf("task intent.SegmentIDs = %v, want [seg-001]", taskIntent.SegmentIDs)
	}
}

func TestStartGenerateRejectsBadSegmentSelection(t *testing.T) {
	cases := []struct {
		name     string
		body     string
		wantText string
	}{
		{name: "unknown id", body: `{"preset":"viral-60-clean","segment_ids":["seg-404"]}`, wantText: `unknown segment id "seg-404"`},
		{name: "duplicate id", body: `{"preset":"viral-60-clean","segment_ids":["seg-001","seg-001"]}`, wantText: `duplicate segment id "seg-001"`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := newFakeRepo()
			store := newFakeStorage()
			queue := &fakeQueue{}
			plan := killplan.NewPlan()
			plan.Segments = []killplan.Segment{{ID: "seg-001"}}
			j := job.Job{ID: uuid.New(), Status: job.StatusParsed, Rules: rules.Default(), KillPlan: &plan}
			repo.jobs[j.ID] = j
			h := NewHandlers(repo, store, queue, WithCapabilities(Capabilities{RecordEnabled: true}))

			rw := postGenerate(t, h, j.ID, tc.body)

			if rw.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400; body=%s", rw.Code, rw.Body.String())
			}
			var body struct {
				Error string `json:"error"`
			}
			if err := json.Unmarshal(rw.Body.Bytes(), &body); err != nil {
				t.Fatalf("decode error body %s: %v", rw.Body.String(), err)
			}
			if body.Error != tc.wantText {
				t.Fatalf("error = %q, want %q", body.Error, tc.wantText)
			}
			if len(queue.enqueued) != 0 {
				t.Fatalf("enqueued = %d, want 0", len(queue.enqueued))
			}
			if _, ok := store.puts[artifacts.GenerateIntentKey(j.ID)]; ok {
				t.Fatal("intent written for a rejected request")
			}
		})
	}
}

func TestStartGenerateRejectsJobNotReady(t *testing.T) {
	repo := newFakeRepo()
	queue := &fakeQueue{}
	// A roster-scanned job has no kill plan yet, so it cannot record.
	j := job.Job{ID: uuid.New(), Status: job.StatusScanned, Rules: rules.Default()}
	repo.jobs[j.ID] = j
	h := NewHandlers(repo, newFakeStorage(), queue, WithCapabilities(Capabilities{RecordEnabled: true}))

	rw := postGenerate(t, h, j.ID, `{"preset":"viral-60-clean"}`)

	if rw.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409; body=%s", rw.Code, rw.Body.String())
	}
	if len(queue.enqueued) != 0 {
		t.Fatalf("enqueued = %d, want 0", len(queue.enqueued))
	}
}

func TestStartGenerateRejectsInvalidEdit(t *testing.T) {
	repo := newFakeRepo()
	queue := &fakeQueue{}
	plan := killplan.NewPlan()
	j := job.Job{ID: uuid.New(), Status: job.StatusParsed, Rules: rules.Default(), KillPlan: &plan}
	repo.jobs[j.ID] = j
	h := NewHandlers(repo, newFakeStorage(), queue, WithCapabilities(Capabilities{RecordEnabled: true}))

	rw := postGenerate(t, h, j.ID, `{"preset":"viral-60-clean","edit":{"killEffect":"explode"}}`)

	if rw.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body=%s", rw.Code, rw.Body.String())
	}
	if len(queue.enqueued) != 0 {
		t.Fatalf("enqueued = %d, want 0", len(queue.enqueued))
	}
}

func TestStartGenerateRejectsBadMusicKey(t *testing.T) {
	repo := newFakeRepo()
	store := newFakeStorage()
	queue := &fakeQueue{}
	plan := killplan.NewPlan()
	j := job.Job{ID: uuid.New(), Status: job.StatusParsed, Rules: rules.Default(), KillPlan: &plan}
	repo.jobs[j.ID] = j
	h := NewHandlers(repo, store, queue, WithCapabilities(Capabilities{RecordEnabled: true}))

	rw := postGenerate(t, h, j.ID, `{"preset":"viral-60-clean","music":"../evil"}`)

	if rw.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body=%s", rw.Code, rw.Body.String())
	}
	if len(queue.enqueued) != 0 {
		t.Fatalf("enqueued = %d, want 0", len(queue.enqueued))
	}
	if _, ok := store.puts[artifacts.GenerateIntentKey(j.ID)]; ok {
		t.Fatal("intent written for a rejected request")
	}
}

func TestStartGeneratePreservesMusicMixForChainedRender(t *testing.T) {
	repo := newFakeRepo()
	store := newFakeStorage()
	queue := &fakeQueue{}
	plan := killplan.NewPlan()
	j := job.Job{ID: uuid.New(), Status: job.StatusParsed, Rules: rules.Default(), KillPlan: &plan}
	repo.jobs[j.ID] = j
	h := NewHandlers(repo, store, queue, WithCapabilities(Capabilities{RecordEnabled: true}))

	rw := postGenerate(t, h, j.ID, `{"preset":"viral-60-clean","music":{"key":"phonk-01","volume":0.35,"game_volume":0.2}}`)
	if rw.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202; body=%s", rw.Code, rw.Body.String())
	}
	intent, ok, err := tasks.GenerateIntentFromTask(queue.enqueued[0])
	if err != nil || !ok {
		t.Fatalf("GenerateIntentFromTask = (%#v, %v, %v)", intent, ok, err)
	}
	if intent.MusicKey != "phonk-01" || intent.MusicVolume != 0.35 || intent.GameVolume == nil || *intent.GameVolume != 0.2 {
		t.Fatalf("music mix = %q/%v/%v, want phonk-01/0.35/0.2", intent.MusicKey, intent.MusicVolume, intent.GameVolume)
	}
}

func TestStartGenerateReusesParsedDemoFromCompletedStatuses(t *testing.T) {
	statuses := []job.Status{job.StatusRecorded, job.StatusComposed, job.StatusDone, job.StatusReviewRequired}
	for _, status := range statuses {
		t.Run(status.String(), func(t *testing.T) {
			repo := newFakeRepo()
			queue := &fakeQueue{}
			plan := killplan.NewPlan()
			j := job.Job{ID: uuid.New(), Status: status, Rules: rules.Default(), KillPlan: &plan}
			repo.jobs[j.ID] = j
			h := NewHandlers(repo, newFakeStorage(), queue, WithCapabilities(Capabilities{RecordEnabled: true}))

			rw := postGenerate(t, h, j.ID, `{"preset":"viral-60-clean"}`)
			if rw.Code != http.StatusAccepted {
				t.Fatalf("status = %d, want 202; body=%s", rw.Code, rw.Body.String())
			}
			if len(queue.enqueued) != 1 || queue.enqueued[0].Type() != tasks.TypeRecordDemo {
				t.Fatalf("queue = %#v, want one cache-aware record task", queue.enqueued)
			}
		})
	}
}

func keysOf(s *fakeStorage) []string {
	out := make([]string, 0, len(s.puts))
	for k := range s.puts {
		out = append(out, k)
	}
	return out
}

func TestWorkbenchGenerateAdapterEnqueuesAndShowsProgress(t *testing.T) {
	repo := newFakeRepo()
	store := newFakeStorage()
	queue := &fakeQueue{}
	plan := killplan.NewPlan()
	plan.Demo.Map = "de_nuke"
	plan.Segments = []killplan.Segment{{ID: "seg-001", TickStart: 1, TickEnd: 2}}
	j := job.Job{ID: uuid.New(), Status: job.StatusParsed, Rules: rules.Default(), KillPlan: &plan}
	repo.jobs[j.ID] = j
	h := NewHandlers(repo, store, queue, WithCapabilities(Capabilities{RecordEnabled: true}))
	r := Routes(h)

	form := "preset=clean-pov-60&music=&format=short-9x16&kill_effect=punch-in&transition=flash&intro=on"
	req := httptest.NewRequest(http.MethodPost, "/ui/jobs/"+j.ID.String()+"/generate", strings.NewReader(form))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rw := httptest.NewRecorder()
	r.ServeHTTP(rw, req)

	if rw.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rw.Code, rw.Body.String())
	}
	if len(queue.enqueued) != 1 || queue.enqueued[0].Type() != tasks.TypeRecordDemo {
		t.Fatalf("queue = %#v, want one record task", queue.enqueued)
	}
	var payload tasks.RecordDemoPayload
	if err := json.Unmarshal(queue.enqueued[0].Payload(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.HUDMode != "clean" {
		t.Fatalf("HUDMode = %q, want clean for clean-pov-60", payload.HUDMode)
	}
	if _, ok := store.puts[artifacts.GenerateIntentKey(j.ID)]; !ok {
		t.Fatal("generate intent not written")
	}
	// The returned fragment shows the unified generating state and self-polls.
	body := rw.Body.String()
	for _, want := range []string{"Starting capture", `hx-trigger="every 3s"`} {
		if !strings.Contains(body, want) {
			t.Fatalf("fragment missing %q: %s", want, body)
		}
	}
}

func TestWorkbenchNewGenerateIgnoresReadyStateFromPriorRun(t *testing.T) {
	repo := newFakeRepo()
	store := newFakeStorage()
	queue := &fakeQueue{}
	plan := killplan.NewPlan()
	plan.Segments = []killplan.Segment{{ID: "seg-001", TickStart: 1, TickEnd: 2}}
	j := job.Job{ID: uuid.New(), Status: job.StatusRecorded, Rules: rules.Default(), KillPlan: &plan}
	repo.jobs[j.ID] = j
	h := NewHandlers(repo, store, queue, WithCapabilities(Capabilities{RecordEnabled: true}))
	loadout, err := renderplan.LoadoutForVariant(editor.PresetViral60Clean)
	if err != nil {
		t.Fatal(err)
	}
	ready, err := renderplan.NewRenderVariantStateForLoadout(renderplan.NewRenderVariantStateForLoadoutOptions{
		JobID:   j.ID,
		Loadout: loadout,
		Status:  renderplan.RenderVariantStatusReady,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := h.writeRenderVariantState(ready); err != nil {
		t.Fatal(err)
	}
	r := Routes(h)

	form := "preset=viral-60-clean&format=short-9x16&kill_effect=punch-in&transition=flash"
	req := httptest.NewRequest(http.MethodPost, "/ui/jobs/"+j.ID.String()+"/generate", strings.NewReader(form))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rw := httptest.NewRecorder()
	r.ServeHTTP(rw, req)

	if rw.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rw.Code, rw.Body.String())
	}
	if !strings.Contains(rw.Body.String(), "Starting capture") {
		t.Fatalf("new run did not replace prior ready phase: %s", rw.Body.String())
	}
	if strings.Contains(rw.Body.String(), "Short ready") {
		t.Fatalf("new run exposed prior ready state: %s", rw.Body.String())
	}
}

func TestWorkbenchShowsInlinePreviewWhenReady(t *testing.T) {
	repo := newFakeRepo()
	store := newFakeStorage()
	plan := killplan.NewPlan()
	plan.Segments = []killplan.Segment{{ID: "seg-001", TickStart: 1, TickEnd: 2}}
	j := job.Job{ID: uuid.New(), Status: job.StatusRecorded, Rules: rules.Default(), KillPlan: &plan}
	repo.jobs[j.ID] = j

	// The persisted choice points the view at the generated variant, and a
	// finished render result makes it "ready" and lists the short to preview.
	intentBytes, err := json.Marshal(renderplan.GenerateIntent{Variant: editor.PresetViral60Clean, Edit: renderplan.DefaultEditRequest()})
	if err != nil {
		t.Fatal(err)
	}
	_ = store.Put(artifacts.GenerateIntentKey(j.ID), bytes.NewReader(intentBytes))

	resultKey, err := artifacts.RenderVariantResultKey(j.ID, editor.PresetViral60Clean)
	if err != nil {
		t.Fatal(err)
	}
	result := editor.Result{
		Preset: editor.PresetViral60Clean,
		Shorts: []editor.ShortResult{{SegmentID: "seg-001", Title: "Ace on Nuke", DurationSeconds: 14, CoverPath: "cover.jpg"}},
	}
	resultBytes, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	_ = store.Put(resultKey, bytes.NewReader(resultBytes))

	h := NewHandlers(repo, store, &fakeQueue{})
	r := Routes(h)
	req := httptest.NewRequest(http.MethodGet, "/ui/jobs/"+j.ID.String(), nil)
	rw := httptest.NewRecorder()
	r.ServeHTTP(rw, req)

	if rw.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rw.Code, rw.Body.String())
	}
	body := rw.Body.String()
	videoSrc := "/api/jobs/" + j.ID.String() + "/renders/" + editor.PresetViral60Clean + "/videos/seg-001"
	for _, want := range []string{"<video", videoSrc, "Ace on Nuke", "Download", "Short ready"} {
		if !strings.Contains(body, want) {
			t.Fatalf("preview missing %q: %s", want, body)
		}
	}
}

// uniqueScopeQueue dedupes the way the inline queue does after the move to
// tasks.UniqueScope: one record task per job regardless of payload, with the
// rejected transition applied before the duplicate error is returned.
type uniqueScopeQueue struct {
	fakeQueue
	mu   sync.Mutex
	seen map[string]struct{}
}

func (q *uniqueScopeQueue) EnqueueWithTransition(t *asynq.Task, transition func(error) error, opts ...asynq.Option) (*asynq.TaskInfo, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.seen == nil {
		q.seen = map[string]struct{}{}
	}
	if key, ok := tasks.UniqueScope(t); ok {
		if _, dup := q.seen[key]; dup {
			if transition != nil {
				if err := transition(asynq.ErrDuplicateTask); err != nil {
					return nil, fmt.Errorf("apply rejected inline queue transition after %v: %w", asynq.ErrDuplicateTask, err)
				}
			}
			return nil, asynq.ErrDuplicateTask
		}
		q.seen[key] = struct{}{}
	}
	return q.fakeQueue.enqueue(t, transition, opts...)
}

// With job-scoped record uniqueness the queue answers duplicate before the
// intent store can refuse an overlapping run. A re-drive of the same reel is
// still a 202 duplicate; a different reel on the same job must get the 409 the
// client already knows how to wait on, not adopt the queued reel's choices.
func TestStartGenerateRefusesDifferentIntentBehindQueuedCapture(t *testing.T) {
	repo := newFakeRepo()
	store := newFakeStorage()
	queue := &uniqueScopeQueue{}
	plan := killplan.NewPlan()
	j := job.Job{ID: uuid.New(), Status: job.StatusParsed, Rules: rules.Default(), KillPlan: &plan}
	repo.jobs[j.ID] = j
	h := NewHandlers(repo, store, queue, WithCapabilities(Capabilities{RecordEnabled: true}))

	first := postGenerate(t, h, j.ID, `{"preset":"clean-pov-60","music":"first-track"}`)
	if first.Code != http.StatusAccepted {
		t.Fatalf("first status = %d, want 202; body=%s", first.Code, first.Body.String())
	}
	redrive := postGenerate(t, h, j.ID, `{"preset":"clean-pov-60","music":"first-track"}`)
	if redrive.Code != http.StatusAccepted || !strings.Contains(redrive.Body.String(), `"duplicate":true`) {
		t.Fatalf("same-intent re-drive = %d %s, want 202 duplicate", redrive.Code, redrive.Body.String())
	}
	other := postGenerate(t, h, j.ID, `{"preset":"viral-60-clean","music":"second-track"}`)
	if other.Code != http.StatusConflict || !strings.Contains(other.Body.String(), generateWorkActive) {
		t.Fatalf("different-intent status = %d body=%s, want 409 %s", other.Code, other.Body.String(), generateWorkActive)
	}
	if len(queue.enqueued) != 1 {
		t.Fatalf("enqueued = %d, want only the first capture", len(queue.enqueued))
	}
	current, ok, err := h.readGenerateIntent(j.ID)
	if err != nil || !ok {
		t.Fatalf("current intent = (%#v, %v, %v)", current, ok, err)
	}
	if current.Variant != "clean-pov-60" || current.MusicKey != "first-track" {
		t.Fatalf("active intent changed after refused overlap: %+v", current)
	}
}

// A queued record:demo without a generate header (or stored intent) is not a
// generate admission. Per-job uniqueness still answers ErrDuplicateTask, but
// 202 would tell the client the render will chain when capture will just end.
func TestStartGenerateDuplicateWithoutIntentIsConflict(t *testing.T) {
	repo := newFakeRepo()
	store := newFakeStorage()
	queue := &fakeQueue{err: asynq.ErrDuplicateTask}
	plan := killplan.NewPlan()
	j := job.Job{ID: uuid.New(), Status: job.StatusParsed, Rules: rules.Default(), KillPlan: &plan}
	repo.jobs[j.ID] = j
	h := NewHandlers(repo, store, queue, WithCapabilities(Capabilities{RecordEnabled: true}))

	rw := postGenerate(t, h, j.ID, `{"preset":"viral-60-clean"}`)
	if rw.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409; body=%s", rw.Code, rw.Body.String())
	}
	var response struct {
		Code string `json:"code"`
	}
	if err := json.Unmarshal(rw.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Code != generateWorkActive {
		t.Fatalf("code = %q, want %q", response.Code, generateWorkActive)
	}
	if strings.Contains(rw.Body.String(), `"duplicate":true`) {
		t.Fatalf("body claimed generate admission: %s", rw.Body.String())
	}
	if _, ok := store.puts[artifacts.GenerateIntentKey(j.ID)]; ok {
		t.Fatal("duplicate-without-intent published a generate intent")
	}
}

func TestStartGenerateRefusesPlainQueuedCaptureWithoutIntent(t *testing.T) {
	repo := newFakeRepo()
	store := newFakeStorage()
	queue := &uniqueScopeQueue{}
	plan := killplan.NewPlan()
	j := job.Job{ID: uuid.New(), Status: job.StatusParsed, Rules: rules.Default(), KillPlan: &plan}
	repo.jobs[j.ID] = j
	h := NewHandlers(repo, store, queue, WithCapabilities(Capabilities{RecordEnabled: true}))

	r := chi.NewRouter()
	r.Post("/api/jobs/{id}/record", h.StartRecording)
	record := httptest.NewRecorder()
	r.ServeHTTP(record, httptest.NewRequest(http.MethodPost, "/api/jobs/"+j.ID.String()+"/record", nil))
	if record.Code != http.StatusAccepted {
		t.Fatalf("record status = %d, want 202; body=%s", record.Code, record.Body.String())
	}
	// Capture uniqueness outlives the recording claim. A later generate that
	// sees a completed-looking job still collides with the queued record:demo.
	current := repo.jobs[j.ID]
	current.Status = job.StatusRecorded
	repo.jobs[j.ID] = current

	rw := postGenerate(t, h, j.ID, `{"preset":"viral-60-clean"}`)
	if rw.Code != http.StatusConflict || !strings.Contains(rw.Body.String(), generateWorkActive) {
		t.Fatalf("generate status = %d body=%s, want 409 %s", rw.Code, rw.Body.String(), generateWorkActive)
	}
	if len(queue.enqueued) != 1 {
		t.Fatalf("enqueued = %d, want only the plain record", len(queue.enqueued))
	}
	if _, ok, err := tasks.GenerateIntentFromTask(queue.enqueued[0]); err != nil || ok {
		t.Fatalf("queued record carried a generate header: ok=%v err=%v", ok, err)
	}
	if _, ok := store.puts[artifacts.GenerateIntentKey(j.ID)]; ok {
		t.Fatal("plain-record collision published a generate intent")
	}
}

// Finish leaves the latest generate choice on disk with ActiveRunID cleared.
// A later /generate that collides with a plain queued record:demo and happens
// to match that leftover capture is still not a generate admission: the plain
// capture will not chain a render.
func TestStartGenerateRefusesStaleIntentBehindPlainQueuedCapture(t *testing.T) {
	repo := newFakeRepo()
	store := newFakeStorage()
	queue := &uniqueScopeQueue{}
	plan := killplan.NewPlan()
	j := job.Job{ID: uuid.New(), Status: job.StatusParsed, Rules: rules.Default(), KillPlan: &plan}
	repo.jobs[j.ID] = j
	h := NewHandlers(repo, store, queue, WithCapabilities(Capabilities{RecordEnabled: true}))

	stale := renderplan.GenerateIntent{
		Variant: editor.PresetViral60Clean,
		Edit:    renderplan.DefaultEditRequest(),
	}
	if err := h.generateIntents.Begin(j.ID, stale, nil); err != nil {
		t.Fatalf("seed stale generate intent: %v", err)
	}

	r := chi.NewRouter()
	r.Post("/api/jobs/{id}/record", h.StartRecording)
	record := httptest.NewRecorder()
	r.ServeHTTP(record, httptest.NewRequest(http.MethodPost, "/api/jobs/"+j.ID.String()+"/record", nil))
	if record.Code != http.StatusAccepted {
		t.Fatalf("record status = %d, want 202; body=%s", record.Code, record.Body.String())
	}

	rw := postGenerate(t, h, j.ID, `{"preset":"viral-60-clean"}`)
	if rw.Code != http.StatusConflict || !strings.Contains(rw.Body.String(), generateWorkActive) {
		t.Fatalf("generate status = %d body=%s, want 409 %s", rw.Code, rw.Body.String(), generateWorkActive)
	}
	if strings.Contains(rw.Body.String(), `"duplicate":true`) {
		t.Fatalf("body claimed generate admission: %s", rw.Body.String())
	}
	if len(queue.enqueued) != 1 {
		t.Fatalf("enqueued = %d, want only the plain record", len(queue.enqueued))
	}
	if _, ok, err := tasks.GenerateIntentFromTask(queue.enqueued[0]); err != nil || ok {
		t.Fatalf("queued record carried a generate header: ok=%v err=%v", ok, err)
	}
	got, ok, err := h.readGenerateIntent(j.ID)
	if err != nil || !ok {
		t.Fatalf("stale intent = (%#v, %v, %v)", got, ok, err)
	}
	if got.ActiveRunID != uuid.Nil || got.Variant != editor.PresetViral60Clean {
		t.Fatalf("stale display intent mutated: %+v", got)
	}
}
