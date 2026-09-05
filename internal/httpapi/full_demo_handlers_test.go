package httpapi

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/rechedev9/cliphub/internal/artifacts"
	"github.com/rechedev9/cliphub/internal/job"
	"github.com/rechedev9/cliphub/internal/killplan"
	"github.com/rechedev9/cliphub/internal/recapplan"
	"github.com/rechedev9/cliphub/internal/renderplan"
	"github.com/rechedev9/cliphub/internal/tasks"
)

func fullDemoAPIFixture(t *testing.T) (*Handlers, job.Job, *fakeStorage, *fakeQueue, recapplan.Options) {
	t.Helper()
	store, repo, queue := newFakeStorage(), newFakeRepo(), &fakeQueue{}
	demo := []byte("contract-fixture-only-not-a-playable-demo")
	hash := sha256.Sum256(demo)
	digest := hex.EncodeToString(hash[:])
	id := uuid.New()
	kp := killplan.NewPlan()
	kp.Demo = killplan.Demo{SHA256: digest, Tickrate: 100, DurationTicks: 25000}
	kp.Target.SteamID64 = "76561198000000001"
	kp.Segments = []killplan.Segment{{ID: "seg-001", TickStart: 2000, TickEnd: 4000}}
	j := job.Job{ID: id, DemoPath: "jobs/" + id.String() + "/source.dem", TargetSteamID: kp.Target.SteamID64, Status: job.StatusParsed, KillPlan: &kp}
	repo.jobs[id] = j
	store.puts[j.DemoPath] = demo
	facts := recapplan.Facts{SchemaVersion: recapplan.DocumentVersion, DemoSHA256: digest, TargetSteamID64: j.TargetSteamID, ClockKind: recapplan.ClockIngame, TickRate: 100, EndTick: 25000, Complete: true, Warnings: []recapplan.Notice{}, Rounds: []recapplan.RoundFacts{
		{ID: "round-001", Number: 1, StartTick: 1000, FreezeEndTick: 2500, RoundEndTick: 9500, NextStartTick: 10000, Evidence: "round-events", Kills: []killplan.Kill{}, Utility: []killplan.UtilityThrow{}},
		{ID: "round-002", Number: 2, StartTick: 10000, FreezeEndTick: 11500, RoundEndTick: 24000, Evidence: "round-events", Kills: []killplan.Kill{}, Utility: []killplan.UtilityThrow{}},
	}}
	if err := recapplan.StoreFacts(store, id, facts); err != nil {
		t.Fatal(err)
	}
	o := recapplan.DefaultOptions()
	o.Capture.Crosshair.AllowCaptureDefault = true
	o.Audio.Voice.Enabled, o.Editorial.KeepFreezeVoice, o.Audio.Music.Enabled, o.Sponsor.Enabled = false, false, false, false
	o.Audio.Game.Gain, o.Audio.Voice.Gain = 0, 0
	h := NewHandlers(repo, store, queue)
	return h, j, store, queue, o
}

func fullDemoAPIRequest(t *testing.T, h *Handlers, j job.Job, endpoint string, body any) *httptest.ResponseRecorder {
	t.Helper()
	b, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	return fullDemoRawAPIRequest(h, j, endpoint, b)
}

func fullDemoRawAPIRequest(h *Handlers, j job.Job, endpoint string, b []byte) *httptest.ResponseRecorder {
	router := chi.NewRouter()
	router.Post("/api/jobs/{id}/full-demo/plan", h.PlanFullDemo)
	router.Post("/api/jobs/{id}/generate", h.StartGenerate)
	router.Post("/api/jobs/{id}/record", h.StartRecording)
	router.Post("/api/jobs/{id}/renders/{variant}", h.StartRenderVariant)
	router.Get("/api/jobs/{id}/full-demo/plan", h.GetFullDemoPlan)
	rw := httptest.NewRecorder()
	router.ServeHTTP(rw, httptest.NewRequest(http.MethodPost, "/api/jobs/"+j.ID.String()+endpoint, bytes.NewReader(b)))
	return rw
}

