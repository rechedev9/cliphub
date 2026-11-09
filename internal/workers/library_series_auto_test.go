package workers

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/library"
	"github.com/rechedev9/cliphub/internal/store"
)

const faceitMatchID = "de078ee6-f038-4000-8f83-12e1cfa860b2"

func analyseLibraryDemo(t *testing.T, repo *store.MemoryLibraryRepository, name string) library.Match {
	t.Helper()
	d := library.Demo{FileName: name, SHA256: name}
	if _, err := repo.CreateDemo(context.Background(), &d); err != nil {
		t.Fatal(err)
	}
	m, err := repo.SaveAnalysis(context.Background(), library.Analysis{
		DemoID: d.ID, Map: "de_mirage", RoundCount: 20,
		Teams: []library.AnalysisTeam{{Key: "a", StartSide: "CT", ClanName: "Alpha"}, {Key: "b", StartSide: "T", ClanName: "Bravo"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func matchSeries(t *testing.T, repo *store.MemoryLibraryRepository, id uuid.UUID) *uuid.UUID {
	t.Helper()
	m, err := repo.GetMatch(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	return m.SeriesID
}

func TestFaceitMapsOfOneMatchJoinOneSeries(t *testing.T) {
	repo := store.NewMemoryLibraryRepository()
	w := NewLibraryWorker(repo, newFakeStorage())
	first := analyseLibraryDemo(t, repo, "1-"+faceitMatchID+"-1-1.dem")
	other := analyseLibraryDemo(t, repo, "1-11111111-2222-3333-4444-555555555555-2-1.dem")
	w.groupFaceitSeries(context.Background(), first.DemoID, "1-"+faceitMatchID+"-1-1.dem")
	if matchSeries(t, repo, first.ID) != nil {
		t.Fatal("a lone FACEIT map got a series")
	}
	second := analyseLibraryDemo(t, repo, "1-"+faceitMatchID+"-2-1.dem")
	w.groupFaceitSeries(context.Background(), second.DemoID, "1-"+faceitMatchID+"-2-1.dem")
	third := analyseLibraryDemo(t, repo, "1-"+faceitMatchID+"-3-1.dem")
	w.groupFaceitSeries(context.Background(), third.DemoID, "1-"+faceitMatchID+"-3-1.dem")

	series := matchSeries(t, repo, first.ID)
	if series == nil {
		t.Fatal("map 1 has no series")
	}
	for _, m := range []library.Match{second, third} {
		if got := matchSeries(t, repo, m.ID); got == nil || *got != *series {
			t.Fatalf("match %s series = %v, want %s", m.ID, got, series)
		}
	}
	if matchSeries(t, repo, other.ID) != nil {
		t.Fatal("a map of another FACEIT match joined the series")
	}
	s, err := repo.GetSeries(context.Background(), *series)
	if err != nil {
		t.Fatal(err)
	}
	if s.Format != "bo3" || len(s.TeamIDs) != 2 || len(s.MatchIDs) != 3 {
		t.Fatalf("series = format %q, %d teams, %d maps; want bo3, 2 teams, 3 maps", s.Format, len(s.TeamIDs), len(s.MatchIDs))
	}
}

func TestFaceitGroupingKeepsTheUsersSeries(t *testing.T) {
	repo := store.NewMemoryLibraryRepository()
	w := NewLibraryWorker(repo, newFakeStorage())
	first := analyseLibraryDemo(t, repo, "1-"+faceitMatchID+"-1-1.dem")
	own := library.Series{Format: "bo5"}
	if err := repo.CreateSeries(context.Background(), &own); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.UpdateMatch(context.Background(), first.ID, library.MatchPatch{SeriesID: &own.ID}); err != nil {
		t.Fatal(err)
	}
	second := analyseLibraryDemo(t, repo, "1-"+faceitMatchID+"-2-1.dem")
	w.groupFaceitSeries(context.Background(), second.DemoID, "1-"+faceitMatchID+"-2-1.dem")
	if got := matchSeries(t, repo, second.ID); got == nil || *got != own.ID {
		t.Fatalf("map 2 series = %v, want the user's %s", got, own.ID)
	}
}

func TestFaceitMatchKeyReadsTheFileName(t *testing.T) {
	key, number, ok := faceitMatchKey("1-DE078EE6-F038-4000-8F83-12E1CFA860B2-3-1.dem")
	if !ok || number != 3 || key != "DE078EE6-F038-4000-8F83-12E1CFA860B2" {
		t.Fatalf("faceitMatchKey = %q %d %v", key, number, ok)
	}
	if _, _, ok := faceitMatchKey("match730_003.dem"); ok {
		t.Fatal("a Valve demo name read as FACEIT")
	}
}
