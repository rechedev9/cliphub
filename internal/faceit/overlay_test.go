package faceit

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestOverlayPlayersLooksUpRosterSteamIDs(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/players" && r.URL.Query().Get("game_player_id") == "76561198000000001":
			_, _ = w.Write([]byte(`{"player_id":"player-1","nickname":"donk666","country":"ru","avatar":"https://assets.faceit-cdn.net/avatars/donk.png","steam_id_64":"76561198000000001","games":{"cs2":{"region":"EU","skill_level":10,"faceit_elo":4370}}}`))
		case strings.HasPrefix(r.URL.Path, "/players/player-1/history"), strings.HasPrefix(r.URL.Path, "/players/player-1/games/cs2/stats"):
			_, _ = w.Write([]byte(`{"items":[]}`))
		case r.URL.Path == "/rankings/games/cs2/regions/EU/players/player-1":
			_, _ = w.Write([]byte(`{"position":1}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	client, err := New(Options{APIKey: "faceit-test-key", BaseURL: server.URL, HTTPClient: server.Client()})
	if err != nil {
		t.Fatal(err)
	}
	got, err := client.OverlayPlayers(context.Background(), []string{"76561198000000001", "", "76561198000000001"})
	if err != nil {
		t.Fatal(err)
	}
	player := got["76561198000000001"]
	if player.Nickname != "donk666" || player.ELO != 4370 || player.Ranking == nil || *player.Ranking != 1 {
		t.Fatalf("overlay player = %+v", player)
	}
	if player.Recent.Matches != nil {
		t.Fatalf("empty match history invented recent stats: %+v", player.Recent)
	}
}

func TestOverlayPlayersSwallowsRankingOutcomes(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name          string
		rankingStatus int
		rankingBody   string
		want          *int
	}{
		{name: "ranked", rankingStatus: http.StatusOK, rankingBody: `{"position":7}`, want: intPtr(7)},
		{name: "items fallback", rankingStatus: http.StatusOK, rankingBody: `{"items":[{"position":42}]}`, want: intPtr(42)},
		{name: "unranked", rankingStatus: http.StatusOK, rankingBody: `{"position":0}`, want: nil},
		{name: "not on the leaderboard", rankingStatus: http.StatusNotFound, rankingBody: `{}`, want: nil},
		{name: "ranking request rejected", rankingStatus: http.StatusBadRequest, rankingBody: `{}`, want: nil},
		{name: "ranking body invalid", rankingStatus: http.StatusOK, rankingBody: `not json`, want: nil},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch {
				case r.URL.Path == "/players" && r.URL.Query().Get("game_player_id") == "76561198000000001":
					_, _ = w.Write([]byte(`{"player_id":"player-1","nickname":"donk666","country":"ru","steam_id_64":"76561198000000001","games":{"cs2":{"region":"EU","skill_level":10,"faceit_elo":4370}}}`))
				case strings.HasPrefix(r.URL.Path, "/players/player-1/"):
					_, _ = w.Write([]byte(`{"items":[]}`))
				case r.URL.Path == "/rankings/games/cs2/regions/EU/players/player-1":
					w.WriteHeader(test.rankingStatus)
					_, _ = w.Write([]byte(test.rankingBody))
				default:
					http.NotFound(w, r)
				}
			}))
			defer server.Close()
			client, err := New(Options{APIKey: "faceit-test-key", BaseURL: server.URL, HTTPClient: server.Client()})
			if err != nil {
				t.Fatal(err)
			}
			got, err := client.OverlayPlayers(context.Background(), []string{"76561198000000001"})
			if err != nil {
				t.Fatalf("a ranking outcome must never fail the roster: %v", err)
			}
			player := got["76561198000000001"]
			if player.Nickname != "donk666" || player.ELO != 4370 {
				t.Fatalf("profile fields lost: %+v", player)
			}
			switch {
			case test.want == nil && player.Ranking != nil:
				t.Fatalf("ranking = %d, want nil", *player.Ranking)
			case test.want != nil && (player.Ranking == nil || *player.Ranking != *test.want):
				t.Fatalf("ranking = %v, want %d", player.Ranking, *test.want)
			}
		})
	}
}

func intPtr(value int) *int {
	return &value
}

func TestOverlayPlayersNilClient(t *testing.T) {
	t.Parallel()
	var client *Client
	got, err := client.OverlayPlayers(context.Background(), []string{"76561198000000001"})
	if !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("error = %v, want ErrNotConfigured", err)
	}
	if len(got) != 0 {
		t.Fatalf("got %+v", got)
	}
}

func TestOverlayPlayersReturnsPartialResultsAndLookupError(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/players" && r.URL.Query().Get("game_player_id") == "ok" {
			_, _ = w.Write([]byte(`{"player_id":"player-ok","nickname":"ready","steam_id_64":"ok","games":{"cs2":{"region":"EU","skill_level":10,"faceit_elo":3000}}}`))
			return
		}
		if strings.HasPrefix(r.URL.Path, "/players/player-ok/") {
			_, _ = w.Write([]byte(`{"items":[]}`))
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()
	client, err := New(Options{APIKey: "faceit-test-key", BaseURL: server.URL, HTTPClient: server.Client()})
	if err != nil {
		t.Fatal(err)
	}
	got, err := client.OverlayPlayers(context.Background(), []string{"missing", "ok"})
	if !errors.Is(err, ErrPlayerNotFound) {
		t.Fatalf("error = %v, want ErrPlayerNotFound", err)
	}
	if len(got) != 1 || got["ok"].Nickname != "ready" {
		t.Fatalf("partial results = %+v, want ready player", got)
	}
}
