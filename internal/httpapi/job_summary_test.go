package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/artifacts"
	"github.com/rechedev9/cliphub/internal/job"
)

func putRoster(t *testing.T, store *fakeStorage, id uuid.UUID, roster rosterArtifact) {
	t.Helper()
	b, err := json.Marshal(roster)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Put(artifacts.RosterKey(id), bytes.NewReader(b)); err != nil {
		t.Fatal(err)
	}
}

func listJobs(t *testing.T, h *Handlers) []struct {
	ID      uuid.UUID   `json:"id"`
	Summary *jobSummary `json:"summary"`
} {
	t.Helper()
	r := chi.NewRouter()
	r.Get("/api/jobs", h.ListJobs)
	req := httptest.NewRequest(http.MethodGet, "/api/jobs?limit=10", nil)
	rw := httptest.NewRecorder()
	r.ServeHTTP(rw, req)
	if rw.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rw.Code, rw.Body.String())
	}
	var resp struct {
		Jobs []struct {
			ID      uuid.UUID   `json:"id"`
			Summary *jobSummary `json:"summary"`
		} `json:"jobs"`
	}
	if err := json.Unmarshal(rw.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode list: %v; body=%s", err, rw.Body.String())
	}
	return resp.Jobs
}

func TestListJobsCarriesRosterSummary(t *testing.T) {
	repo := newFakeRepo()
	store := newFakeStorage()
	target := rosterPlayer{SteamID64: "76561198000000002", Name: "donk", Team: "T", Kills: 25, Deaths: 13}
	roster := rosterArtifact{
		Players: []rosterPlayer{{SteamID64: "76561198000000001", Name: "zywoo", Team: "CT", Kills: 20, Deaths: 15}, target},
		Match:   rosterMatch{Map: "de_cache", ScoreCT: 13, ScoreT: 9, Rounds: 22},
	}
	cases := []struct {
		name       string
		stored     job.Job
		withRoster bool
		wantMatch  bool
		wantTarget *rosterPlayer
	}{
		{
			name:       "parsed with target",
			stored:     job.Job{Status: job.StatusParsed, TargetSteamID: target.SteamID64},
			withRoster: true,
			wantMatch:  true,
			wantTarget: &target,
		},
		{
			name:       "scanned without target",
			stored:     job.Job{Status: job.StatusScanned},
			withRoster: true,
			wantMatch:  true,
		},
		{
			name:       "target absent from roster",
			stored:     job.Job{Status: job.StatusParsed, TargetSteamID: "76561198000000009"},
			withRoster: true,
			wantMatch:  true,
		},
		{
			name:   "no roster yet",
			stored: job.Job{Status: job.StatusScanning},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			id := uuid.New()
			tc.stored.ID = id
			repo.jobs = map[uuid.UUID]job.Job{id: tc.stored}
			if tc.withRoster {
				putRoster(t, store, id, roster)
			}
			h := NewHandlers(repo, store, &fakeQueue{})
			jobs := listJobs(t, h)
			if len(jobs) != 1 || jobs[0].ID != id {
				t.Fatalf("jobs = %+v, want one row for %s", jobs, id)
			}
			got := jobs[0].Summary
			if !tc.wantMatch {
				if got != nil {
					t.Fatalf("summary = %+v, want none", got)
				}
				return
			}
			if got == nil {
				t.Fatal("summary missing")
			}
			if got.Match != roster.Match {
				t.Fatalf("match = %+v, want %+v", got.Match, roster.Match)
			}
			switch {
			case tc.wantTarget == nil && got.Target != nil:
				t.Fatalf("target = %+v, want none", got.Target)
			case tc.wantTarget != nil && (got.Target == nil || *got.Target != *tc.wantTarget):
				t.Fatalf("target = %+v, want %+v", got.Target, tc.wantTarget)
			}
		})
	}
}

func TestListJobsReadsEachRosterOnceUntilDeleted(t *testing.T) {
	repo := newFakeRepo()
	store := newFakeStorage()
	id := uuid.New()
	repo.jobs[id] = job.Job{ID: id, Status: job.StatusScanned}
	putRoster(t, store, id, rosterArtifact{Match: rosterMatch{Map: "de_mirage"}})
	h := NewHandlers(repo, store, &fakeQueue{})

	if got := listJobs(t, h)[0].Summary; got == nil || got.Match.Map != "de_mirage" {
		t.Fatalf("first list summary = %+v", got)
	}
	// The artifact is immutable once scanned: a second list must serve the
	// cached decode, so rewriting the file underneath does not change the row.
	putRoster(t, store, id, rosterArtifact{Match: rosterMatch{Map: "de_inferno"}})
	if got := listJobs(t, h)[0].Summary; got == nil || got.Match.Map != "de_mirage" {
		t.Fatalf("cached list summary = %+v, want the first decode", got)
	}
	h.rosterCache.evict(id)
	if got := listJobs(t, h)[0].Summary; got == nil || got.Match.Map != "de_inferno" {
		t.Fatalf("post-evict summary = %+v, want a fresh decode", got)
	}
}
