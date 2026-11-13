package store

import (
	"context"
	"database/sql"
	"testing"
)

func TestSyncLogKeyChangeLogsOldKeyDelete(t *testing.T) {
	ctx := context.Background()
	db, log := newTestSyncLog(t)
	seedSyncScenario(t, db, `C:\demos\a.dem`)
	start, err := log.Changes(ctx, 0, 1000)
	if err != nil {
		t.Fatalf("Changes: %v", err)
	}
	if _, err := db.ExecContext(ctx, `UPDATE library_playlist_rounds SET position = 5 WHERE playlist_id = 'pl1' AND position = 1`); err != nil {
		t.Fatalf("update key: %v", err)
	}
	batch, err := log.Changes(ctx, start.Next, 1000)
	if err != nil {
		t.Fatalf("Changes: %v", err)
	}
	if len(batch.Changes) != 2 {
		t.Fatalf("changes = %+v, want delete of old key then upsert of new key", batch.Changes)
	}
	del, up := batch.Changes[0], batch.Changes[1]
	if del.Op != "delete" || del.Key[1] != int64(1) || up.Op != "upsert" || up.Key[1] != int64(5) {
		t.Fatalf("changes = %+v %+v", del, up)
	}

	other, otherLog := newTestSyncLog(t)
	seedSyncScenario(t, other, `D:\other\a.dem`)
	if err := otherLog.Apply(ctx, batch.Changes); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	var positions []int
	rows, err := other.QueryContext(ctx, `SELECT position FROM library_playlist_rounds WHERE playlist_id = 'pl1' ORDER BY position`)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	defer rows.Close()
	for rows.Next() {
		var p int
		if err := rows.Scan(&p); err != nil {
			t.Fatalf("scan: %v", err)
		}
		positions = append(positions, p)
	}
	if len(positions) != 2 || positions[0] != 0 || positions[1] != 5 {
		t.Fatalf("positions on the other device = %v, want [0 5]", positions)
	}
}

