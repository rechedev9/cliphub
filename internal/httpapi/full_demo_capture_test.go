package httpapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/job"
	"github.com/rechedev9/cliphub/internal/recapplan"
	"github.com/rechedev9/cliphub/internal/recording"
)

func TestFullDemoCaptureCheckSaysWhichRoundsAnEditMustRecord(t *testing.T) {
	h, j, store, _, o := fullDemoAPIFixture(t)
	first := o
	first.Rounds = []int{1}
	if got := fullDemoCaptureCheck(t, h, j, first); !got.Needed || !slices.Equal(got.MissingRounds, []int{1}) {
		t.Fatalf("before any capture = %+v, want round 1 missing", got)
	}
	recorded := fullDemoAPIPlan(t, h, j, first)
	storeFullDemoCapture(t, store, j.ID, recorded.Document)

	design := first
	design.Overlays.HUDTheme = "focus"
	if got := fullDemoCaptureCheck(t, h, j, design); got.Needed || len(got.MissingRounds) != 0 {
		t.Fatalf("design change = %+v, want only a new render", got)
	}
	all := first
	all.Rounds = nil
	if got := fullDemoCaptureCheck(t, h, j, all); !got.Needed || !slices.Equal(got.MissingRounds, []int{2}) {
		t.Fatalf("one more round = %+v, want only round 2 missing", got)
	}
	trueView := first
	trueView.Capture.TrueView = true
	if got := fullDemoCaptureCheck(t, h, j, trueView); !got.Needed || !slices.Equal(got.MissingRounds, []int{1}) {
		t.Fatalf("POV 1:1 = %+v, want round 1 recorded again", got)
	}

	router := chi.NewRouter()
	router.Get("/api/jobs/{id}/full-demo/plan", h.GetFullDemoPlan)
	rw := httptest.NewRecorder()
	router.ServeHTTP(rw, httptest.NewRequest(http.MethodGet, "/api/jobs/"+j.ID.String()+"/full-demo/plan", nil))
	var saved struct {
		Capture fullDemoCapture `json:"capture"`
	}
	if err := json.Unmarshal(rw.Body.Bytes(), &saved); err != nil || saved.Capture.Needed {
		t.Fatalf("saved plan capture = %+v (%v); body=%s", saved.Capture, err, rw.Body.String())
	}

	for k := range store.puts {
		if strings.Contains(k, "/recording/revisions/") && strings.HasSuffix(k, ".mp4") {
			delete(store.puts, k)
		}
	}
	if got := fullDemoCaptureCheck(t, h, j, design); !got.Needed || !slices.Equal(got.MissingRounds, []int{1}) {
		t.Fatalf("freed captures = %+v, want round 1 recorded again", got)
	}
}

