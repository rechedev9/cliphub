package main

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/rechedev9/cliphub/internal/tuiclient"
)

const testTeamRoundsJSON = `{"team_name":"Vitality","rounds":2,"skipped":[{"match_id":"` + testRivalID + `","reason":"opponent_unranked"}],
	"matches":[{"match_id":"` + testRivalID + `","map":"de_mirage","played_at":"2026-10-12T18:00:00Z","opponent":"MOUZ","total":24,
	"rounds":[{"round":1,"side":"CT","winner":"CT","buy":"pistol","opponent_buy":"pistol","site":"A"},{"round":13,"side":"T","winner":"CT","buy":"pistol","opponent_buy":"pistol","site":"B"}]}]}`

func TestTeamRoundsSendsTheScopeAndOpensTheReplay(t *testing.T) {
	rec := &recorder{seen: map[string]string{}}
	mux := http.NewServeMux()
	rec.handle(mux, "GET /api/library/teams/{id}/rounds", testTeamRoundsJSON)
	cs := serveTools(t, mux, func(server *mcp.Server, s *studio) { registerTeamRoundsTool(server, s) })

	var out teamRoundsOut
	callTool(t, cs, "team_rounds", map[string]any{"team_id": testTeamID, "map": "de_mirage", "side": "CT", "max_matches": 5,
		"buy": []string{"pistol"}, "opponent_buy": []string{"pistol"}, "awp": "with", "advantage": []string{"4v5"}, "opponent_top": 30,
		"whose": "others", "form": "recorridos"}, &out)
	sent := rec.get("GET /api/library/teams/{id}/rounds")
	for _, want := range []string{"max_matches=5", "buy=pistol", "opponent_buy=pistol", "awp=with", "advantage=4v5", "opponent_top=30", "whose=others", "map=de_mirage"} {
		if !strings.Contains(sent, want) {
			t.Errorf("request %q lacks %s", sent, want)
		}
	}
	if out.Rounds != 2 || out.Unranked != 1 || len(out.Matches) != 1 || out.Matches[0].Kept != 2 || out.Matches[0].Rounds[1].Ref != testRivalID+"/13" {
		t.Fatalf("out = %+v", out)
	}
	path, err := url.Parse(out.StudioPath)
	if err != nil || path.Path != "/equipos/"+testTeamID {
		t.Fatalf("studio_path = %q", out.StudioPath)
	}
	q := path.Query()
	if q.Get("tab") != "replay" || q.Get("forma") != "recorridos" || q.Get("limit") != "5" || q.Get("opponent_top") != "30" || q.Get("side") != "CT" {
		t.Fatalf("studio_path query = %v", q)
	}
	if !strings.HasPrefix(out.PlaylistPath, "/replay?r="+testRivalID+"%3A1") {
		t.Fatalf("playlist_path = %q", out.PlaylistPath)
	}

	for _, args := range map[string]map[string]any{
		"an opponent level the bar does not offer": {"opponent_top": 10},
		"an unknown whose":                         {"whose": "mine"},
		"a bad advantage":                          {"advantage": []string{"6v5"}},
		"an AWP value":                             {"awp": "yes"},
		"a period shortcut the bar lacks":          {"last_days": 5},
		"last_days with from":                      {"last_days": 7, "from": "2026-10-01"},
		"too many matches":                         {"max_matches": 51},
	} {
		args["team_id"] = testTeamID
		toolFails(t, cs, "team_rounds", args, "")
	}
}

func TestUpdateTeamSetsTheProfileWithoutARename(t *testing.T) {
	rec := &recorder{seen: map[string]string{}}
	mux := http.NewServeMux()
	rec.handle(mux, "GET /api/library/teams/{id}", testTeamJSON)
	rec.handle(mux, "PUT /api/library/teams/{id}/profile", `{"team_id":"`+testTeamID+`","own":true,"hltv_rank":3,"own_teams":1}`)
	cs := serveTools(t, mux, registerTeamTools)

	var out teamOut
	callTool(t, cs, "update_team", map[string]any{"team_id": testTeamID, "own": true, "hltv_rank": 3}, &out)
	if got := rec.get("PUT /api/library/teams/{id}/profile"); !strings.Contains(got, `"own":true`) || !strings.Contains(got, `"hltv_rank":3`) {
		t.Fatalf("profile request = %q", got)
	}
	if out.Name != "Vitality" || !out.Own || out.HLTVRank != 3 {
		t.Fatalf("out = %+v", out)
	}
	toolFails(t, cs, "update_team", map[string]any{"team_id": testTeamID, "hltv_rank": 1001}, "hltv_rank must be")
}

func TestTeamReplayPathWritesHalfBuys(t *testing.T) {
	q := tuiclient.AnalysisQuery{Scope: url.Values{"buy": {"semi", "force"}, "opponent_buy": {"full"}}}
	raw, err := teamReplayPath(replayPathIn{TeamID: testTeamID, Map: "de_nuke", Query: q})
	path, _ := url.Parse(raw)
	if got := path.Query(); err != nil || got.Get("buy") != "half" || got.Get("opponent_buy") != "full" || got.Get("forma") != "" || got.Get("map") != "de_nuke" {
		t.Fatalf("path query = %v, %v; want buy=half, the resolved map and the default form", got, err)
	}
	for name, scope := range map[string]url.Values{
		"two buys the bar cannot hold": {"buy": {"eco", "full"}},
		"two advantages":               {"advantage": {"4v5", "5v4"}},
	} {
		if _, err := teamReplayPath(replayPathIn{TeamID: testTeamID, Query: tuiclient.AnalysisQuery{Scope: scope}}); err == nil {
			t.Errorf("%s: no error", name)
		}
	}
}
