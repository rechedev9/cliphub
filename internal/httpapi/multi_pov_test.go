package httpapi

import (
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/artifacts"
	"github.com/rechedev9/cliphub/internal/job"
	"github.com/rechedev9/cliphub/internal/killplan"
	"github.com/rechedev9/cliphub/internal/rules"
)

const multiPOVRoster = `{"players":[
	{"steamid64":"76561198000000001","name":"a","team":"CT"},
	{"steamid64":"76561198000000002","name":"b","team":"CT"},
	{"steamid64":"76561198000000003","name":"c","team":"T"}]}`

func postParse(t *testing.T, body string) (*httptest.ResponseRecorder, job.Job) {
	t.Helper()
	repo := newFakeRepo()
	j := job.Job{ID: uuid.New(), Status: job.StatusScanned, Rules: rules.Default()}
	repo.jobs[j.ID] = j
	store := newFakeStorage()
	if err := store.Put(artifacts.RosterKey(j.ID), strings.NewReader(multiPOVRoster)); err != nil {
		t.Fatal(err)
	}
	h := NewHandlers(repo, store, &fakeQueue{})
	r := chi.NewRouter()
	r.Post("/api/jobs/{id}/parse", h.StartParse)
	rw := httptest.NewRecorder()
	r.ServeHTTP(rw, httptest.NewRequest(http.MethodPost, "/api/jobs/"+j.ID.String()+"/parse", strings.NewReader(body)))
	return rw, repo.jobs[j.ID]
}

func TestStartParseTakesSeveralTeammates(t *testing.T) {
	rw, got := postParse(t, `{"target_steamids":["76561198000000002","76561198000000001"]}`)
	if rw.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202; body=%s", rw.Code, rw.Body.String())
	}
	if got.TargetSteamID != "76561198000000002" || got.MultiPOV == nil ||
		!slices.Equal(got.MultiPOV.SteamIDs, []string{"76561198000000002", "76561198000000001"}) {
		t.Fatalf("job = target %q multi %+v, want the first pick as anchor and both players", got.TargetSteamID, got.MultiPOV)
	}
}

func TestStartParseOneTargetInTheListIsASinglePOV(t *testing.T) {
	rw, got := postParse(t, `{"target_steamids":["76561198000000003"]}`)
	if rw.Code != http.StatusAccepted || got.TargetSteamID != "76561198000000003" || got.MultiPOV != nil {
		t.Fatalf("status %d target %q multi %+v, want a plain single-POV parse", rw.Code, got.TargetSteamID, got.MultiPOV)
	}
}

func TestStartParseRejectsBadMultiPOVPicks(t *testing.T) {
	for name, body := range map[string]string{
		"two teams":     `{"target_steamids":["76561198000000001","76561198000000003"]}`,
		"not in demo":   `{"target_steamids":["76561198000000001","76561198000000009"]}`,
		"repeated":      `{"target_steamids":["76561198000000001","76561198000000001"]}`,
		"both fields":   `{"target_steamid":"76561198000000001","target_steamids":["76561198000000002","76561198000000001"]}`,
		"too many":      `{"target_steamids":["1","2","3","4","5","6"]}`,
		"not a steamid": `{"target_steamids":["76561198000000001","x"]}`,
	} {
		rw, got := postParse(t, body)
		if rw.Code != http.StatusBadRequest || got.Status != job.StatusScanned {
			t.Errorf("%s: status %d job %s, want 400 and the job left scanned", name, rw.Code, got.Status)
		}
	}
}

func multiPOVJob() job.Job {
	plan := killplan.NewPlan()
	plan.Demo.Tickrate = 64
	a, b := killplan.Target{SteamID64: "1", NameInDemo: "a"}, killplan.Target{SteamID64: "2", NameInDemo: "b"}
	plan.Target = a
	plan.Segments = []killplan.Segment{
		{ID: "seg-001", TickStart: 1000, TickEnd: 1500, POV: &a},
		{ID: "seg-002", TickStart: 1400, TickEnd: 1900, POV: &b},
		{ID: "seg-003", TickStart: 5000, TickEnd: 5500, POV: &b},
	}
	return job.Job{ID: uuid.New(), Status: job.StatusParsed, TargetSteamID: "1", MultiPOV: &job.MultiPOV{SteamIDs: []string{"1", "2"}}, KillPlan: &plan}
}

func TestMultiPOVSelectionRules(t *testing.T) {
	h := NewHandlers(newFakeRepo(), newFakeStorage(), &fakeQueue{})
	cases := []struct {
		name      string
		ids       []string
		deathReel bool
		ok        bool
	}{
		{"filmable pick", []string{"seg-001", "seg-003"}, false, true},
		{"two players at once", []string{"seg-001", "seg-002"}, false, false},
		{"no pick", nil, false, false},
		{"death reel", []string{"seg-001"}, true, false},
	}
	for _, c := range cases {
		rw := httptest.NewRecorder()
		if got := h.validateSegmentSelection(rw, multiPOVJob(), c.ids, c.deathReel); got != c.ok {
			t.Errorf("%s: ok = %v, want %v (%s)", c.name, got, c.ok, rw.Body.String())
		}
	}
}

func TestFullDemoRefusesAMultiPOVJob(t *testing.T) {
	repo := newFakeRepo()
	j := multiPOVJob()
	repo.jobs[j.ID] = j
	h := NewHandlers(repo, newFakeStorage(), &fakeQueue{})
	r := chi.NewRouter()
	r.Post("/api/jobs/{id}/full-demo/plan", h.PlanFullDemo)
	rw := httptest.NewRecorder()
	r.ServeHTTP(rw, httptest.NewRequest(http.MethodPost, "/api/jobs/"+j.ID.String()+"/full-demo/plan", strings.NewReader(`{"options":{}}`)))
	if rw.Code != http.StatusConflict || !strings.Contains(rw.Body.String(), multiPOVFullDemoCode) {
		t.Fatalf("status = %d body %s, want 409 %s", rw.Code, rw.Body.String(), multiPOVFullDemoCode)
	}
}
