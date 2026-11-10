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
			"matches":3,"rounds":40,"group_teams":1,"group_matches":20,"group_capped":true,"group_min_rounds":20,"compare_skipped":[],"rating_formula":"ClipHub rating",
			"match_list":[{"match_id":"` + testMatchID + `","map":"de_nuke","played_at":"2026-10-01T18:00:00Z","opponent":"MOUZ\u0007","rounds":12,"won":true}],
			"overall":{"rounds_won":{"value":55,"count":22,"total":40,"group_avg":50,"group_teams":1},"pistols_won":{"value":null,"group_avg":null}},
			"ct":{},"t":{},"players":[{"steamid64":"` + testSteamID + `","name":"ZywOo\u0007","role":"awp","rounds":40,"rating":1.31,"kd":1.5,"adr":88.2,"kpr":0.85,"dpr":0.55}]}`))
	})
	cs := serveTools(t, mux, registerTeamSideStatsTool)

	var out teamSideStatsOut
	callTool(t, cs, "team_side_stats", map[string]any{
		"team_id": testTeamID, "map": "de_nuke", "side": "t", "buy": []string{"Semi", "force"}, "opponent_buy": []string{"full"}, "compare_to": []string{testTeamID},
		"from": "2026-09-01", "to": "2026-10-31", "opponent_id": []string{testTeamID}, "awp": "with", "round_from": 3, "max_matches": 5,
	}, &out)
	for _, want := range []string{"buy=semi", "buy=force", "opponent_buy=full", "awp=with", "round_from=3", "max_matches=5", "side=T", "map=de_nuke", "compare_team=" + testTeamID} {
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
	if len(out.MatchList) != 1 || out.MatchList[0].Opponent != "MOUZ" || !out.MatchList[0].Won || !out.GroupCapped || out.GroupMatches != 20 {
		t.Fatalf("match list %+v, group %d capped %v", out.MatchList, out.GroupMatches, out.GroupCapped)
	}
	for _, want := range []string{"tab=estadisticas", "comparar=", "from=2026-09-01", "to=2026-10-31", "opponent=" + testTeamID, "buy=half", "opponent_buy=full"} {
		if !strings.Contains(out.StudioPath, want) {
			t.Fatalf("studio path = %s lacks %s", out.StudioPath, want)
		}
	}
	toolFails(t, cs, "team_side_stats", map[string]any{"team_id": testTeamID, "buy": []string{"rich"}}, "buy")
	toolFails(t, cs, "team_side_stats", map[string]any{"team_id": testTeamID, "buy": []string{"eco", "full"}}, "holds one buy")
	toolFails(t, cs, "team_side_stats", map[string]any{"team_id": testTeamID, "compare_to": []string{"nope"}}, "compare_to")
}