func TestFullDemoAdmissionRejectsNullAndConflictingSelections(t *testing.T) {
	for _, endpoint := range []string{"/generate", "/record", "/renders/gameplay-pov-60"} {
		for _, damage := range []string{"null profile", "duplicate profile", "unknown snapshot field", "missing approval", "legacy segment selection"} {
			t.Run(endpoint+"/"+damage, func(t *testing.T) {
				h, j, _, queue, options := fullDemoAPIFixture(t)
				h.capabilities.RecordEnabled = true
				snapshot := fullDemoAPIPlan(t, h, j, options)
				if strings.HasPrefix(endpoint, "/renders/") {
					j.Status = job.StatusRecorded
					h.repo.(*fakeRepo).jobs[j.ID] = j
				}
				b, err := json.Marshal(snapshot)
				if err != nil {
					t.Fatal(err)
				}
				profile := string(b)
				fields := `"full_demo":` + profile
				segments := ""
				switch damage {
				case "null profile":
					fields = `"full_demo":null`
				case "duplicate profile":
					fields += `,"full_demo":` + profile
				case "unknown snapshot field":
					fields = `"full_demo":{"invented":true,` + profile[1:]
				case "missing approval":
					b, _ = json.Marshal(snapshot.Document)
					fields = `"full_demo":{"document":` + string(b) + `}`
				case "legacy segment selection":
					edit, _ := json.Marshal(renderplan.FullDemoEditRequest(snapshot))
					fields = string(edit[1 : len(edit)-1])
					segments = `,"segment_ids":["seg-001"]`
				}
				body := []byte(`{"preset":"gameplay-pov-60","edit":{` + fields + `}` + segments + `}`)
				if strings.HasPrefix(endpoint, "/renders/") {
					body = bytes.Replace(body, []byte(`"preset":"gameplay-pov-60",`), nil, 1)
				}
				rw := fullDemoRawAPIRequest(h, j, endpoint, body)
				if rw.Code != http.StatusBadRequest || len(queue.enqueued) != 0 {
					t.Fatalf("code=%d enqueued=%d: %s", rw.Code, len(queue.enqueued), rw.Body.String())
				}
			})
		}
	}
}

func TestFullDemoActiveRecordDuplicateMatchesApproval(t *testing.T) {
	h, j, _, queue, o := fullDemoAPIFixture(t)
	h.capabilities.RecordEnabled = true
	first := fullDemoAPIPlan(t, h, j, o)
	o.Audio.Game.Gain = .5
	second := fullDemoAPIPlan(t, h, j, o)
	if rw := fullDemoAPIRequest(t, h, j, "/generate", map[string]any{"preset": "gameplay-pov-60", "edit": renderplan.FullDemoEditRequest(first)}); rw.Code != 202 {
		t.Fatal(rw.Body.String())
	}
	j.Status = job.StatusRecording
	h.repo.(*fakeRepo).jobs[j.ID] = j
	for _, tc := range []struct {
		name   string
		body   any
		status int
	}{
		{"same plan", map[string]any{"preset": "gameplay-pov-60", "edit": renderplan.FullDemoEditRequest(first)}, 202},
		{"changed plan", map[string]any{"preset": "gameplay-pov-60", "edit": renderplan.FullDemoEditRequest(second)}, 409},
		{"empty retry", map[string]any{}, 202},
		{"null profile", map[string]any{"edit": map[string]any{"full_demo": nil}}, 400},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rw := fullDemoAPIRequest(t, h, j, "/record", tc.body)
			if rw.Code != tc.status || len(queue.enqueued) != 1 {
				t.Fatalf("%d %s", rw.Code, rw.Body.String())
			}
		})
	}
}

