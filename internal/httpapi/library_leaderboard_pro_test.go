package httpapi

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/rechedev9/cliphub/internal/community"
	"github.com/rechedev9/cliphub/internal/store"
)

func TestProLeaderboardWithFiltersComesFromTheCommunityServer(t *testing.T) {
	var asked url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		asked = r.URL.Query()
		_, _ = w.Write([]byte(`{"map":"de_anubis","side":"T","role":"b_lurk","days":90,"top":30,"metric":"rating","higher_is_better":true,"min_rounds":30,"maps":4,"rounds":96,"built_at":"2026-11-09T03:00:00Z",
			"players":[{"rank":1,"steamid64":"1","name":"donk","team":"Spirit","rounds":96,"matches":4,"value":1.31,"metrics":{"rating":1.31},"roles":{"t":"b_lurk"}}]}`))
	}))
	defer srv.Close()
	client, err := community.NewClient(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	h := NewHandlers(newFakeRepo(), newFakeStorage(), &fakeQueue{}, WithLibrary(store.NewMemoryLibraryRepository()), WithCommunity(CommunityDeps{Client: client}))
	env := libraryTestEnv{t: t, repo: store.NewMemoryLibraryRepository(), h: h, router: Routes(h)}

	var resp leaderboardResponse
	env.json(http.MethodGet, "/api/library/leaderboard?scope=pro&metric=rating&map=de_anubis&role=b_lurk&days=90&top=30", nil, http.StatusOK, &resp)
	if resp.Source != "server" || resp.Role != "b_lurk" || resp.Side != "T" || resp.Days != 90 || resp.Top != 30 || len(resp.Players) != 1 || resp.Players[0].Team != "Spirit" || resp.Matches != 4 {
		t.Fatalf("pro board = %+v", resp)
	}
	for key, want := range map[string]string{"map": "de_anubis", "side": "T", "role": "b_lurk", "days": "90", "top": "30", "metric": "rating"} {
		if asked.Get(key) != want {
			t.Errorf("the server was asked %s=%q, want %q", key, asked.Get(key), want)
		}
	}
	for _, bad := range []string{"&scope=pro&from=2026-01-01T00:00:00Z", "&scope=pro&source=hltv", "&top=5", "&days=30", "&scope=pro&top=51"} {
		if rw := env.do(http.MethodGet, "/api/library/leaderboard?metric=rating"+bad, nil, ""); rw.Code != http.StatusBadRequest {
			t.Errorf("leaderboard%s = %d, want 400", bad, rw.Code)
		}
	}
}

func TestProLeaderboardWithFiltersNeedsTheCommunityServer(t *testing.T) {
	h := NewHandlers(newFakeRepo(), newFakeStorage(), &fakeQueue{}, WithLibrary(store.NewMemoryLibraryRepository()), WithCommunity(CommunityDeps{}))
	env := libraryTestEnv{t: t, repo: store.NewMemoryLibraryRepository(), h: h, router: Routes(h)}
	env.json(http.MethodGet, "/api/library/leaderboard?scope=pro&metric=rating&days=30", nil, http.StatusServiceUnavailable, nil)
}
