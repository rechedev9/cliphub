package httpapi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/artifacts"
	"github.com/rechedev9/cliphub/internal/job"
)

var benchmarkJobsListBodyBytes int

func BenchmarkListJobsUnchangedPoll(b *testing.B) {
	h, router := benchmarkJobsListRouter(50)
	warm := httptest.NewRecorder()
	router.ServeHTTP(warm, httptest.NewRequest(http.MethodGet, "/api/jobs?limit=50", nil))
	if warm.Code != http.StatusOK {
		b.Fatalf("warm status = %d", warm.Code)
	}
	etag := warm.Header().Get("ETag")
	req := httptest.NewRequest(http.MethodGet, "/api/jobs?limit=50", nil)
	if etag != "" {
		req.Header.Set("If-None-Match", etag)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		response := httptest.NewRecorder()
		router.ServeHTTP(response, req)
		if response.Code != http.StatusOK && response.Code != http.StatusNotModified {
			b.Fatalf("status = %d, want 200 or 304", response.Code)
		}
		benchmarkJobsListBodyBytes = response.Body.Len()
	}
	_ = h
	b.ReportMetric(float64(benchmarkJobsListBodyBytes), "body-B")
}

func BenchmarkListJobsFirstPoll(b *testing.B) {
	h, router := benchmarkJobsListRouter(50)
	req := httptest.NewRequest(http.MethodGet, "/api/jobs?limit=50", nil)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		response := httptest.NewRecorder()
		router.ServeHTTP(response, req)
		if response.Code != http.StatusOK {
			b.Fatalf("status = %d, want 200", response.Code)
		}
		benchmarkJobsListBodyBytes = response.Body.Len()
	}
	_ = h
	b.ReportMetric(float64(benchmarkJobsListBodyBytes), "body-B")
}

func benchmarkJobsListRouter(n int) (*Handlers, *chi.Mux) {
	repo := newFakeRepo()
	store := newFakeStorage()
	now := time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)
	for i := 0; i < n; i++ {
		id := uuid.MustParse(fmt.Sprintf("11111111-1111-4111-8111-%012d", i))
		repo.jobs[id] = job.Job{
			ID:            id,
			Status:        job.StatusParsed,
			DemoFileName:  fmt.Sprintf("match-%02d.dem", i),
			TargetSteamID: "76561198000000002",
			CreatedAt:     now,
			UpdatedAt:     now,
		}
		roster, err := json.Marshal(rosterArtifact{
			Players: []rosterPlayer{{
				SteamID64: "76561198000000002",
				Name:      "donk",
				Team:      "T",
				Kills:     20 + i,
				Deaths:    10,
			}},
			Match: rosterMatch{Map: "de_mirage", ScoreCT: 9, ScoreT: 13, Rounds: 22},
		})
		if err != nil {
			panic(err)
		}
		if err := store.Put(artifacts.RosterKey(id), bytes.NewReader(roster)); err != nil {
			panic(err)
		}
	}
	h := NewHandlers(repo, store, &fakeQueue{})
	router := chi.NewRouter()
	router.Get("/api/jobs", h.ListJobs)
	return h, router
}
