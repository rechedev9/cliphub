package main

import (
	"net/http"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	testSeriesID   = "6f7a8b9c-adbe-4fc0-b142-5d6e7f8091a2"
	testSeriesJSON = `{"id":"` + testSeriesID + `","format":"bo3","event":"IEM","veto":"","teams":[{"slot":"a","team_id":"` + testTeamID + `","name":"Vitality"},
		{"slot":"b","team_id":"` + testRivalID + `","name":"MOUZ"}],"vetoes":[{"team":"a","action":"ban","map":"de_nuke"}],"match_ids":["` + testMatchID + `"],"updated_at":"2026-11-01T00:00:00Z"}`
)

func seriesStudio(t *testing.T) (*mcp.ClientSession, *recorder) {
	t.Helper()
	rec := &recorder{seen: map[string]string{}}
	mux := http.NewServeMux()
	rec.handle(mux, "GET /api/library/series", `{"series":[`+testSeriesJSON+`]}`)
	rec.handle(mux, "GET /api/library/series/{id}", testSeriesJSON)
	rec.handle(mux, "POST /api/library/series", testSeriesJSON)
	rec.handle(mux, "PATCH /api/library/series/{id}", testSeriesJSON)
	rec.handle(mux, "DELETE /api/library/series/{id}", ``)
	rec.handle(mux, "PATCH /api/library/matches/{id}", `{"id":"`+testMatchID+`","map":"de_nuke"}`)
	rec.handle(mux, "GET /api/library/teams/{id}/maps", `{"team_name":"Vitality","matches":4,"series":1,"maps":[{"map":"de_nuke","matches":4,"wins":3,"losses":1,
		"rounds_won":52,"rounds_lost":40,"sides":[{"side":"CT","rounds":46,"won":{"count":30}},{"side":"T","rounds":46,"won":{"count":22}}],
		"veto":{"picked":{"count":2},"banned":{"count":0}},"preference":{"score":64}}]}`)
	rec.handle(mux, "GET /api/library/teams/{id}/map-counts", `{"maps":[{"map":"de_nuke","matches":2},{"map":"de_mirage","matches":0}]}`)
	rec.handle(mux, "GET /api/library/teams/{id}/veto", `{"team_name":"Vitality","opponent_name":"MOUZ","plan":{"format":"bo3","first":"a",
		"maps":[{"map":"de_nuke","our_matches":4,"our_rate":0.5652,"their_matches":2,"their_rate":0.48,"edge":0.0852,"their_bans":1}],
		"steps":[{"team":"a","action":"ban","map":"de_train","basis":"chosen"},{"team":"b","action":"ban","map":"de_nuke","basis":"history"}],
		"played":["de_mirage","de_inferno","de_ancient"],"opponent_history":3}}`)
	cs := serveTools(t, mux, func(server *mcp.Server, s *studio) { registerLibrarySeriesTools(server, s) })
	return cs, rec
}

