package httpapi

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/rechedev9/cliphub/internal/community"
	"github.com/rechedev9/cliphub/internal/library"
	"github.com/rechedev9/cliphub/internal/store"
)

func TestSaveProLineupTwiceKeepsOneLibretaCopy(t *testing.T) {
	// The second index is a rebuild: the same id with its mean spot and aim moved a little.
	var served atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		origin, yaw := "[1292,31,-133]", "-165.91"
		if served.Add(1) > 1 {
			origin, yaw = "[1293,31,-133]", "-165.93"
		}
		_, _ = fmt.Fprintf(w, `{"version":1,"map":"de_mirage","lineups":[{"id":"smoke-t-40-1--18--10-1","title":"Humo a Window desde T Spawn","kind":"smoke","side":"T","origin":%s,"yaw":%s,"pitch":-29.05,"landing":[-1150,-650,-160]}],"combos":[]}`, origin, yaw)
	}))
	defer srv.Close()
	client, err := community.NewClient(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	repo := store.NewMemoryLibraryRepository()
	env := func() libraryTestEnv {
		h := NewHandlers(newFakeRepo(), newFakeStorage(), &fakeQueue{}, WithLibrary(repo),
			WithCommunity(CommunityDeps{Client: client, Lineups: community.OpenLineupsCache(t.TempDir(), client)}))
		return libraryTestEnv{t: t, repo: repo, h: h, router: Routes(h)}
	}

	path := "/api/library/pro-lineups/de_mirage/lineups/smoke-t-40-1--18--10-1/save"
	var first, second library.Lineup
	env().json(http.MethodPost, path, nil, http.StatusCreated, &first)
	env().json(http.MethodPost, path, nil, http.StatusOK, &second)
	if second.ID != first.ID || served.Load() != 2 {
		t.Fatalf("second save made %s after %d fetches, want the first copy %s", second.ID, served.Load(), first.ID)
	}
	saved, err := repo.ListLineups(context.Background(), library.LineupFilter{Tag: "pro"})
	if err != nil || len(saved) != 1 {
		t.Fatalf("Libreta has %d pro lineups (err %v), want 1", len(saved), err)
	}
	env().json(http.MethodPost, "/api/library/pro-lineups/ALL/lineups/smoke-t-40-1--18--10-1/save", nil, http.StatusBadRequest, nil)
}

func TestProLineupsWithCommunityOffSayDisabled(t *testing.T) {
	h := NewHandlers(newFakeRepo(), newFakeStorage(), &fakeQueue{}, WithLibrary(store.NewMemoryLibraryRepository()),
		WithCommunity(CommunityDeps{Lineups: community.OpenLineupsCache(t.TempDir(), nil)}))
	rec := httptest.NewRecorder()
	Routes(h).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/library/pro-lineups", nil))
	if rec.Code != http.StatusServiceUnavailable || !strings.Contains(rec.Body.String(), codeCommunityOff) {
		t.Fatalf("community off answered %d %s", rec.Code, rec.Body.String())
	}
}
