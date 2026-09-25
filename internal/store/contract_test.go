package store

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/job"
	"github.com/rechedev9/cliphub/internal/killplan"
	"github.com/rechedev9/cliphub/internal/rules"
)

// The memory repository backs ZV_DATABASE_URL=memory and most handler tests;
// SQLite backs production. Every behavioral difference between them is a bug
// the test suite cannot see, so each contract below runs against both.
func jobRepositoriesUnderTest(t *testing.T) map[string]JobRepository {
	t.Helper()
	return map[string]JobRepository{
		"memory": NewMemoryJobRepository(),
		"sqlite": newTestSQLiteRepo(t),
	}
}

func contractJob(status job.Status, segments ...string) *job.Job {
	plan := killplan.NewPlan()
	for _, id := range segments {
		plan.Segments = append(plan.Segments, killplan.Segment{ID: id, TickStart: 1, TickEnd: 2})
	}
	return &job.Job{
		Status:        status,
		DemoPath:      "demos/x.dem",
		DemoSHA256:    "sha",
		TargetSteamID: "76561198000000000",
		Rules:         rules.Default(),
		KillPlan:      &plan,
	}
}

func TestJobRepositoryContract(t *testing.T) {
	ctx := context.Background()
	cases := []struct {
		name string
		run  func(t *testing.T, repo JobRepository)
	}{
		{
			name: "Create assigns identity and SetKillPlan is returned by Get",
			run: func(t *testing.T, repo JobRepository) {
				j := &job.Job{Status: job.StatusQueued, DemoPath: "m.dem", DemoSHA256: "abc", Rules: rules.Default()}
				if err := repo.Create(ctx, j); err != nil {
					t.Fatal(err)
				}
				if j.ID == uuid.Nil || j.CreatedAt.IsZero() || j.UpdatedAt.IsZero() {
					t.Fatalf("Create left id/timestamps unset: %+v", j)
				}
				plan := killplan.NewPlan()
				plan.Segments = []killplan.Segment{{ID: "seg-001", TickStart: 64, TickEnd: 128}}
				if err := repo.SetKillPlan(ctx, j.ID, plan); err != nil {
					t.Fatal(err)
				}
				if err := repo.UpdateStatus(ctx, j.ID, job.StatusParsed, ""); err != nil {
					t.Fatal(err)
				}
				got, err := repo.Get(ctx, j.ID)
				if err != nil {
					t.Fatal(err)
				}
				if got.Status != job.StatusParsed || got.DemoPath != "m.dem" || got.KillPlan == nil || len(got.KillPlan.Segments) != 1 || got.KillPlan.Segments[0].ID != "seg-001" {
					t.Fatalf("Get = %+v, want parsed m.dem with seg-001", got)
				}
				status, reason, segments, err := repo.GetStatus(ctx, j.ID)
				if err != nil || status != job.StatusParsed || reason != "" || segments != 0 {
					t.Fatalf("GetStatus = %s/%q/%d/%v, want parsed/empty/0 (segments only while recording)", status, reason, segments, err)
				}
			},
		},
		{
			name: "ListBySeries returns only the series in upload order without kill plans",
			run: func(t *testing.T, repo JobRepository) {
				series := uuid.NewString()
				if got, err := repo.ListBySeries(ctx, series); err != nil || len(got) != 0 {
					t.Fatalf("ListBySeries(unknown) = %v, %v; want no jobs", got, err)
				}
				var seriesIDs []uuid.UUID
				for range 3 {
					j := &job.Job{Status: job.StatusQueued, SeriesID: series}
					if err := repo.Create(ctx, j); err != nil {
						t.Fatal(err)
					}
					seriesIDs = append(seriesIDs, j.ID)
					time.Sleep(2 * time.Millisecond)
				}
				// A different series and a standalone job must be excluded.
				if err := repo.Create(ctx, &job.Job{Status: job.StatusQueued, SeriesID: uuid.NewString()}); err != nil {
					t.Fatal(err)
				}
				if err := repo.Create(ctx, &job.Job{Status: job.StatusQueued}); err != nil {
					t.Fatal(err)
				}
				if err := repo.SetKillPlan(ctx, seriesIDs[0], killplan.NewPlan()); err != nil {
					t.Fatal(err)
				}
				got, err := repo.ListBySeries(ctx, series)
				if err != nil {
					t.Fatal(err)
				}
				if len(got) != len(seriesIDs) {
					t.Fatalf("ListBySeries returned %d jobs, want %d", len(got), len(seriesIDs))
				}
				for i, id := range seriesIDs {
					if got[i].ID != id || got[i].SeriesID != series || got[i].KillPlan != nil {
						t.Fatalf("ListBySeries[%d] = id %s series %q plan %v, want id %s (upload order), series %q, no plan", i, got[i].ID, got[i].SeriesID, got[i].KillPlan != nil, id, series)
					}
				}
			},
		},
		{
			name: "Delete removes the job from Get and ListBySeries",
			run: func(t *testing.T, repo JobRepository) {
				series := uuid.NewString()
				j := &job.Job{Status: job.StatusDone, SeriesID: series}
				if err := repo.Create(ctx, j); err != nil {
					t.Fatal(err)
				}
				if err := repo.SetKillPlan(ctx, j.ID, killplan.NewPlan()); err != nil {
					t.Fatal(err)
				}
				if err := repo.Delete(ctx, j.ID); err != nil {
					t.Fatal(err)
				}
				if _, err := repo.Get(ctx, j.ID); !errors.Is(err, job.ErrNotFound) {
					t.Fatalf("Get after Delete = %v, want ErrNotFound", err)
				}
				if got, err := repo.ListBySeries(ctx, series); err != nil || len(got) != 0 {
					t.Fatalf("ListBySeries after Delete = %v, %v; want no jobs", got, err)
				}
			},
		},
		{
			name: "Create refuses an existing id",
			run: func(t *testing.T, repo JobRepository) {
				j := contractJob(job.StatusParsed)
				if err := repo.Create(ctx, j); err != nil {
					t.Fatal(err)
				}
				dup := contractJob(job.StatusParsed)
				dup.ID = j.ID
				if err := repo.Create(ctx, dup); err == nil {
					t.Fatal("second Create with the same id succeeded")
				}
			},
		},
		{
			name: "Get returns a deep copy the caller cannot corrupt",
			run: func(t *testing.T, repo JobRepository) {
				j := contractJob(job.StatusParsed, "seg-001", "seg-002")
				j.Rules.Weapons = []string{"ak47"}
				if err := repo.Create(ctx, j); err != nil {
					t.Fatal(err)
				}
				got, err := repo.Get(ctx, j.ID)
				if err != nil {
					t.Fatal(err)
				}
				got.KillPlan.Segments[0].ID = "corrupted"
				got.Rules.Weapons[0] = "corrupted"
				again, err := repo.Get(ctx, j.ID)
				if err != nil {
					t.Fatal(err)
				}
				if again.KillPlan.Segments[0].ID != "seg-001" || again.Rules.Weapons[0] != "ak47" {
					t.Fatalf("stored job aliased the returned value: %+v", again)
				}
			},
		},
		{
			name: "GetMeta, List and ListByStatus strip the kill plan",
			run: func(t *testing.T, repo JobRepository) {
				j := contractJob(job.StatusRecorded, "seg-001")
				if err := repo.Create(ctx, j); err != nil {
					t.Fatal(err)
				}
				meta, err := repo.GetMeta(ctx, j.ID)
				if err != nil {
					t.Fatal(err)
				}
				listed, err := repo.List(ctx, 10)
				if err != nil {
					t.Fatal(err)
				}
				byStatus, err := repo.ListByStatus(ctx, job.StatusRecorded)
				if err != nil {
					t.Fatal(err)
				}
				if meta.KillPlan != nil || len(listed) != 1 || listed[0].KillPlan != nil || len(byStatus) != 1 || byStatus[0].KillPlan != nil {
					t.Fatalf("kill plan leaked into a metadata read: meta=%v list=%v byStatus=%v", meta.KillPlan != nil, listed[0].KillPlan != nil, byStatus[0].KillPlan != nil)
				}
			},
		},
		{
			name: "GetStatus exposes the failure reason only while failed and segments only while recording",
			run: func(t *testing.T, repo JobRepository) {
				j := contractJob(job.StatusParsed, "seg-001", "seg-002", "seg-003")
				if err := repo.Create(ctx, j); err != nil {
					t.Fatal(err)
				}
				if err := repo.UpdateStatus(ctx, j.ID, job.StatusFailed, "capture_flake: lost POV"); err != nil {
					t.Fatal(err)
				}
				status, reason, segments, err := repo.GetStatus(ctx, j.ID)
				if err != nil || status != job.StatusFailed || reason != "capture_flake: lost POV" || segments != 0 {
					t.Fatalf("failed: status=%s reason=%q segments=%d err=%v", status, reason, segments, err)
				}
				if err := repo.UpdateStatus(ctx, j.ID, job.StatusRecording, "stale reason must not leak"); err != nil {
					t.Fatal(err)
				}
				status, reason, segments, err = repo.GetStatus(ctx, j.ID)
				if err != nil || status != job.StatusRecording || reason != "" || segments != 3 {
					t.Fatalf("recording: status=%s reason=%q segments=%d err=%v", status, reason, segments, err)
				}
			},
		},
		{
			name: "GetStatuses returns the same rows as one GetStatus per id and omits an unknown id",
			run: func(t *testing.T, repo JobRepository) {
				failed := contractJob(job.StatusFailed, "seg-001")
				recording := contractJob(job.StatusParsed, "seg-001", "seg-002", "seg-003")
				parsed := contractJob(job.StatusParsed)
				for _, j := range []*job.Job{failed, recording, parsed} {
					if err := repo.Create(ctx, j); err != nil {
						t.Fatal(err)
					}
				}
				if err := repo.UpdateStatus(ctx, failed.ID, job.StatusFailed, "capture_flake: lost POV"); err != nil {
					t.Fatal(err)
				}
				if err := repo.UpdateStatus(ctx, recording.ID, job.StatusRecording, "stale reason must not leak"); err != nil {
					t.Fatal(err)
				}
				missing := uuid.New()
				// The same id twice: one job can be polled under two variants.
				ids := []uuid.UUID{failed.ID, recording.ID, parsed.ID, missing, recording.ID}
				rows, err := repo.GetStatuses(ctx, ids)
				if err != nil {
					t.Fatalf("GetStatuses: %v", err)
				}
				if len(rows) != 3 {
					t.Fatalf("rows = %d, want 3 (the unknown id is omitted, not an error): %+v", len(rows), rows)
				}
				if _, ok := rows[missing]; ok {
					t.Fatalf("unknown id %s is in the map: %+v", missing, rows[missing])
				}
				for _, id := range []uuid.UUID{failed.ID, recording.ID, parsed.ID} {
					status, reason, segments, err := repo.GetStatus(ctx, id)
					if err != nil {
						t.Fatalf("GetStatus %s: %v", id, err)
					}
					want := job.StatusRow{Status: status, FailureReason: reason, SegmentCount: segments}
					if got := rows[id]; got != want {
						t.Fatalf("GetStatuses[%s] = %+v, want the single read %+v", id, got, want)
					}
				}
				empty, err := repo.GetStatuses(ctx, nil)
				if err != nil || len(empty) != 0 {
					t.Fatalf("GetStatuses(nil) = %+v, %v; want an empty map and no error", empty, err)
				}
			},
		},
		{
			name: "GetStatuses reads more ids than one IN list holds",
			run: func(t *testing.T, repo JobRepository) {
				// The batch-status endpoint caps items at 100; the repository
				// chunks anyway so a larger caller cannot exhaust SQLite's
				// bound-variable limit or silently drop the tail.
				ids := make([]uuid.UUID, 0, 205)
				for range 205 {
					j := contractJob(job.StatusParsed)
					if err := repo.Create(ctx, j); err != nil {
						t.Fatal(err)
					}
					ids = append(ids, j.ID)
				}
				rows, err := repo.GetStatuses(ctx, ids)
				if err != nil {
					t.Fatalf("GetStatuses: %v", err)
				}
				if len(rows) != len(ids) {
					t.Fatalf("rows = %d, want %d", len(rows), len(ids))
				}
				for _, id := range ids {
					if rows[id].Status != job.StatusParsed {
						t.Fatalf("row %s = %+v, want parsed", id, rows[id])
					}
				}
			},
		},
		{
			name: "ListByStatus orders newest update first like List",
			run: func(t *testing.T, repo JobRepository) {
				first := contractJob(job.StatusParsed)
				second := contractJob(job.StatusParsed)
				for _, j := range []*job.Job{first, second} {
					if err := repo.Create(ctx, j); err != nil {
						t.Fatal(err)
					}
					time.Sleep(2 * time.Millisecond)
				}
				if err := repo.UpdateStatus(ctx, first.ID, job.StatusParsed, ""); err != nil {
					t.Fatal(err)
				}
				got, err := repo.ListByStatus(ctx, job.StatusParsed)
				if err != nil {
					t.Fatal(err)
				}
				if len(got) != 2 || got[0].ID != first.ID || got[1].ID != second.ID {
					t.Fatalf("order = %v, want most recently updated first (%s, %s)", ids(got), first.ID, second.ID)
				}
				if empty, err := repo.ListByStatus(ctx, job.StatusDone); err != nil || empty == nil || len(empty) != 0 {
					t.Fatalf("empty ListByStatus = %v (%v), want non-nil empty slice", empty, err)
				}
			},
		},
		{
			name: "UpdateStatus, Delete, Get and GetStatus report a missing job consistently",
			run: func(t *testing.T, repo JobRepository) {
				if err := repo.UpdateStatus(ctx, uuid.New(), job.StatusFailed, "x"); !errors.Is(err, job.ErrNotFound) {
					t.Fatalf("UpdateStatus unknown = %v, want ErrNotFound", err)
				}
				if err := repo.Delete(ctx, uuid.New()); err != nil {
					t.Fatalf("Delete unknown = %v, want idempotent nil", err)
				}
				if _, err := repo.Get(ctx, uuid.New()); !errors.Is(err, job.ErrNotFound) {
					t.Fatalf("Get unknown = %v, want ErrNotFound", err)
				}
				if _, _, _, err := repo.GetStatus(ctx, uuid.New()); !errors.Is(err, job.ErrNotFound) {
					t.Fatalf("GetStatus unknown = %v, want ErrNotFound", err)
				}
			},
		},
	}
	for _, tc := range cases {
		for name, repo := range jobRepositoriesUnderTest(t) {
			t.Run(tc.name+"/"+name, func(t *testing.T) { tc.run(t, repo) })
		}
	}
}

func ids(jobs []job.Job) []uuid.UUID {
	out := make([]uuid.UUID, len(jobs))
	for i, j := range jobs {
		out[i] = j.ID
	}
	return out
}
