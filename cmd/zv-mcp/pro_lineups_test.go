package main

import (
	"net/http"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const testProIndexJSON = `{"version":1,"map":"de_mirage","recent_days":90,"fresh":true,"geometry":{"big":true},
	"lineups":[
	{"id":"smoke-t-1-2-3-4-0","slug":"smoke/window-from-tspawn","title":"Humo a Window desde TSpawn","kind":"smoke","side":"T",
	 "throw_place":"TSpawn","land_place":"Window","origin":[-1200.5,200,-160],"yaw":135.25,"pitch":-12.5,"movement":"standing",
	 "throws":{"recent":40,"all":90},"matches":{"recent":20,"all":45},"round_share":0.4,"round_win":0.55,"attempts":44,"landed":40,
	 "clock":{"median":3.5,"buckets":[30,10]},"insta":true,
	 "teams":[{"key":"a","name":"Vitality","count":9},{"key":"b","name":"Spirit","count":8},{"key":"c","name":"MOUZ","count":7},{"key":"d","name":"FaZe","count":2}],
	 "with":[{"key":"molotov-ct-5-6-7-8-1","name":"Molotov","count":4,"share":0.2}],"rank":{"position":1,"of":2},
	 "clip":"1/a-m1-mirage-r3-t5000-76561198000000001.mp4",
	 "examples":[{"match":"vitality-vs-spirit","played_at":"2026-10-01T00:00:00Z","round":3,"clock":3.4,"tick":5000,
	   "steamid64":"76561198000000001","clip":"1/a-m1-mirage-r3-t5000-76561198000000001.mp4"},
	  {"match":"1","round":9,"tick":8000,"clip":"../secret"}]},
	{"id":"molotov-ct-5-6-7-8-1","slug":"molotov/bench-from-ct","title":"Molotov a Bench desde CT","kind":"molotov","side":"CT",
	 "throw_place":"CTSpawn","land_place":"Bench","origin":[10,20,30],"yaw":90,"pitch":-5,"movement":"running","jump":true,
	 "throws":{"recent":12,"all":20},"clock":{"median":40}}],
	"combos":[
	{"id":"molotov-ct-5-6-7-8-1+smoke-t-1-2-3-4-0","title":"Molotov Bench + Humo Window","side":"CT","lineups":["molotov-ct-5-6-7-8-1","smoke-t-1-2-3-4-0"],
	 "rounds":{"recent":7,"all":7},"team_count":3,"round_win":0.5,"clock":20,"teams":[{"name":"G2","count":3}],
	 "clip":"1/a-m1-mirage-r3-t5000-76561198000000001-combo.mp4"},
	{"id":"x+y","title":"T combo","side":"T","lineups":["x","y"],"rounds":{"recent":6,"all":6}}]}`

func proStudio(t *testing.T) (*mcp.ClientSession, *recorder) {
	t.Helper()
	rec := &recorder{seen: map[string]string{}}
	mux := http.NewServeMux()
	rec.handle(mux, "GET /api/library/pro-lineups/{map}", testProIndexJSON)
	rec.handle(mux, "POST /api/library/pro-lineups/{map}/lineups/{id}/save", testLineupJSON)
	rec.handle(mux, "POST /api/practice/launch", `{"launched":true,"spots":[]}`)
	mux.HandleFunc("GET /api/library/pro-lineups", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"code":"community_disabled","error":"the community server is disabled"}`))
	})
	return serveTools(t, mux, func(server *mcp.Server, s *studio) {
		registerProLineupTools(server, s)
		registerCommunityTools(server, s)
	}), rec
}

func TestFindProLineupsFiltersAndShapes(t *testing.T) {
	cs, _ := proStudio(t)
	var all findProLineupsOut
	callTool(t, cs, "find_pro_lineups", map[string]any{"map": "DE_MIRAGE", "limit": 1}, &all)
	if all.Total != 2 || len(all.Lineups) != 1 || !all.Truncated || all.Lineups[0].ID != "smoke-t-1-2-3-4-0" {
		t.Fatalf("all = %+v", all)
	}
	if got := all.Lineups[0].Teams; len(got) != 3 || got[2] != "MOUZ" {
		t.Errorf("teams = %v", got)
	}
	for _, args := range []map[string]any{{"side": "ct"}, {"kind": "molotov"}, {"landing": "BEN"}, {"from": "ctspawn"}} {
		args["map"] = "de_mirage"
		var out findProLineupsOut
		callTool(t, cs, "find_pro_lineups", args, &out)
		if out.Total != 1 || out.Lineups[0].ID != "molotov-ct-5-6-7-8-1" {
			t.Errorf("%v = %+v", args, out)
		}
	}
	var insta findProLineupsOut
	callTool(t, cs, "find_pro_lineups", map[string]any{"map": "de_mirage", "insta": true}, &insta)
	if insta.Total != 1 || !insta.Lineups[0].Insta {
		t.Errorf("insta = %+v", insta)
	}
	toolFails(t, cs, "find_pro_lineups", map[string]any{"map": "de_mirage", "kind": "decoy"}, "kind must be one of")
	toolFails(t, cs, "find_pro_lineups", map[string]any{"map": "de_mirage", "limit": 101}, "limit must be")
	toolFails(t, cs, "list_pro_lineup_maps", map[string]any{}, "community server is turned off")
}

func TestGetProLineupAndCombos(t *testing.T) {
	t.Setenv("ZV_COMMUNITY_URL", "https://community.test")
	cs, _ := proStudio(t)
	var l proLineupOut
	callTool(t, cs, "get_pro_lineup", map[string]any{"map": "de_mirage", "id": "smoke/window-from-tspawn"}, &l)
	if l.Setpos != "setpos -1200.50 200.00 -160.00;setang -12.50 135.25 0" || l.Rank != "1 of 2" || len(l.TeamUsage) != 4 ||
		len(l.With) != 1 || l.Examples[0].PlayedAt != "2026-10-01" {
		t.Fatalf("lineup = %+v", l)
	}
	clip := "https://community.test/v1/pro-clips/1/a-m1-mirage-r3-t5000-76561198000000001.mp4"
	if l.ClipURL != clip || l.Examples[0].ClipURL != clip || l.Examples[0].SteamID64 != "76561198000000001" || l.Examples[1].ClipURL != "" {
		t.Fatalf("clips = %q %+v", l.ClipURL, l.Examples)
	}
	toolFails(t, cs, "get_pro_lineup", map[string]any{"map": "de_mirage", "id": "nope"}, "no pro lineup")

	var combos proCombosOut
	callTool(t, cs, "list_pro_combos", map[string]any{"map": "de_mirage", "side": "CT"}, &combos)
	if combos.Total != 1 || len(combos.Combos[0].Lineups) != 2 || combos.Combos[0].Lineups[1].Title != "Humo a Window desde TSpawn" ||
		combos.Combos[0].ClipURL != "https://community.test/v1/pro-clips/1/a-m1-mirage-r3-t5000-76561198000000001-combo.mp4" {
		t.Fatalf("combos = %+v", combos)
	}
}

func TestSaveAndPractiseProLineups(t *testing.T) {
	cs, rec := proStudio(t)
	var saved lineupOut
	callTool(t, cs, "save_pro_lineup", map[string]any{"map": "de_mirage", "id": "molotov/bench-from-ct"}, &saved)
	if got := rec.get("POST /api/library/pro-lineups/{map}/lineups/{id}/save"); !strings.Contains(got, "/de_mirage/lineups/molotov-ct-5-6-7-8-1/save") {
		t.Errorf("save = %s", got)
	}
	args := map[string]any{"map": "de_mirage", "pro_lineup_ids": []string{"molotov-ct-5-6-7-8-1"}}
	var pending practiceOut
	callTool(t, cs, "launch_practice", args, &pending)
	args["approve"] = pending.ApprovalCode
	var out practiceOut
	callTool(t, cs, "launch_practice", args, &out)
	want := `"spots":[{"label":"Molotov a Bench desde CT","pos":[10,20,30],"ang":[-5,90],"grenade":"incendiary"}]`
	if got := rec.get("POST /api/practice/launch"); !strings.Contains(got, want) {
		t.Errorf("launch = %s, want %s", got, want)
	}
	toolFails(t, cs, "launch_practice", map[string]any{"map": "de_mirage", "pro_lineup_ids": []string{"x"}, "lineup_ids": []string{testLineupID}}, "only one of")
}
