package store

import (
	"context"
	"testing"
	"time"

	"github.com/rechedev9/cliphub/internal/library"
)

// A match takes the date its demo shows when the importer gave none, and a later scan
// replaces only a date that is still the import time.
func TestLibraryMatchDateContract(t *testing.T) {
	for name, repo := range libraryRepositoriesUnderTest(t) {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			undated := func(sha string) library.Demo {
				t.Helper()
				d := library.Demo{SHA256: sha, FileName: sha + ".dem", Path: "library/" + sha}
				if created, err := repo.CreateDemo(ctx, &d); err != nil || !created {
					t.Fatalf("CreateDemo(%s) = %v, %v", sha, created, err)
				}
				return d
			}
			save := func(d library.Demo, shown *time.Time) library.Match {
				t.Helper()
				m, err := repo.SaveAnalysis(ctx, library.Analysis{DemoID: d.ID, Map: "de_mirage", Tickrate: 64, RoundCount: 24, PlayedAt: shown})
				if err != nil {
					t.Fatalf("SaveAnalysis: %v", err)
				}
				return m
			}
			wantDate := func(what string, m library.Match, want time.Time) {
				t.Helper()
				if !m.PlayedAt.Equal(want) {
					t.Fatalf("%s: played_at = %v, want %v", what, m.PlayedAt, want)
				}
			}

			fresh := undated("fresh")
			shown := fresh.ImportedAt.Add(-27 * 24 * time.Hour).Truncate(time.Second)
			wantDate("a new match", save(fresh, &shown), shown)
			late := undated("late")
			afterImport := late.ImportedAt.Add(time.Hour)
			wantDate("a demo date after the import", save(late, &afterImport), late.ImportedAt)

			old := undated("old")
			wantDate("a scan that read no date", save(old, nil), old.ImportedAt)
			wantDate("a rescan that read one", save(old, &shown), shown)
			other := shown.Add(time.Hour)
			wantDate("a rescan of a match the demo already dated", save(old, &other), shown)

			byHand := undated("by-hand")
			m := save(byHand, nil)
			edited := time.Date(2026, time.October, 18, 20, 0, 0, 0, time.UTC)
			if _, err := repo.UpdateMatch(ctx, m.ID, library.MatchPatch{PlayedAt: &edited}); err != nil {
				t.Fatal(err)
			}
			wantDate("a rescan of a date set by hand", save(byHand, &shown), edited)

			told := libraryFixture{t: t, ctx: ctx, repo: repo}.demo("told", 3)
			wantDate("a date the importer gave", save(told, &shown), *told.PlayedAt)
		})
	}
}
