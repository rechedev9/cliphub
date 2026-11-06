package main

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	testPlayID   = "3c4d5e6f-7a8b-4c9d-8e1f-2a3b4c5d6e7f"
	testReportID = "4d5e6f7a-8b9c-4dae-9f20-3b4c5d6e7f80"
	testPlayJSON = `{"id":"` + testPlayID + `","title":"A exec","team_id":"` + testTeamID + `","map":"de_mirage","side":"T","kind":"execute","site":"A",
		"origin":"round","positions_seconds":15,"grenades":[{"kind":"smoke","seconds":12.34,"pos":[1,2,3],"callout":"CT","player":"apEX","share":0.75}],
		"positions":[{"player":"ZywOo","pos":[4,5,6],"callout":"Palace"}],"notes":"línea 1\nlínea 2","examples":[{"match_id":"` + testMatchID + `","round":4,"tick":100}],
		"updated_at":"2026-11-01T00:00:00Z"}`
)

func stratbookStudio(t *testing.T) (*mcp.ClientSession, *recorder) {
	t.Helper()
	rec := &recorder{seen: map[string]string{}}
	mux := http.NewServeMux()
	rec.handle(mux, "GET /api/library/plays", `{"plays":[`+testPlayJSON+`,`+testPlayJSON+`]}`)
	rec.handle(mux, "GET /api/library/plays/{id}", testPlayJSON)
	rec.handle(mux, "POST /api/library/plays", testPlayJSON)
	rec.handle(mux, "PATCH /api/library/plays/{id}", testPlayJSON)
	rec.handle(mux, "DELETE /api/library/plays/{id}", ``)
	rec.handle(mux, "GET /api/library/plays/export", `{"format":"cliphub-stratbook","version":1,"plays":[]}`)
	rec.handle(mux, "POST /api/library/plays/import", `{"imported":1,"duplicates":2,"rejected":[{"index":3,"title":"bad","error":"kind is wrong"}],"plays":[`+testPlayJSON+`]}`)
	rec.handle(mux, "GET /api/library/matches/{id}/tactical/rounds/{round}/play", `{"draft":{"team_id":"`+testTeamID+`","map":"de_mirage","side":"T","kind":"execute","site":"A",
		"origin":"round","positions_seconds":15,"grenades":[],"positions":[],"examples":[{"match_id":"`+testMatchID+`","round":4,"tick":100}]}}`)
	rec.handle(mux, "GET /api/library/teams/{id}/plays", `{"team_name":"Vitality","matches":3,"rounds":40,"compared":30,"thresholds":{"position_seconds":20},
		"plays":[{"map":"de_mirage","side":"T","kind":"execute","site":"B","rounds":5,"share":{"pct":12.5},"win_rate":{"pct":60,"reliable":true},
		"grenades":[{"kind":"flash","seconds":30,"pos":[0,0,0]}],"positions":[{"player":"x","pos":[0,0,0]}],
		"seed":{"match_id":"`+testMatchID+`","round":7},"examples":[{"match_id":"`+testMatchID+`","round":7},{"match_id":"`+testMatchID+`","round":9}]}]}`)
	rec.handle(mux, "GET /api/library/reports", `{"reports":[{"id":"`+testReportID+`","title":"Vitality Mirage","team_id":"`+testTeamID+`","match_ids":["`+testMatchID+`"],"side":"T","notes":"","updated_at":"2026-11-01T00:00:00Z"}]}`)
	rec.handle(mux, "GET /api/library/reports/{id}", `{"id":"`+testReportID+`","title":"Vitality Mirage","team_id":"`+testTeamID+`","team_name":"Vitality","match_ids":[],"side":"","notes":"a\u0007b\nc","updated_at":"2026-11-01T00:00:00Z"}`)
	rec.handle(mux, "GET /api/library/reports/{id}/scout", `{"team_name":"Vitality","tendencies":{"round_count":20,"wins":10}}`)
	rec.handle(mux, "GET /api/library/reports/{id}/metrics", `{"players":[{"steamid64":"`+testSteamID+`","name":"ZywOo","rounds":20,"kills":20,"deaths":10,"box":{"damage":1700}}]}`)
	rec.handle(mux, "POST /api/library/reports", `{"id":"`+testReportID+`","title":"Vitality Mirage","team_id":"`+testTeamID+`"}`)
	rec.handle(mux, "PATCH /api/library/reports/{id}", `{"id":"`+testReportID+`","title":"Nuevo"}`)
	rec.handle(mux, "DELETE /api/library/reports/{id}", ``)
	cs := serveTools(t, mux, func(server *mcp.Server, s *studio) {
		registerStratbookTools(server, s)
		registerReportTools(server, s)
	})
	return cs, rec
}

