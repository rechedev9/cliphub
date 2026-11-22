package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/rechedev9/cliphub/internal/artifacts"
	"github.com/rechedev9/cliphub/internal/library"
	"github.com/rechedev9/cliphub/internal/store"
	"github.com/rechedev9/cliphub/internal/tacticalplan"
)

func (f transcriptLibraryFixture) correctedTwice(sha, libraryPath string) library.Match {
	f.t.Helper()
	m := f.match(sha, "de_mirage")
	raw, err := json.Marshal(tacticalplan.Document{Demo: tacticalplan.Demo{Map: "de_mirage"}})
	if err != nil {
		f.t.Fatal(err)
	}
	tactical := filepath.Join(filepath.Dir(libraryPath), filepath.FromSlash(artifacts.LibraryTacticalIndexKey(m.DemoID)))
	if err := os.MkdirAll(filepath.Dir(tactical), 0o750); err != nil {
		f.t.Fatal(err)
	}
	if err := os.WriteFile(tactical, raw, 0o600); err != nil {
		f.t.Fatal(err)
	}
	lines := f.frases(m, "flesh now", "flesh go")
	for i, text := range []string{"flash now", "flash go"} {
		f.edit(library.DemoVoiceLinePatch{MatchID: m.ID, ID: lines[i].ID, Text: &text})
	}
	return m
}

func TestTranscriptMeasureReplaysLearnedCorrectionsFromTheOtherTranscripts(t *testing.T) {
	path := filepath.Join(t.TempDir(), "jobs.db")
	repos, err := store.OpenSQLite(path)
	if err != nil {
		t.Fatal(err)
	}
	f := transcriptLibraryFixture{t: t, ctx: context.Background(), repo: repos.Library}
	first := f.correctedTwice("first", path)
	if err := repos.Close(); err != nil {
		t.Fatal(err)
	}
	alone := measureJSON(t, path).Transcripts[0].Result
	if alone.VocabFixedWords != 0 || alone.VocabCorrectedWords != 2 {
		t.Fatalf("a demo alone = %+v; want no rule learned from the frases being measured", alone)
	}

	repos, err = store.OpenSQLite(path)
	if err != nil {
		t.Fatal(err)
	}
	f.repo = repos.Library
	f.correctedTwice("second", path)
	if err := repos.Close(); err != nil {
		t.Fatal(err)
	}
	rows := measureJSON(t, path).Transcripts
	if len(rows) != 2 || rows[0].ID != first.ID.String() && rows[1].ID != first.ID.String() {
		t.Fatalf("transcripts = %+v; want both demos", rows)
	}
	for _, row := range rows {
		if row.Result.VocabFixedWords != 2 || row.Result.VocabCorrectedWords != 0 {
			t.Fatalf("a demo beside another = %+v; want the flash learned from the other demo replayed", row.Result)
		}
	}
}
