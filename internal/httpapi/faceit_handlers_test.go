package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
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
		Enabled bool `json:"enabled"`
		Players []struct {
			ID       string `json:"id"`
			Nickname string `json:"nickname"`
		} `json:"players"`
	}
	if err := json.Unmarshal(listed.Body.Bytes(), &listBody); err != nil {
		t.Fatal(err)
	}
	if !listBody.Enabled || len(listBody.Players) != 1 || listBody.Players[0].ID != "player-1" {
		t.Fatalf("followed = %#v", listBody)
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
		Players []json.RawMessage `json:"players"`
	}
	if err := json.Unmarshal(empty.Body.Bytes(), &emptyBody); err != nil {
		t.Fatal(err)
	}
	if len(emptyBody.Players) != 0 {
		t.Fatalf("players after unfollow = %s", empty.Body.String())
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
