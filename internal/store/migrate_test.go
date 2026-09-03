package store

import (
	"context"
	"database/sql"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rechedev9/cliphub/internal/job"
	"github.com/rechedev9/cliphub/internal/killplan"
	"github.com/rechedev9/cliphub/internal/rules"
)

func TestOpenSQLiteBringsSchemaToCurrentVersionAndIsIdempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "jobs.db")
	for round := range 2 {
		db, err := openSQLite(path)
		if err != nil {
			t.Fatalf("round %d openSQLite: %v", round, err)
		}
		version, err := schemaVersion(db)
		if err != nil {
			t.Fatal(err)
		}
		if version != len(migrations) {
			t.Fatalf("round %d: user_version = %d, want %d", round, version, len(migrations))
		}
		var foreignKeys int
		if err := db.QueryRow(`PRAGMA foreign_keys`).Scan(&foreignKeys); err != nil {
			t.Fatal(err)
		}
		if foreignKeys != 1 {
			t.Fatal("foreign_keys pragma is off; the kill plan cascade would be decorative")
		}
		for _, index := range []string{"jobs_recent", "jobs_status_recent", "jobs_series", "stream_jobs_recent", "editor_assets_sha256", "editor_projects_status_recent"} {
			var name string
			err := db.QueryRow(`SELECT name FROM sqlite_master WHERE type = 'index' AND name = ?`, index).Scan(&name)
			if err != nil {
				t.Fatalf("round %d: index %s missing: %v", round, index, err)
			}
		}
		if err := db.Close(); err != nil {
			t.Fatal(err)
		}
	}
}

func TestOpenSQLiteRefusesADatabaseFromANewerBuild(t *testing.T) {
	path := filepath.Join(t.TempDir(), "jobs.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`PRAGMA user_version = 99`); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := openSQLite(path); err == nil || !strings.Contains(err.Error(), "newer than this build") {
		t.Fatalf("openSQLite error = %v, want newer-schema refusal", err)
	}
}

func TestKillPlansCascadeWithJobRow(t *testing.T) {
	repo := newTestSQLiteRepo(t)
	ctx := context.Background()
	plan := killplan.NewPlan()
	j := &job.Job{Status: job.StatusParsed, DemoPath: "demos/x.dem", DemoSHA256: "sha", TargetSteamID: "76561198000000000", Rules: rules.Default(), KillPlan: &plan}
	if err := repo.Create(ctx, j); err != nil {
		t.Fatal(err)
	}
	var plans int
	if err := repo.db.QueryRow(`SELECT COUNT(*) FROM job_kill_plans WHERE job_id = ?`, j.ID.String()).Scan(&plans); err != nil {
		t.Fatal(err)
	}
	if plans != 1 {
		t.Fatalf("kill plan rows = %d, want 1", plans)
	}
	// A raw row delete (not the repository's two-statement Delete) must not
	// leave an orphan: that is what the foreign key is for.
	if _, err := repo.db.Exec(`DELETE FROM jobs WHERE id = ?`, j.ID.String()); err != nil {
		t.Fatal(err)
	}
	if err := repo.db.QueryRow(`SELECT COUNT(*) FROM job_kill_plans WHERE job_id = ?`, j.ID.String()).Scan(&plans); err != nil {
		t.Fatal(err)
	}
	if plans != 0 {
		t.Fatalf("orphan kill plan rows = %d after job delete", plans)
	}
	if _, err := repo.db.Exec(`INSERT INTO job_kill_plans (job_id, plan) VALUES ('missing-job', '{}')`); err == nil {
		t.Fatal("inserted a kill plan for a job that does not exist")
	}
}

func TestMigrateV2DropsOrphanKillPlansFromLegacyDatabase(t *testing.T) {
	path := filepath.Join(t.TempDir(), "jobs.db")
	legacy, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	for _, stmt := range []string{
		`CREATE TABLE jobs (id TEXT PRIMARY KEY, data BLOB NOT NULL, kill_plan BLOB, status TEXT NOT NULL, created_at INTEGER NOT NULL, updated_at INTEGER NOT NULL)`,
		`CREATE TABLE job_kill_plans (job_id TEXT PRIMARY KEY, plan BLOB NOT NULL)`,
		`INSERT INTO jobs (id, data, status, created_at, updated_at) VALUES ('kept', '{"id":"kept","status":"parsed"}', 'parsed', 1, 1)`,
		`INSERT INTO job_kill_plans (job_id, plan) VALUES ('kept', '{}'), ('orphan', '{}')`,
	} {
		if _, err := legacy.Exec(stmt); err != nil {
			t.Fatalf("%s: %v", stmt, err)
		}
	}
	if err := legacy.Close(); err != nil {
		t.Fatal(err)
	}

	db, err := openSQLite(path)
	if err != nil {
		t.Fatalf("openSQLite: %v", err)
	}
	defer db.Close()
	var ids []string
	rows, err := db.Query(`SELECT job_id FROM job_kill_plans ORDER BY job_id`)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
	}
	if err := rows.Close(); err != nil {
		t.Fatal(err)
	}
	if len(ids) != 1 || ids[0] != "kept" {
		t.Fatalf("kill plan rows after migrate = %v, want only kept", ids)
	}
	hasLegacyColumn, err := columnExists(db, "jobs", "kill_plan")
	if err != nil {
		t.Fatal(err)
	}
	if hasLegacyColumn {
		t.Fatal("legacy jobs.kill_plan column survived migration")
	}
}

// database/sql drops a connection the driver marks bad (an interrupted
// statement does that) and opens a fresh one; per-connection pragmas must be
// part of the DSN so the replacement still enforces foreign keys.
func TestOpenSQLitePragmasSurviveConnectionReplacement(t *testing.T) {
	db, err := openSQLite(filepath.Join(t.TempDir(), "jobs.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	// Interrupted statement: the driver flags the connection and the pool
	// replaces it on the next use.
	_, _ = db.ExecContext(cancelled, `SELECT 1`)
	var foreignKeys, busyTimeout int
	if err := db.QueryRow(`PRAGMA foreign_keys`).Scan(&foreignKeys); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`PRAGMA busy_timeout`).Scan(&busyTimeout); err != nil {
		t.Fatal(err)
	}
	if foreignKeys != 1 || busyTimeout != 5000 {
		t.Fatalf("after connection replacement foreign_keys=%d busy_timeout=%d, want 1 and 5000", foreignKeys, busyTimeout)
	}
}
