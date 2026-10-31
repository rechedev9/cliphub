package workers

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/artifacts"
	"github.com/rechedev9/cliphub/internal/library"
	"github.com/rechedev9/cliphub/internal/store"
)

func importLibraryDemo(t *testing.T, repo *store.MemoryLibraryRepository, files *fakeStorage, name string, data []byte) library.Demo {
	t.Helper()
	id := uuid.New()
	d := library.Demo{ID: id, FileName: name, Path: artifacts.LibraryDemoKey(id), SizeBytes: int64(len(data))}
	if data != nil {
		files.files[d.Path] = data
	}
	if _, err := repo.CreateDemo(context.Background(), &d); err != nil {
		t.Fatal(err)
	}
	return d
}

func TestLibraryWorkerDropsADeletedDemo(t *testing.T) {
	w := NewLibraryWorker(store.NewMemoryLibraryRepository(), newFakeStorage())
	if err := w.ProcessAnalyzeLibraryDemo(context.Background(), uuid.New()); err != nil {
		t.Fatalf("ProcessAnalyzeLibraryDemo(unknown) = %v, want nil so the task is not retried", err)
	}
}

func TestLibraryWorkerRecordsAnUnreadableDemo(t *testing.T) {
	repo := store.NewMemoryLibraryRepository()
	files := newFakeStorage()
	d := importLibraryDemo(t, repo, files, "corrupt.dem", []byte("PBDEMS2\x00 not really a demo"))

	err := NewLibraryWorker(repo, files).ProcessAnalyzeLibraryDemo(context.Background(), d.ID)
	if err == nil {
		t.Fatal("ProcessAnalyzeLibraryDemo = nil, want the parse failure")
	}
	got, _ := repo.GetDemo(context.Background(), d.ID)
	if got.State != library.ParseFailed || got.ErrorCode == "" || got.Error == "" || got.MatchID != nil {
		t.Fatalf("demo after a failed scan = %+v, want failed with a code and a message", got)
	}
	if _, ok := files.files[artifacts.LibraryTacticalIndexKey(d.ID)]; ok {
		t.Fatal("a failed scan published a tactical document")
	}
}

func TestLibraryWorkerSkipsAReadyDemo(t *testing.T) {
	repo := store.NewMemoryLibraryRepository()
	files := newFakeStorage()
	// No demo bytes: a worker that rescanned instead of skipping would fail.
	d := importLibraryDemo(t, repo, files, "ready.dem", nil)
	if _, err := repo.SaveAnalysis(context.Background(), library.Analysis{DemoID: d.ID, Map: "de_mirage"}); err != nil {
		t.Fatal(err)
	}
	if err := NewLibraryWorker(repo, files).ProcessAnalyzeLibraryDemo(context.Background(), d.ID); err != nil {
		t.Fatalf("ProcessAnalyzeLibraryDemo(ready) = %v", err)
	}
}

// TestLibraryWorkerScansARealDemo runs the whole import on TEST_DEMO_PATH (or
// the default fixture) when one is on disk. Each side's clan name must belong
// to the side whose players it lists: the scan keys a team by its start side,
// while clan names swap with the teams at half time.
func TestLibraryWorkerScansARealDemo(t *testing.T) {
	data := loadRealDemo(t)
	repo := store.NewMemoryLibraryRepository()
	files := newFakeStorage()
	d := importLibraryDemo(t, repo, files, "real.dem", data)

	if err := NewLibraryWorker(repo, files).ProcessAnalyzeLibraryDemo(context.Background(), d.ID); err != nil {
		t.Fatalf("ProcessAnalyzeLibraryDemo = %v", err)
	}
	got, err := repo.GetDemo(context.Background(), d.ID)
	if err != nil || got.State != library.ParseReady || got.MatchID == nil || got.CS2Build == 0 {
		t.Fatalf("demo = %+v, %v; want ready with a match and a build", got, err)
	}
	match, err := repo.GetMatch(context.Background(), *got.MatchID)
	if err != nil {
		t.Fatal(err)
	}
	if len(match.Teams) != 2 || len(match.Players) < 2 || match.RoundCount == 0 {
		t.Fatalf("match = %+v", match)
	}
	if score := match.Teams[0].Score + match.Teams[1].Score; score > match.RoundCount {
		t.Fatalf("scores %d exceed %d rounds", score, match.RoundCount)
	}
	for _, key := range []string{artifacts.LibraryTacticalIndexKey(d.ID), artifacts.LibraryTacticalPositionsKey(d.ID)} {
		if len(files.files[key]) == 0 {
			t.Fatalf("artifact %s missing", key)
		}
	}
	// FACEIT names a team after its captain ("team_<nick>"), which pins the
	// clan name to one side's players.
	for _, team := range match.Teams {
		captain, ok := strings.CutPrefix(team.ClanName, "team_")
		if !ok {
			continue
		}
		for _, p := range match.Players {
			if strings.EqualFold(p.Name, captain) && p.TeamKey != team.Key {
				t.Fatalf("clan %q is on side %s but its captain %s played on %s", team.ClanName, team.Key, p.Name, p.TeamKey)
			}
		}
	}
}
