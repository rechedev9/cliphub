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
			name: "UpdateStatus and Delete report a missing job consistently",
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
