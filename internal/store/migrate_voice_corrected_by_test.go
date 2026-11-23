package store

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/library"
)

func TestMigrateV38MarksTheCorrectionsMadeBeforeItAsTheUsers(t *testing.T) {
	path := filepath.Join(t.TempDir(), "jobs.db")
	db, err := openSQLite(path)
	if err != nil {
		t.Fatal(err)
	}
	f := libraryFixture{t: t, ctx: context.Background(), repo: NewSQLiteLibraryRepository(db)}
	m := f.analyze(f.demo("demo-voice-corrected-by", 0), "de_mirage", side{clan: "FUT", score: 13}, side{clan: "OG", score: 8})
	fixed, heard := uuid.New(), uuid.New()
	lines := []library.DemoVoiceLine{
		{ID: fixed, SteamID64: "76561198000000001", Name: "ropz", TeamKey: "ct-start", StartMS: 1_000, EndMS: 2_000, Tick: 64, Round: 1, Text: "flesh now"},
		{ID: heard, SteamID64: "76561198000000001", Name: "ropz", TeamKey: "ct-start", StartMS: 3_000, EndMS: 4_000, Tick: 192, Round: 1, Text: "vamos B"},
	}
	if err := f.repo.SetDemoVoice(f.ctx, library.DemoVoiceUpdate{MatchID: m.ID, Transcript: library.DemoVoiceTranscript{State: library.TranscriptReady}, Lines: &lines}); err != nil {
		t.Fatal(err)
	}
	text := "flash now"
	if _, err := f.repo.UpdateDemoVoiceLine(f.ctx, library.DemoVoiceLinePatch{MatchID: m.ID, ID: fixed, Text: &text}); err != nil {
		t.Fatal(err)
	}
	for _, stmt := range []string{
		`ALTER TABLE library_demo_voice_lines DROP COLUMN corrected_by`,
		`ALTER TABLE stream_voice_lines DROP COLUMN corrected_by`,
		`PRAGMA user_version = 37`,
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("%s: %v", stmt, err)
		}
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	db, err = openSQLite(path)
	if err != nil {
		t.Fatalf("openSQLite: %v", err)
	}
	defer db.Close()
	if found, err := columnExists(db, "stream_voice_lines", "corrected_by"); err != nil || !found {
		t.Fatalf("stream_voice_lines.corrected_by after migrate: found=%v err=%v", found, err)
	}
	got, err := NewSQLiteLibraryRepository(db).ListDemoVoiceLines(f.ctx, library.DemoVoiceLineFilter{MatchIDs: []uuid.UUID{m.ID}, All: true})
	if err != nil || len(got) != 2 {
		t.Fatalf("phrases after migrate = %+v, %v", got, err)
	}
	if got[0].CorrectedBy != library.CorrectedByUser || got[1].CorrectedBy != "" {
		t.Fatalf("after migrate = %+v; want the old correction the user's and the heard phrase without an author", got)
	}
}