func TestPlaysReadCompactly(t *testing.T) {
	cs, rec := stratbookStudio(t)
	var list listPlaysOut
	callTool(t, cs, "list_plays", map[string]any{"map": "DE_MIRAGE", "side": "t", "kind": "execute"}, &list)
	if got := rec.get("GET /api/library/plays"); !strings.Contains(got, "map=de_mirage") || !strings.Contains(got, "side=T") || !strings.Contains(got, "limit=51") {
		t.Errorf("list query = %s", got)
	}
	if len(list.Plays) != 2 || list.Truncated || list.Plays[0].Grenades != 1 || list.Plays[0].TeamID != testTeamID {
		t.Fatalf("plays = %+v", list)
	}
	callTool(t, cs, "list_plays", map[string]any{"limit": 1, "offset": 3}, &list)
	if got := rec.get("GET /api/library/plays"); !strings.Contains(got, "limit=2") || !strings.Contains(got, "offset=3") {
		t.Errorf("page query = %s", got)
	}
	if len(list.Plays) != 1 || !list.Truncated {
		t.Errorf("page = %+v, want one play and truncated", list)
	}
	toolFails(t, cs, "list_plays", map[string]any{"limit": 201}, "limit must be from 1 to 200")
	toolFails(t, cs, "list_plays", map[string]any{"kind": "rush"}, "kind must be one of")

	var p playDetailOut
	callTool(t, cs, "get_play", map[string]any{"play_id": testPlayID}, &p)
	if p.Notes != "línea 1\nlínea 2" || len(p.GrenadeList) != 1 || p.GrenadeList[0].Seconds != 12.3 || p.GrenadeList[0].SharePct != 75 {
		t.Errorf("play = %+v", p)
	}
	if len(p.Refs) != 1 || p.Refs[0] != testMatchID+"/4" || len(p.Positions) != 1 || p.Positions[0].Callout != "Palace" {
		t.Errorf("refs %v, positions %+v", p.Refs, p.Positions)
	}
}

func TestPlaysAreSavedFromRoundsAndDetections(t *testing.T) {
	cs, rec := stratbookStudio(t)
	var p playDetailOut
	callTool(t, cs, "save_round_as_play", map[string]any{"match_id": testMatchID, "round": 4, "team_key": "t-start", "title": " A exec ", "notes": "go"}, &p)
	if got := rec.get("GET /api/library/matches/{id}/tactical/rounds/{round}/play"); !strings.Contains(got, "/rounds/4/play?team_key=t-start") {
		t.Errorf("draft request = %s", got)
	}
	posted := rec.get("POST /api/library/plays")
	if !strings.Contains(posted, `"title":"A exec"`) || !strings.Contains(posted, `"notes":"go"`) || !strings.Contains(posted, `"origin":"round"`) || strings.Contains(posted, `"id"`) {
		t.Errorf("posted = %s", posted)
	}
	toolFails(t, cs, "save_round_as_play", map[string]any{"match_id": testMatchID, "round": 4, "team_key": "T", "title": "x"}, "team_key")

	var det detectPlaysOut
	callTool(t, cs, "detect_team_plays", map[string]any{"team_id": testTeamID, "map": "de_mirage"}, &det)
	if len(det.Plays) != 1 || det.Truncated || det.Plays[0].Ref != testMatchID+"/7" || det.Plays[0].Site != "B" {
		t.Fatalf("detection = %+v", det)
	}
	toolFails(t, cs, "detect_team_plays", map[string]any{"team_id": testTeamID, "limit": 101}, "limit must be from 1 to 100")
	callTool(t, cs, "save_detected_play", map[string]any{"team_id": testTeamID, "map": "de_mirage", "play_ref": det.Plays[0].Ref, "title": "B split"}, &p)
	posted = rec.get("POST /api/library/plays")
	for _, want := range []string{`"origin":"detected"`, `"positions_seconds":20`, `"site":"B"`, `"team_id":"` + testTeamID + `"`, `"round":9`} {
		if !strings.Contains(posted, want) {
			t.Errorf("posted detected play %s, want %s", posted, want)
		}
	}
	toolFails(t, cs, "save_detected_play", map[string]any{"team_id": testTeamID, "play_ref": testMatchID + "/8", "title": "x"}, "no detected play has that ref")
}

