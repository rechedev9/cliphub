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
	testCallID   = "5e6f7a8b-9c0d-4e1f-8a2b-3c4d5e6f7a8b"
	testCallJSON = `{"id":"` + testCallID + `","name":"Rotar","side":"CT","meaning":"a B por mid","status":"active","tags":["ret"],"updated_at":"2026-11-07T10:00:00Z"}`
)

func callStudio(t *testing.T) (*mcp.ClientSession, *recorder) {
	t.Helper()
	rec := &recorder{seen: map[string]string{}}
	mux := http.NewServeMux()
	rec.handle(mux, "GET /api/library/calls", `{"calls":[`+testCallJSON+`,`+testCallJSON+`]}`)
	rec.handle(mux, "GET /api/library/calls/{id}", testCallJSON)
	rec.handle(mux, "POST /api/library/calls", testCallJSON)
	rec.handle(mux, "PATCH /api/library/calls/{id}", testCallJSON)
	rec.handle(mux, "DELETE /api/library/calls/{id}", ``)
	rec.handle(mux, "GET /api/library/maps/{map}/callouts", `{"map":"de_mirage","callouts":[{"token":"BombsiteB","name":"Bé"},{"token":"CTSpawn"}]}`)
	rec.handle(mux, "PUT /api/library/maps/{map}/callouts/{token}", `{}`)
	rec.handle(mux, "DELETE /api/library/maps/{map}/callouts/{token}", ``)
	rec.handle(mux, "GET /api/library/libreta/export", `{"format":"cliphub-stratbook","version":2,"plays":[],"calls":[`+testCallJSON+`]}`)
	cs := serveTools(t, mux, func(server *mcp.Server, s *studio) { registerCallTools(server, s) })
	return cs, rec
}

func TestCallToolsListSaveEditDelete(t *testing.T) {
	cs, rec := callStudio(t)
	var list listCallsOut
	callTool(t, cs, "list_calls", map[string]any{"side": "ct", "limit": 1}, &list)
	if got := rec.get("GET /api/library/calls"); !strings.Contains(got, "side=CT") || !strings.Contains(got, "limit=2") {
		t.Errorf("list query = %s", got)
	}
	if len(list.Calls) != 1 || !list.Truncated || list.Calls[0].Meaning != "a B por mid" {
		t.Fatalf("list = %+v", list)
	}

	var saved callOut
	callTool(t, cs, "save_call", map[string]any{"name": " Bait ", "meaning": "uno entra\ny cambia"}, &saved)
	got := rec.get("POST /api/library/calls")
	for _, want := range []string{`"name":"Bait"`, `"side":"both"`, `"meaning":"uno entra\ny cambia"`} {
		if !strings.Contains(got, want) {
			t.Errorf("save body = %s, want %s", got, want)
		}
	}
	toolFails(t, cs, "save_call", map[string]any{"name": "x", "side": "mid"}, "side must be")
	toolFails(t, cs, "save_call", map[string]any{"name": ""}, "name must not be empty")

	callTool(t, cs, "update_call", map[string]any{"call_id": testCallID, "status": "archived", "tags": []string{}}, &saved)
	if got := rec.get("PATCH /api/library/calls/{id}"); !strings.Contains(got, `{"status":"archived","tags":[]}`) {
		t.Errorf("patch body = %s", got)
	}
	toolFails(t, cs, "update_call", map[string]any{"call_id": testCallID}, "set name")

	var del deleteOut
	callTool(t, cs, "delete_call", map[string]any{"call_id": testCallID}, &del)
	if !del.NeedsApproval || !strings.Contains(del.Summary, "Rotar") || rec.get("DELETE /api/library/calls/{id}") != "" {
		t.Fatalf("delete without approval = %+v", del)
	}
}

func TestCalloutToolsListAndRename(t *testing.T) {
	cs, rec := callStudio(t)
	var list listCalloutsOut
	callTool(t, cs, "list_callouts", map[string]any{"map": "DE_MIRAGE"}, &list)
	if len(list.Callouts) != 2 || list.Callouts[0].Name != "Bé" || list.Callouts[1].CS2Name == "" || list.Callouts[1].Name != "" {
		t.Fatalf("callouts = %+v", list)
	}
	var out calloutOut
	callTool(t, cs, "rename_callout", map[string]any{"map": "de_mirage", "token": "BombsiteB", "name": " Bé "}, &out)
	if got := rec.get("PUT /api/library/maps/{map}/callouts/{token}"); !strings.Contains(got, "/de_mirage/callouts/BombsiteB") || !strings.Contains(got, `{"name":"Bé"}`) {
		t.Errorf("rename = %s", got)
	}
	callTool(t, cs, "rename_callout", map[string]any{"map": "de_mirage", "token": "BombsiteB"}, &out)
	if got := rec.get("DELETE /api/library/maps/{map}/callouts/{token}"); !strings.Contains(got, "/de_mirage/callouts/BombsiteB") {
		t.Errorf("reset = %s", got)
	}
	toolFails(t, cs, "rename_callout", map[string]any{"map": "de_mirage", "token": "", "name": "x"}, "token")
}

func TestExportLibretaWritesANewFile(t *testing.T) {
	cs, _ := callStudio(t)
	path := filepath.Join(t.TempDir(), "libreta.json")
	var out exportPlaysOut
	callTool(t, cs, "export_libreta", map[string]any{"path": path}, &out)
	raw, err := os.ReadFile(path)
	if err != nil || !strings.Contains(string(raw), `"calls"`) || out.Bytes != len(raw) {
		t.Fatalf("export = %+v, %q, %v", out, raw, err)
	}
	toolFails(t, cs, "export_libreta", map[string]any{"path": path}, "")
}
