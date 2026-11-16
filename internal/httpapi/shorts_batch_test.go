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
	"github.com/rechedev9/cliphub/internal/rules"
)

func shortsBatchRequest(t *testing.T, j job.Job, query string) *httptest.ResponseRecorder {
	t.Helper()
	repo := newFakeRepo()
	repo.jobs[j.ID] = j
	h := NewHandlers(repo, newFakeStorage(), &fakeQueue{})
	r := chi.NewRouter()
	r.Get("/api/jobs/{id}/shorts-batch", h.GetShortsBatch)
	rw := httptest.NewRecorder()
	r.ServeHTTP(rw, httptest.NewRequest(http.MethodGet, "/api/jobs/"+j.ID.String()+"/shorts-batch"+query, nil))
	return rw
}

func batchKills(n int) []killplan.Kill {
	kills := make([]killplan.Kill, n)
	for i := range kills {
		kills[i] = killplan.Kill{Tick: 100 + i*64, Weapon: "weapon_ak47", Headshot: true}
	}
	return kills
}

func TestGetShortsBatchPicksOnePlayPerRoundOverTheBar(t *testing.T) {
	plan := killplan.NewPlan()
	plan.Demo.Tickrate = 64
	plan.Segments = []killplan.Segment{
		{ID: "seg-001", Round: 2, TickStart: 64, TickEnd: 640, Kills: batchKills(1)},
		{ID: "seg-002", Round: 4, TickStart: 1000, TickEnd: 1600, Kills: batchKills(4)},
		{ID: "seg-003", Round: 4, TickStart: 1700, TickEnd: 2300, Kills: batchKills(3)},
		{ID: "seg-004", Round: 7, TickStart: 3000, TickEnd: 3600, Kills: batchKills(3)},
	}
	j := job.Job{ID: uuid.New(), Status: job.StatusParsed, Rules: rules.Default(), KillPlan: &plan}

	rw := shortsBatchRequest(t, j, "?min_score=0.6&limit=5")
	if rw.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rw.Code, rw.Body.String())
	}
	var got shortsBatchResponse
	if err := json.Unmarshal(rw.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if want := []string{"seg-002", "seg-004"}; !slices.Equal(got.SegmentIDs, want) {
		t.Fatalf("segment_ids = %v, want %v", got.SegmentIDs, want)
	}
	if got.MinScore != 0.6 || got.Limit != 5 || len(got.Moments) != 2 {
		t.Fatalf("response = %+v", got)
	}
}

func TestGetShortsBatchRejectsBadOptionsAndUnparsedJobs(t *testing.T) {
	plan := killplan.NewPlan()
	parsed := job.Job{ID: uuid.New(), Status: job.StatusParsed, Rules: rules.Default(), KillPlan: &plan}
	for _, query := range []string{"?min_score=2", "?limit=0x", "?limit=99"} {
		if rw := shortsBatchRequest(t, parsed, query); rw.Code != http.StatusBadRequest {
			t.Errorf("%s: status = %d, want 400", query, rw.Code)
		}
	}
	queued := job.Job{ID: uuid.New(), Status: job.StatusQueued, Rules: rules.Default()}
	if rw := shortsBatchRequest(t, queued, ""); rw.Code != http.StatusConflict {
		t.Fatalf("unparsed job: status = %d, want 409", rw.Code)
	}
}