func TestFullDemoRenderDiscardCannotOverwriteAdvancedState(t *testing.T) {
	for _, advanced := range []string{"rendering", "ready", "different approval", "still queued"} {
		t.Run(advanced, func(t *testing.T) {
			h, j, _, queue, o := fullDemoAPIFixture(t)
			first := fullDemoAPIPlan(t, h, j, o)
			o.Audio.Game.Gain = .5
			second := fullDemoAPIPlan(t, h, j, o)
			j.Status = job.StatusRecorded
			h.repo.(*fakeRepo).jobs[j.ID] = j
			if rw := fullDemoAPIRequest(t, h, j, "/renders/gameplay-pov-60", map[string]any{"edit": renderplan.FullDemoEditRequest(first)}); rw.Code != 202 {
				t.Fatal(rw.Body.String())
			}
			state, found, err := h.readRenderVariantState(j.ID, "gameplay-pov-60")
			if err != nil || !found {
				t.Fatalf("state: %v", err)
			}
			switch advanced {
			case "rendering", "ready":
				state.Status = advanced
			case "different approval":
				state.FullDemo = &second
			}
			if err := h.writeRenderVariantState(*state); err != nil {
				t.Fatal(err)
			}
			if err := queue.transitions[0](errors.New("discarded by shutdown")); err != nil {
				t.Fatal(err)
			}
			after, _, err := h.readRenderVariantState(j.ID, "gameplay-pov-60")
			if err != nil {
				t.Fatal(err)
			}
			if advanced == "still queued" {
				if after.Status != renderplan.RenderVariantStatusFailed || !renderplan.SameFullDemoRequest(after.FullDemo, &first) {
					t.Fatal("owned discard lost approval/failure")
				}
			} else if same, err := sameRenderVariantState(state, after); err != nil || !same {
				t.Fatalf("stale discard overwrote state: %v", err)
			}
		})
	}
}

func TestFullDemoEmptyRetryUsesDurableApproval(t *testing.T) {
	for _, endpoint := range []string{"/record", "/renders/gameplay-pov-60"} {
		for _, body := range []string{"", "{}", `{"edit":{}}`} {
			t.Run(endpoint+"/"+body, func(t *testing.T) {
				h, j, store, queue, options := fullDemoAPIFixture(t)
				snapshot := fullDemoAPIPlan(t, h, j, options)
				intent := renderplan.GenerateIntent{Variant: "gameplay-pov-60", Edit: renderplan.FullDemoEditRequest(snapshot)}
				if err := h.generateIntents.Begin(j.ID, intent, nil); err != nil {
					t.Fatal(err)
				}
				if strings.HasPrefix(endpoint, "/renders/") {
					j.Status = job.StatusRecorded
					h.repo.(*fakeRepo).jobs[j.ID] = j
				}
				restarted := NewHandlers(h.repo, store, queue, WithCapabilities(Capabilities{RecordEnabled: true}))
				router := chi.NewRouter()
				router.Post("/api/jobs/{id}/record", restarted.StartRecording)
				router.Post("/api/jobs/{id}/renders/{variant}", restarted.StartRenderVariant)
				rw := httptest.NewRecorder()
				request := httptest.NewRequest(http.MethodPost, "/api/jobs/"+j.ID.String()+endpoint, strings.NewReader(body))
				if body == "" {
					request.Body = nil
				}
				router.ServeHTTP(rw, request)
				if rw.Code != http.StatusAccepted || len(queue.enqueued) != 1 {
					t.Fatalf("%d %s", rw.Code, rw.Body.String())
				}
				var edit renderplan.EditRequest
				if endpoint == "/record" {
					saved, _, err := tasks.GenerateIntentFromTask(queue.enqueued[0])
					if err != nil {
						t.Fatal(err)
					}
					edit = saved.Edit
				} else {
					var payload tasks.RenderVariantPayload
					if err := json.Unmarshal(queue.enqueued[0].Payload(), &payload); err != nil {
						t.Fatal(err)
					}
					edit = payload.Edit
					state, found, err := restarted.readRenderVariantState(j.ID, intent.Variant)
					if err != nil || !found || !renderplan.SameFullDemoRequest(state.FullDemo, &snapshot) {
						t.Fatalf("missing durable render approval: %v", err)
					}
				}
				if edit.FullDemo == nil || edit.FullDemo.Approval != snapshot.Approval || edit.FullDemo.Document.Options.Audio.Game.Gain != 0 || edit.CoverStrategy != "no-cover" {
					t.Fatalf("retry substituted the approval: %+v", edit)
				}
			})
		}
	}
}

