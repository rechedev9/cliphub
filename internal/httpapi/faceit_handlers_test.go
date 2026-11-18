package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rechedev9/cliphub/internal/faceit"
)

func TestFaceitHandlersRequireConfiguration(t *testing.T) {
	t.Parallel()
	h := NewHandlers(newFakeRepo(), newFakeStorage(), &fakeQueue{})
	router := Routes(h)

	tests := []struct {
		method string
		path   string
		body   string
		want   int
	}{
		{method: http.MethodGet, path: "/api/faceit/players?nickname=m0NESY", want: http.StatusServiceUnavailable},
		{method: http.MethodGet, path: "/api/faceit/players/player-1/matches", want: http.StatusServiceUnavailable},
		{method: http.MethodGet, path: "/api/faceit/followed", want: http.StatusServiceUnavailable},
		{method: http.MethodPost, path: "/api/faceit/followed", body: `{"nickname":"m0NESY"}`, want: http.StatusServiceUnavailable},
		{method: http.MethodDelete, path: "/api/faceit/followed/player-1", want: http.StatusServiceUnavailable},
		{method: http.MethodGet, path: "/api/faceit/rankings?kind=country&code=es", want: http.StatusServiceUnavailable},
	}
	for _, test := range tests {
		t.Run(test.method+" "+test.path, func(t *testing.T) {
			t.Parallel()
			var body *bytes.Reader
			if test.body != "" {
				body = bytes.NewReader([]byte(test.body))
			} else {
				body = bytes.NewReader(nil)
			}
			rw := httptest.NewRecorder()
			req := httptest.NewRequest(test.method, test.path, body)
			router.ServeHTTP(rw, req)
			if rw.Code != test.want {
				t.Fatalf("status = %d, want %d body=%s", rw.Code, test.want, rw.Body.String())
			}
			if !strings.Contains(rw.Body.String(), faceitNotConfigured) && test.method != http.MethodDelete {
				if !strings.Contains(rw.Body.String(), faceitNotConfigured) {
					t.Fatalf("body = %s, want code %s", rw.Body.String(), faceitNotConfigured)
				}
			}
		})
	}
}

