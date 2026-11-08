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

func TestProLineupClipIsFetchedOnceAndKept(t *testing.T) {
	var served, down atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		served.Add(1)
		if down.Load() == 1 || !strings.HasSuffix(r.URL.Path, "-r7-t46593-76561198079183361.mp4") {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "video/mp4")
		_, _ = w.Write([]byte("mp4-bytes"))
	}))
	defer srv.Close()
	client, err := community.NewClient(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	h := NewHandlers(newFakeRepo(), newFakeStorage(), &fakeQueue{}, WithLibrary(store.NewMemoryLibraryRepository()),
		WithCommunity(CommunityDeps{Client: client, Lineups: community.OpenLineupsCache(t.TempDir(), client)}))
	get := func(path string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		Routes(h).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		return rec
	}
	clip := "/api/library/pro-lineups/clips/2398992/iberian-soul-vs-og-m1-anubis-r7-t46593-76561198079183361.mp4"
	if rec := get(clip); rec.Code != http.StatusOK || rec.Body.String() != "mp4-bytes" || rec.Header().Get("Content-Type") != "video/mp4" {
		t.Fatalf("clip answered %d %q %q", rec.Code, rec.Body.String(), rec.Header().Get("Content-Type"))
	}
	down.Store(1)
	if rec := get(clip); rec.Code != http.StatusOK || rec.Body.String() != "mp4-bytes" || served.Load() != 1 {
		t.Fatalf("a kept clip must not ask again: %d after %d fetches", rec.Code, served.Load())
	}
	if rec := get("/api/library/pro-lineups/clips/2398992/other-r1-t1-76561198079183361.mp4"); rec.Code != http.StatusNotFound || !strings.Contains(rec.Body.String(), codeNoProClip) {
		t.Fatalf("missing clip answered %d %s", rec.Code, rec.Body.String())
	}
	if rec := get("/api/library/pro-lineups/clips/2398992/..%2Fsecret.mp4"); rec.Code != http.StatusBadRequest {
		t.Fatalf("bad name answered %d", rec.Code)
	}
}
