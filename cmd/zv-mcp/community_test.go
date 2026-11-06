package main

import (
	"net/http"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func communityStudio(t *testing.T) (*mcp.ClientSession, *recorder) {
	t.Helper()
	rec := &recorder{seen: map[string]string{}}
	mux := http.NewServeMux()
	rec.handle(mux, "GET /api/faceit/players/{id}/matches", `{"matches":[{"id":"1-abc","finished_at":"2026-11-02T20:00:00Z","demo_available":true,
		"score":{"for":13,"against":9},"stats":{"map":"de_ancient","result":"win","kills":21,"deaths":14,"assists":4,"adr":88.2,"headshots_percent":51}}]}`)
	rec.handle(mux, "POST /api/practice/launch", `{"launched":true,"next_key":"F6","spots":[{"label":"Ventana","key":"F7"}]}`)
	rec.handle(mux, "GET /api/library/community/settings", `{"rankings_enabled":true,"server_enabled":true,"steam_id_set":true,"last_submitted_at":"2026-11-05T10:00:00Z"}`)
	rec.handle(mux, "GET /api/library/community/rankings", `{"map":"de_mirage","rounds":240,"min_installs":5,"metrics":[{"metric":"adr","value":82.1,"installs":40,"percentile":71,"median":76}]}`)
	rec.handle(mux, "PUT /api/library/community/settings", `{"rankings_enabled":false}`)
	rec.handle(mux, "DELETE /api/library/community/rankings", `{"rankings_enabled":false}`)
	cs := serveTools(t, mux, func(server *mcp.Server, s *studio) { registerCommunityTools(server, s) })
	return cs, rec
}

func TestFaceitMatchesScoreAndLimit(t *testing.T) {
	cs, rec := communityStudio(t)
	var out faceitMatchesOut
	callTool(t, cs, "faceit_matches", map[string]any{"player_id": "abc-123"}, &out)
	m := out.Matches[0]
	if m.Score != "13-9" || m.Map != "de_ancient" || m.Finished != "2026-11-02" || m.Kills != 21 || !m.DemoAvailable {
		t.Errorf("match = %+v", m)
	}
	if got := rec.get("GET /api/faceit/players/{id}/matches"); !strings.Contains(got, "/abc-123/matches?limit=10") {
		t.Errorf("request = %s", got)
	}
	toolFails(t, cs, "faceit_matches", map[string]any{"player_id": "../x"}, "player_id")
	toolFails(t, cs, "faceit_matches", map[string]any{"player_id": "abc", "limit": 50}, "limit")
}

func TestLaunchPracticeAsksThenSendsPitchYaw(t *testing.T) {
	cs, rec := communityStudio(t)
	args := map[string]any{"map": "de_mirage", "spots": []map[string]any{
		{"label": "Ventana", "origin": []float64{-1200.5, 200, -160}, "view": []float64{135.2, -12.5}, "kind": "molotov", "side": "CT"}}}
	var pending practiceOut
	callTool(t, cs, "launch_practice", args, &pending)
	if !pending.NeedsApproval || pending.ApprovalCode == "" || !strings.Contains(pending.Summary, "de_mirage con 1 posición") || rec.get("POST /api/practice/launch") != "" {
		t.Fatalf("pending = %+v", pending)
	}
	args["approve"] = pending.ApprovalCode
	var out practiceOut
	callTool(t, cs, "launch_practice", args, &out)
	want := `{"map":"de_mirage","spots":[{"label":"Ventana","pos":[-1200.5,200,-160],"ang":[-12.5,135.2],"grenade":"incendiary"}]}`
	if got := rec.get("POST /api/practice/launch"); !strings.HasSuffix(got, want) {
		t.Errorf("launch = %s\nwant %s", got, want)
	}
	if !out.Launched || out.NextKey != "F6" || len(out.Keys) != 1 || out.Keys[0] != "Ventana: F7" {
		t.Errorf("out = %+v", out)
	}
	toolFails(t, cs, "launch_practice", map[string]any{"map": "mirage", "spots": args["spots"]}, "map")
	toolFails(t, cs, "launch_practice", map[string]any{"map": "de_mirage", "spots": []map[string]any{{"label": "x", "origin": []float64{0, 0, 0}, "view": []float64{0, 120}}}}, "yaw and pitch")
	toolFails(t, cs, "launch_practice", map[string]any{"map": "de_mirage", "spots": []map[string]any{{"label": "x", "origin": []float64{0, 0, 0}, "view": []float64{0, 0}, "kind": "c4"}}}, "kind must be")
}

func TestCommunityRankingSharingNeedsApproval(t *testing.T) {
	cs, rec := communityStudio(t)
	var rk communityOut
	callTool(t, cs, "community_ranking", map[string]any{"map": "de_mirage"}, &rk)
	if !rk.RankingsEnabled || rk.LastSubmitted != "2026-11-05" || len(rk.Metrics) != 1 || *rk.Metrics[0].Percentile != 71 {
		t.Errorf("ranking = %+v", rk)
	}
	if got := rec.get("GET /api/library/community/rankings"); !strings.Contains(got, "?map=de_mirage") {
		t.Errorf("rankings request = %s", got)
	}

	var on rankingSettingsOut
	callTool(t, cs, "set_community_ranking", map[string]any{"enabled": true}, &on)
	if !on.NeedsApproval || rec.get("PUT /api/library/community/settings") != "" {
		t.Fatalf("enable = %+v", on)
	}
	var off rankingSettingsOut
	callTool(t, cs, "set_community_ranking", map[string]any{"enabled": false}, &off)
	if off.NeedsApproval || !strings.HasSuffix(rec.get("PUT /api/library/community/settings"), `{"rankings_enabled":false}`) {
		t.Errorf("disable = %+v, %s", off, rec.get("PUT /api/library/community/settings"))
	}

	var del rankingSettingsOut
	callTool(t, cs, "delete_community_ranking", map[string]any{}, &del)
	if !del.NeedsApproval || rec.get("DELETE /api/library/community/rankings") != "" {
		t.Fatalf("delete = %+v", del)
	}
}
