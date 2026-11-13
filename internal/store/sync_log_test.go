package store

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"reflect"
	"slices"
	"testing"
)

func newTestSyncLog(t *testing.T) (*sql.DB, *SyncLog) {
	t.Helper()
	repo, err := NewSQLiteJobRepository(filepath.Join(t.TempDir(), "jobs.db"))
	if err != nil {
		t.Fatalf("NewSQLiteJobRepository: %v", err)
	}
	t.Cleanup(func() { _ = repo.Close() })
	return repo.DB(), NewSyncLog(repo.DB())
}

func schemaObjectExists(db *sql.DB, name string) bool {
	var found int
	err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE name = ?`, name).Scan(&found)
	return err == nil && found > 0
}

func rewindSyncLogToV25(t *testing.T, db *sql.DB) {
	t.Helper()
	for _, table := range syncTables {
		for _, event := range []string{"ai", "au", "ad", "ak"} {
			if _, err := db.Exec(`DROP TRIGGER IF EXISTS ` + syncTriggerName(table.name, event)); err != nil {
				t.Fatalf("drop trigger: %v", err)
			}
		}
	}
	for _, stmt := range []string{`DROP TABLE IF EXISTS sync_changes`, `DROP TABLE IF EXISTS sync_meta`, `PRAGMA user_version = 25`} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("%s: %v", stmt, err)
		}
	}
}

func seedSyncScenario(t *testing.T, db *sql.DB, demoPath string) {
	t.Helper()
	ctx := context.Background()
	exec := func(stmt string, args ...any) {
		t.Helper()
		if _, err := db.ExecContext(ctx, stmt, args...); err != nil {
			t.Fatalf("%s: %v", stmt, err)
		}
	}
	exec(`INSERT INTO library_demos (id, sha256, file_name, path, size_bytes, source, state, imported_at, updated_at) VALUES ('d1', 'sha1', 'a.dem', ?, 100, 'manual', 'ready', 1, 1)`, demoPath)
	exec(`INSERT INTO library_matches (id, demo_id, map, played_at, tickrate, duration_ticks, round_count, created_at, updated_at) VALUES ('m1', 'd1', 'de_dust2', 1000, 64.0, 100000, 24, 1, 1)`)
	exec(`INSERT INTO library_players (steam_id, nick, nick_key, created_at, updated_at) VALUES ('s1', 'Alice', 'alice', 1, 1)`)
	exec(`INSERT INTO library_players (steam_id, nick, nick_key, created_at, updated_at) VALUES ('s2', 'Bob', 'bob', 1, 1)`)
	exec(`INSERT INTO library_match_players (match_id, steam_id, team_key, slot, name, rounds, kills, deaths, assists, damage) VALUES ('m1', 's1', 'ct', 0, 'Alice', 24, 10, 8, 2, 500)`)
	exec(`INSERT INTO library_match_players (match_id, steam_id, team_key, slot, name, rounds, kills, deaths, assists, damage) VALUES ('m1', 's2', 't', 0, 'Bob', 24, 12, 9, 3, 600)`)
	exec(`INSERT INTO library_teams (id, name, name_key, created_at, updated_at) VALUES ('t1', 'Team A', 'team a', 1, 1)`)
	exec(`INSERT INTO library_playlists (id, name, created_at, updated_at) VALUES ('pl1', 'My List', 1, 1)`)
	exec(`INSERT INTO library_playlist_rounds (playlist_id, position, match_id, round) VALUES ('pl1', 0, 'm1', 1)`)
	exec(`INSERT INTO library_playlist_rounds (playlist_id, position, match_id, round) VALUES ('pl1', 1, 'm1', 2)`)
}

func TestMigrateV26SyncLogFromEmptyAndVersion25(t *testing.T) {
	empty, err := openSQLite(filepath.Join(t.TempDir(), "jobs.db"))
	if err != nil {
		t.Fatalf("openSQLite: %v", err)
	}
	defer empty.Close()
	if version, err := schemaVersion(empty); err != nil || version != len(migrations) {
		t.Fatalf("user_version = %d (%v), want %d", version, err, len(migrations))
	}
	for _, name := range []string{"sync_changes", "sync_meta", syncTriggerName("library_teams", "ai")} {
		if !schemaObjectExists(empty, name) {
			t.Fatalf("schema object %s missing", name)
		}
	}

	path := filepath.Join(t.TempDir(), "jobs.db")
	db, err := openSQLite(path)
	if err != nil {
		t.Fatal(err)
	}
	rewindSyncLogToV25(t, db)
	if _, err := db.Exec(`INSERT INTO library_teams (id, name, name_key, created_at, updated_at) VALUES ('t1', 'Team One', 'team one', 1, 1)`); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	db, err = openSQLite(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if version, err := schemaVersion(db); err != nil || version != len(migrations) {
		t.Fatalf("user_version = %d (%v), want %d", version, err, len(migrations))
	}
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM library_teams WHERE id = 't1' AND name = 'Team One'`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("team row lost: count = %d", count)
	}
}

