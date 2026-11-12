package store

import (
	"context"
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