func TestUpdateDeleteExportImportPlays(t *testing.T) {
	cs, rec := stratbookStudio(t)
	var p playDetailOut
	callTool(t, cs, "update_play", map[string]any{"play_id": testPlayID, "detach_team": true, "site": "mid"}, &p)
	if got := rec.get("PATCH /api/library/plays/{id}"); !strings.HasSuffix(got, `{"site":"mid","team_id":null}`) {
		t.Errorf("patch = %s", got)
	}
	toolFails(t, cs, "update_play", map[string]any{"play_id": testPlayID, "site": "C"}, "site must be")

	var del deleteOut
	callTool(t, cs, "delete_play", map[string]any{"play_id": testPlayID}, &del)
	if !del.NeedsApproval || rec.get("DELETE /api/library/plays/{id}") != "" {
		t.Fatalf("delete without approval = %+v", del)
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "stratbook.json")
	var exp exportPlaysOut
	callTool(t, cs, "export_plays", map[string]any{"path": path, "team_id": testTeamID}, &exp)
	data, err := os.ReadFile(path)
	if err != nil || !strings.Contains(string(data), "cliphub-stratbook") {
		t.Fatalf("exported %q, %v", data, err)
	}
	toolFails(t, cs, "export_plays", map[string]any{"path": path}, "already exists")
	toolFails(t, cs, "export_plays", map[string]any{"path": filepath.Join(dir, "x.txt")}, "must end in .json")

	var imp importPlaysOut
	callTool(t, cs, "import_plays", map[string]any{"path": path}, &imp)
	if got := rec.get("POST /api/library/plays/import"); !strings.Contains(got, `"format":"cliphub-stratbook"`) {
		t.Errorf("import body = %s", got)
	}
	if imp.Imported != 1 || imp.Duplicates != 2 || imp.RejectedCount != 1 || len(imp.Rejected) != 1 || !strings.Contains(imp.Rejected[0], "kind is wrong") {
		t.Errorf("import = %+v", imp)
	}
	bad := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(bad, []byte("{nope"), 0o644); err != nil {
		t.Fatal(err)
	}
	toolFails(t, cs, "import_plays", map[string]any{"path": bad}, "not valid JSON")
}

func TestReportTools(t *testing.T) {
	cs, rec := stratbookStudio(t)
	var list listReportsOut
	callTool(t, cs, "list_reports", map[string]any{"team_id": testTeamID}, &list)
	if len(list.Reports) != 1 || list.Reports[0].Side != "T" {
		t.Fatalf("reports = %+v", list)
	}
	var r reportDetailOut
	callTool(t, cs, "get_report", map[string]any{"report_id": testReportID}, &r)
	if r.Notes != "ab\nc" || r.Scout.Rounds != 20 || len(r.Players) != 1 || r.Players[0].ADR != 85 {
		t.Fatalf("report = %+v", r)
	}

	var created reportOut
	callTool(t, cs, "create_report", map[string]any{"title": "Vitality Mirage", "team_id": testTeamID, "match_ids": []string{testMatchID}, "side": "ct"}, &created)
	want := `{"title":"Vitality Mirage","team_id":"` + testTeamID + `","match_ids":["` + testMatchID + `"],"side":"CT","notes":""}`
	if got := rec.get("POST /api/library/reports"); !strings.HasSuffix(got, want) {
		t.Errorf("create = %s, want %s", got, want)
	}
	toolFails(t, cs, "create_report", map[string]any{"title": "x", "team_id": testTeamID, "side": "both"}, "side must be CT or T")

	callTool(t, cs, "update_report", map[string]any{"report_id": testReportID, "side": "", "match_ids": []string{}}, &created)
	if got := rec.get("PATCH /api/library/reports/{id}"); !strings.HasSuffix(got, `{"match_ids":[],"side":""}`) {
		t.Errorf("patch = %s, want an empty side (both) and match list", got)
	}
	toolFails(t, cs, "update_report", map[string]any{"report_id": testReportID}, "set title, match_ids, side or notes")

	var del deleteOut
	callTool(t, cs, "delete_report", map[string]any{"report_id": testReportID}, &del)
	if !del.NeedsApproval || !strings.Contains(del.Summary, "«Vitality Mirage»") || rec.get("DELETE /api/library/reports/{id}") != "" {
		t.Fatalf("delete = %+v", del)
	}
	callTool(t, cs, "delete_report", map[string]any{"report_id": testReportID, "approve": del.ApprovalCode}, &del)
	if !del.Deleted || rec.get("DELETE /api/library/reports/{id}") == "" {
		t.Errorf("approved delete = %+v", del)
	}
}
