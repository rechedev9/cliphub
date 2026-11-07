package httpapi

import (
	"context"
	"net/http"
	"testing"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/library"
	"github.com/rechedev9/cliphub/internal/tacticalplan"
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

func TestSearchLibraryFindsAPlayerByAnAccentedNick(t *testing.T) {
	env := newPlaysTestEnv(t)
	doc := tacticalplan.Document{Teams: []tacticalplan.Team{{Key: "ct-start", Name: "Ñu", StartSide: tacticalplan.SideCT}},
		Players: []tacticalplan.Player{{SteamID64: "76561198000000009", Name: "Peña", TeamKey: "ct-start", StartSide: tacticalplan.SideCT}}}
	doc.Demo.Map = "de_nuke"
	d := library.Demo{ID: uuid.New(), SHA256: "accent", FileName: "accent.dem"}
	if _, err := env.repo.CreateDemo(context.Background(), &d); err != nil {
		t.Fatal(err)
	}
	if _, err := env.repo.SaveAnalysis(context.Background(), library.AnalysisFromDocument(d.ID, doc)); err != nil {
		t.Fatal(err)
	}
	var resp searchResponse
	env.json(http.MethodGet, "/api/library/search?q=Pe%C3%B1a", nil, http.StatusOK, &resp)
	if len(resp.Results) == 0 || resp.Results[0].Kind != searchPlayer || resp.Results[0].Title != "Peña" {
		t.Fatalf("search Peña = %+v, want the player Peña", resp.Results)
	}
}
