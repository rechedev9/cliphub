package store

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/library"
)

func TestOpenSQLiteLibraryReadOnlyReadsAndRefusesWrites(t *testing.T) {
	path := filepath.Join(t.TempDir(), "has space #1", "jobs.db")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	repos, err := OpenSQLite(path)
	if err != nil {
		t.Fatal(err)
	}
	f := libraryFixture{t: t, ctx: context.Background(), repo: repos.Library}
	m := f.analyze(f.demo("read-only", 0), "de_mirage", side{clan: "FUT", score: 13}, side{clan: "OG", score: 8})
	lines := []library.DemoVoiceLine{{ID: uuid.New(), SteamID64: "76561198000000001", Name: "ropz", TeamKey: "ct-start", EndMS: 2_000, Text: "eco"}}
	if err := repos.Library.SetDemoVoice(f.ctx, library.DemoVoiceUpdate{MatchID: m.ID, Transcript: library.DemoVoiceTranscript{State: library.TranscriptReady}, Lines: &lines}); err != nil {
		t.Fatal(err)
	}
	if err := repos.Close(); err != nil {
		t.Fatal(err)
	}

	readOnly, err := OpenSQLiteLibraryReadOnly(path)
	if err != nil {
		t.Fatal(err)
	}
	defer readOnly.Close()
	got, err := readOnly.Library.ListDemoVoiceLines(f.ctx, library.DemoVoiceLineFilter{All: true})
	if err != nil || len(got) != 1 || got[0].Text != "eco" {
		t.Fatalf("lines through the read-only library = %+v, %v; want the stored frase", got, err)
	}
	fixed := "eco b"
	if _, err := readOnly.Library.UpdateDemoVoiceLine(f.ctx, library.DemoVoiceLinePatch{MatchID: m.ID, ID: lines[0].ID, Text: &fixed}); err == nil {
		t.Fatal("a correction through the read-only library succeeded, want it refused")
	}
	if got, _ = readOnly.Library.ListDemoVoiceLines(f.ctx, library.DemoVoiceLineFilter{All: true}); len(got) != 1 || got[0].Text != "eco" {
		t.Fatalf("lines after the refused write = %+v; want them unchanged", got)
	}
}

func TestOpenSQLiteLibraryReadOnlyDoesNotCreateAMissingLibrary(t *testing.T) {
	path := filepath.Join(t.TempDir(), "jobs.db")
	if _, err := OpenSQLiteLibraryReadOnly(path); err == nil {
		t.Fatal("opening a missing library succeeded, want an error")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("stat after the failed open = %v; want no file created", err)
	}
}

func TestOpenSQLiteLibraryReadOnlyRefusesAnotherSchemaWithoutMigrating(t *testing.T) {
	path := filepath.Join(t.TempDir(), "jobs.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`PRAGMA user_version = 5`); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := OpenSQLiteLibraryReadOnly(path); err == nil || !strings.Contains(err.Error(), "schema version 5") {
		t.Fatalf("read-only open of an older library = %v, want a schema version refusal", err)
	}
	db, err = sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if version, err := schemaVersion(db); err != nil || version != 5 {
		t.Fatalf("schema version after the refusal = %d, %v; want 5, untouched", version, err)
	}
}
