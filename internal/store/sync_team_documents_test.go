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
		`INSERT INTO library_team_aliases (alias_key, alias, team_id) VALUES ('sega-dll', 'SEGA-dll', 't1')`,
	} {
		exec(studio, stmt)
	}
	first, err := studioLog.Changes(ctx, 0, 1000)
	if err != nil {
		t.Fatal(err)
	}
	if err := webLog.Apply(ctx, first.Changes); err != nil {
		t.Fatal(err)
	}
	// Documents stay on the device that stored them.
	exec(web, `INSERT INTO library_team_documents (id, team_id, title, content_type, size_bytes, sha256, content, created_at, updated_at)
		VALUES ('doc1', 't1', 'Antistrats', 'text/html', 3, 's', X'3c703e', 1, 1)`)
	exec(web, `INSERT INTO library_team_documents (id, team_id, title, content_type, size_bytes, sha256, content, created_at, updated_at)
		VALUES ('doc3', 't3', 'Vetos', 'text/html', 3, 's', X'3c703e', 1, 1)`)

	// Studio merges SEGA-dll into SEGA, as MergeTeams does, and drops the rival team.
	exec(studio, `UPDATE library_team_aliases SET team_id = 't2' WHERE team_id = 't1'`)
	exec(studio, `DELETE FROM library_teams WHERE id = 't1'`)
	exec(studio, `DELETE FROM library_teams WHERE id = 't3'`)
	second, err := studioLog.Changes(ctx, first.Next, 1000)
	if err != nil {
		t.Fatal(err)
	}
	if err := webLog.Apply(ctx, second.Changes); err != nil {
		t.Fatal(err)
	}

	owner := func(doc string) string {
		var team string
		if err := web.QueryRowContext(ctx, `SELECT team_id FROM library_team_documents WHERE id = ?`, doc).Scan(&team); err != nil {
			t.Fatalf("document %s after the sync: %v", doc, err)
		}
		return team
	}
	if got := owner("doc1"); got != "t2" {
		t.Fatalf("merged team's document belongs to %q, want t2, the team that took its name", got)
	}
	if got := owner("doc3"); got != "t3" {
		t.Fatalf("dropped team's document belongs to %q, want t3 kept on this device", got)
	}
	var teams int
	if err := web.QueryRowContext(ctx, `SELECT COUNT(*) FROM library_teams WHERE id IN ('t1', 't3')`).Scan(&teams); err != nil || teams != 1 {
		t.Fatalf("teams t1 and t3 left = %d (%v), want only t3, which still holds a document", teams, err)
	}
}
