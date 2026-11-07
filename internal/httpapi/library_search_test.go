package httpapi

import (
	"net/http"
	"testing"
)

func TestSearchLibraryFindsEveryKind(t *testing.T) {
	env := newPlaysTestEnv(t)
	env.seedLibraryMatch("search", "Vitality", "MOUZ")
	env.json(http.MethodPost, "/api/library/plays", map[string]any{"title": "Ejecución A rápida", "map": "de_mirage", "side": "T", "kind": "execute",
		"origin": "manual", "tags": []string{"Vitality"}}, http.StatusCreated, nil)
	env.json(http.MethodPost, "/api/library/plays", map[string]any{"title": "Ejecución B", "map": "de_nuke", "side": "T", "kind": "execute", "origin": "manual"}, http.StatusCreated, nil)
	env.json(http.MethodPost, "/api/library/lineups", map[string]any{"title": "Humo ventana", "map": "de_mirage", "side": "T", "grenade": "smoke",
		"origin": []float64{1, 2, 3}, "pitch": -10, "yaw": 90}, http.StatusCreated, nil)
	env.json(http.MethodPost, "/api/library/lineups", map[string]any{"title": "Flash ventana", "map": "de_mirage", "side": "T", "grenade": "flash",
		"origin": []float64{1, 2, 3}, "pitch": -10, "yaw": 90}, http.StatusCreated, nil)
	env.json(http.MethodPost, "/api/library/calls", map[string]any{"name": "Rotación", "meaning": "a B & mid"}, http.StatusCreated, nil)

	count := func(query string) map[string]int {
		t.Helper()
		var resp searchResponse
		env.json(http.MethodGet, "/api/library/search?q="+query, nil, http.StatusOK, &resp)
		kinds := map[string]int{}
		for _, r := range resp.Results {
			kinds[r.Kind]++
		}
		return kinds
	}
	for query, want := range map[string]map[string]int{
		"ejecucion":          {searchPlay: 2},
		"ejecucion+mirage":   {searchPlay: 1},
		"ventana+humo":       {searchLineup: 1},
		"ventana":            {searchLineup: 2},
		"rotacion+y+b":       {searchCall: 1},
		"vitality":           {searchPlay: 1, searchTeam: 1, searchMatch: 1},
		"mirage":             {searchPlay: 1, searchLineup: 2, searchMatch: 1},
		"nada+que+encontrar": {},
		"":                   {},
	} {
		got := count(query)
		if len(got) != len(want) {
			t.Fatalf("search %q = %v, want %v", query, got, want)
		}
		for kind, n := range want {
			if got[kind] != n {
				t.Fatalf("search %q = %v, want %v", query, got, want)
			}
		}
	}
	if rw := env.do(http.MethodGet, "/api/library/search?q=x&limit=99", nil, ""); rw.Code != http.StatusBadRequest {
		t.Fatalf("limit 99 = %d, want 400", rw.Code)
	}
}
