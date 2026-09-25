package store

import (
	"context"
	"math"
	"sort"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/job"
	"github.com/rechedev9/cliphub/internal/killplan"
	"github.com/rechedev9/cliphub/internal/rules"
)

func TestMemoryJobRepositoryListBySeriesBreaksCreatedAtTiesByID(t *testing.T) {
	repo := NewMemoryJobRepository()
	ctx := context.Background()
	series := uuid.NewString()

	var ids []string
	for range 2 {
		j := &job.Job{Status: job.StatusQueued, SeriesID: series}
		if err := repo.Create(ctx, j); err != nil {
			t.Fatalf("Create: %v", err)
		}
		ids = append(ids, j.ID.String())
	}
	// Force an identical CreatedAt on both jobs (white-box: the repo is in this
	// package) so only the id tie-break decides the order.
	tie := time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC)
	for _, raw := range ids {
		id := uuid.MustParse(raw)
		j := repo.jobs[id]
		j.CreatedAt = tie
		repo.jobs[id] = j
	}
	sort.Strings(ids)

	got, err := repo.ListBySeries(ctx, series)
	if err != nil {
		t.Fatalf("ListBySeries: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("ListBySeries returned %d jobs, want 2", len(got))
	}
	for i, want := range ids {
		if got[i].ID.String() != want {
			t.Fatalf("ListBySeries[%d].ID = %s, want %s (id tie-break ascending)", i, got[i].ID, want)
		}
	}
}

// The memory repository clones through JSON like the SQLite row; a plan that
// cannot be encoded must be refused at write time, never panic on read.
func TestMemoryJobRepositorySetKillPlanRejectsNonFiniteFloats(t *testing.T) {
	repo := NewMemoryJobRepository()
	ctx := context.Background()
	j := &job.Job{Status: job.StatusParsed, Rules: rules.Default()}
	if err := repo.Create(ctx, j); err != nil {
		t.Fatal(err)
	}
	plan := killplan.NewPlan()
	plan.Segments = []killplan.Segment{{ID: "seg-001", Kills: []killplan.Kill{{KillerPos: [3]float64{math.NaN(), 0, 0}}}}}
	if err := repo.SetKillPlan(ctx, j.ID, plan); err == nil {
		t.Fatal("SetKillPlan accepted a NaN position")
	}
	got, err := repo.Get(ctx, j.ID)
	if err != nil {
		t.Fatalf("Get after refused plan: %v", err)
	}
	if got.KillPlan != nil {
		t.Fatal("refused plan was stored")
	}
}
