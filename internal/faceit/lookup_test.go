package faceit

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

func TestLookupPlayerReturnsProfile(t *testing.T) {
	t.Parallel()

	const apiKey = "faceit-lookup-secret"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got, want := r.Header.Get("Authorization"), "Bearer "+apiKey; got != want {
			t.Errorf("authorization = %q, want %q", got, want)
		}
		if r.URL.Path != "/players" {
			http.NotFound(w, r)
			return
		}
		assertQueryValue(t, r.URL.Query(), "nickname", "m0NESY")
		assertQueryValue(t, r.URL.Query(), "game", "cs2")
		_, _ = w.Write([]byte(`{
			"player_id":"player-1","nickname":"m0NESY","country":"ru",
			"avatar":"https://assets.faceit-cdn.net/avatars/m0nesy.png",
			"steam_id_64":"76561198000000001",
			"games":{"cs2":{"region":"EU","skill_level":10,"faceit_elo":4000,"game_player_id":"76561198000000001"}}
		}`))
	}))
	defer server.Close()

	client, err := New(Options{APIKey: apiKey, BaseURL: server.URL, HTTPClient: server.Client()})
	if err != nil {
		t.Fatal(err)
	}
	player, err := client.LookupPlayer(context.Background(), "https://www.faceit.com/en/players/m0NESY")
	if err != nil {
		t.Fatalf("LookupPlayer: %v", err)
	}
	want := Player{
		ID:         "player-1",
		Nickname:   "m0NESY",
		Avatar:     "https://assets.faceit-cdn.net/avatars/m0nesy.png",
		SteamID64:  "76561198000000001",
		ProfileURL: "https://www.faceit.com/en/players/m0NESY",
		Country:    "ru",
		Region:     "EU",
		SkillLevel: 10,
		ELO:        4000,
	}
	if player != want {
		t.Fatalf("player = %#v, want %#v", player, want)
	}
}

func TestLookupPlayerTable(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		apiKey  string
		status  int
		body    string
		profile string
		wantErr error
	}{
		{name: "missing key", profile: "m0NESY", wantErr: ErrNotConfigured},
		{name: "not found", apiKey: "faceit-test-key", status: http.StatusNotFound, body: `{}`, profile: "missing", wantErr: ErrPlayerNotFound},
		{name: "unauthorized", apiKey: "faceit-test-key", status: http.StatusUnauthorized, body: `{}`, profile: "m0NESY", wantErr: ErrUnauthorized},
		{name: "invalid nickname", apiKey: "faceit-test-key", profile: "name/slash", wantErr: errors.New("unsupported")},
		{name: "credential reflected", apiKey: "leaked-key", status: http.StatusOK, body: `{"player_id":"player-1","nickname":"leaked-key","games":{"cs2":{}}}`, profile: "m0NESY", wantErr: ErrInvalidResponse},
		{name: "invalid id", apiKey: "faceit-test-key", status: http.StatusOK, body: `{"player_id":"bad id","nickname":"x","games":{"cs2":{}}}`, profile: "m0NESY", wantErr: ErrInvalidResponse},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(test.status)
				_, _ = w.Write([]byte(test.body))
			}))
			t.Cleanup(server.Close)
			client, err := New(Options{APIKey: test.apiKey, BaseURL: server.URL, HTTPClient: server.Client()})
			if err != nil {
				t.Fatal(err)
			}
			_, err = client.LookupPlayer(context.Background(), test.profile)
			if test.wantErr == nil {
				if err != nil {
					t.Fatalf("error = %v, want nil", err)
				}
				return
			}
			if err == nil {
				t.Fatal("error = nil, want error")
			}
			if errors.Is(test.wantErr, ErrNotConfigured) || errors.Is(test.wantErr, ErrPlayerNotFound) || errors.Is(test.wantErr, ErrUnauthorized) || errors.Is(test.wantErr, ErrInvalidResponse) {
				if !errors.Is(err, test.wantErr) {
					t.Fatalf("error = %v, want %v", err, test.wantErr)
				}
			}
			if test.apiKey != "" && strings.Contains(fmt.Sprint(err), test.apiKey) {
				t.Fatal("error reflected API key")
			}
		})
	}
}

