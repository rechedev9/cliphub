package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/editor"
	"github.com/rechedev9/cliphub/internal/job"
	"github.com/rechedev9/cliphub/internal/killplan"
	"github.com/rechedev9/cliphub/internal/recording"
	"github.com/rechedev9/cliphub/internal/renderplan"
	"github.com/rechedev9/cliphub/internal/rules"
	"github.com/rechedev9/cliphub/internal/storage"
	"github.com/rechedev9/cliphub/internal/tasks"
)

func TestCrearHorizontalDraftsTheSummaryOfTheShortsPlays(t *testing.T) {
	h, j := horizontalFixture(t)
	game := 0.4
	short := renderplan.EditRequest{Format: renderplan.FormatShort9x16, KillEffect: renderplan.KillEffectGlitch, Transition: renderplan.TransitionWhip,
		Intro: true, IntroText: "ACE en Mirage", KillCounter: true, PlayBadges: true, VerticalHUD: true, StatsCard: true, Radar: true, PreRollSeconds: 5}
	commitShortVersion(t, h, shortVersion{job: j.ID, variant: editor.PresetViral60Clean, segments: []string{"seg-002", "seg-001"}, edit: short,
		music: &renderplan.MusicSnapshot{Key: "song-1", Volume: 0.6, GameVolume: &game}})

	draft := getHorizontalDraft(t, h, j.ID, editor.PresetViral60Clean)

	if draft.Preset != editor.PresetGameplayPOV60 || !slices.Equal(draft.SegmentIDs, []string{"seg-002", "seg-001"}) {
		t.Fatalf("draft = %s with %v, want the 16:9 preset with the Short's plays in its order", draft.Preset, draft.SegmentIDs)
	}
	e := draft.Edit
	if e.Format != renderplan.FormatLandscape16x9 || e.CaptureHUD != "deathnotices" || e.NativeHUD || e.PreRollSeconds != 5 {
		t.Fatalf("capture = format %s, hud %q, native %v, pre-roll %d; want 16:9 in the Short's Killfeed style with its pre-roll", e.Format, e.CaptureHUD, e.NativeHUD, e.PreRollSeconds)
	}
	if e.KillEffect != renderplan.KillEffectGlitch || e.Transition != renderplan.TransitionWhip || !e.Intro || e.IntroText != "ACE en Mirage" || !e.PlayBadges {
		t.Fatalf("edit = %+v, want the Short's effect, transition, opening text and badges", e)
	}
	if e.VerticalHUD || e.CenteredKillfeed || e.StatsCard || e.Radar || e.KillCounter {
		t.Fatalf("edit = %+v, want no 9:16 option left", e)
	}
	if want := []string{"vertical_hud", "stats_card", "radar", "kill_counter"}; !slices.Equal(draft.Dropped, want) {
		t.Fatalf("dropped = %v, want %v", draft.Dropped, want)
	}
	if draft.Music == nil || draft.Music.Key != "song-1" || draft.Music.Volume != 0.6 || draft.Music.GameVolume == nil || *draft.Music.GameVolume != 0.4 {
		t.Fatalf("music = %+v, want the Short's track and levels", draft.Music)
	}
	if !draft.Capture.Needed || !slices.Equal(draft.Capture.MissingSegmentIDs, []string{"seg-002", "seg-001"}) {
		t.Fatalf("capture before any recording = %+v, want every play missing", draft.Capture)
	}
}