func TestSyncLogTriggersLogTeamsOps(t *testing.T) {
	db, _ := newTestSyncLog(t)
	ctx := context.Background()
	for _, stmt := range []string{
		`INSERT INTO library_teams (id, name, name_key, created_at, updated_at) VALUES ('t1', 'Team One', 'team one', 1, 1)`,
		`UPDATE library_teams SET name = 'Team Uno' WHERE id = 't1'`,
		`DELETE FROM library_teams WHERE id = 't1'`,
	} {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			t.Fatalf("%s: %v", stmt, err)
		}
	}
	rows, err := db.QueryContext(ctx, `SELECT op, row_key FROM sync_changes WHERE tbl = 'library_teams' ORDER BY seq`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var ops, keys []string
	for rows.Next() {
		var op, key string
		if err := rows.Scan(&op, &key); err != nil {
			t.Fatal(err)
		}
		ops = append(ops, op)
		keys = append(keys, key)
	}
	if !slices.Equal(ops, []string{"upsert", "upsert", "delete"}) {
		t.Fatalf("ops = %v, want [upsert upsert delete]", ops)
	}
	for _, key := range keys {
		if key != `["t1"]` {
			t.Fatalf("row_key = %s, want [\"t1\"]", key)
		}
	}
}

func TestSyncLogCompositeKey(t *testing.T) {
	db, log := newTestSyncLog(t)
	ctx := context.Background()
	for _, stmt := range []string{
		`INSERT INTO library_playlists (id, name, created_at, updated_at) VALUES ('pl1', 'My List', 1, 1)`,
		`INSERT INTO library_playlist_rounds (playlist_id, position, match_id, round) VALUES ('pl1', 0, 'm1', 1)`,
	} {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			t.Fatalf("%s: %v", stmt, err)
		}
	}
	batch, err := log.Changes(ctx, 0, 1000)
	if err != nil {
		t.Fatal(err)
	}
	var rounds []SyncChange
	for _, change := range batch.Changes {
		if change.Table == "library_playlist_rounds" {
			rounds = append(rounds, change)
		}
	}
	if len(rounds) != 1 {
		t.Fatalf("playlist round changes = %d, want 1", len(rounds))
	}
	if want := []any{"pl1", int64(0)}; !reflect.DeepEqual(rounds[0].Key, want) {
		t.Fatalf("Key = %#v, want %#v", rounds[0].Key, want)
	}
}

func TestSyncLogCascadeDeleteLogsChildren(t *testing.T) {
	db, _ := newTestSyncLog(t)
	ctx := context.Background()
	for _, stmt := range []string{
		`INSERT INTO library_demos (id, sha256, file_name, path, size_bytes, source, state, imported_at, updated_at) VALUES ('d1', 'sha1', 'a.dem', 'C:/a.dem', 100, 'manual', 'ready', 1, 1)`,
		`INSERT INTO library_matches (id, demo_id, map, played_at, tickrate, duration_ticks, round_count, created_at, updated_at) VALUES ('m1', 'd1', 'de_dust2', 1000, 64.0, 100000, 24, 1, 1)`,
		`INSERT INTO library_players (steam_id, nick, nick_key, created_at, updated_at) VALUES ('s1', 'Alice', 'alice', 1, 1)`,
		`INSERT INTO library_match_players (match_id, steam_id, team_key, slot, name, rounds, kills, deaths, assists, damage) VALUES ('m1', 's1', 'ct', 0, 'Alice', 24, 10, 8, 2, 500)`,
	} {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			t.Fatalf("%s: %v", stmt, err)
		}
	}
	if _, err := db.ExecContext(ctx, `DELETE FROM library_matches WHERE id = 'm1'`); err != nil {
		t.Fatal(err)
	}
	deleted := map[string]bool{}
	rows, err := db.QueryContext(ctx, `SELECT tbl FROM sync_changes WHERE op = 'delete'`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var tbl string
		if err := rows.Scan(&tbl); err != nil {
			t.Fatal(err)
		}
		deleted[tbl] = true
	}
	for _, tbl := range []string{"library_matches", "library_match_players"} {
		if !deleted[tbl] {
			t.Fatalf("no delete logged for %s", tbl)
		}
	}
}