func TestRecentMatchesMergesHistoryAndStats(t *testing.T) {
	t.Parallel()

	const apiKey = "faceit-recent-secret"
	var historyLimit, statsLimit string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/players/player-1/history":
			historyLimit = r.URL.Query().Get("limit")
			assertQueryValue(t, r.URL.Query(), "game", "cs2")
			_, _ = w.Write([]byte(`{"items":[
				{"match_id":"match-old","competition_name":"FACEIT 5v5","started_at":1767225600,"finished_at":1767229200,
				 "teams":{"faction1":{"players":[{"player_id":"player-1"}]},"faction2":{"players":[{"player_id":"other"}]}},
				 "results":{"winner":"faction1","score":{"faction1":13,"faction2":8}}},
				{"match_id":"match-new","competition_name":"FACEIT 5v5","started_at":1767312000,"finished_at":1767315600,
				 "teams":{"faction1":{"players":[{"player_id":"other"}]},"faction2":{"players":[{"player_id":"player-1"}]}},
				 "results":{"winner":"faction1","score":{"faction1":13,"faction2":11}}}
			]}`))
		case "/players/player-1/games/cs2/stats":
			statsLimit = r.URL.Query().Get("limit")
			_, _ = w.Write([]byte(`{"items":[
				{"stats":{"Match Id":"match-new","Map":"de_ancient","Result":"0","Kills":"30","Deaths":"18","Assists":"4","ADR":"110.0","K/D Ratio":"1.67","Headshots %":"40"}},
				{"stats":{"Match Id":"match-old","Map":"de_mirage","Result":"1","Kills":"22","Deaths":"14","Assists":"5","ADR":"100.0","K/D Ratio":"1.57","Headshots %":"22"}}
			]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client, err := New(Options{APIKey: apiKey, BaseURL: server.URL, HTTPClient: server.Client()})
	if err != nil {
		t.Fatal(err)
	}
	matches, err := client.RecentMatches(context.Background(), "player-1", 10)
	if err != nil {
		t.Fatalf("RecentMatches: %v", err)
	}
	if historyLimit != "10" || statsLimit != "10" {
		t.Fatalf("limits history=%q stats=%q, want 10", historyLimit, statsLimit)
	}
	if len(matches) != 2 {
		t.Fatalf("matches = %d, want 2", len(matches))
	}
	if matches[0].ID != "match-new" || matches[0].Stats == nil || matches[0].Stats.Result != "loss" || matches[0].Stats.Map != "de_ancient" {
		t.Fatalf("first match = %#v", matches[0])
	}
	if matches[0].Score.For != 11 || matches[0].Score.Against != 13 {
		t.Fatalf("score = %#v", matches[0].Score)
	}
	if matches[1].Stats == nil || matches[1].Stats.Result != "win" {
		t.Fatalf("second match = %#v", matches[1])
	}
	serialized, err := json.Marshal(matches)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(serialized), apiKey) {
		t.Fatal("serialized matches contain API key")
	}
}

func TestRecentMatchesClampsLimitAndRejectsInvalidID(t *testing.T) {
	t.Parallel()

	// RecentMatches fetches history and statistics concurrently, so both
	// handler goroutines record the limit they were sent.
	var (
		mu        sync.Mutex
		seenLimit string
	)
	lastLimit := func() string {
		mu.Lock()
		defer mu.Unlock()
		return seenLimit
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		seenLimit = r.URL.Query().Get("limit")
		mu.Unlock()
		_, _ = w.Write([]byte(`{"items":[]}`))
	}))
	defer server.Close()
	client, err := New(Options{APIKey: "faceit-test-key", BaseURL: server.URL, HTTPClient: server.Client()})
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name    string
		id      string
		limit   int
		wantErr bool
	}{
		{name: "invalid id", id: "bad id", limit: 5, wantErr: true},
		{name: "empty id", id: "", limit: 5, wantErr: true},
		{name: "clamped high", id: "player-1", limit: 99},
		{name: "default limit", id: "player-1", limit: 0},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := client.RecentMatches(context.Background(), test.id, test.limit)
			if test.wantErr {
				if err == nil {
					t.Fatal("error = nil, want error")
				}
				return
			}
			if err != nil {
				t.Fatalf("error = %v", err)
			}
			switch test.limit {
			case 99:
				if got := lastLimit(); got != "30" {
					t.Fatalf("limit = %q, want 30", got)
				}
			case 0:
				if got := lastLimit(); got != "10" {
					t.Fatalf("limit = %q, want 10", got)
				}
			}
		})
	}
}

func TestRecentMatchesRequiresAPIKey(t *testing.T) {
	t.Parallel()
	client, err := New(Options{})
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.RecentMatches(context.Background(), "player-1", 10)
	if !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("error = %v, want ErrNotConfigured", err)
	}
}

func TestRecentMatchesDropsCredentialInRoomMetadata(t *testing.T) {
	t.Parallel()
	const apiKey = "faceit-reflected-in-match"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/players/player-1/history":
			_, _ = w.Write([]byte(`{"items":[{"match_id":"match-1","competition_name":"` + apiKey + `","teams":{},"results":{}}]}`))
		case "/players/player-1/games/cs2/stats":
			_, _ = w.Write([]byte(`{"items":[]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	client, err := New(Options{APIKey: apiKey, BaseURL: server.URL, HTTPClient: server.Client()})
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.RecentMatches(context.Background(), "player-1", 10)
	if !errors.Is(err, ErrInvalidResponse) {
		t.Fatalf("error = %v, want ErrInvalidResponse", err)
	}
}

func TestClampRecentMatchLimit(t *testing.T) {
	t.Parallel()
	tests := []struct {
		in   int
		want int
	}{
		{in: 0, want: 10},
		{in: -3, want: 10},
		{in: 5, want: 5},
		{in: 20, want: 20},
		{in: 30, want: 30},
		{in: 31, want: 30},
	}
	for _, test := range tests {
		if got := clampRecentMatchLimit(test.in); got != test.want {
			t.Errorf("clampRecentMatchLimit(%d) = %d, want %d", test.in, got, test.want)
		}
	}
}

func TestLookupPlayerDropsInvalidAvatar(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"player_id":"player-1","nickname":"x","avatar":"javascript:alert(1)","games":{"cs2":{}}}`))
	}))
	defer server.Close()
	client, err := New(Options{APIKey: "faceit-test-key", BaseURL: server.URL, HTTPClient: server.Client()})
	if err != nil {
		t.Fatal(err)
	}
	player, err := client.LookupPlayer(context.Background(), "x")
	if err != nil {
		t.Fatal(err)
	}
	if player.Avatar != "" {
		t.Fatalf("avatar = %q, want empty", player.Avatar)
	}
}

func TestRecentMatchesDerivesResultWithoutStats(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/players/player-1/history":
			_, _ = w.Write([]byte(`{"items":[{"match_id":"match-1","finished_at":1767315600,
				"teams":{"faction1":{"players":[{"player_id":"player-1"}]}},
				"results":{"winner":"faction1","score":{"faction1":13,"faction2":4}}}]}`))
		case "/players/player-1/games/cs2/stats":
			_, _ = w.Write([]byte(`{"items":[]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	client, err := New(Options{APIKey: "faceit-test-key", BaseURL: server.URL, HTTPClient: server.Client()})
	if err != nil {
		t.Fatal(err)
	}
	matches, err := client.RecentMatches(context.Background(), "player-1", 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 1 || matches[0].Stats == nil || matches[0].Stats.Result != "win" {
		t.Fatalf("matches = %#v", matches)
	}
}

func TestRankingPositionReadsPlayerSlot(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rankings/games/cs2/regions/EU/players/player-1" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(`{"position":31,"items":[{"position":31}]}`))
	}))
	defer server.Close()
	client, err := New(Options{APIKey: "faceit-test-key", BaseURL: server.URL, HTTPClient: server.Client()})
	if err != nil {
		t.Fatal(err)
	}
	got, err := client.RankingPosition(context.Background(), "EU", "player-1")
	if err != nil {
		t.Fatal(err)
	}
	if got != 31 {
		t.Fatalf("ranking = %d, want 31", got)
	}
}

func TestLookupBySteamIDReturnsProfile(t *testing.T) {
	t.Parallel()
	const apiKey = "faceit-steam-secret"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/players" {
			http.NotFound(w, r)
			return
		}
		assertQueryValue(t, r.URL.Query(), "game_player_id", "76561198000000001")
		assertQueryValue(t, r.URL.Query(), "game", "cs2")
		_, _ = w.Write([]byte(`{
			"player_id":"player-1","nickname":"donk666","country":"ru",
			"steam_id_64":"76561198000000001",
			"games":{"cs2":{"skill_level":10,"faceit_elo":4370,"game_player_id":"76561198000000001"}}
		}`))
	}))
	defer server.Close()
	client, err := New(Options{APIKey: apiKey, BaseURL: server.URL, HTTPClient: server.Client()})
	if err != nil {
		t.Fatal(err)
	}
	player, err := client.LookupBySteamID(context.Background(), "76561198000000001")
	if err != nil {
		t.Fatal(err)
	}
	if player.Nickname != "donk666" || player.ELO != 4370 || player.SkillLevel != 10 {
		t.Fatalf("player = %+v", player)
	}
}

func TestAggregateLast20OmitsRatingAndSwing(t *testing.T) {
	t.Parallel()
	got := AggregateLast20([]RecentMatch{
		{Stats: &MatchStats{Result: "win", Kills: 20, Deaths: 10, Assists: 4, KDRatio: 2, KRRatio: 1, ADR: 100}},
		{Stats: &MatchStats{Result: "loss", Kills: 10, Deaths: 20, Assists: 2, KDRatio: 0.5, KRRatio: 0.5, ADR: 60}},
	})
	if got.Matches == nil || *got.Matches != 2 {
		t.Fatalf("matches = %v", got.Matches)
	}
	if got.WinPct == nil || *got.WinPct != 50 {
		t.Fatalf("winpct = %v", got.WinPct)
	}
	if got.Kills == nil || *got.Kills != 30 {
		t.Fatalf("kills = %v", got.Kills)
	}
	if got.KD == nil || *got.KD != 1.25 {
		t.Fatalf("kd = %v", got.KD)
	}
	empty := AggregateLast20(nil)
	if empty.Matches != nil || empty.WinPct != nil {
		t.Fatalf("empty aggregate invented values: %+v", empty)
	}
}
