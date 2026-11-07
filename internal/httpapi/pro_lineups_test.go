package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rechedev9/cliphub/internal/community"
	"github.com/rechedev9/cliphub/internal/library"
	"github.com/rechedev9/cliphub/internal/store"
)

func TestSaveProLineupTwiceKeepsOneLibretaCopy(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"version":1,"map":"de_mirage","lineups":[{"id":"smoke-t-40-1--18--10-1","title":"Humo a Window desde T Spawn","kind":"smoke","side":"T","origin":[1292,31,-133],"yaw":-165.91,"pitch":-29.05,"landing":[-1150,-650,-160]}],"combos":[]}`))
	}))
	defer srv.Close()
	client, err := community.NewClient(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	repo := store.NewMemoryLibraryRepository()
	h := NewHandlers(newFakeRepo(), newFakeStorage(), &fakeQueue{}, WithLibrary(repo),
		WithCommunity(CommunityDeps{Client: client, Lineups: community.OpenLineupsCache(t.TempDir(), client)}))
	env := libraryTestEnv{t: t, repo: repo, h: h, router: Routes(h)}

	path := "/api/library/pro-lineups/de_mirage/lineups/smoke-t-40-1--18--10-1/save"
	var first, second library.Lineup
	env.json(http.MethodPost, path, nil, http.StatusCreated, &first)
	env.json(http.MethodPost, path, nil, http.StatusOK, &second)
	if second.ID != first.ID {
		t.Fatalf("second save made %s, want the first copy %s", second.ID, first.ID)
	}
	saved, err := repo.ListLineups(context.Background(), library.LineupFilter{Tag: "pro"})
	if err != nil || len(saved) != 1 {
		t.Fatalf("Libreta has %d pro lineups (err %v), want 1", len(saved), err)
	}
}