func fullDemoCaptureCheck(t *testing.T, h *Handlers, j job.Job, o recapplan.Options) fullDemoCapture {
	t.Helper()
	rw := fullDemoAPIRequest(t, h, j, "/full-demo/capture-check", map[string]any{"options": o})
	if rw.Code != http.StatusOK {
		t.Fatalf("capture check: %d %s", rw.Code, rw.Body.String())
	}
	var got fullDemoCapture
	if err := json.Unmarshal(rw.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	return got
}

// storeFullDemoCapture commits a verified capture of every round of d, as the record worker leaves it.
func storeFullDemoCapture(t *testing.T, store *fakeStorage, id uuid.UUID, d recapplan.Document) {
	t.Helper()
	revision := uuid.NewString()
	result := recording.RecordingResult{
		CaptureMode: recording.CaptureModeReal, CaptureVerified: true, CaptureRevision: revision,
		Plan:             recording.RecordingPlan{FullDemo: &d, CaptureContract: recording.CaptureContractVersion, DemoSHA256: d.Input.DemoSHA256, TargetSteamID64: d.Input.TargetSteamID64, Tickrate: d.Clock.TickRate},
		FullDemoEvidence: &recording.FullDemoCaptureEvidence{SchemaVersion: "1.0", Restored: true, CertifiedEnds: map[string]int{}},
	}
	for _, r := range d.Rounds {
		frames, err := recapplan.TickFrames(r.CaptureEndTick-r.CaptureStartTick, d.Clock.TickRate)
		if err != nil {
			t.Fatal(err)
		}
		key := "jobs/" + id.String() + "/recording/revisions/" + revision + "/segments/" + r.ID + ".mp4"
		result.Plan.Segments = append(result.Plan.Segments, recording.RecordingSegment{ID: r.ID, Round: r.Number, TickStart: r.CaptureStartTick, TickEnd: r.CaptureEndTick, LiveEndTick: r.LiveEndTick, ExactWindow: true})
		result.Artifacts = append(result.Artifacts, recording.RecordingArtifact{SegmentID: r.ID, Role: "segment", Type: "video", Path: r.ID + ".mp4", StorageKey: key, SizeBytes: 5, FrameCount: frames, FrameRate: "60/1", ContentSHA256: strings.Repeat("c", 64)})
		result.FullDemoEvidence.CertifiedEnds[r.ID] = r.CaptureEndTick
		if err := store.Put(key, bytes.NewReader([]byte("round"))); err != nil {
			t.Fatal(err)
		}
	}
	b, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Put(recording.ResultArtifactKey(id), bytes.NewReader(b)); err != nil {
		t.Fatal(err)
	}
}

// existsFails is storage whose Exists of one key fails, as a disk error would.
type existsFails struct {
	*fakeStorage
	key string
}

func (s existsFails) Exists(key string) (bool, error) {
	if key == s.key {
		return false, errors.New("disk read failed")
	}
	return s.fakeStorage.Exists(key)
}

// The capture hint is advisory: a plan whose capture check fails still opens, without the hint.
func TestFullDemoPlanLoadsWhenItsCaptureCheckFails(t *testing.T) {
	h, j, store, _, o := fullDemoAPIFixture(t)
	planned := fullDemoAPIPlan(t, h, j, o)
	h.storage = existsFails{fakeStorage: store, key: recording.ResultArtifactKey(j.ID)}
	router := chi.NewRouter()
	router.Get("/api/jobs/{id}/full-demo/plan", h.GetFullDemoPlan)
	rw := httptest.NewRecorder()
	router.ServeHTTP(rw, httptest.NewRequest(http.MethodGet, "/api/jobs/"+j.ID.String()+"/full-demo/plan", nil))
	var got struct {
		Document *recapplan.Document `json:"document"`
		Capture  *fullDemoCapture    `json:"capture"`
	}
	if err := json.Unmarshal(rw.Body.Bytes(), &got); rw.Code != http.StatusOK || err != nil {
		t.Fatalf("plan = %d %s, want the saved plan", rw.Code, rw.Body.String())
	}
	if got.Document == nil || got.Document.PlanID != planned.Document.PlanID || got.Capture != nil {
		t.Fatalf("plan = %+v, capture %+v; want the saved plan without a capture hint", got.Document, got.Capture)
	}
}

func TestFullDemoClipHubRadarNeedsAMapWithARadar(t *testing.T) {
	h, j, _, _, o := fullDemoAPIFixture(t)
	o.Capture.ClipHubRadar = true
	j.KillPlan.Demo.Map = "de_unknown"
	for _, endpoint := range []string{"/full-demo/plan", "/full-demo/capture-check"} {
		rw := fullDemoAPIRequest(t, h, j, endpoint, map[string]any{"options": o})
		if rw.Code != http.StatusBadRequest || !strings.Contains(rw.Body.String(), "ClipHub radar") {
			t.Fatalf("%s on a map without a radar: %d %s", endpoint, rw.Code, rw.Body.String())
		}
	}
	j.KillPlan.Demo.Map = "de_mirage"
	if got := fullDemoAPIPlan(t, h, j, o); !got.Document.Options.Capture.ClipHubRadar {
		t.Fatalf("ClipHub radar dropped from the plan: %+v", got.Document.Options.Capture)
	}
	if got := fullDemoCaptureCheck(t, h, j, o); !got.Needed {
		t.Fatalf("ClipHub radar capture check = %+v, want a new capture", got)
	}
}
