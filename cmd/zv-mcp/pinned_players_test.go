package main

import (
	"net/http"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestPinnedPlayersReadAndReplace(t *testing.T) {
	rec := &recorder{seen: map[string]string{}}
	mux := http.NewServeMux()
	rec.handle(mux, "GET /api/library/pinned-players", `{"players":[{"steamid64":"`+testSteamID+`","name":"ZywOo"}]}`)
	rec.handle(mux, "PUT /api/library/pinned-players", `{"players":[{"steamid64":"`+testSteamID+`","name":"donk"}]}`)
	cs := serveTools(t, mux, func(server *mcp.Server, s *studio) { registerPinnedPlayerTools(server, s) })

	var got pinnedPlayersOut
	callTool(t, cs, "list_pinned_players", map[string]any{}, &got)
	if len(got.Players) != 1 || got.Players[0].Name != "ZywOo" {
		t.Fatalf("pins = %+v", got)
	}
	callTool(t, cs, "set_pinned_players", map[string]any{"players": []map[string]any{{"steamid64": testSteamID, "name": " donk "}}}, &got)
	if body := rec.get("PUT /api/library/pinned-players"); !strings.Contains(body, `{"players":[{"steamid64":"`+testSteamID+`","name":"donk"}]}`) {
		t.Errorf("put body = %s", body)
	}
	toolFails(t, cs, "set_pinned_players", map[string]any{"players": []map[string]any{{"steamid64": "12"}}}, "SteamID64")
}

func TestLeaderboardRowsMarkPinnedPlayers(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/library/leaderboard", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"scope":"library","metric":"adr","higher_is_better":true,"min_rounds":50,"matches":2,"rounds":90,"skipped":0,
			"players":[{"rank":1,"steamid64":"` + testSteamID + `","name":"ZywOo","rounds":60,"matches":2,"value":90},
			{"rank":2,"steamid64":"76561198000000002","name":"donk","rounds":60,"matches":2,"value":80}]}`))
	})
	mux.HandleFunc("GET /api/library/pinned-players", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"players":[{"steamid64":"76561198000000002","name":"donk"}]}`))
	})
	cs := serveTools(t, mux, func(server *mcp.Server, s *studio) { registerPlayerReportTools(server, s) })
	var board leaderboardOut
	callTool(t, cs, "leaderboard", map[string]any{"metric": "adr"}, &board)
	if len(board.Players) != 2 || board.Players[0].Pinned || !board.Players[1].Pinned {
		t.Fatalf("players = %+v, want only donk pinned", board.Players)
	}
}