func TestSyncLogVoiceLinesRoundTrip(t *testing.T) {
	ctx := context.Background()
	db, log := newTestSyncLog(t)
	seedSyncScenario(t, db, `C:\demos\a.dem`)
	for _, stmt := range []string{
		`INSERT INTO library_match_voice (match_id, sync_state, created_at, updated_at) VALUES ('m1', 'synced', 1, 1)`,
		`INSERT INTO library_voice_lines (match_id, track, start_ms, end_ms, text, text_key) VALUES ('m1', 0, 1000, 2000, 'rush B', 'rush b')`,
		`INSERT INTO library_voice_lines (match_id, track, start_ms, end_ms, text, text_key) VALUES ('m1', 1, 1000, 1500, 'uno en apps', 'uno en apps')`,
	} {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			t.Fatalf("%s: %v", stmt, err)
		}
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO library_voice_lines (match_id, track, start_ms, end_ms, text, text_key) VALUES ('m1', 0, 1000, 2500, 'otra', 'otra')`); err == nil {
		t.Fatal("a second line on the same track and start was accepted")
	}
	batch, err := log.Changes(ctx, 0, 1000)
	if err != nil {
		t.Fatalf("Changes: %v", err)
	}
	other, otherLog := newTestSyncLog(t)
	if err := otherLog.Apply(ctx, batch.Changes); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	var n int
	if err := other.QueryRowContext(ctx, `SELECT COUNT(*) FROM library_voice_lines WHERE match_id = 'm1'`).Scan(&n); err != nil || n != 2 {
		t.Fatalf("voice lines on the other device = %d (%v), want 2", n, err)
	}
}

func TestSyncLogApplyOrdersReferencesAndDeletes(t *testing.T) {
	ctx := context.Background()
	db, log := newTestSyncLog(t)
	seedSyncScenario(t, db, `C:\demos\a.dem`)
	for _, stmt := range []string{
		`INSERT INTO library_plays (id, title, map, side, kind, origin, created_at, updated_at) VALUES ('p1', 'Rush B', 'de_dust2', 'T', 'execute', 'manual', 1, 1)`,
		`INSERT INTO library_patterns (id, name, map, detail, play_id, created_at, updated_at) VALUES ('pa1', 'Rush B', 'de_dust2', '{}', 'p1', 1, 1)`,
	} {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			t.Fatalf("%s: %v", stmt, err)
		}
	}
	first, err := log.Changes(ctx, 0, 1000)
	if err != nil {
		t.Fatalf("Changes: %v", err)
	}
	other, otherLog := newTestSyncLog(t)
	if err := otherLog.Apply(ctx, first.Changes); err != nil {
		t.Fatalf("Apply of a pattern and the play it points to: %v", err)
	}

	for _, stmt := range []string{
		`DELETE FROM library_match_players WHERE steam_id = 's1'`,
		`DELETE FROM library_players WHERE steam_id = 's1'`,
	} {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			t.Fatalf("%s: %v", stmt, err)
		}
	}
	second, err := log.Changes(ctx, first.Next, 1000)
	if err != nil {
		t.Fatalf("Changes: %v", err)
	}
	if err := otherLog.Apply(ctx, second.Changes); err != nil {
		t.Fatalf("Apply of a player delete with its match rows: %v", err)
	}
	var n int
	if err := other.QueryRowContext(ctx, `SELECT COUNT(*) FROM library_players WHERE steam_id = 's1'`).Scan(&n); err != nil || n != 0 {
		t.Fatalf("player s1 on the other device = %d (%v), want 0", n, err)
	}
}

func TestSyncHubLogsAppliedChangesWithoutEchoToTheSender(t *testing.T) {
	ctx := context.Background()
	device, deviceLog := newTestSyncLog(t)
	seedSyncScenario(t, device, `C:\demos\a.dem`)
	pushed, err := deviceLog.Changes(ctx, 0, 1000)
	if err != nil {
		t.Fatalf("Changes: %v", err)
	}

	hub, hubLog := newTestSyncLog(t)
	if err := hubLog.ApplyFrom(ctx, "", pushed.Changes); err == nil {
		t.Fatal("ApplyFrom accepted an empty origin")
	}
	if err := hubLog.ApplyFrom(ctx, "phone", pushed.Changes); err != nil {
		t.Fatalf("ApplyFrom: %v", err)
	}
	var matches int
	if err := hub.QueryRowContext(ctx, `SELECT COUNT(*) FROM library_matches`).Scan(&matches); err != nil || matches != 1 {
		t.Fatalf("hub matches = %d (%v), want 1", matches, err)
	}
	var markers int
	if err := hub.QueryRowContext(ctx, `SELECT COUNT(*) FROM sync_meta`).Scan(&markers); err != nil || markers != 0 {
		t.Fatalf("sync_meta rows after apply = %d (%v), want 0", markers, err)
	}

	own, err := hubLog.ChangesExcluding(ctx, 0, 1000, "phone")
	if err != nil {
		t.Fatalf("ChangesExcluding: %v", err)
	}
	if len(own.Changes) != 0 || own.Next == 0 {
		t.Fatalf("the sender got its own push back: %d changes, next %d", len(own.Changes), own.Next)
	}
	other, err := hubLog.ChangesExcluding(ctx, 0, 1000, "laptop")
	if err != nil || len(other.Changes) == 0 {
		t.Fatalf("another device got %d changes (%v), want the pushed ones", len(other.Changes), err)
	}
}

func TestSyncLogBackfillSendsExistingRows(t *testing.T) {
	ctx := context.Background()
	db, log := newTestSyncLog(t)
	seedSyncScenario(t, db, `C:\demos\a.dem`)
	if _, err := db.ExecContext(ctx, `DELETE FROM sync_changes`); err != nil {
		t.Fatal(err)
	}
	if err := log.Backfill(ctx); err != nil {
		t.Fatalf("Backfill: %v", err)
	}
	batch, err := log.Changes(ctx, 0, 1000)
	if err != nil || len(batch.Changes) == 0 {
		t.Fatalf("Changes after Backfill = %d (%v)", len(batch.Changes), err)
	}
	other, otherLog := newTestSyncLog(t)
	if err := otherLog.Apply(ctx, batch.Changes); err != nil {
		t.Fatalf("Apply of the backfill: %v", err)
	}
	var matches, rounds int
	_ = other.QueryRowContext(ctx, `SELECT COUNT(*) FROM library_matches`).Scan(&matches)
	_ = other.QueryRowContext(ctx, `SELECT COUNT(*) FROM library_playlist_rounds`).Scan(&rounds)
	if matches != 1 || rounds != 2 {
		t.Fatalf("other device has %d matches and %d playlist rounds, want 1 and 2", matches, rounds)
	}
	if n, err := log.Pending(ctx, 0); err != nil || n == 0 {
		t.Fatalf("Pending = %d (%v)", n, err)
	}
}

// Two people import the same demo before syncing: each device gave it its own
// id. Receiving the other copy must not stop the sync, and "our team" stays
// each person's own choice.
func TestSyncLogApplyKeepsGoingWhenTheSameDemoHasTwoIDs(t *testing.T) {
	ctx := context.Background()
	dbA, logA := newTestSyncLog(t)
	seedSyncScenario(t, dbA, "C:/a/match.dem")
	mustExec(t, dbA, `INSERT INTO library_team_profiles (team_id, own, hltv_rank, updated_at) VALUES ('t1', 1, 0, 1)`)
	changes, err := logA.Changes(ctx, 0, 1000)
	if err != nil {
		t.Fatal(err)
	}

	dbB, logB := newTestSyncLog(t)
	mustExec(t, dbB, `INSERT INTO library_demos (id, sha256, file_name, path, size_bytes, source, state, imported_at, updated_at) VALUES ('dB', 'sha1', 'a.dem', 'C:/b/match.dem', 100, 'manual', 'ready', 1, 1)`)
	mustExec(t, dbB, `INSERT INTO library_matches (id, demo_id, map, played_at, tickrate, duration_ticks, round_count, created_at, updated_at) VALUES ('mB', 'dB', 'de_dust2', 1000, 64.0, 100000, 24, 1, 1)`)
	mustExec(t, dbB, `INSERT INTO library_teams (id, name, name_key, created_at, updated_at) VALUES ('tB', 'Team B', 'team b', 1, 1)`)
	mustExec(t, dbB, `INSERT INTO library_team_profiles (team_id, own, hltv_rank, updated_at) VALUES ('tB', 1, 0, 1)`)

	if err := logB.Apply(ctx, changes.Changes); err != nil {
		t.Fatalf("Apply with the same demo under another id: %v", err)
	}
	var demos, teams, own int
	if err := dbB.QueryRow(`SELECT COUNT(*) FROM library_demos`).Scan(&demos); err != nil {
		t.Fatal(err)
	}
	if err := dbB.QueryRow(`SELECT COUNT(*) FROM library_teams`).Scan(&teams); err != nil {
		t.Fatal(err)
	}
	if err := dbB.QueryRow(`SELECT COUNT(*) FROM library_team_profiles WHERE own = 1`).Scan(&own); err != nil {
		t.Fatal(err)
	}
	if demos != 1 || teams != 2 || own != 1 {
		t.Fatalf("after apply: %d demos (want B's own 1), %d teams (want 2), %d own teams (want 1)", demos, teams, own)
	}
}

func mustExec(t *testing.T, db *sql.DB, stmt string) {
	t.Helper()
	if _, err := db.Exec(stmt); err != nil {
		t.Fatalf("%s: %v", stmt, err)
	}
}
