package httpapi

import (
	"net/http"
	"strings"
	"testing"
)

func TestLibraryPinnedPlayersReplaceInOrderAndRefuseBadIDs(t *testing.T) {
	env := newLibraryTestEnv(t)
	var got pinnedPlayersBody
	env.json(http.MethodGet, "/api/library/pinned-players", nil, http.StatusOK, &got)
	if got.Players == nil || len(got.Players) != 0 {
		t.Fatalf("empty list = %+v, want []", got.Players)
	}
	put := map[string]any{"players": []map[string]any{
		{"steamid64": "76561198000000002", "name": "  ZywOo  "},
		{"steamid64": "76561198000000001", "name": "donk"},
		{"steamid64": "76561198000000002", "name": "again"},
	}}
	env.json(http.MethodPut, "/api/library/pinned-players", put, http.StatusOK, &got)
	if len(got.Players) != 2 || got.Players[0].Name != "ZywOo" || got.Players[1].SteamID64 != "76561198000000001" {
		t.Fatalf("saved = %+v, want two pins in order", got.Players)
	}
	env.json(http.MethodGet, "/api/library/pinned-players", nil, http.StatusOK, &got)
	if len(got.Players) != 2 || got.Players[0].Name != "ZywOo" {
		t.Fatalf("read back = %+v", got.Players)
	}
	many := strings.Repeat(`{"steamid64":"76561198000000001"},`, 100) + `{"steamid64":"76561198000000001"}`
	for _, bad := range []string{`{"players":[{"steamid64":"abc"}]}`, `{"players":[` + many + `]}`, `{"players":[],"x":1}`} {
		if rw := env.do(http.MethodPut, "/api/library/pinned-players", []byte(bad), "application/json"); rw.Code != http.StatusBadRequest {
			t.Fatalf("PUT %.60s = %d, want 400", bad, rw.Code)
		}
	}
	env.json(http.MethodPut, "/api/library/pinned-players", map[string]any{"players": []map[string]any{}}, http.StatusOK, &got)
	if len(got.Players) != 0 {
		t.Fatalf("cleared = %+v", got.Players)
	}
}
