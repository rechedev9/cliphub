package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/job"
	"github.com/rechedev9/cliphub/internal/killplan"
	"github.com/rechedev9/cliphub/internal/moments"
	"github.com/rechedev9/cliphub/internal/rules"
)

func autoPOVRequest(t *testing.T, j job.Job, query string) *httptest.ResponseRecorder {
	t.Helper()
	repo := newFakeRepo()
	repo.jobs[j.ID] = j
	h := NewHandlers(repo, newFakeStorage(), &fakeQueue{})
	r := chi.NewRouter()
	r.Get("/api/jobs/{id}/auto-pov", h.GetAutoPOV)
	rw := httptest.NewRecorder()
	r.ServeHTTP(rw, httptest.NewRequest(http.MethodGet, "/api/jobs/"+j.ID.String()+"/auto-pov"+query, nil))
	return rw
}

func TestGetAutoPOVPicksPlaysOfSeveralPlayersWithoutClashes(t *testing.T) {
	a, b := killplan.Target{SteamID64: "1", NameInDemo: "ropz"}, killplan.Target{SteamID64: "2", NameInDemo: "b1t"}
	plan := killplan.NewPlan()
	plan.Demo.Tickrate = 64
	plan.Target = a
	plan.Segments = []killplan.Segment{
		{ID: "seg-001", Round: 2, TickStart: 1000, TickEnd: 1640, POV: &a, Kills: batchKills(1)},
		{ID: "seg-002", Round: 2, TickStart: 1200, TickEnd: 1840, POV: &b, Kills: batchKills(4)},
		{ID: "seg-003", Round: 6, TickStart: 6000, TickEnd: 6640, POV: &a, Kills: batchKills(3)},
	}
	j := job.Job{ID: uuid.New(), Status: job.StatusParsed, Rules: rules.Default(), KillPlan: &plan, MultiPOV: &job.MultiPOV{SteamIDs: []string{"1", "2"}}}

	rw := autoPOVRequest(t, j, "?target_seconds=60&extra_seconds=1.5")
	if rw.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rw.Code, rw.Body.String())
	}
	var got moments.Direction
	if err := json.Unmarshal(rw.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if want := []string{"seg-002", "seg-003"}; !slices.Equal(got.SegmentIDs, want) {
		t.Fatalf("segment_ids = %v, want %v", got.SegmentIDs, want)
	}
	if got.TargetSeconds != 60 || got.ExtraSeconds != 1.5 || got.TotalSeconds != 23 || len(got.Players) != 2 {
		t.Fatalf("direction = %+v", got)
	}
}

func TestGetAutoPOVPicksThePlaysOfOnePlayerWithTheChosenPreRoll(t *testing.T) {
	a := killplan.Target{SteamID64: "1", NameInDemo: "ropz"}
	plan := killplan.NewPlan()
	plan.Demo.Tickrate = 64
	plan.Target = a
	// Two plays of round 2, 9 s apart, that overlap once 5 s come before each kill.
	plan.Segments = []killplan.Segment{
		{ID: "seg-001", Round: 2, TickStart: 1000 - 192, TickEnd: 1000 + 320, Kills: []killplan.Kill{{Tick: 1000}}},
		{ID: "seg-002", Round: 2, TickStart: 1576 - 192, TickEnd: 1576 + 320, Kills: []killplan.Kill{{Tick: 1576}}},
		{ID: "seg-003", Round: 6, TickStart: 6000 - 192, TickEnd: 6000 + 320, Kills: []killplan.Kill{{Tick: 6000}}},
	}
	single := job.Job{ID: uuid.New(), Status: job.StatusParsed, Rules: rules.Default(), KillPlan: &plan}
	for query, want := range map[string][]string{"": {"seg-001", "seg-002", "seg-003"}, "?pre_roll_seconds=5": {"seg-001", "seg-003"}} {
		rw := autoPOVRequest(t, single, query)
		if rw.Code != http.StatusOK {
			t.Fatalf("%q: status = %d, body=%s", query, rw.Code, rw.Body.String())
		}
		var got moments.Direction
		if err := json.Unmarshal(rw.Body.Bytes(), &got); err != nil {
			t.Fatal(err)
		}
		if !slices.Equal(got.SegmentIDs, want) {
			t.Errorf("%q: segment_ids = %v, want %v", query, got.SegmentIDs, want)
		}
	}
	if rw := autoPOVRequest(t, single, "?pre_roll_seconds=4"); rw.Code != http.StatusBadRequest {
		t.Errorf("pre_roll_seconds=4: status = %d, want 400", rw.Code)
	}
}

func TestGetAutoPOVRejectsBadOptionsAndUnparsedJobs(t *testing.T) {
	plan := killplan.NewPlan()
	multi := job.Job{ID: uuid.New(), Status: job.StatusParsed, Rules: rules.Default(), KillPlan: &plan, MultiPOV: &job.MultiPOV{SteamIDs: []string{"1", "2"}}}
	for _, query := range []string{"?target_seconds=5", "?target_seconds=x", "?extra_seconds=20"} {
		if rw := autoPOVRequest(t, multi, query); rw.Code != http.StatusBadRequest {
			t.Errorf("%s: status = %d, want 400", query, rw.Code)
		}
	}
	if rw := autoPOVRequest(t, multi, "?pre_roll_seconds=5"); rw.Code != http.StatusBadRequest {
		t.Errorf("pre-roll for several players: status = %d, want 400", rw.Code)
	}
	queued := job.Job{ID: uuid.New(), Status: job.StatusQueued, Rules: rules.Default(), MultiPOV: multi.MultiPOV}
	if rw := autoPOVRequest(t, queued, ""); rw.Code != http.StatusConflict {
		t.Fatalf("unparsed job: status = %d, want 409", rw.Code)
	}
}