func TestSyncLogChangesCollapsesAndHonoursCursor(t *testing.T) {
	db, log := newTestSyncLog(t)
	ctx := context.Background()
	for _, stmt := range []string{
		`INSERT INTO library_teams (id, name, name_key, created_at, updated_at) VALUES ('t1', 'Team One', 'team one', 1, 1)`,
		`INSERT INTO library_teams (id, name, name_key, created_at, updated_at) VALUES ('t2', 'Team Two', 'team two', 1, 1)`,
		`UPDATE library_teams SET name = 'Team Uno' WHERE id = 't1'`,
		`DELETE FROM library_teams WHERE id = 't1'`,
		`INSERT INTO library_teams (id, name, name_key, created_at, updated_at) VALUES ('t3', 'Team Three', 'team three', 1, 1)`,
	} {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			t.Fatalf("%s: %v", stmt, err)
		}
	}
	batch, err := log.Changes(ctx, 0, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(batch.Changes) != 2 || batch.Next != 2 {
		t.Fatalf("first batch = %d changes, next %d; want 2 and 2", len(batch.Changes), batch.Next)
	}
	batch, err = log.Changes(ctx, batch.Next, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(batch.Changes) != 1 || batch.Changes[0].Op != "delete" || batch.Next != 4 {
		t.Fatalf("second batch = %+v, next %d; want collapsed t1 delete and next 4", batch.Changes, batch.Next)
	}
	batch, err = log.Changes(ctx, batch.Next, 1000)
	if err != nil {
		t.Fatal(err)
	}
	if len(batch.Changes) != 1 || batch.Changes[0].Op != "upsert" {
		t.Fatalf("third batch = %+v, want t3 upsert", batch.Changes)
	}
}

func TestSyncLogChangesDropsDeviceLocalPath(t *testing.T) {
	db, log := newTestSyncLog(t)
	ctx := context.Background()
	if _, err := db.ExecContext(ctx, `INSERT INTO library_demos (id, sha256, file_name, path, size_bytes, source, state, imported_at, updated_at) VALUES ('d1', 'sha1', 'a.dem', 'C:/a.dem', 100, 'manual', 'ready', 1, 1)`); err != nil {
		t.Fatal(err)
	}
	batch, err := log.Changes(ctx, 0, 1000)
	if err != nil {
		t.Fatal(err)
	}
	var demo *SyncChange
	for i := range batch.Changes {
		if batch.Changes[i].Table == "library_demos" {
			demo = &batch.Changes[i]
		}
	}
	if demo == nil {
		t.Fatal("no demo change")
	}
	if _, ok := demo.Row["path"]; ok {
		t.Fatal("Row contains device-local path")
	}
	if _, ok := demo.Row["sha256"]; !ok {
		t.Fatal("Row missing sha256")
	}
}

func TestSyncLogApplyReproducesAndRejects(t *testing.T) {
	ctx := context.Background()
	db1, log1 := newTestSyncLog(t)
	seedSyncScenario(t, db1, "C:/one/a.dem")

	changes, err := log1.Changes(ctx, 0, 1000)
	if err != nil {
		t.Fatal(err)
	}
	if len(changes.Changes) == 0 {
		t.Fatal("no changes produced")
	}

	db2, log2 := newTestSyncLog(t)
	if _, err := db2.ExecContext(ctx, `INSERT INTO library_demos (id, sha256, file_name, path, size_bytes, source, state, imported_at, updated_at) VALUES ('d1', 'sha1', 'a.dem', 'C:/two/a.dem', 100, 'manual', 'ready', 1, 1)`); err != nil {
		t.Fatal(err)
	}
	var before int
	if err := db2.QueryRow(`SELECT COUNT(*) FROM sync_changes`).Scan(&before); err != nil {
		t.Fatal(err)
	}
	if err := log2.Apply(ctx, changes.Changes); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	var after int
	if err := db2.QueryRow(`SELECT COUNT(*) FROM sync_changes`).Scan(&after); err != nil {
		t.Fatal(err)
	}
	if after != before {
		t.Fatalf("Apply added %d sync rows", after-before)
	}
	var path string
	if err := db2.QueryRow(`SELECT path FROM library_demos WHERE id = 'd1'`).Scan(&path); err != nil {
		t.Fatal(err)
	}
	if path != "C:/two/a.dem" {
		t.Fatalf("demo path = %q, want the local C:/two/a.dem", path)
	}
	for _, check := range []struct {
		query string
		want  int
	}{
		{`SELECT COUNT(*) FROM library_matches WHERE id = 'm1'`, 1},
		{`SELECT COUNT(*) FROM library_players`, 2},
		{`SELECT COUNT(*) FROM library_match_players WHERE match_id = 'm1'`, 2},
		{`SELECT COUNT(*) FROM library_teams WHERE id = 't1'`, 1},
		{`SELECT COUNT(*) FROM library_playlists WHERE id = 'pl1'`, 1},
		{`SELECT COUNT(*) FROM library_playlist_rounds WHERE playlist_id = 'pl1'`, 2},
	} {
		var got int
		if err := db2.QueryRow(check.query).Scan(&got); err != nil {
			t.Fatal(err)
		}
		if got != check.want {
			t.Fatalf("%s = %d, want %d", check.query, got, check.want)
		}
	}

	var input *SyncInputError
	if err := log2.Apply(ctx, []SyncChange{{Table: "nope", Op: "delete", Key: []any{"x"}}}); !errors.As(err, &input) {
		t.Fatalf("unknown table error = %v, want SyncInputError", err)
	}
	if err := log2.Apply(ctx, []SyncChange{{Table: "library_teams", Op: "upsert", Key: []any{"t9"}, Row: map[string]any{"id": "t9", "name": "x", "name_key": "x", "created_at": int64(1), "updated_at": int64(1), "bogus": int64(1)}}}); !errors.As(err, &input) {
		t.Fatalf("unknown column error = %v, want SyncInputError", err)
	}
}
