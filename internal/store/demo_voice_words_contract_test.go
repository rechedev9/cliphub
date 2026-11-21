package store

import (
	"context"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/library"
)

func TestDemoVoiceLinesKeepTheirWords(t *testing.T) {
	for name, repo := range libraryRepositoriesUnderTest(t) {
		t.Run(name, func(t *testing.T) {
			f := libraryFixture{t: t, ctx: context.Background(), repo: repo}
			m := f.analyze(f.demo("demo-voice-words", 0), "de_mirage", side{clan: "FUT", score: 13}, side{clan: "OG", score: 8})
			words := []library.DemoVoiceWord{{Text: "Stack", StartMS: 64_000, EndMS: 64_400}, {Text: "B", StartMS: 64_500, EndMS: 64_700}, {Text: "site", StartMS: 65_300, EndMS: 66_000}}
			call := library.DemoVoiceLine{ID: uuid.New(), SteamID64: "76561198000000001", Name: "ropz", TeamKey: "ct-start", StartMS: 64_000, EndMS: 66_000, Tick: 4096, Round: 2, Text: "Stack B site", Words: words}
			old := library.DemoVoiceLine{ID: uuid.New(), SteamID64: "76561198000000002", Name: "b1t", TeamKey: "ct-start", StartMS: 1_000, EndMS: 2_000, Tick: 64, Round: 1, Text: "eco"}
			lines := []library.DemoVoiceLine{call, old}
			ready := library.DemoVoiceTranscript{State: library.TranscriptReady}
			if err := repo.SetDemoVoice(f.ctx, library.DemoVoiceUpdate{MatchID: m.ID, Transcript: ready, Lines: &lines}); err != nil {
				t.Fatal(err)
			}
			words[0].Text = "changed by the caller"
			read := func() []library.DemoVoiceLine {
				t.Helper()
				got, err := repo.ListDemoVoiceLines(f.ctx, library.DemoVoiceLineFilter{MatchIDs: []uuid.UUID{m.ID}})
				if err != nil || len(got) != 2 {
					t.Fatalf("lines = %+v, %v", got, err)
				}
				return got
			}
			stored := []library.DemoVoiceWord{{Text: "Stack", StartMS: 64_000, EndMS: 64_400}, {Text: "B", StartMS: 64_500, EndMS: 64_700}, {Text: "site", StartMS: 65_300, EndMS: 66_000}}
			got := read()
			if len(got[0].Words) != 0 || !reflect.DeepEqual(got[1].Words, stored) {
				t.Fatalf("words = %+v and %+v; want none for the frase stored without them and the stored ones for the other", got[0].Words, got[1].Words)
			}
			if err := repo.SetDemoVoice(f.ctx, library.DemoVoiceUpdate{MatchID: m.ID, Transcript: ready}); err != nil {
				t.Fatal(err)
			}
			if got := read(); !reflect.DeepEqual(got[1].Words, stored) {
				t.Fatalf("words after a state update = %+v, want them kept", got[1].Words)
			}
			spread := []library.DemoVoiceWord{{Text: "Stack", StartMS: 64_000, EndMS: 64_714}, {Text: "B,", StartMS: 64_714, EndMS: 65_000}, {Text: "they", StartMS: 65_000, EndMS: 65_571},
				{Text: "go", StartMS: 65_571, EndMS: 65_857}, {Text: "B", StartMS: 65_857, EndMS: 66_000}}
			fixed := "Stack B, they go B"
			line, err := repo.UpdateDemoVoiceLine(f.ctx, library.DemoVoiceLinePatch{MatchID: m.ID, ID: call.ID, Text: &fixed})
			if err != nil || !reflect.DeepEqual(line.Words, spread) {
				t.Fatalf("corrected words = %+v, %v; want the frase's time shared by word length", line.Words, err)
			}
			if got := read(); !reflect.DeepEqual(got[1].Words, spread) {
				t.Fatalf("stored words after a correction = %+v, want %+v", got[1].Words, spread)
			}
			discard := true
			if line, err = repo.UpdateDemoVoiceLine(f.ctx, library.DemoVoiceLinePatch{MatchID: m.ID, ID: old.ID, Discarded: &discard}); err != nil || len(line.Words) != 0 {
				t.Fatalf("words of a frase discarded without a correction = %+v, %v; want them untouched", line.Words, err)
			}
			call.Words = stored
			again := []library.DemoVoiceLine{call, old}
			if err := repo.SetDemoVoice(f.ctx, library.DemoVoiceUpdate{MatchID: m.ID, Transcript: ready, Lines: &again}); err != nil {
				t.Fatal(err)
			}
			if got := read(); got[1].Text != fixed || !reflect.DeepEqual(got[1].Words, spread) {
				t.Fatalf("after transcribing again = %q with words %+v; want the correction kept with its own words", got[1].Text, got[1].Words)
			}
		})
	}
}

func TestMigrateV34KeepsTheFrasesStoredBeforeWordTimings(t *testing.T) {
	path := filepath.Join(t.TempDir(), "jobs.db")
	db, err := openSQLite(path)
	if err != nil {
		t.Fatal(err)
	}
	f := libraryFixture{t: t, ctx: context.Background(), repo: NewSQLiteLibraryRepository(db)}
	m := f.analyze(f.demo("demo-voice-before-words", 0), "de_mirage", side{clan: "FUT", score: 13}, side{clan: "OG", score: 8})
	lines := []library.DemoVoiceLine{{ID: uuid.New(), SteamID64: "76561198000000001", Name: "ropz", TeamKey: "ct-start", StartMS: 1_000, EndMS: 2_000, Tick: 64, Round: 1, Text: "vamos B"}}
	if err := f.repo.SetDemoVoice(f.ctx, library.DemoVoiceUpdate{MatchID: m.ID, Transcript: library.DemoVoiceTranscript{State: library.TranscriptReady}, Lines: &lines}); err != nil {
		t.Fatal(err)
	}
	for _, stmt := range []string{`ALTER TABLE library_demo_voice_lines DROP COLUMN words`, `PRAGMA user_version = 33`} {
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
	got, err := NewSQLiteLibraryRepository(db).ListDemoVoiceLines(f.ctx, library.DemoVoiceLineFilter{MatchIDs: []uuid.UUID{m.ID}})
	if err != nil || len(got) != 1 || got[0].Text != "vamos B" || len(got[0].Words) != 0 {
		t.Fatalf("frases after migrate = %+v, %v; want the old frase without words", got, err)
	}
}
