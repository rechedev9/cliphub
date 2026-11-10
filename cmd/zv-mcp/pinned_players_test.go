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
