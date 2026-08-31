package faceit

import (
	"context"
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
	got := client.OverlayPlayers(context.Background(), []string{"76561198000000001", "", "76561198000000001"})
	player := got["76561198000000001"]
	if player.Nickname != "donk666" || player.ELO != 4370 || player.Ranking == nil || *player.Ranking != 1 {
		t.Fatalf("overlay player = %+v", player)
	}
	if player.Recent.Matches != nil {
		t.Fatalf("empty match history invented recent stats: %+v", player.Recent)
	}
}

func TestOverlayPlayersNilClient(t *testing.T) {
	t.Parallel()
	var client *Client
	if got := client.OverlayPlayers(context.Background(), []string{"76561198000000001"}); len(got) != 0 {
		t.Fatalf("got %+v", got)
	}
}