func TestFullDemoRenderDuplicateKeepsOriginalRequest(t *testing.T) {
	h, j, _, queue, options := fullDemoAPIFixture(t)
	first := fullDemoAPIPlan(t, h, j, options)
	options.Audio.Game.Gain = 0.5
	second := fullDemoAPIPlan(t, h, j, options)
	j.Status = job.StatusRecorded
	h.repo.(*fakeRepo).jobs[j.ID] = j
	for _, tc := range []struct {
		snapshot  recapplan.Snapshot
		status    int
		duplicate bool
	}{
		{first, http.StatusAccepted, false}, {second, http.StatusConflict, true}, {first, http.StatusAccepted, true},
	} {
		if tc.duplicate {
			queue.err = asynq.ErrDuplicateTask
		}
		rw := fullDemoAPIRequest(t, h, j, "/renders/gameplay-pov-60", map[string]any{"edit": renderplan.FullDemoEditRequest(tc.snapshot)})
		if rw.Code != tc.status {
			t.Fatalf("%d %s", rw.Code, rw.Body.String())
		}
		state, found, err := h.readRenderVariantState(j.ID, "gameplay-pov-60")
		if err != nil || !found || !renderplan.SameFullDemoRequest(state.FullDemo, &first) {
			t.Fatalf("replaced accepted render request: %v", err)
		}
	}
}

func fullDemoAPIPlan(t *testing.T, h *Handlers, j job.Job, o recapplan.Options) recapplan.Snapshot {
	t.Helper()
	rw := fullDemoAPIRequest(t, h, j, "/full-demo/plan", map[string]any{"options": o})
	if rw.Code != http.StatusCreated {
		t.Fatalf("plan: %d %s", rw.Code, rw.Body.String())
	}
	var d recapplan.Document
	if err := json.Unmarshal(rw.Body.Bytes(), &d); err != nil {
		t.Fatal(err)
	}
	return recapplan.Snapshot{Document: d, Approval: recapplan.Approval{PlanHash: d.PlanHash, AllowSafeTailTrim: o.Editorial.AllowSafeTailTrim, Timestamp: time.Now().UTC()}}
}

