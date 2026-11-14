package store

import (
	"context"
	"database/sql"
	"testing"
)

func TestSyncedTeamDeleteKeepsTheDocumentsOfThisDevice(t *testing.T) {
	ctx := context.Background()
	studio, studioLog := newTestSyncLog(t)
	web, webLog := newTestSyncLog(t)
	exec := func(db *sql.DB, stmt string) {
		t.Helper()
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			t.Fatalf("%s: %v", stmt, err)
		}
	}
	for _, stmt := range []string{
		`INSERT INTO library_teams (id, name, name_key, created_at, updated_at) VALUES ('t1', 'SEGA-dll', 'sega-dll', 1, 1)`,
		`INSERT INTO library_teams (id, name, name_key, created_at, updated_at) VALUES ('t2', 'SEGA', 'sega', 1, 1)`,
		`INSERT INTO library_teams (id, name, name_key, created_at, updated_at) VALUES ('t3', 'Rival', 'rival', 1, 1)`,
		`INSERT INTO library_teams (id, name, name_key, created_at, updated_at) VALUES ('t5', 'Nuevo', 'nuevo', 1, 1)`,
		`INSERT INTO library_teams (id, name, name_key, created_at, updated_at) VALUES ('t6', 'Mix', 'mix', 1, 1)`,
		`INSERT INTO library_team_aliases (alias_key, alias, team_id) VALUES ('sega-dll', 'SEGA-dll', 't1')`,
		// t5 was renamed: its alias keeps the old name and nothing names the new one.
		`INSERT INTO library_team_aliases (alias_key, alias, team_id) VALUES ('viejo', 'Viejo', 't5')`,
		`INSERT INTO library_team_aliases (alias_key, alias, team_id) VALUES ('mix', 'Mix', 't6')`,
	} {
		exec(studio, stmt)
	}
	first, err := studioLog.Changes(ctx, 0, 1000)
	if err != nil {
		t.Fatal(err)
	}
	// ClipMind is the hub: it applies Studio's pushes and logs them for the other devices.
	if err := webLog.ApplyFrom(ctx, "studio", first.Changes); err != nil {
		t.Fatal(err)
	}
	// Documents stay on the device that stored them.
	for _, doc := range [][2]string{{"doc1", "t1"}, {"doc3", "t3"}, {"doc5", "t5"}, {"doc6", "t6"}} {
		exec(web, `INSERT INTO library_team_documents (id, team_id, title, content_type, size_bytes, sha256, content, created_at, updated_at)
			VALUES ('`+doc[0]+`', '`+doc[1]+`', 'Antistrats', 'text/html', 3, 's', X'3c703e', 1, 1)`)
	}

	// Studio merges as MergeTeams does: SEGA-dll and the renamed team into SEGA, Mix into a team made
	// in the same batch. It also drops the rival team.
	exec(studio, `INSERT INTO library_teams (id, name, name_key, created_at, updated_at) VALUES ('t7', 'Mix Gaming', 'mix gaming', 2, 2)`)
	exec(studio, `UPDATE library_team_aliases SET team_id = 't2' WHERE team_id IN ('t1', 't5')`)
	exec(studio, `UPDATE library_team_aliases SET team_id = 't7' WHERE team_id = 't6'`)
	exec(studio, `DELETE FROM library_teams WHERE id IN ('t1', 't3', 't5', 't6')`)
	second, err := studioLog.Changes(ctx, first.Next, 1000)
	if err != nil {
		t.Fatal(err)
	}
	if err := webLog.ApplyFrom(ctx, "studio", second.Changes); err != nil {
		t.Fatal(err)
	}

	owner := func(doc string) string {
		var team string
		if err := web.QueryRowContext(ctx, `SELECT team_id FROM library_team_documents WHERE id = ?`, doc).Scan(&team); err != nil {
			t.Fatalf("document %s after the sync: %v", doc, err)
		}
		return team
	}
	for doc, want := range map[string]string{"doc1": "t2", "doc5": "t2", "doc6": "t7", "doc3": "t3"} {
		if got := owner(doc); got != want {
			t.Errorf("%s belongs to %q, want %q", doc, got, want)
		}
	}
	var left int
	if err := web.QueryRowContext(ctx, `SELECT COUNT(*) FROM library_teams WHERE id IN ('t1', 't3', 't5', 't6')`).Scan(&left); err != nil || left != 1 {
		t.Fatalf("deleted teams left = %d (%v), want only t3, which has no heir and still holds a document", left, err)
	}

	// The kept team goes back out, so the device that dropped it gets it again instead of drifting.
	back, err := webLog.ChangesExcluding(ctx, 0, 1000, "studio")
	if err != nil {
		t.Fatal(err)
	}
	if len(back.Changes) != 1 || back.Changes[0].Table != "library_teams" || back.Changes[0].Op != "upsert" || back.Changes[0].Key[0] != "t3" {
		t.Fatalf("hub log for studio = %+v, want only the upsert of t3", back.Changes)
	}
	others, err := webLog.ChangesExcluding(ctx, 0, 1000, "laptop")
	if err != nil {
		t.Fatal(err)
	}
	ops := map[string]string{}
	for _, change := range others.Changes {
		if id, ok := change.Key[0].(string); ok && change.Table == "library_teams" {
			ops[id] = change.Op
		}
	}
	for id, want := range map[string]string{"t1": "delete", "t5": "delete", "t6": "delete", "t3": "upsert", "t7": "upsert"} {
		if ops[id] != want {
			t.Errorf("another device gets %s for team %s, want %s", ops[id], id, want)
		}
	}
	if err := studioLog.Apply(ctx, back.Changes); err != nil {
		t.Fatal(err)
	}
	var name string
	if err := studio.QueryRowContext(ctx, `SELECT name FROM library_teams WHERE id = 't3'`).Scan(&name); err != nil || name != "Rival" {
		t.Fatalf("studio t3 = %q (%v), want Rival back", name, err)
	}
}