func TestSeriesToolsSendWhatTheUISends(t *testing.T) {
	cs, rec := seriesStudio(t)
	var list listSeriesOut
	callTool(t, cs, "list_library_series", map[string]any{"team_id": testTeamID}, &list)
	if len(list.Series) != 1 || len(list.Series[0].Teams) != 2 || list.Series[0].Vetoes[0].Map != "de_nuke" {
		t.Fatalf("series = %+v", list)
	}

	var created seriesOut
	callTool(t, cs, "create_library_series", map[string]any{"event": " IEM ", "team_ids": []string{testTeamID, testRivalID},
		"vetoes": []map[string]string{{"team": "A", "action": "Ban", "map": "de_nuke"}, {"action": "decider", "map": "de_mirage"}}}, &created)
	want := `{"format":"bo3","event":"IEM","team_ids":["` + testTeamID + `","` + testRivalID + `"],"vetoes":[{"team":"a","action":"ban","map":"de_nuke"},{"team":"","action":"decider","map":"de_mirage"}]}`
	if got := rec.get("POST /api/library/series"); !strings.HasSuffix(got, want) {
		t.Errorf("create = %s\nwant %s", got, want)
	}
	toolFails(t, cs, "create_library_series", map[string]any{"team_ids": []string{testTeamID}}, "two teams")
	toolFails(t, cs, "create_library_series", map[string]any{"vetoes": []map[string]string{{"team": "a", "action": "decider", "map": "de_nuke"}}}, "veto step 1")
	toolFails(t, cs, "update_library_series", map[string]any{"series_id": testSeriesID}, "set format")

	var m matchSummary
	callTool(t, cs, "set_match_series", map[string]any{"match_id": testMatchID}, &m)
	if got := rec.get("PATCH /api/library/matches/{id}"); !strings.HasSuffix(got, `{"series_id":null}`) {
		t.Errorf("take out = %s", got)
	}
	callTool(t, cs, "set_match_series", map[string]any{"match_id": testMatchID, "series_id": testSeriesID}, &m)
	if got := rec.get("PATCH /api/library/matches/{id}"); !strings.HasSuffix(got, `{"series_id":"`+testSeriesID+`"}`) {
		t.Errorf("join = %s", got)
	}

	var del deleteOut
	callTool(t, cs, "delete_library_series", map[string]any{"series_id": testSeriesID}, &del)
	if !del.NeedsApproval || !strings.Contains(del.Summary, "BO3 «Vitality» contra «MOUZ» con su veto. Su partida se queda") || rec.get("DELETE /api/library/series/{id}") != "" {
		t.Fatalf("delete = %+v", del)
	}
}

func TestTeamMapsAndVeto(t *testing.T) {
	cs, rec := seriesStudio(t)
	var maps teamMapsOut
	callTool(t, cs, "team_maps", map[string]any{"team_id": testTeamID}, &maps)
	m := maps.Maps[0]
	if m.Record != "3-1-0" || m.RoundWinPct != 56.5 || m.CTWinPct != 65.2 || m.Vetoes["picked"] != 2 || len(m.Vetoes) != 1 || m.Preference != 64 {
		t.Errorf("map = %+v", m)
	}

	if m.ScopeMatches != 2 {
		t.Errorf("scope_matches = %d, want the 2 matches the server counted on de_nuke", m.ScopeMatches)
	}
	callTool(t, cs, "team_maps", map[string]any{"team_id": testTeamID, "opponent_id": []string{testRivalID}, "lineup": []string{testSteamID}, "max_matches": 5, "whose": "own"}, &maps)
	if got := rec.get("GET /api/library/teams/{id}/map-counts"); !strings.Contains(got, "opponent_id="+testRivalID) || !strings.Contains(got, "lineup="+testSteamID) || !strings.Contains(got, "max_matches=5") || !strings.Contains(got, "whose=own") {
		t.Errorf("map-counts query = %s", got)
	}
	toolFails(t, cs, "team_maps", map[string]any{"team_id": testTeamID, "opponent_top": 7}, "opponent_top")

	var v vetoOut
	callTool(t, cs, "simulate_veto", map[string]any{"team_id": testTeamID, "opponent_id": testRivalID, "format": "bo3", "done": []string{"DE_TRAIN"}}, &v)
	if got := rec.get("GET /api/library/teams/{id}/veto"); !strings.Contains(got, "done=de_train") || !strings.Contains(got, "opponent_id="+testRivalID) {
		t.Errorf("veto query = %s", got)
	}
	if len(v.Steps) != 2 || v.Steps[1].Basis != "history" || v.Maps[0].EdgePct != 8.5 || v.Maps[0].OurRate != 56.5 || len(v.Played) != 3 {
		t.Errorf("veto = %+v", v)
	}
	toolFails(t, cs, "simulate_veto", map[string]any{"team_id": testTeamID, "opponent_id": testTeamID}, "another team")
}