func TestFullDemoPlanningAdmissionAndRetryPreserveApproval(t *testing.T) {
	h, j, store, queue, o := fullDemoAPIFixture(t)
	s := fullDemoAPIPlan(t, h, j, o)
	if len(queue.enqueued) != 0 || h.capabilities.RecordEnabled {
		t.Fatal("planning must work without capture")
	}
	if err := s.Validate(); err != nil {
		t.Fatal(err)
	}
	if len(s.Document.Rounds) != 2 || s.Document.Options.Audio.Game.Gain != 0 || s.Document.Options.Sponsor.Enabled {
		t.Fatal("plan lost zero-kill rounds or negative decisions")
	}
	h.capabilities.RecordEnabled = true
	rw := fullDemoAPIRequest(t, h, j, "/generate", map[string]any{"preset": "gameplay-pov-60", "edit": renderplan.FullDemoEditRequest(s)})
	if rw.Code != http.StatusAccepted {
		t.Fatalf("generate: %d %s", rw.Code, rw.Body.String())
	}
	intent, found, err := tasks.GenerateIntentFromTask(queue.enqueued[0])
	if err != nil || !found || intent.Edit.FullDemo.Document.PlanHash != s.Document.PlanHash {
		t.Fatalf("header dropped approval: %v", err)
	}
	for _, opt := range queue.options[0] {
		if opt.Type() == asynq.MaxRetry(0).Type() && opt.Value() != 0 {
			t.Fatal("capture retries were enabled")
		}
	}
	render, err := tasks.NewRenderVariantTask(j.ID, intent.Variant, "", 0, nil, intent.Edit, nil)
	if err != nil {
		t.Fatal(err)
	}
	var payload tasks.RenderVariantPayload
	if err := json.Unmarshal(render.Payload(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Edit.FullDemo.Approval != s.Approval || *payload.Edit.VoiceVolume != 0 {
		t.Fatal("render payload changed explicit decisions")
	}
	if _, err := h.generateIntents.Finish(j.ID, intent.ActiveRunID, nil); err != nil {
		t.Fatal(err)
	}
	// A fresh handler represents process restart: no in-memory options survive.
	restarted := NewHandlers(h.repo, store, queue, WithCapabilities(Capabilities{RecordEnabled: true}))
	rw = fullDemoAPIRequest(t, restarted, j, "/record", map[string]any{"preset": "gameplay-pov-60", "edit": renderplan.RecapEditRequest()})
	if rw.Code != http.StatusAccepted {
		t.Fatalf("retry: %d %s", rw.Code, rw.Body.String())
	}
	retry, _, err := tasks.GenerateIntentFromTask(queue.enqueued[1])
	if err != nil || retry.Edit.FullDemo.Approval != s.Approval || retry.Edit.CoverStrategy != "no-cover" || retry.Edit.FullDemo.Document.Options.Audio.Game.Gain != 0 {
		t.Fatalf("retry substituted defaults: %+v %v", retry, err)
	}
}

func TestFullDemoRejectsStaleInputsBeforeQueueAdmission(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*fakeStorage, job.Job, *recapplan.Snapshot)
	}{
		{"demo bytes", func(st *fakeStorage, j job.Job, _ *recapplan.Snapshot) { st.puts[j.DemoPath] = []byte("changed") }},
		{"facts removed", func(st *fakeStorage, j job.Job, _ *recapplan.Snapshot) {
			delete(st.puts, artifacts.FullDemoFactsKey(j.ID))
		}},
		{"approval", func(_ *fakeStorage, _ job.Job, s *recapplan.Snapshot) { s.Approval.PlanHash = strings.Repeat("a", 64) }},
		{"forged document", func(_ *fakeStorage, _ job.Job, s *recapplan.Snapshot) {
			s.Document.Options.Audio.Game.Gain = 0.5
			s.Document.PlanHash, _ = s.Document.Hash()
			s.Approval.PlanHash = s.Document.PlanHash
		}},
		{"unsafe trim approval", func(_ *fakeStorage, _ job.Job, s *recapplan.Snapshot) { s.Approval.AllowSafeTailTrim = false }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h, j, store, queue, o := fullDemoAPIFixture(t)
			s := fullDemoAPIPlan(t, h, j, o)
			tc.mutate(store, j, &s)
			h.capabilities.RecordEnabled = true
			rw := fullDemoAPIRequest(t, h, j, "/generate", map[string]any{"preset": "gameplay-pov-60", "edit": renderplan.FullDemoEditRequest(s)})
			if rw.Code != http.StatusConflict || !strings.Contains(rw.Body.String(), recapplan.ErrPlanStale) || len(queue.enqueued) != 0 {
				t.Fatalf("stale admission: %d %s", rw.Code, rw.Body.String())
			}
		})
	}
}

func TestFullDemoMusicChangeIsNotAnAdmissionDuplicate(t *testing.T) {
	h, j, _, queue, o := fullDemoAPIFixture(t)
	s := fullDemoAPIPlan(t, h, j, o)
	o.Audio.Game.Gain = 0.5
	changed := fullDemoAPIPlan(t, h, j, o)
	h.capabilities.RecordEnabled = true
	body := func(s recapplan.Snapshot) any {
		return map[string]any{"preset": "gameplay-pov-60", "edit": renderplan.FullDemoEditRequest(s)}
	}
	if rw := fullDemoAPIRequest(t, h, j, "/generate", body(s)); rw.Code != 202 {
		t.Fatal(rw.Body.String())
	}
	queue.err = asynq.ErrDuplicateTask
	if rw := fullDemoAPIRequest(t, h, j, "/generate", body(changed)); rw.Code != 409 {
		t.Fatalf("changed mix accepted as duplicate: %d %s", rw.Code, rw.Body.String())
	}
	if rw := fullDemoAPIRequest(t, h, j, "/generate", body(s)); rw.Code != 202 {
		t.Fatalf("same intent: %d %s", rw.Code, rw.Body.String())
	}
	current, _, err := h.readGenerateIntent(j.ID)
	if err != nil || current.Edit.FullDemo.Document.PlanHash != s.Document.PlanHash {
		t.Fatal("rejected request replaced accepted intent")
	}
}
