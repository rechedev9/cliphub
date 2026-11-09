package main

import (
	"net/http"
	"strings"
	"testing"
)

func TestTeamSideStatsForwardsTheScopeAndKeepsGroupAverages(t *testing.T) {
	mux := http.NewServeMux()
	var query string
	mux.HandleFunc("GET /api/library/teams/"+testTeamID+"/side-stats", func(w http.ResponseWriter, r *http.Request) {
		query = r.URL.RawQuery
		_, _ = w.Write([]byte(`{"team_name":"Vitality","skipped":[],"compare":"teams","compare_teams":[{"team_id":"` + testTeamID + `","name":"MOUZ"}],
			"matches":3,"rounds":40,"group_teams":1,"rating_formula":"ClipHub rating",
			"overall":{"rounds_won":{"value":55,"count":22,"total":40,"group_avg":50,"group_teams":1},"pistols_won":{"value":null,"group_avg":null}},
			"ct":{},"t":{},"players":[{"steamid64":"` + testSteamID + `","name":"ZywOo\u0007","role":"awp","rounds":40,"rating":1.31,"kd":1.5,"adr":88.2,"kpr":0.85,"dpr":0.55}]}`))
	})
	cs := serveTools(t, mux, registerTeamSideStatsTool)

	var out teamSideStatsOut
	callTool(t, cs, "team_side_stats", map[string]any{
		"team_id": testTeamID, "map": "de_nuke", "side": "t", "buy": "Full", "compare_to": []string{testTeamID},
	}, &out)
	for _, want := range []string{"buy=full", "side=T", "map=de_nuke", "compare_team=" + testTeamID} {
		if !strings.Contains(query, want) {
			t.Fatalf("query %q lacks %s", query, want)
		}
	}
	rw := out.Overall.RoundsWon
	if rw.Value == nil || *rw.Value != 55 || rw.GroupAvg == nil || *rw.GroupAvg != 50 || out.Overall.PistolsWon.Value != nil {
		t.Fatalf("overall = %+v", out.Overall)
	}
	if len(out.Players) != 1 || out.Players[0].Name != "ZywOo" || out.Players[0].Rating != 1.31 || len(out.CompareTo) != 1 || out.CompareTo[0] != "MOUZ" {
		t.Fatalf("players %+v, compare %v", out.Players, out.CompareTo)
	}
	if !strings.Contains(out.StudioPath, "tab=estadisticas") || !strings.Contains(out.StudioPath, "comparar=") {
		t.Fatalf("studio path = %s", out.StudioPath)
	}
	toolFails(t, cs, "team_side_stats", map[string]any{"team_id": testTeamID, "buy": "rich"}, "buy must be one of")
	toolFails(t, cs, "team_side_stats", map[string]any{"team_id": testTeamID, "compare_to": []string{"nope"}}, "compare_to")
}
