package main

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	testPatternID   = "6a7b8c9d-0e1f-4a2b-8c3d-4e5f6a7b8c9d"
	testPatternTeam = "1b2c3d4e-5f6a-4b7c-8d9e-0f1a2b3c4d5e"
	testPatternJSON = `{"id":"` + testPatternID + `","name":"Banana","map":"de_inferno","team_id":"` + testPatternTeam + `",` +
		`"pattern":{"side":"T","blocks":[{"kind":"player","from":0}]},"scope":{"matches":10},"updated_at":"2026-11-07T10:00:00Z",` +
		`"stats":{"rounds":40,"matched":12,"won":8,"win_rate":0.667,"recent_matched":5,"trend":"up"}}`
	testPatternRun = `{"map":"de_inferno","matches":[{"match_id":"` + testCallID + `"}],"pattern":{"side":"T","blocks":[{"kind":"player","from":0}]},` +
		`"result":{"rounds":24,"matched":2,"won":1,"win_rate":0.5,"blocks":[{"matched":2,"rounds":24}],"missing_positions":0,"recent_rounds":24,` +
		`"recent_matched":2,"trend":"","months":[{"month":"2026-11","rounds":24,"matched":2}],"matches":[{"match_id":"` + testCallID + `",` +
		`"played_at":"2026-11-05T18:00:00Z","team":"FUT","opponent":"BetBoom","rounds":24,"matched":[{"round":5,"side":"T","won":true,"hits":[],"seconds":13,"tick":9000},` +
		`{"round":9,"side":"T","won":false,"hits":[]}]}]}}`
)

func patternStudio(t *testing.T) (*mcp.ClientSession, *recorder) {
	t.Helper()
	rec := &recorder{seen: map[string]string{}}
	mux := http.NewServeMux()
	rec.handle(mux, "GET /api/library/teams/{id}/pattern", testPatternRun)
	rec.handle(mux, "GET /api/library/patterns", `{"patterns":[`+testPatternJSON+`]}`)
	rec.handle(mux, "GET /api/library/patterns/{id}", testPatternJSON)
	rec.handle(mux, "GET /api/library/patterns/{id}/rounds", testPatternRun)
	rec.handle(mux, "POST /api/library/patterns", testPatternJSON)
	rec.handle(mux, "DELETE /api/library/patterns/{id}", ``)
	cs := serveTools(t, mux, func(server *mcp.Server, s *studio) { registerPatternTools(server, s) })
	return cs, rec
}

func TestFindPlayerPatternSendsOneBlockAndOpensTheFinder(t *testing.T) {
	cs, rec := patternStudio(t)
	var out patternRunOut
	callTool(t, cs, "find_player_pattern", map[string]any{
		"team_id": testPatternTeam, "map": "de_inferno", "side": "t", "players": []string{"76561198000000001"},
		"area": map[string]any{"min_x": -100, "min_y": 0, "max_x": 300, "max_y": 200}, "clock_from": 5, "clock_to": 30, "invert": true, "matches": 10,
	}, &out)
	got, err := url.QueryUnescape(rec.get("GET /api/library/teams/{id}/pattern"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"map=de_inferno", "matches=10", `"side":"T"`, `"kind":"player"`, `"players":["76561198000000001"]`, `"min_x":-100`, `"from":5`, `"to":30`, `"invert":true`} {
		if !strings.Contains(got, want) {
			t.Errorf("request %s lacks %s", got, want)
		}
	}
	if out.Matched != 2 || len(out.Matches) != 1 || len(out.Matches[0].Matched) != 2 {
		t.Fatalf("out = %+v", out)
	}
	if row := out.Matches[0].Matched[0]; row.Ref != testCallID+"/5" || row.Clock != "1:42" || !row.Won {
		t.Errorf("first row = %+v", row)
	}
	if !strings.Contains(out.ReplayPath, url.QueryEscape(testCallID+":5:9000")) {
		t.Errorf("replay_path = %s", out.ReplayPath)
	}
	studio, err := url.Parse(out.StudioPath)
	if err != nil || studio.Path != "/equipos/"+testPatternTeam || studio.Query().Get("forma") != "patrones" || studio.Query().Get("limit") != "10" ||
		!strings.Contains(studio.Query().Get("patron"), `"invert":true`) {
		t.Errorf("studio_path = %s", out.StudioPath)
	}

	toolFails(t, cs, "find_player_pattern", map[string]any{"team_id": testPatternTeam, "side": ""}, "side must be T or CT")
	toolFails(t, cs, "find_pattern_rounds", map[string]any{"team_id": testPatternTeam, "side": "CT", "blocks": []map[string]any{{"kind": "kill"}}}, "kind")
	toolFails(t, cs, "find_player_pattern", map[string]any{"team_id": testPatternTeam, "side": "T", "area": map[string]any{"min_x": 5, "min_y": 0, "max_x": 1, "max_y": 9}}, "area needs")
}

func TestPatternToolsSaveListRunDelete(t *testing.T) {
	cs, rec := patternStudio(t)
	var saved savedPatternOut
	callTool(t, cs, "save_pattern", map[string]any{"name": " Banana ", "map": "de_inferno", "side": "T",
		"blocks": []map[string]any{{"kind": "utility", "grenade": "smoke", "rival": true}}, "buy": "FULL"}, &saved)
	body := rec.get("POST /api/library/patterns")
	for _, want := range []string{`"name":"Banana"`, `"kind":"utility"`, `"grenade":"smoke"`, `"rival":true`, `"buy":"full"`} {
		if !strings.Contains(body, want) {
			t.Errorf("save body %s lacks %s", body, want)
		}
	}
	if strings.Contains(body, "team_id") {
		t.Errorf("a library-wide save sent a team: %s", body)
	}

	var list listPatternsOut
	callTool(t, cs, "list_patterns", map[string]any{"map": "de_inferno"}, &list)
	if got := rec.get("GET /api/library/patterns"); !strings.Contains(got, "stats=1") || strings.Contains(got, "team_id") {
		t.Errorf("list query = %s", got)
	}
	if len(list.Patterns) != 1 || list.Patterns[0].Stats == nil || list.Patterns[0].Stats.Trend != "up" || list.Patterns[0].StudioPath == "" {
		t.Fatalf("list = %+v", list)
	}

	var run patternRunOut
	callTool(t, cs, "pattern_rounds", map[string]any{"pattern_id": testPatternID}, &run)
	if run.Rounds != 24 || !strings.Contains(run.StudioPath, testPatternTeam) {
		t.Errorf("rounds = %+v", run)
	}

	var del deleteOut
	callTool(t, cs, "delete_pattern", map[string]any{"pattern_id": testPatternID}, &del)
	if !del.NeedsApproval || !strings.Contains(del.Summary, "Banana") || rec.get("DELETE /api/library/patterns/{id}") != "" {
		t.Fatalf("delete without approval = %+v", del)
	}
}
