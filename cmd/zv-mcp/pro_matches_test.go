package main

import (
	"net/http"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/rechedev9/cliphub/internal/community"
)

const testProMatchesJSON = `{"status":"ok","updated_at":"2026-11-09T03:00:00Z","days":7,"page":1,"pages":1,"total":1,
	"matches":[{"id":2400001,"event":"IEM Chengdu","played_at":"2026-11-08T15:00:00Z",
	 "teams":[{"id":9565,"name":"Vitality","rank":1,"followed":true},{"id":6667,"name":"FaZe","followed":false}],
	 "maps":[{"entry":"vitality-vs-faze-m1-cache","map":"de_cache","rounds":24,"analysis_version":6,"radar_may_be_off":true,
	   "library_match_id":"11111111-1111-4111-8111-111111111111"},{"entry":"vitality-vs-faze-m2-nuke","map":"de_nuke","rounds":20,"analysis_version":6}],
	 "series":{"format":"bo3","maps":[{"number":1,"map":"de_cache","picked_by":9565},{"number":2,"map":"de_nuke","picked_by":6667},{"number":3,"map":"de_mirage","decider":true}],
	   "veto":[{"team_id":6667,"team":"FaZe","action":"ban","map":"de_inferno"},{"team":"Faze Clan","action":"ban","map":"de_train"},{"action":"decider","map":"de_mirage"}]}}],
	"facets":{"teams":[{"id":9565,"name":"Vitality","matches":1}],"events":[{"name":"IEM Chengdu","matches":1}],"maps":[{"name":"de_cache","matches":1}]},
	"follows":[{"team_id":9565,"name":"Vitality","followed_at":"2026-11-01T00:00:00Z"}]}`

func proMatchStudio(t *testing.T, catalog string) (*mcp.ClientSession, *recorder) {
	t.Helper()
	rec := &recorder{seen: map[string]string{}}
	mux := http.NewServeMux()
	rec.handle(mux, "GET /api/library/pro-matches", catalog)
	rec.handle(mux, "POST /api/library/pro-matches/{match}/{entry}/import",
		`{"match_id":"22222222-2222-4222-8222-222222222222","demo_id":"33333333-3333-4333-8333-333333333333","created":true,"studio_path":"/partidas/22222222-2222-4222-8222-222222222222?tab=replay"}`)
	rec.handle(mux, "POST /api/library/pro-follows", `{"team_id":6667,"name":"FaZe","followed_at":"2026-11-10T00:00:00Z"}`)
	rec.handle(mux, "GET /api/library/pro-follows", `{"follows":[{"team_id":6667,"name":"FaZe","followed_at":"2026-11-10T00:00:00Z"}]}`)
	mux.HandleFunc("DELETE /api/library/pro-follows/{team}", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	return serveTools(t, mux, registerProMatchTools), rec
}

func TestListAndImportProMatches(t *testing.T) {
	cs, rec := proMatchStudio(t, testProMatchesJSON)
	var list listProMatchesOut
	callTool(t, cs, "list_pro_matches", map[string]any{"team": "9565", "days": 30, "map": "DE_CACHE"}, &list)
	if got := rec.seen["GET /api/library/pro-matches"]; got != "GET /api/library/pro-matches?days=30&map=de_cache&team=9565" {
		t.Errorf("query = %q", got)
	}
	if len(list.Matches) != 1 || list.Matches[0].PlayedAt != "2026-11-08" || !list.Matches[0].Maps[0].RadarMayBeOff ||
		list.Matches[0].Maps[0].LibraryMatchID == "" || !list.Matches[0].Teams[0].Followed || len(list.Follows) != 1 {
		t.Fatalf("list = %+v", list)
	}
	series, veto := list.Matches[0].Series, list.Matches[0].Veto
	if series.Format != "bo3" || len(series.Maps) != 3 || series.Maps[0].PickBy != "Vitality" || !series.Maps[2].Decider || series.Maps[2].PickBy != "" {
		t.Fatalf("series = %+v", series)
	}
	if len(veto) != 3 || veto[0].Team != "FaZe" || veto[1].Team != "Faze Clan" || veto[2] != (proVetoStepOut{Action: "decider", Map: "de_mirage"}) {
		t.Fatalf("veto = %+v", veto)
	}
	toolFails(t, cs, "list_pro_matches", map[string]any{"days": 500}, "days must be")

	var imported importProMatchOut
	callTool(t, cs, "import_pro_match", map[string]any{"hltv_match_id": 2400001, "entry": "vitality-vs-faze-m2-nuke.dem"}, &imported)
	if !imported.Created || imported.StudioPath != "/partidas/22222222-2222-4222-8222-222222222222?tab=replay" {
		t.Fatalf("import = %+v", imported)
	}
	toolFails(t, cs, "import_pro_match", map[string]any{"hltv_match_id": 2400001, "entry": "../x"}, "must come from list_pro_matches")

	var f proFollowOut
	callTool(t, cs, "follow_pro_team", map[string]any{"team_id": 6667, "name": "FaZe"}, &f)
	if f.TeamID != 6667 || f.FollowedAt != "2026-11-10" {
		t.Fatalf("follow = %+v", f)
	}
	var follows proFollowsOut
	callTool(t, cs, "list_pro_follows", map[string]any{}, &follows)
	if len(follows.Follows) != 1 {
		t.Fatalf("follows = %+v", follows)
	}
	var none empty
	callTool(t, cs, "unfollow_pro_team", map[string]any{"team_id": 6667}, &none)
}

func TestListProMatchesExplainsAMissingCatalog(t *testing.T) {
	cs, _ := proMatchStudio(t, `{"status":"missing","matches":[],"facets":{"teams":[],"events":[],"maps":[]},"follows":[]}`)
	toolFails(t, cs, "list_pro_matches", map[string]any{}, "no pro match catalog yet")
}

func TestProListPathCarriesTheFilters(t *testing.T) {
	if got := proListPath(community.ProMatchQuery{Days: community.DefaultProMatchDays, Page: 1}); got != "/partidas/pro" {
		t.Fatalf("defaults = %q", got)
	}
	got := proListPath(community.ProMatchQuery{Days: 90, Team: "9565", Event: "IEM Chengdu", Map: "de_mirage", Page: 2})
	if got != "/partidas/pro?dias=90&equipo=9565&evento=IEM+Chengdu&mapa=de_mirage&pagina=2" {
		t.Fatalf("filters = %q", got)
	}
}
