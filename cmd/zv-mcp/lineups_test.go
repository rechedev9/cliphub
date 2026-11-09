package main

import (
	"net/http"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/rechedev9/cliphub/internal/tuiclient"
)

// TestSetposCommandMatchesTheWebByteForByte pins the web's toFixed(2) output, which
// player_setpos must match: no "-0.00", and an exact half rounds away from zero.
func TestSetposCommandMatchesTheWebByteForByte(t *testing.T) {
	got := setposCommand(tuiclient.Lineup{Origin: [3]float64{-0.001, 0.125, -167.96875}, Pitch: -0.625, Yaw: 1.005})
	if want := "setpos 0.00 0.13 -167.97;setang -0.63 1.00 0"; got != want {
		t.Fatalf("setposCommand = %q, want %q", got, want)
	}
}

const (
	testLineupID   = "9a8b7c6d-5e4f-4a3b-8c2d-1e0f9a8b7c6d"
	testLineupJSON = `{"id":"` + testLineupID + `","title":"Molo bancos","map":"de_mirage","side":"CT","grenade":"molotov","status":"active",
		"tags":["ret"],"origin":[-1200.5,200,-160],"pitch":-12.5,"yaw":135.25,"throw_callout":"CTSpawn","landing_callout":"Bench",
		"movement":"running","jump":true,"click":"left","technique_estimated":true,
		"source":{"match_id":"` + testMatchID + `","round":4,"tick":900},"updated_at":"2026-11-07T10:00:00Z"}`
)

func lineupStudio(t *testing.T) (*mcp.ClientSession, *recorder) {
	t.Helper()
	rec := &recorder{seen: map[string]string{}}
	mux := http.NewServeMux()
	rec.handle(mux, "GET /api/library/lineups", `{"lineups":[`+testLineupJSON+`,`+testLineupJSON+`]}`)
	rec.handle(mux, "GET /api/library/lineups/{id}", testLineupJSON)
	rec.handle(mux, "POST /api/library/lineups", testLineupJSON)
	rec.handle(mux, "PATCH /api/library/lineups/{id}", testLineupJSON)
	rec.handle(mux, "DELETE /api/library/lineups/{id}", ``)
	rec.handle(mux, "POST /api/practice/launch", `{"launched":true,"next_key":"F6","spots":[{"label":"Molo bancos","key":"F7"}]}`)
	cs := serveTools(t, mux, func(server *mcp.Server, s *studio) {
		registerLineupTools(server, s)
		registerCommunityTools(server, s)
	})
	return cs, rec
}

func TestLineupToolsListSaveAndEdit(t *testing.T) {
	cs, rec := lineupStudio(t)
	var list listLineupsOut
	callTool(t, cs, "list_lineups", map[string]any{"map": "DE_MIRAGE", "side": "ct", "grenade": "molotov", "limit": 1}, &list)
	if got := rec.get("GET /api/library/lineups"); !strings.Contains(got, "map=de_mirage") || !strings.Contains(got, "side=CT") || !strings.Contains(got, "limit=2") {
		t.Errorf("list query = %s", got)
	}
	l := list.Lineups[0]
	if len(list.Lineups) != 1 || !list.Truncated || l.Setpos != "setpos -1200.50 200.00 -160.00;setang -12.50 135.25 0" ||
		l.Ref != testMatchID+"/4" || !l.TechniqueEstimated {
		t.Fatalf("list = %+v", list)
	}
	toolFails(t, cs, "list_lineups", map[string]any{"grenade": "c4"}, "grenade must be one of")

	var saved lineupOut
	callTool(t, cs, "save_lineup", map[string]any{"title": "Molo", "match_id": testMatchID, "round": 4, "throw_tick": 900}, &saved)
	if got := rec.get("POST /api/library/lineups"); !strings.Contains(got, `"from":{"match_id":"`+testMatchID+`","round":4,"throw_tick":900}`) {
		t.Errorf("save from throw = %s", got)
	}
	callTool(t, cs, "save_lineup", map[string]any{"title": "Pop", "setpos": "setpos 1.5 -2 3;setang -10 90 0", "map": "de_nuke", "side": "t", "grenade": "flash"}, &saved)
	got := rec.get("POST /api/library/lineups")
	for _, want := range []string{`"map":"de_nuke"`, `"side":"T"`, `"origin":[1.5,-2,3]`, `"pitch":-10`, `"yaw":90`} {
		if !strings.Contains(got, want) || strings.Contains(got, `"from"`) {
			t.Errorf("save by hand = %s, want %s", got, want)
		}
	}
	toolFails(t, cs, "save_lineup", map[string]any{"title": "x"}, "pass match_id")
	toolFails(t, cs, "save_lineup", map[string]any{"title": "x", "setpos": "setpos 1 2", "map": "de_nuke", "side": "T", "grenade": "he"}, "setpos must read like")
	toolFails(t, cs, "save_lineup", map[string]any{"title": "x", "setpos": "setpos 1 2 3;setang 0 0 0", "map": "de_nuke", "side": "", "grenade": "he"}, "side must be")

	callTool(t, cs, "update_lineup", map[string]any{"lineup_id": testLineupID, "movement": "walking", "jump": false, "tags": []string{}}, &saved)
	if got := rec.get("PATCH /api/library/lineups/{id}"); !strings.Contains(got, `"movement":"walking"`) || !strings.Contains(got, `"jump":false`) || !strings.Contains(got, `"tags":[]`) {
		t.Errorf("patch = %s", got)
	}
	toolFails(t, cs, "update_lineup", map[string]any{"lineup_id": testLineupID, "click": "middle"}, "click must be one of")
	toolFails(t, cs, "update_lineup", map[string]any{"lineup_id": testLineupID}, "set title")

	var del deleteOut
	callTool(t, cs, "delete_lineup", map[string]any{"lineup_id": testLineupID}, &del)
	if !del.NeedsApproval || rec.get("DELETE /api/library/lineups/{id}") != "" {
		t.Fatalf("delete without approval = %+v", del)
	}
}

func TestLaunchPracticeFromLineups(t *testing.T) {
	cs, rec := lineupStudio(t)
	args := map[string]any{"map": "de_mirage", "lineup_ids": []string{testLineupID}}
	var pending practiceOut
	callTool(t, cs, "launch_practice", args, &pending)
	args["approve"] = pending.ApprovalCode
	var out practiceOut
	callTool(t, cs, "launch_practice", args, &out)
	want := `"spots":[{"label":"Molo bancos","pos":[-1200.5,200,-160],"ang":[-12.5,135.25],"grenade":"incendiary"}]`
	if got := rec.get("POST /api/practice/launch"); !strings.Contains(got, want) || !out.Launched {
		t.Errorf("launch = %s, want %s", got, want)
	}
	toolFails(t, cs, "launch_practice", map[string]any{"map": "de_nuke", "lineup_ids": []string{testLineupID}}, "is on de_mirage")
	toolFails(t, cs, "launch_practice", map[string]any{"map": "de_mirage", "lineup_ids": []string{"x"}}, "lineup_ids")
}