func TestFaceitLookupAndFollowFlow(t *testing.T) {
	t.Parallel()

	const apiKey = "faceit-http-secret"
	var playerHits, historyHits, statsHits int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/players":
			playerHits++
			_, _ = w.Write([]byte(`{
				"player_id":"player-1","nickname":"m0NESY","country":"ru",
				"avatar":"https://assets.faceit-cdn.net/avatars/m0nesy.png",
				"games":{"cs2":{"skill_level":10,"faceit_elo":4000}}
			}`))
		case "/players/player-1/history":
			historyHits++
			_, _ = w.Write([]byte(`{"items":[{"match_id":"match-1","finished_at":1767315600,
				"teams":{"faction1":{"players":[{"player_id":"player-1"}]}},
				"results":{"winner":"faction1","score":{"faction1":13,"faction2":7}}}]}`))
		case "/players/player-1/games/cs2/stats":
			statsHits++
			_, _ = w.Write([]byte(`{"items":[{"stats":{"Match Id":"match-1","Map":"de_mirage","Result":"1","Kills":"20","Deaths":"10","Assists":"4","ADR":"90"}}]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)

	client, err := faceit.New(faceit.Options{APIKey: apiKey, BaseURL: server.URL, HTTPClient: server.Client()})
	if err != nil {
		t.Fatal(err)
	}
	follows, err := faceit.NewFollowStore(filepath.Join(t.TempDir(), "followed.json"), func() time.Time {
		return time.Date(2026, 8, 17, 12, 0, 0, 0, time.UTC)
	})
	if err != nil {
		t.Fatal(err)
	}
	h := NewHandlers(newFakeRepo(), newFakeStorage(), &fakeQueue{}, WithFaceit(client, follows), WithCapabilities(Capabilities{FaceitEnabled: true}))
	router := Routes(h)

	lookup := httptest.NewRecorder()
	router.ServeHTTP(lookup, httptest.NewRequest(http.MethodGet, "/api/faceit/players?nickname=m0NESY", nil))
	if lookup.Code != http.StatusOK {
		t.Fatalf("lookup status = %d body=%s", lookup.Code, lookup.Body.String())
	}
	cached := httptest.NewRecorder()
	router.ServeHTTP(cached, httptest.NewRequest(http.MethodGet, "/api/faceit/players?nickname=m0NESY", nil))
	if cached.Code != http.StatusOK || playerHits != 1 {
		t.Fatalf("cached lookup status=%d hits=%d, want 200/1", cached.Code, playerHits)
	}

	follow := httptest.NewRecorder()
	router.ServeHTTP(follow, httptest.NewRequest(http.MethodPost, "/api/faceit/followed", strings.NewReader(`{"nickname":"m0NESY"}`)))
	if follow.Code != http.StatusOK {
		t.Fatalf("follow status = %d body=%s", follow.Code, follow.Body.String())
	}
	if playerHits != 1 {
		t.Fatalf("follow replayed lookup, hits=%d", playerHits)
	}

	listed := httptest.NewRecorder()
	router.ServeHTTP(listed, httptest.NewRequest(http.MethodGet, "/api/faceit/followed", nil))
	if listed.Code != http.StatusOK {
		t.Fatalf("list status = %d body=%s", listed.Code, listed.Body.String())
	}
	var listBody struct {
		Enabled bool                 `json:"enabled"`
		Players []listedFaceitPlayer `json:"players"`
	}
	if err := json.Unmarshal(listed.Body.Bytes(), &listBody); err != nil {
		t.Fatal(err)
	}
	if !listBody.Enabled || len(listBody.Players) < 1 || listBody.Players[0].ID != "player-1" {
		t.Fatalf("followed = %#v", listBody)
	}
	if len(listBody.Players) != 1 {
		t.Fatalf("followed = %#v, want only the followed player", listBody.Players)
	}

	matches := httptest.NewRecorder()
	router.ServeHTTP(matches, httptest.NewRequest(http.MethodGet, "/api/faceit/players/player-1/matches?limit=10", nil))
	if matches.Code != http.StatusOK {
		t.Fatalf("matches status = %d body=%s", matches.Code, matches.Body.String())
	}
	cachedMatches := httptest.NewRecorder()
	router.ServeHTTP(cachedMatches, httptest.NewRequest(http.MethodGet, "/api/faceit/players/player-1/matches?limit=10", nil))
	if cachedMatches.Code != http.StatusOK || historyHits != 1 || statsHits != 1 {
		t.Fatalf("cached matches status=%d history=%d stats=%d", cachedMatches.Code, historyHits, statsHits)
	}

	unfollow := httptest.NewRecorder()
	router.ServeHTTP(unfollow, httptest.NewRequest(http.MethodDelete, "/api/faceit/followed/player-1", nil))
	if unfollow.Code != http.StatusNoContent {
		t.Fatalf("unfollow status = %d body=%s", unfollow.Code, unfollow.Body.String())
	}
	empty := httptest.NewRecorder()
	router.ServeHTTP(empty, httptest.NewRequest(http.MethodGet, "/api/faceit/followed", nil))
	var emptyBody struct {
		Players []listedFaceitPlayer `json:"players"`
	}
	if err := json.Unmarshal(empty.Body.Bytes(), &emptyBody); err != nil {
		t.Fatal(err)
	}
	if len(emptyBody.Players) != 0 {
		t.Fatalf("players after unfollow = %s, want nobody", empty.Body.String())
	}
}

func TestFaceitHandlersTable(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		status int
		path   string
		want   int
		code   string
	}{
		{name: "player missing", status: http.StatusNotFound, path: "/api/faceit/players?nickname=missing", want: http.StatusNotFound},
		{name: "invalid nickname", status: http.StatusOK, path: "/api/faceit/players?nickname=", want: http.StatusBadRequest},
		{name: "invalid player id", status: http.StatusOK, path: "/api/faceit/players/bad%20id/matches", want: http.StatusBadRequest},
		{name: "unauthorized", status: http.StatusUnauthorized, path: "/api/faceit/players?nickname=m0NESY", want: http.StatusBadGateway, code: "faceit_unauthorized"},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(test.status)
				_, _ = w.Write([]byte(`{}`))
			}))
			t.Cleanup(server.Close)
			client, err := faceit.New(faceit.Options{APIKey: "faceit-test-key", BaseURL: server.URL, HTTPClient: server.Client()})
			if err != nil {
				t.Fatal(err)
			}
			follows, err := faceit.NewFollowStore(filepath.Join(t.TempDir(), "followed.json"), time.Now)
			if err != nil {
				t.Fatal(err)
			}
			h := NewHandlers(newFakeRepo(), newFakeStorage(), &fakeQueue{}, WithFaceit(client, follows))
			rw := httptest.NewRecorder()
			Routes(h).ServeHTTP(rw, httptest.NewRequest(http.MethodGet, test.path, nil))
			if rw.Code != test.want {
				t.Fatalf("status = %d, want %d body=%s", rw.Code, test.want, rw.Body.String())
			}
			if test.code != "" && !strings.Contains(rw.Body.String(), test.code) {
				t.Fatalf("body = %s, want code %s", rw.Body.String(), test.code)
			}
		})
	}
}

func TestGetCapabilitiesReportsFaceitEnabled(t *testing.T) {
	t.Parallel()
	h := NewHandlers(newFakeRepo(), newFakeStorage(), &fakeQueue{}, WithCapabilities(Capabilities{FaceitEnabled: true}))
	rw := httptest.NewRecorder()
	h.GetCapabilities(rw, httptest.NewRequest(http.MethodGet, "/api/capabilities", nil))
	var got struct {
		Faceit struct {
			Enabled bool `json:"enabled"`
		} `json:"faceit"`
	}
	if err := json.Unmarshal(rw.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if !got.Faceit.Enabled {
		t.Fatal("faceit.enabled = false, want true")
	}
}

func TestFollowedListWorksWithoutAPIKey(t *testing.T) {
	t.Parallel()
	follows, err := faceit.NewFollowStore(filepath.Join(t.TempDir(), "followed.json"), time.Now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := follows.Follow(faceit.Player{ID: "player-1", Nickname: "m0NESY"}); err != nil {
		t.Fatal(err)
	}
	h := NewHandlers(newFakeRepo(), newFakeStorage(), &fakeQueue{}, WithFaceit(nil, follows))
	rw := httptest.NewRecorder()
	Routes(h).ServeHTTP(rw, httptest.NewRequest(http.MethodGet, "/api/faceit/followed", nil))
	if rw.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rw.Code, rw.Body.String())
	}
	if !strings.Contains(rw.Body.String(), `"enabled":false`) || !strings.Contains(rw.Body.String(), "m0NESY") {
		t.Fatalf("body = %s", rw.Body.String())
	}
}

// Studio no longer lists zone players of its own: a fresh install follows nobody.
func TestFollowedListStartsEmpty(t *testing.T) {
	t.Parallel()
	follows, err := faceit.NewFollowStore(filepath.Join(t.TempDir(), "followed.json"), time.Now)
	if err != nil {
		t.Fatal(err)
	}
	h := NewHandlers(newFakeRepo(), newFakeStorage(), &fakeQueue{}, WithFaceit(nil, follows))
	rw := httptest.NewRecorder()
	Routes(h).ServeHTTP(rw, httptest.NewRequest(http.MethodGet, "/api/faceit/followed", nil))
	if rw.Code != http.StatusOK || !strings.Contains(rw.Body.String(), `"players":[]`) {
		t.Fatalf("status = %d body=%s, want an empty list", rw.Code, rw.Body.String())
	}
}

// Every Studio start rereads the followed profiles in the background, and
// only those: no ranking is read until the page asks for one, and a list read
// never reaches FACEIT.
func TestStartupRefreshUpdatesFollowedPlayersOnly(t *testing.T) {
	t.Parallel()
	var faceitHits, rankingHits atomic.Int32
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		faceitHits.Add(1)
		switch {
		case strings.HasPrefix(r.URL.Path, "/rankings/"):
			rankingHits.Add(1)
			_, _ = w.Write([]byte(`{"items":[]}`))
		case r.URL.Path == "/players/player-1":
			<-release
			// The default FACEIT avatar: the Steam one stored at follow time must survive.
			_, _ = w.Write([]byte(`{"player_id":"player-1","nickname":"m0NESY-renamed","country":"ru",
				"avatar":"https://assets.faceit-cdn.net/avatars/3b536dda-e3dd-40cd-baed-7e66ab050c8f.png",
				"games":{"cs2":{"skill_level":10,"faceit_elo":4123}}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)

	client, err := faceit.New(faceit.Options{APIKey: "faceit-test-key", BaseURL: server.URL, HTTPClient: server.Client()})
	if err != nil {
		t.Fatal(err)
	}
	follows, err := faceit.NewFollowStore(filepath.Join(t.TempDir(), "followed.json"), time.Now)
	if err != nil {
		t.Fatal(err)
	}
	const steamAvatar = "https://avatars.steamstatic.com/m0nesy_full.jpg"
	if _, err := follows.Follow(faceit.Player{ID: "player-1", Nickname: "m0NESY", ELO: 3000, Avatar: steamAvatar}); err != nil {
		t.Fatal(err)
	}
	h := NewHandlers(newFakeRepo(), newFakeStorage(), &fakeQueue{}, WithFaceit(client, follows))
	router := Routes(h)
	list := func() (bool, []listedFaceitPlayer) {
		t.Helper()
		rw := httptest.NewRecorder()
		router.ServeHTTP(rw, httptest.NewRequest(http.MethodGet, "/api/faceit/followed", nil))
		var body struct {
			Refreshing bool                 `json:"refreshing"`
			Players    []listedFaceitPlayer `json:"players"`
		}
		if err := json.Unmarshal(rw.Body.Bytes(), &body); err != nil {
			t.Fatalf("list body %s: %v", rw.Body.String(), err)
		}
		return body.Refreshing, body.Players
	}

	if refreshing, _ := list(); refreshing || faceitHits.Load() != 0 {
		t.Fatalf("list before the startup refresh: refreshing=%v hits=%d", refreshing, faceitHits.Load())
	}
	h.StartFaceitRosterRefresh()
	if refreshing, _ := list(); !refreshing {
		t.Fatal("list during the startup refresh reports refreshing=false")
	}
	close(release)
	h.faceitRoster.wg.Wait()

	refreshing, players := list()
	if refreshing || len(players) != 1 || players[0].Nickname != "m0NESY-renamed" || players[0].ELO != 4123 || players[0].Avatar != steamAvatar {
		t.Fatalf("after refresh: refreshing=%v players=%#v, want the live profile", refreshing, players)
	}
	if faceitHits.Load() != 1 || rankingHits.Load() != 0 {
		t.Fatalf("startup refresh made %d requests, %d of them rankings; want the one profile", faceitHits.Load(), rankingHits.Load())
	}
}

type listedFaceitPlayer struct {
	ID       string `json:"id"`
	Nickname string `json:"nickname"`
	ELO      int    `json:"elo"`
	Avatar   string `json:"avatar"`
}

func TestProxyFaceitAvatarOnlyFetchesAllowedAvatarHosts(t *testing.T) {
	t.Parallel()
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		_, _ = w.Write([]byte("<html>internal admin page</html>"))
	}))
	t.Cleanup(server.Close)
	h := NewHandlers(newFakeRepo(), newFakeStorage(), &fakeQueue{})
	h.faceitCache.storeAvatar("player-1", server.URL+"/avatar.png")

	res := httptest.NewRecorder()
	Routes(h).ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/api/faceit/players/player-1/avatar", nil))
	if res.Code != http.StatusNotFound || hits.Load() != 0 {
		t.Fatalf("status=%d upstream hits=%d, want 404 and no request", res.Code, hits.Load())
	}
}