func TestCrearHorizontalSaysWhenTheShortsRecordingsAreGone(t *testing.T) {
	h, j := horizontalFixture(t)
	short := renderplan.EditRequest{Format: renderplan.FormatShort9x16, KillEffect: renderplan.KillEffectPunchIn, Transition: renderplan.TransitionFlash}
	commitShortVersion(t, h, shortVersion{job: j.ID, variant: editor.PresetViral60Clean, segments: []string{"seg-001", "seg-002"}, edit: short})
	storeShortCapture(t, h.storage, shortCapture{job: j.ID, hud: recording.HUDModeDeathnotices, portraitSafe: true, segments: []string{"seg-001", "seg-002"}})

	if got := getHorizontalDraft(t, h, j.ID, editor.PresetViral60Clean).Capture; got.Needed || len(got.MissingSegmentIDs) != 0 {
		t.Fatalf("capture with the Short's clips on disk = %+v, want only a render", got)
	}
	if err := h.storage.(interface{ DeleteTree(string) error }).DeleteTree(shortClipKey(t, j.ID, "seg-002")); err != nil {
		t.Fatal(err)
	}
	if got := getHorizontalDraft(t, h, j.ID, editor.PresetViral60Clean).Capture; !got.Needed || !slices.Equal(got.MissingSegmentIDs, []string{"seg-002"}) {
		t.Fatalf("capture after freeing one clip = %+v, want seg-002 recorded again", got)
	}
}

func TestCrearHorizontalIsOnlyForAShortOfKills(t *testing.T) {
	h, j := horizontalFixture(t)
	commitShortVersion(t, h, shortVersion{job: j.ID, variant: editor.PresetGameplayPOV60, segments: []string{"seg-001"},
		edit: renderplan.EditRequest{Format: renderplan.FormatLandscape16x9, KillEffect: renderplan.KillEffectClean, Transition: renderplan.TransitionCut, NativeHUD: true}})
	commitShortVersion(t, h, shortVersion{job: j.ID, variant: editor.PresetCleanPOV60, segments: []string{"seg-001"},
		edit: renderplan.EditRequest{Format: renderplan.FormatShort9x16, KillEffect: renderplan.KillEffectClean, Transition: renderplan.TransitionCut, DeathReel: true}})
	for _, variant := range []string{editor.PresetGameplayPOV60, editor.PresetCleanPOV60, editor.PresetFullHUD60} {
		rw := requestHorizontalDraft(h, j.ID, variant)
		if rw.Code == http.StatusOK {
			t.Errorf("%s: draft = %s, want a refusal", variant, rw.Body.String())
		}
	}
}

// TestTheHorizontalAsksForTheShortsCapture: an equal record task reuses every clip without CS2
// (TestRecordWorkerSkipsWhenSelectedSegmentAlreadyRecorded), so nothing is recorded again.
func TestTheHorizontalAsksForTheShortsCapture(t *testing.T) {
	cases := []struct {
		preset   string
		edit     renderplan.EditRequest
		multiPOV bool
	}{
		{editor.PresetViral60Clean, renderplan.EditRequest{PreRollSeconds: 8}, false},
		{editor.PresetViral60Clean, renderplan.EditRequest{CenteredKillfeed: true, VictimReplay: true}, false},
		{editor.PresetFullHUD60, renderplan.EditRequest{HookText: true}, false},
		{editor.PresetCleanPOV60, renderplan.EditRequest{StatsCard: true}, false},
		{editor.PresetViral60Clean, renderplan.EditRequest{}, true},
	}
	for _, tc := range cases {
		h, j := horizontalFixture(t)
		if tc.multiPOV {
			j.MultiPOV = &job.MultiPOV{SteamIDs: []string{"76561198000000001", "76561198000000002"}}
			h.repo.(*fakeRepo).jobs[j.ID] = j
		}
		edit := tc.edit
		edit.Format, edit.KillEffect, edit.Transition = renderplan.FormatShort9x16, renderplan.KillEffectPunchIn, renderplan.TransitionFlash
		shortBody, err := json.Marshal(map[string]any{"preset": tc.preset, "segment_ids": []string{"seg-001", "seg-002"}, "edit": edit})
		if err != nil {
			t.Fatal(err)
		}
		shortTask := enqueuedCapture(t, h, j.ID, string(shortBody))
		commitShortVersion(t, h, shortVersion{job: j.ID, variant: tc.preset, segments: []string{"seg-001", "seg-002"}, edit: edit})

		draft := getHorizontalDraft(t, h, j.ID, tc.preset)
		body, err := json.Marshal(map[string]any{"preset": draft.Preset, "segment_ids": draft.SegmentIDs, "edit": draft.Edit})
		if err != nil {
			t.Fatal(err)
		}
		if got := enqueuedCapture(t, h, j.ID, string(body)); !sameRecordTask(got, shortTask) {
			t.Errorf("%s %+v: horizontal capture = %+v, want the Short's %+v", tc.preset, tc.edit, got, shortTask)
		}
	}
}

