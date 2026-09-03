package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/job"
	"github.com/rechedev9/cliphub/internal/killplan"
	"github.com/rechedev9/cliphub/internal/rules"
)

func newTestSQLiteRepo(t *testing.T) *SQLiteJobRepository {
	t.Helper()
	repo, err := NewSQLiteJobRepository(filepath.Join(t.TempDir(), "jobs.db"))
	if err != nil {
		t.Fatalf("NewSQLiteJobRepository: %v", err)
	}
	t.Cleanup(func() { _ = repo.Close() })
	return repo
}

func TestSQLiteRepoCreateAndGet(t *testing.T) {
	repo := newTestSQLiteRepo(t)
	ctx := context.Background()

	j := &job.Job{Status: job.StatusScanned, DemoPath: "m.dem", DemoSHA256: "abc"}
	if err := repo.Create(ctx, j); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if j.ID == uuid.Nil {
		t.Fatal("Create did not assign an id")
	}
	if j.CreatedAt.IsZero() || j.UpdatedAt.IsZero() {
		t.Fatal("Create did not set timestamps")
	}

	got, err := repo.Get(ctx, j.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.ID != j.ID || got.DemoPath != "m.dem" || got.Status != job.StatusScanned {
		t.Fatalf("Get: got %+v, want id=%s demo=m.dem status=scanned", got, j.ID)
	}

	if _, err := repo.Get(ctx, uuid.New()); !errors.Is(err, job.ErrNotFound) {
		t.Fatalf("Get unknown: got %v, want ErrNotFound", err)
	}
}

func TestSQLiteRepoGetMetaAndListStripKillPlan(t *testing.T) {
	repo := newTestSQLiteRepo(t)
	ctx := context.Background()

	j := &job.Job{Status: job.StatusScanned}
	if err := repo.Create(ctx, j); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := repo.SetKillPlan(ctx, j.ID, killplan.Plan{}); err != nil {
		t.Fatalf("SetKillPlan: %v", err)
	}

	full, err := repo.Get(ctx, j.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if full.KillPlan == nil {
		t.Fatal("Get: want KillPlan set, got nil")
	}

	meta, err := repo.GetMeta(ctx, j.ID)
	if err != nil {
		t.Fatalf("GetMeta: %v", err)
	}
	if meta.KillPlan != nil {
		t.Fatal("GetMeta: want KillPlan nil, got non-nil")
	}

	list, err := repo.List(ctx, 10)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("List: got %d jobs, want 1", len(list))
	}
	if list[0].KillPlan != nil {
		t.Fatal("List: want KillPlan nil, got non-nil")
	}

	byStatus, err := repo.ListByStatus(ctx, job.StatusScanned)
	if err != nil {
		t.Fatalf("ListByStatus: %v", err)
	}
	if len(byStatus) != 1 {
		t.Fatalf("ListByStatus: got %d jobs, want 1", len(byStatus))
	}
	if byStatus[0].KillPlan != nil {
		t.Fatal("ListByStatus: want KillPlan nil, got non-nil")
	}
}

func TestSQLiteRepoGetStatusReturnsOnlyLifecycleSummary(t *testing.T) {
	repo := newTestSQLiteRepo(t)
	ctx := context.Background()
	plan := killplan.NewPlan()
	plan.Segments = []killplan.Segment{{ID: "s1"}, {ID: "s2"}, {ID: "s3"}}
	j := &job.Job{Status: job.StatusRecording, KillPlan: &plan}
	if err := repo.Create(ctx, j); err != nil {
		t.Fatalf("Create: %v", err)
	}

	status, reason, segments, err := repo.GetStatus(ctx, j.ID)
	if err != nil {
		t.Fatalf("GetStatus recording: %v", err)
	}
	if status != job.StatusRecording || reason != "" || segments != 3 {
		t.Fatalf("recording status = %s/%q/%d, want recording/empty/3", status, reason, segments)
	}

	if err := repo.UpdateStatus(ctx, j.ID, job.StatusFailed, "capture failed"); err != nil {
		t.Fatalf("UpdateStatus: %v", err)
	}
	status, reason, segments, err = repo.GetStatus(ctx, j.ID)
	if err != nil {
		t.Fatalf("GetStatus failed: %v", err)
	}
	if status != job.StatusFailed || reason != "capture failed" || segments != 0 {
		t.Fatalf("failed status = %s/%q/%d, want failed/capture failed/0", status, reason, segments)
	}

	if _, _, _, err := repo.GetStatus(ctx, uuid.New()); !errors.Is(err, job.ErrNotFound) {
		t.Fatalf("GetStatus unknown error = %v, want ErrNotFound", err)
	}
}

func TestSQLiteRepoLargePlanGetMetaStripsSegmentsGetStatusSkipsPlan(t *testing.T) {
	repo := newTestSQLiteRepo(t)
	ctx := context.Background()
	plan := killplan.NewPlan()
	plan.Segments = make([]killplan.Segment, 240)
	for i := range plan.Segments {
		plan.Segments[i].ID = killplan.FormatSegmentID(i + 1)
		plan.Segments[i].Kills = make([]killplan.Kill, 8)
		for k := range plan.Segments[i].Kills {
			plan.Segments[i].Kills[k] = killplan.Kill{
				Tick:   i*640 + k*64,
				Weapon: "ak47",
				Victim: killplan.Player{SteamID64: "76561198000000000", NameInDemo: "player", TeamAtKill: "CT"},
			}
		}
	}
	j := &job.Job{Status: job.StatusParsed, KillPlan: &plan}
	if err := repo.Create(ctx, j); err != nil {
		t.Fatalf("Create: %v", err)
	}

	full, err := repo.Get(ctx, j.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if full.KillPlan == nil || len(full.KillPlan.Segments) != len(plan.Segments) {
		t.Fatalf("Get segments = %v, want %d", full.KillPlan, len(plan.Segments))
	}

	meta, err := repo.GetMeta(ctx, j.ID)
	if err != nil {
		t.Fatalf("GetMeta: %v", err)
	}
	if meta.KillPlan != nil {
		t.Fatal("GetMeta returned kill plan")
	}
	if meta.Status != job.StatusParsed || meta.ID != j.ID {
		t.Fatalf("GetMeta = %+v, want parsed id=%s", meta, j.ID)
	}

	status, reason, segments, err := repo.GetStatus(ctx, j.ID)
	if err != nil {
		t.Fatalf("GetStatus: %v", err)
	}
	if status != job.StatusParsed || reason != "" || segments != 0 {
		t.Fatalf("GetStatus parsed = %s/%q/%d, want parsed/empty/0 (segment count only while recording)", status, reason, segments)
	}

	list, err := repo.List(ctx, 10)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 1 || list[0].ID != j.ID || list[0].Status != job.StatusParsed {
		t.Fatalf("List = %+v, want parsed id=%s", list, j.ID)
	}
	if list[0].KillPlan != nil {
		t.Fatal("List returned kill plan")
	}
}

func TestSQLiteRepoListOrdersByUpdatedThenLimits(t *testing.T) {
	repo := newTestSQLiteRepo(t)
	ctx := context.Background()

	var ids []uuid.UUID
	for range 3 {
		j := &job.Job{Status: job.StatusScanned}
		if err := repo.Create(ctx, j); err != nil {
			t.Fatalf("Create: %v", err)
		}
		ids = append(ids, j.ID)
		time.Sleep(2 * time.Millisecond) // distinct created/updated timestamps
	}
	// Touch the first-created job so it becomes the most-recently-updated.
	if err := repo.UpdateStatus(ctx, ids[0], job.StatusParsing, ""); err != nil {
		t.Fatalf("UpdateStatus: %v", err)
	}

	list, err := repo.List(ctx, 10)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 3 {
		t.Fatalf("List: got %d, want 3", len(list))
	}
	if list[0].ID != ids[0] {
		t.Fatalf("List order: got head %s, want the just-updated %s", list[0].ID, ids[0])
	}

	limited, err := repo.List(ctx, 2)
	if err != nil {
		t.Fatalf("List limited: %v", err)
	}
	if len(limited) != 2 {
		t.Fatalf("List limit: got %d, want 2", len(limited))
	}
}

func TestSQLiteRepoDelete(t *testing.T) {
	repo := newTestSQLiteRepo(t)
	ctx := context.Background()
	series := uuid.NewString()

	j := &job.Job{Status: job.StatusDone, SeriesID: series}
	if err := repo.Create(ctx, j); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := repo.SetKillPlan(ctx, j.ID, killplan.NewPlan()); err != nil {
		t.Fatalf("SetKillPlan: %v", err)
	}

	if err := repo.Delete(ctx, j.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := repo.Get(ctx, j.ID); !errors.Is(err, job.ErrNotFound) {
		t.Fatalf("Get after Delete: got %v, want ErrNotFound", err)
	}
	var leftover []byte
	if err := repo.db.QueryRow(`SELECT plan FROM job_kill_plans WHERE job_id = ?`, j.ID.String()).Scan(&leftover); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("job_kill_plans after Delete: got %v/%q, want ErrNoRows", err, leftover)
	}
	got, err := repo.ListBySeries(ctx, series)
	if err != nil {
		t.Fatalf("ListBySeries: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("ListBySeries after Delete returned %d jobs, want 0", len(got))
	}

	// Deleting a missing id is a no-op.
	if err := repo.Delete(ctx, uuid.New()); err != nil {
		t.Fatalf("Delete missing id: %v", err)
	}
}

func TestSQLiteRepoListBySeries(t *testing.T) {
	repo := newTestSQLiteRepo(t)
	ctx := context.Background()
	series := uuid.NewString()

	// An unknown series returns an empty, non-nil slice.
	got, err := repo.ListBySeries(ctx, series)
	if err != nil {
		t.Fatalf("ListBySeries empty: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("ListBySeries empty: got %d jobs, want 0", len(got))
	}

	// Three jobs in the series, created in order with distinct created_at.
	var ids []uuid.UUID
	for range 3 {
		j := &job.Job{Status: job.StatusQueued, SeriesID: series}
		if err := repo.Create(ctx, j); err != nil {
			t.Fatalf("Create: %v", err)
		}
		ids = append(ids, j.ID)
		time.Sleep(2 * time.Millisecond)
	}
	// A different series and a standalone job must be excluded.
	if err := repo.Create(ctx, &job.Job{Status: job.StatusQueued, SeriesID: uuid.NewString()}); err != nil {
		t.Fatalf("Create other series: %v", err)
	}
	if err := repo.Create(ctx, &job.Job{Status: job.StatusQueued}); err != nil {
		t.Fatalf("Create standalone: %v", err)
	}
	// A kill plan on a series job confirms ListBySeries strips it.
	if err := repo.SetKillPlan(ctx, ids[0], killplan.NewPlan()); err != nil {
		t.Fatalf("SetKillPlan: %v", err)
	}

	got, err = repo.ListBySeries(ctx, series)
	if err != nil {
		t.Fatalf("ListBySeries: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("ListBySeries: got %d jobs, want 3", len(got))
	}
	for i, id := range ids {
		if got[i].ID != id {
			t.Fatalf("ListBySeries[%d].ID = %s, want %s (upload order)", i, got[i].ID, id)
		}
		if got[i].SeriesID != series {
			t.Fatalf("ListBySeries[%d].SeriesID = %q, want %q", i, got[i].SeriesID, series)
		}
		if got[i].KillPlan != nil {
			t.Fatalf("ListBySeries[%d] carried a kill plan, want stripped", i)
		}
	}
}

func TestSQLiteRepoListBySeriesBreaksCreatedAtTiesByID(t *testing.T) {
	repo := newTestSQLiteRepo(t)
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
	// Force an identical created_at mirror column on both rows so only the id
	// tie-break decides the order.
	tie := time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC).UnixNano()
	if _, err := repo.db.ExecContext(ctx, `UPDATE jobs SET created_at = ? WHERE id IN (?, ?)`, tie, ids[0], ids[1]); err != nil {
		t.Fatalf("force equal created_at: %v", err)
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

func TestSQLiteRepoUpdateStatus(t *testing.T) {
	repo := newTestSQLiteRepo(t)
	ctx := context.Background()

	plan := killplan.NewPlan()
	plan.Segments = []killplan.Segment{{ID: "seg-001", TickStart: 64, TickEnd: 128}}
	j := &job.Job{Status: job.StatusRecording, KillPlan: &plan}
	if err := repo.Create(ctx, j); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := repo.UpdateStatus(ctx, j.ID, job.StatusFailed, "boom"); err != nil {
		t.Fatalf("UpdateStatus: %v", err)
	}
	got, err := repo.Get(ctx, j.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Status != job.StatusFailed || got.FailureReason != "boom" || got.FailureCode != "" {
		t.Fatalf("UpdateStatus: got status=%s reason=%q code=%q, want failed/boom/empty", got.Status, got.FailureReason, got.FailureCode)
	}
	if got.KillPlan == nil || len(got.KillPlan.Segments) != 1 || got.KillPlan.Segments[0].ID != "seg-001" {
		t.Fatalf("UpdateStatus changed kill plan: %#v", got.KillPlan)
	}
	var mirroredUpdatedAt int64
	if err := repo.db.QueryRowContext(ctx, `SELECT updated_at FROM jobs WHERE id = ?`, j.ID.String()).Scan(&mirroredUpdatedAt); err != nil {
		t.Fatalf("read mirrored updated_at: %v", err)
	}
	if got, want := mirroredUpdatedAt, got.UpdatedAt.UnixNano(); got != want {
		t.Fatalf("mirrored updated_at = %d, want JSON timestamp %d", got, want)
	}
	const missingPlate = `composite keydrop banner code "HUASO": keydrop banner style "jcorko" plate is missing`
	if err := repo.UpdateStatus(ctx, j.ID, job.StatusFailed, missingPlate); err != nil {
		t.Fatalf("UpdateStatus classified: %v", err)
	}
	got, err = repo.Get(ctx, j.ID)
	if err != nil {
		t.Fatalf("Get classified: %v", err)
	}
	if got.FailureReason != missingPlate || got.FailureCode != "missing_plate" {
		t.Fatalf("classified failure = reason %q code %q, want missing_plate", got.FailureReason, got.FailureCode)
	}
	if err := repo.UpdateStatus(ctx, j.ID, job.StatusDone, ""); err != nil {
		t.Fatalf("UpdateStatus clear failure: %v", err)
	}
	got, err = repo.Get(ctx, j.ID)
	if err != nil {
		t.Fatalf("Get after clear failure: %v", err)
	}
	if got.Status != job.StatusDone || got.FailureReason != "" || got.FailureCode != "" {
		t.Fatalf("UpdateStatus clear failure: got status=%s reason=%q code=%q, want done/empty", got.Status, got.FailureReason, got.FailureCode)
	}
	if got.KillPlan == nil || len(got.KillPlan.Segments) != 1 || got.KillPlan.Segments[0].ID != "seg-001" {
		t.Fatalf("UpdateStatus clear failure changed kill plan: %#v", got.KillPlan)
	}
	byFailed, err := repo.ListByStatus(ctx, job.StatusFailed)
	if err != nil {
		t.Fatalf("ListByStatus failed: %v", err)
	}
	if len(byFailed) != 0 {
		t.Fatalf("ListByStatus failed returned %d jobs after done transition, want 0", len(byFailed))
	}
	byDone, err := repo.ListByStatus(ctx, job.StatusDone)
	if err != nil {
		t.Fatalf("ListByStatus done: %v", err)
	}
	if len(byDone) != 1 || byDone[0].ID != j.ID {
		t.Fatalf("ListByStatus done = %+v, want job %s", byDone, j.ID)
	}
	if err := repo.UpdateStatus(ctx, uuid.New(), job.StatusDone, ""); !errors.Is(err, job.ErrNotFound) {
		t.Fatalf("UpdateStatus unknown: got %v, want ErrNotFound", err)
	}
}

func TestSQLiteRepoSetParseInputs(t *testing.T) {
	repo := newTestSQLiteRepo(t)
	ctx := context.Background()

	// scanned -> parsing is allowed and records the target.
	scanned := &job.Job{Status: job.StatusScanned}
	if err := repo.Create(ctx, scanned); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := repo.SetParseInputs(ctx, scanned.ID, "76561199237188983", rules.Rules{}); err != nil {
		t.Fatalf("SetParseInputs: %v", err)
	}
	got, err := repo.Get(ctx, scanned.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Status != job.StatusParsing || got.TargetSteamID != "76561199237188983" {
		t.Fatalf("SetParseInputs: got status=%s target=%q, want parsing/76561199237188983", got.Status, got.TargetSteamID)
	}

	// A job that was never scanned is a conflict, not a silent success.
	queued := &job.Job{Status: job.StatusQueued}
	if err := repo.Create(ctx, queued); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := repo.SetParseInputs(ctx, queued.ID, "1", rules.Rules{}); !errors.Is(err, job.ErrConflict) {
		t.Fatalf("SetParseInputs wrong state: got %v, want ErrConflict", err)
	}

	if err := repo.SetParseInputs(ctx, uuid.New(), "1", rules.Rules{}); !errors.Is(err, job.ErrNotFound) {
		t.Fatalf("SetParseInputs unknown: got %v, want ErrNotFound", err)
	}
}

// The whole point of the SQLite repo: job state outlives the process. Create a
// job, close the database, reopen the same file, and the job is still there.
func TestSQLiteRepoPersistsAcrossReopen(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "jobs.db")

	repo, err := NewSQLiteJobRepository(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	j := &job.Job{Status: job.StatusParsed, DemoPath: "keep.dem", TargetSteamID: "76561199237188983"}
	if err := repo.Create(ctx, j); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := repo.SetKillPlan(ctx, j.ID, killplan.Plan{}); err != nil {
		t.Fatalf("SetKillPlan: %v", err)
	}
	if err := repo.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	reopened, err := NewSQLiteJobRepository(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	t.Cleanup(func() { _ = reopened.Close() })

	got, err := reopened.Get(ctx, j.ID)
	if err != nil {
		t.Fatalf("Get after reopen: %v", err)
	}
	if got.DemoPath != "keep.dem" || got.TargetSteamID != "76561199237188983" || got.Status != job.StatusParsed {
		t.Fatalf("after reopen: got %+v, want demo=keep.dem status=parsed", got)
	}
	if got.KillPlan == nil {
		t.Fatal("after reopen: want KillPlan persisted, got nil")
	}
}

func TestSQLiteRepoStoresKillPlanOutsideJobDocument(t *testing.T) {
	repo := newTestSQLiteRepo(t)
	ctx := context.Background()
	plan := killplan.NewPlan()
	plan.Segments = []killplan.Segment{{ID: "seg-001", TickStart: 64, TickEnd: 128}}

	cases := []struct {
		name string
		act  func(*job.Job)
	}{
		{
			name: "create",
			act:  func(*job.Job) {},
		},
		{
			name: "update_status",
			act: func(j *job.Job) {
				if err := repo.UpdateStatus(ctx, j.ID, job.StatusFailed, "boom"); err != nil {
					t.Fatalf("UpdateStatus: %v", err)
				}
			},
		},
		{
			name: "set_kill_plan",
			act: func(j *job.Job) {
				next := killplan.NewPlan()
				next.Segments = []killplan.Segment{{ID: "seg-002", TickStart: 128, TickEnd: 256}}
				if err := repo.SetKillPlan(ctx, j.ID, next); err != nil {
					t.Fatalf("SetKillPlan: %v", err)
				}
			},
		},
		{
			name: "set_parse_inputs",
			act: func(j *job.Job) {
				if err := repo.UpdateStatus(ctx, j.ID, job.StatusParsed, ""); err != nil {
					t.Fatalf("UpdateStatus parsed: %v", err)
				}
				if err := repo.SetParseInputs(ctx, j.ID, "76561198000000000", rules.Default()); err != nil {
					t.Fatalf("SetParseInputs: %v", err)
				}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			j := &job.Job{Status: job.StatusParsed, KillPlan: &plan, DemoPath: "m.dem"}
			if err := repo.Create(ctx, j); err != nil {
				t.Fatalf("Create: %v", err)
			}
			tc.act(j)
			assertKillPlanOutsideData(t, repo, j.ID)
			got, err := repo.Get(ctx, j.ID)
			if err != nil {
				t.Fatalf("Get: %v", err)
			}
			if got.KillPlan == nil || len(got.KillPlan.Segments) == 0 {
				t.Fatalf("Get lost kill plan after %s: %#v", tc.name, got.KillPlan)
			}
		})
	}
}

func TestSQLiteRepoMigratesLegacyEmbeddedKillPlan(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "jobs.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open legacy db: %v", err)
	}
	if _, err := db.Exec(`CREATE TABLE jobs (
		id         TEXT PRIMARY KEY,
		data       BLOB    NOT NULL,
		status     TEXT    NOT NULL,
		created_at INTEGER NOT NULL,
		updated_at INTEGER NOT NULL
	)`); err != nil {
		_ = db.Close()
		t.Fatalf("create legacy table: %v", err)
	}
	id := uuid.New()
	plan := killplan.NewPlan()
	plan.Segments = []killplan.Segment{{ID: "seg-legacy", TickStart: 10, TickEnd: 20}}
	now := time.Now().UTC()
	legacy := job.Job{
		ID:        id,
		Status:    job.StatusParsed,
		DemoPath:  "legacy.dem",
		KillPlan:  &plan,
		CreatedAt: now,
		UpdatedAt: now,
	}
	data, err := json.Marshal(legacy)
	if err != nil {
		_ = db.Close()
		t.Fatalf("marshal legacy job: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO jobs (id, data, status, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`,
		id.String(), data, job.StatusParsed.String(), now.UnixNano(), now.UnixNano()); err != nil {
		_ = db.Close()
		t.Fatalf("insert legacy job: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close legacy db: %v", err)
	}

	repo, err := NewSQLiteJobRepository(path)
	if err != nil {
		t.Fatalf("open migrated repo: %v", err)
	}
	t.Cleanup(func() { _ = repo.Close() })

	got, err := repo.Get(ctx, id)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.DemoPath != "legacy.dem" || got.KillPlan == nil || len(got.KillPlan.Segments) != 1 || got.KillPlan.Segments[0].ID != "seg-legacy" {
		t.Fatalf("migrated Get = %#v, want legacy demo and seg-legacy", got)
	}
	assertKillPlanOutsideData(t, repo, id)

	meta, err := repo.GetMeta(ctx, id)
	if err != nil {
		t.Fatalf("GetMeta: %v", err)
	}
	if meta.KillPlan != nil {
		t.Fatal("GetMeta returned migrated kill plan")
	}

	if err := repo.UpdateStatus(ctx, id, job.StatusFailed, "after migrate"); err != nil {
		t.Fatalf("UpdateStatus: %v", err)
	}
	got, err = repo.Get(ctx, id)
	if err != nil {
		t.Fatalf("Get after UpdateStatus: %v", err)
	}
	if got.Status != job.StatusFailed || got.KillPlan == nil || got.KillPlan.Segments[0].ID != "seg-legacy" {
		t.Fatalf("post-migrate UpdateStatus dropped plan: %#v", got)
	}
	assertKillPlanOutsideData(t, repo, id)
}

func TestSQLiteRepoMigratesKillPlanColumnToSiblingTable(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "jobs.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open column-schema db: %v", err)
	}
	if _, err := db.Exec(`CREATE TABLE jobs (
		id         TEXT PRIMARY KEY,
		data       BLOB    NOT NULL,
		kill_plan  BLOB,
		status     TEXT    NOT NULL,
		created_at INTEGER NOT NULL,
		updated_at INTEGER NOT NULL
	)`); err != nil {
		_ = db.Close()
		t.Fatalf("create column-schema table: %v", err)
	}
	id := uuid.New()
	plan := killplan.NewPlan()
	plan.Segments = []killplan.Segment{{ID: "seg-column", TickStart: 10, TickEnd: 20}}
	now := time.Now().UTC()
	meta := job.Job{
		ID:        id,
		Status:    job.StatusParsed,
		DemoPath:  "column.dem",
		CreatedAt: now,
		UpdatedAt: now,
	}
	data, err := json.Marshal(meta)
	if err != nil {
		_ = db.Close()
		t.Fatalf("marshal job metadata: %v", err)
	}
	planJSON, err := json.Marshal(plan)
	if err != nil {
		_ = db.Close()
		t.Fatalf("marshal kill plan: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO jobs (id, data, kill_plan, status, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)`,
		id.String(), data, planJSON, job.StatusParsed.String(), now.UnixNano(), now.UnixNano()); err != nil {
		_ = db.Close()
		t.Fatalf("insert column-schema job: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close column-schema db: %v", err)
	}

	repo, err := NewSQLiteJobRepository(path)
	if err != nil {
		t.Fatalf("open migrated repo: %v", err)
	}
	t.Cleanup(func() { _ = repo.Close() })

	got, err := repo.Get(ctx, id)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.DemoPath != "column.dem" || got.KillPlan == nil || len(got.KillPlan.Segments) != 1 || got.KillPlan.Segments[0].ID != "seg-column" {
		t.Fatalf("migrated Get = %#v, want column.dem and seg-column", got)
	}
	assertKillPlanOutsideData(t, repo, id)

	list, err := repo.List(ctx, 10)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 1 || list[0].KillPlan != nil {
		t.Fatalf("List after column migrate = %+v, want one metadata row", list)
	}
}

func TestSQLiteRepoMigratesJSONNullKillPlanWithoutAbortingOpen(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "jobs.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open null-plan db: %v", err)
	}
	if _, err := db.Exec(`CREATE TABLE jobs (
		id         TEXT PRIMARY KEY,
		data       BLOB    NOT NULL,
		status     TEXT    NOT NULL,
		created_at INTEGER NOT NULL,
		updated_at INTEGER NOT NULL
	)`); err != nil {
		_ = db.Close()
		t.Fatalf("create jobs table: %v", err)
	}
	id := uuid.New()
	now := time.Now().UTC()
	data := []byte(`{"id":"` + id.String() + `","status":"parsed","demo_path":"null-plan.dem","kill_plan":null}`)
	if _, err := db.Exec(`INSERT INTO jobs (id, data, status, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`,
		id.String(), data, job.StatusParsed.String(), now.UnixNano(), now.UnixNano()); err != nil {
		_ = db.Close()
		t.Fatalf("insert json-null kill_plan job: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close null-plan db: %v", err)
	}

	repo, err := NewSQLiteJobRepository(path)
	if err != nil {
		t.Fatalf("open repo with json-null kill_plan: %v", err)
	}
	t.Cleanup(func() { _ = repo.Close() })

	got, err := repo.Get(ctx, id)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.DemoPath != "null-plan.dem" || got.KillPlan != nil {
		t.Fatalf("Get = %#v, want null-plan.dem and no kill plan", got)
	}
	var embedded sql.NullString
	if err := repo.db.QueryRow(`SELECT json_type(data, '$.kill_plan') FROM jobs WHERE id = ?`, id.String()).Scan(&embedded); err != nil {
		t.Fatalf("inspect data kill_plan: %v", err)
	}
	if embedded.Valid {
		t.Fatalf("data still embeds kill_plan json_type=%q", embedded.String)
	}
	var planBytes []byte
	if err := repo.db.QueryRow(`SELECT plan FROM job_kill_plans WHERE job_id = ?`, id.String()).Scan(&planBytes); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("job_kill_plans after json-null migrate: got %v/%q, want ErrNoRows", err, planBytes)
	}
}

func TestSQLiteRepoSetKillPlanMissingID(t *testing.T) {
	repo := newTestSQLiteRepo(t)
	ctx := context.Background()
	missing := uuid.New()
	if err := repo.SetKillPlan(ctx, missing, killplan.NewPlan()); !errors.Is(err, job.ErrNotFound) {
		t.Fatalf("SetKillPlan missing: got %v, want ErrNotFound", err)
	}
	var planBytes []byte
	if err := repo.db.QueryRow(`SELECT plan FROM job_kill_plans WHERE job_id = ?`, missing.String()).Scan(&planBytes); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("orphan job_kill_plans row after missing SetKillPlan: %v/%q", err, planBytes)
	}
}

func TestSQLiteRepoListAndStatusKeepLifecycleFieldsWithoutPlan(t *testing.T) {
	repo := newTestSQLiteRepo(t)
	ctx := context.Background()
	plan := killplan.NewPlan()
	plan.Segments = []killplan.Segment{{ID: "s1"}, {ID: "s2"}}

	cases := []struct {
		name           string
		status         job.Status
		reason         string
		wantSegments   int
		wantListReason string
	}{
		{name: "parsed_idle", status: job.StatusParsed, wantSegments: 0},
		{name: "recording", status: job.StatusRecording, wantSegments: 2},
		{name: "failed", status: job.StatusFailed, reason: "capture failed", wantSegments: 0, wantListReason: "capture failed"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			j := &job.Job{
				Status:        tc.status,
				FailureReason: tc.reason,
				DemoFileName:  "match.dem",
				KillPlan:      &plan,
			}
			if err := repo.Create(ctx, j); err != nil {
				t.Fatalf("Create: %v", err)
			}

			status, reason, segments, err := repo.GetStatus(ctx, j.ID)
			if err != nil {
				t.Fatalf("GetStatus: %v", err)
			}
			if status != tc.status || reason != tc.reason || segments != tc.wantSegments {
				t.Fatalf("GetStatus = %s/%q/%d, want %s/%q/%d", status, reason, segments, tc.status, tc.reason, tc.wantSegments)
			}

			list, err := repo.List(ctx, 100)
			if err != nil {
				t.Fatalf("List: %v", err)
			}
			var listed *job.Job
			for i := range list {
				if list[i].ID == j.ID {
					listed = &list[i]
					break
				}
			}
			if listed == nil {
				t.Fatalf("List missing job %s", j.ID)
			}
			if listed.KillPlan != nil {
				t.Fatal("List returned kill plan")
			}
			if listed.Status != tc.status || listed.FailureReason != tc.wantListReason || listed.DemoFileName != "match.dem" {
				t.Fatalf("List row = status=%s reason=%q file=%q, want %s/%q/match.dem", listed.Status, listed.FailureReason, listed.DemoFileName, tc.status, tc.wantListReason)
			}
			assertKillPlanOutsideData(t, repo, j.ID)
		})
	}
}

func assertKillPlanOutsideData(t *testing.T, repo *SQLiteJobRepository, id uuid.UUID) {
	t.Helper()
	var embedded sql.NullString
	if err := repo.db.QueryRow(`SELECT json_type(data, '$.kill_plan') FROM jobs WHERE id = ?`, id.String()).Scan(&embedded); err != nil {
		t.Fatalf("inspect data kill_plan: %v", err)
	}
	if embedded.Valid {
		t.Fatalf("data still embeds kill_plan json_type=%q", embedded.String)
	}
	var planBytes []byte
	if err := repo.db.QueryRow(`SELECT plan FROM job_kill_plans WHERE job_id = ?`, id.String()).Scan(&planBytes); err != nil {
		t.Fatalf("inspect job_kill_plans: %v", err)
	}
	if len(planBytes) == 0 {
		t.Fatal("job_kill_plans row is empty")
	}
	if !json.Valid(planBytes) {
		t.Fatalf("job_kill_plans.plan is not JSON: %q", planBytes)
	}
	// Schema v2 drops the leftover column outright.
	hasColumn, err := columnExists(repo.db, "jobs", "kill_plan")
	if err != nil {
		t.Fatalf("inspect kill_plan column: %v", err)
	}
	if hasColumn {
		t.Fatal("jobs.kill_plan column survived the schema migration")
	}
}
