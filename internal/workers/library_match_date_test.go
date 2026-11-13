package workers

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/rechedev9/cliphub/internal/library"
	"github.com/rechedev9/cliphub/internal/store"
)

type fakeFaceitDates struct {
	started map[string]time.Time
	asked   []string
	err     error
}

func (f *fakeFaceitDates) MatchStartedAt(_ context.Context, matchID string) (time.Time, error) {
	f.asked = append(f.asked, matchID)
	return f.started[matchID], f.err
}

// analysedDemo imports a demo and saves a scan that read no date from it.
func analysedDemo(t *testing.T, repo *store.MemoryLibraryRepository, name string, told *time.Time) (library.Demo, library.Match) {
	t.Helper()
	ctx := context.Background()
	d := library.Demo{SHA256: name, FileName: name, Path: "library/" + name, Source: library.DetectSource("", name), PlayedAt: told}
	if _, err := repo.CreateDemo(ctx, &d); err != nil {
		t.Fatal(err)
	}
	m, err := repo.SaveAnalysis(ctx, library.Analysis{DemoID: d.ID, Map: "de_mirage", Tickrate: 64, RoundCount: 24})
	if err != nil {
		t.Fatal(err)
	}
	return d, m
}

func TestReconcileFaceitMatchDatesDatesOnlyMatchesOnTheirImportTime(t *testing.T) {
	ctx := context.Background()
	repo := store.NewMemoryLibraryRepository()
	told := time.Date(2026, time.October, 18, 20, 0, 0, 0, time.UTC)
	started := time.Date(2026, time.October, 17, 16, 20, 0, 0, time.UTC)
	_, undated := analysedDemo(t, repo, "1-d424a72f-c3f4-4e81-b4e5-1d0d8ea1d7a8-1-1.dem", nil)
	_, toldMatch := analysedDemo(t, repo, "1-8118f582-140f-4241-b800-f86129e1109e-1-1.dem", &told)
	_, future := analysedDemo(t, repo, "1-9f31ab51-a580-4bd6-80d0-e93f227d64e4-1-1.dem", nil)
	_, unknown := analysedDemo(t, repo, "1-4e880f46-e0a5-43d1-a2d0-482888a4da97-1-1.dem", nil)
	analysedDemo(t, repo, "scrim-mirage.dem", nil)
	dates := &fakeFaceitDates{started: map[string]time.Time{
		"1-d424a72f-c3f4-4e81-b4e5-1d0d8ea1d7a8": started,
		"1-8118f582-140f-4241-b800-f86129e1109e": started,
		"1-9f31ab51-a580-4bd6-80d0-e93f227d64e4": time.Now().Add(time.Hour),
	}}

	fixed, err := ReconcileFaceitMatchDates(ctx, repo, dates)
	if err != nil || fixed != 1 {
		t.Fatalf("ReconcileFaceitMatchDates = %d, %v; want 1 match dated", fixed, err)
	}
	if len(dates.asked) != 3 {
		t.Fatalf("asked FACEIT about %v; want the three FACEIT matches still on their import time", dates.asked)
	}
	for _, c := range []struct {
		what string
		m    library.Match
		want time.Time
	}{
		{"a FACEIT match on its import time", undated, started},
		{"a match the importer dated", toldMatch, told},
		{"a FACEIT start after the import", future, future.PlayedAt},
		{"a match FACEIT does not know", unknown, unknown.PlayedAt},
	} {
		got, err := repo.GetMatch(ctx, c.m.ID)
		if err != nil || !got.PlayedAt.Equal(c.want) {
			t.Errorf("%s: played_at = %v, %v; want %v", c.what, got.PlayedAt, err, c.want)
		}
	}

	again := &fakeFaceitDates{started: dates.started}
	if fixed, err := ReconcileFaceitMatchDates(ctx, repo, again); err != nil || fixed != 0 || len(again.asked) != 2 {
		t.Fatalf("second reconcile = %d, %v, asked %v; want nothing new and no question about the dated match", fixed, err, again.asked)
	}
}

func TestReconcileFaceitMatchDatesStopsAtTheFirstFaceitError(t *testing.T) {
	repo := store.NewMemoryLibraryRepository()
	analysedDemo(t, repo, "1-d424a72f-c3f4-4e81-b4e5-1d0d8ea1d7a8-1-1.dem", nil)
	analysedDemo(t, repo, "1-8118f582-140f-4241-b800-f86129e1109e-1-1.dem", nil)
	down := &fakeFaceitDates{err: errors.New("FACEIT is unavailable")}
	if _, err := ReconcileFaceitMatchDates(context.Background(), repo, down); err == nil || len(down.asked) != 1 {
		t.Fatalf("reconcile = %v after %v; want the error after one request", err, down.asked)
	}
}