func TestOnlyAVideoInSixteenByNineKeepsAShortsKillFeedFraming(t *testing.T) {
	h, j := horizontalFixture(t)
	rw := postGenerate(t, h, j.ID, `{"preset":"viral-60-clean","segment_ids":["seg-001"],"edit":{"format":"short-9x16","portrait_safe_killfeed":true}}`)
	if rw.Code != http.StatusBadRequest {
		t.Fatalf("a Short asking for the framing = %d %s, want 400", rw.Code, rw.Body.String())
	}
}

type horizontalDraftResponse struct {
	Preset     string                    `json:"preset"`
	SegmentIDs []string                  `json:"segment_ids"`
	Edit       renderplan.EditRequest    `json:"edit"`
	Music      *renderplan.MusicSnapshot `json:"music"`
	Dropped    []string                  `json:"dropped"`
	Capture    struct {
		Needed            bool     `json:"needed"`
		MissingSegmentIDs []string `json:"missing_segment_ids"`
	} `json:"capture"`
}

func horizontalFixture(t *testing.T) (*Handlers, job.Job) {
	t.Helper()
	store, err := storage.NewLocal(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	plan := killplan.NewPlan()
	plan.Segments = []killplan.Segment{{ID: "seg-001"}, {ID: "seg-002"}}
	j := job.Job{ID: uuid.New(), Status: job.StatusDone, Rules: rules.Default(), KillPlan: &plan}
	repo := newFakeRepo()
	repo.jobs[j.ID] = j
	return NewHandlers(repo, store, &fakeQueue{}, WithCapabilities(Capabilities{RecordEnabled: true})), j
}

func requestHorizontalDraft(h *Handlers, id uuid.UUID, variant string) *httptest.ResponseRecorder {
	r := chi.NewRouter()
	r.Get("/api/jobs/{id}/renders/{variant}/horizontal", h.GetHorizontalDraft)
	rw := httptest.NewRecorder()
	r.ServeHTTP(rw, httptest.NewRequest(http.MethodGet, "/api/jobs/"+id.String()+"/renders/"+variant+"/horizontal", nil))
	return rw
}

func getHorizontalDraft(t *testing.T, h *Handlers, id uuid.UUID, variant string) horizontalDraftResponse {
	t.Helper()
	rw := requestHorizontalDraft(h, id, variant)
	var draft horizontalDraftResponse
	if err := json.Unmarshal(rw.Body.Bytes(), &draft); rw.Code != http.StatusOK || err != nil {
		t.Fatalf("horizontal draft = %d %s", rw.Code, rw.Body.String())
	}
	return draft
}

type shortVersion struct {
	job      uuid.UUID
	variant  string
	segments []string
	edit     renderplan.EditRequest
	music    *renderplan.MusicSnapshot
}

// commitShortVersion stores the finished render of one version, as the render worker commits it.
func commitShortVersion(t *testing.T, h *Handlers, v shortVersion) {
	t.Helper()
	loadout, err := renderplan.LoadoutForVariant(v.variant)
	if err != nil {
		t.Fatal(err)
	}
	state, err := renderplan.NewRenderVariantStateForLoadout(renderplan.NewRenderVariantStateForLoadoutOptions{JobID: v.job, Loadout: loadout, Status: renderplan.RenderVariantStatusReady, RevisionID: uuid.New(), Now: time.Now()})
	if err != nil {
		t.Fatal(err)
	}
	doc, err := renderplan.NewEditDocumentForLoadout(renderplan.NewEditDocumentForLoadoutOptions{JobID: v.job, Loadout: loadout, SegmentIDs: v.segments, Edit: v.edit, Music: v.music})
	if err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	for key, body := range map[string][]byte{state.EditDocumentKey: b, state.ArtifactPrefix + "/videos/" + v.segments[0] + ".mp4": []byte("video")} {
		if err := h.storage.Put(key, bytes.NewReader(body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := h.writeRenderVariantState(state); err != nil {
		t.Fatal(err)
	}
}

type shortCapture struct {
	job          uuid.UUID
	hud          recording.HUDMode
	portraitSafe bool
	segments     []string
}

func shortClipKey(t *testing.T, id uuid.UUID, segment string) string {
	t.Helper()
	key, err := recording.SegmentClipArtifactKey(id, segment)
	if err != nil {
		t.Fatal(err)
	}
	return key
}

// storeShortCapture commits a verified capture of the segments, as the record worker leaves it.
func storeShortCapture(t *testing.T, store storage.Storage, c shortCapture) {
	t.Helper()
	stream := recording.DefaultStreamConfig()
	stream.HUDMode, stream.PortraitSafeKillfeed = c.hud, c.portraitSafe
	result := recording.RecordingResult{CaptureMode: recording.CaptureModeReal, CaptureVerified: true,
		Plan: recording.RecordingPlan{CaptureContract: recording.CaptureContractVersion, Stream: stream}}
	for _, id := range c.segments {
		key := shortClipKey(t, c.job, id)
		result.Plan.Segments = append(result.Plan.Segments, recording.RecordingSegment{ID: id})
		result.Artifacts = append(result.Artifacts, recording.RecordingArtifact{SegmentID: id, Role: "segment", Type: "video", Path: id + ".mp4", SizeBytes: 4, ContentSHA256: strings.Repeat("c", 64)})
		if err := store.Put(key, bytes.NewReader([]byte("clip"))); err != nil {
			t.Fatal(err)
		}
	}
	b, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Put(recording.ResultArtifactKey(c.job), bytes.NewReader(b)); err != nil {
		t.Fatal(err)
	}
}

type recordTask struct {
	payload tasks.RecordDemoPayload
	replay  bool
}

func enqueuedCapture(t *testing.T, h *Handlers, id uuid.UUID, body string) recordTask {
	t.Helper()
	queue := &fakeQueue{}
	h.queue = queue
	if rw := postGenerate(t, h, id, body); rw.Code != http.StatusAccepted {
		t.Fatalf("generate %s = %d %s", body, rw.Code, rw.Body.String())
	}
	var payload tasks.RecordDemoPayload
	if err := json.Unmarshal(queue.enqueued[0].Payload(), &payload); err != nil {
		t.Fatal(err)
	}
	intent, _, err := tasks.GenerateIntentFromTask(queue.enqueued[0])
	if err != nil {
		t.Fatal(err)
	}
	if err := h.generateIntents.Complete(id, intent.ActiveRunID); err != nil {
		t.Fatal(err)
	}
	return recordTask{payload: payload, replay: intent.Edit.VictimReplay}
}

func sameRecordTask(a, b recordTask) bool {
	return a.payload.HUDMode == b.payload.HUDMode && a.payload.PortraitSafeKillfeed == b.payload.PortraitSafeKillfeed &&
		a.payload.PreRollSeconds == b.payload.PreRollSeconds && a.payload.UseDeathPlan == b.payload.UseDeathPlan &&
		a.payload.UseRecapPlan == b.payload.UseRecapPlan && slices.Equal(a.payload.SegmentIDs, b.payload.SegmentIDs) && a.replay == b.replay
}
