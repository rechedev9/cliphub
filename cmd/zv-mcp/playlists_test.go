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
	testPlaylistID   = "7a8b9c0d-1e2f-4a3b-8c4d-5e6f7a8b9c0d"
	testPlaylistJSON = `{"id":"` + testPlaylistID + `","name":"FUT T","folder":"Rivales","favorite":true,"author":"mopoz","maps":["de_mirage"],` +
		`"rounds":[{"match_id":"` + testMatchID + `","round":3,"map":"de_mirage","comment":"humo tarde",` +
		`"notes":[{"id":"n1","tick":4200,"text":"aquí rota","strokes":[{"color":"accent","width":3,"arrow":true,"points":[[0,0],[5,5]]}]}]},` +
		`{"match_id":"` + testMatchID + `","round":4,"map":"de_inferno","comment":"","missing":true}],"updated_at":"2026-11-09T10:00:00Z"}`
)

func playlistStudio(t *testing.T) (*mcp.ClientSession, *recorder) {
	t.Helper()
	rec := &recorder{seen: map[string]string{}}
	mux := http.NewServeMux()
	rec.handle(mux, "GET /api/library/playlists", `{"playlists":[`+testPlaylistJSON+`,`+testPlaylistJSON+`]}`)
	rec.handle(mux, "GET /api/library/playlists/{id}", testPlaylistJSON)
	rec.handle(mux, "POST /api/library/playlists", testPlaylistJSON)
	rec.handle(mux, "PATCH /api/library/playlists/{id}", testPlaylistJSON)
	rec.handle(mux, "POST /api/library/playlists/{id}/rounds", testPlaylistJSON)
	rec.handle(mux, "PATCH /api/library/playlists/{id}/rounds/{match}/{round}", testPlaylistJSON)
	rec.handle(mux, "DELETE /api/library/playlists/{id}", ``)
	rec.handle(mux, "GET /api/library/playlists/export", `{"format":"cliphub-playlists","version":1,"playlists":[`+testPlaylistJSON+`],"matches":[]}`)
	rec.handle(mux, "POST /api/library/playlists/import", `{"imported":1,"duplicates":0,"rejected":[],"playlists":[`+testPlaylistJSON+`],`+
		`"missing_matches":[{"match_id":"`+testMatchID+`","map":"de_inferno","teams":["FUT","OG"],"played_at":"2026-11-01T18:00:00Z","file_name":"fut-og.dem"}]}`)
	cs := serveTools(t, mux, func(server *mcp.Server, s *studio) { registerPlaylistTools(server, s) })
	return cs, rec
}

func TestPlaylistToolsListCreateEditAndDelete(t *testing.T) {
	cs, rec := playlistStudio(t)
	var list listPlaylistsOut
	callTool(t, cs, "list_playlists", map[string]any{"map": "DE_MIRAGE", "no_folder": true, "limit": 1}, &list)
	if got := rec.get("GET /api/library/playlists"); !strings.Contains(got, "map=de_mirage") || !strings.Contains(got, "no_folder=true") || !strings.Contains(got, "limit=2") {
		t.Errorf("list query = %s", got)
	}
	if len(list.Playlists) != 1 || !list.Truncated || list.Playlists[0].Rounds != 2 || list.Playlists[0].Missing != 1 || list.Playlists[0].StudioPath != "/replay?p="+testPlaylistID {
		t.Fatalf("list = %+v", list)
	}

	var p playlistDetailOut
	callTool(t, cs, "create_playlist", map[string]any{"name": " FUT T ", "folder": " Rivales ", "rounds": []map[string]any{{"ref": testMatchID + "/3", "comment": "humo tarde"}}}, &p)
	got := rec.get("POST /api/library/playlists")
	for _, want := range []string{`"name":"FUT T"`, `"folder":"Rivales"`, `"rounds":[{"match_id":"` + testMatchID + `","round":3,"comment":"humo tarde"}]`} {
		if !strings.Contains(got, want) {
			t.Errorf("create body = %s, want %s", got, want)
		}
	}
	if len(p.RoundList) != 2 || p.RoundList[0].Ref != testMatchID+"/3" || p.RoundList[0].Comment != "humo tarde" || !p.RoundList[1].Missing {
		t.Fatalf("created = %+v", p)
	}
	if n := p.RoundList[0].Notes; len(n) != 1 || n[0].Tick != 4200 || n[0].Text != "aquí rota" || n[0].Strokes != 1 || p.RoundList[1].Notes != nil {
		t.Fatalf("round notes = %+v; want the drawing named by its tick and stroke count", p.RoundList)
	}
	toolFails(t, cs, "create_playlist", map[string]any{"name": ""}, "name must not be empty")
	toolFails(t, cs, "create_playlist", map[string]any{"name": "x", "rounds": []map[string]any{{"ref": "nope"}}}, "not a round ref")

	callTool(t, cs, "update_playlist", map[string]any{"playlist_id": testPlaylistID, "folder": "", "favorite": false}, &p)
	if got := rec.get("PATCH /api/library/playlists/{id}"); !strings.Contains(got, `{"folder":"","favorite":false}`) {
		t.Errorf("patch body = %s", got)
	}
	toolFails(t, cs, "update_playlist", map[string]any{"playlist_id": testPlaylistID}, "set name")
	callTool(t, cs, "update_playlist", map[string]any{"playlist_id": testPlaylistID, "rounds": []map[string]any{{"ref": testMatchID + "/4"}, {"ref": testMatchID + "/3"}}}, &p)
	if got := rec.get("PATCH /api/library/playlists/{id}"); strings.Contains(got, "comment") {
		t.Errorf("reorder body = %s; a round without a comment must leave it out so the server keeps it", got)
	}

	callTool(t, cs, "update_playlist_round", map[string]any{"playlist_id": testPlaylistID, "ref": testMatchID + "/3", "comment": "", "position": 1,
		"notes": []map[string]any{{"id": "n1", "text": "rota tarde"}, {"tick": 5000, "strokes": []map[string]any{{"color": "t", "width": 2, "points": [][2]float64{{1, 2}, {3, 4}}}}}}}, &p)
	got = rec.get("PATCH /api/library/playlists/{id}/rounds/{match}/{round}")
	for _, want := range []string{`"comment":""`, `"position":1`, `{"id":"n1","tick":4200,"text":"rota tarde","strokes":[{"color":"accent"`,
		`{"tick":5000,"text":"","strokes":[{"color":"t","width":2,"points":[[1,2],[3,4]]}]}`} {
		if !strings.Contains(got, want) {
			t.Errorf("round patch body = %s, want %s", got, want)
		}
	}
	toolFails(t, cs, "update_playlist_round", map[string]any{"playlist_id": testPlaylistID, "ref": testMatchID + "/3"}, "set comment")
	toolFails(t, cs, "update_playlist_round", map[string]any{"playlist_id": testPlaylistID, "ref": testMatchID + "/3", "notes": []map[string]any{{"id": "zz"}}}, "not on this round")
	toolFails(t, cs, "update_playlist_round", map[string]any{"playlist_id": testPlaylistID, "ref": testMatchID + "/9", "notes": []map[string]any{}}, "not in this playlist")

	callTool(t, cs, "add_playlist_rounds", map[string]any{"playlist_id": testPlaylistID, "position": 0, "rounds": []map[string]any{{"ref": testMatchID + "/5"}}}, &p)
	if got := rec.get("POST /api/library/playlists/{id}/rounds"); !strings.Contains(got, `"round":5`) || !strings.Contains(got, `"position":0`) {
		t.Errorf("add body = %s", got)
	}

	var del deleteOut
	callTool(t, cs, "delete_playlist", map[string]any{"playlist_id": testPlaylistID}, &del)
	if !del.NeedsApproval || !strings.Contains(del.Summary, "FUT T") || rec.get("DELETE /api/library/playlists/{id}") != "" {
		t.Fatalf("delete without approval = %+v", del)
	}
}

func TestPlaylistFileToolsWriteAndReadBack(t *testing.T) {
	cs, rec := playlistStudio(t)
	path := filepath.Join(t.TempDir(), "playlists.json")
	var out exportPlaysOut
	callTool(t, cs, "export_playlists", map[string]any{"path": path, "playlist_ids": []string{testPlaylistID}}, &out)
	raw, err := os.ReadFile(path)
	if err != nil || !strings.Contains(string(raw), `"cliphub-playlists"`) || out.Bytes != len(raw) {
		t.Fatalf("export = %+v, %q, %v", out, raw, err)
	}
	if got := rec.get("GET /api/library/playlists/export"); !strings.Contains(got, "id="+testPlaylistID) {
		t.Errorf("export query = %s", got)
	}
	toolFails(t, cs, "export_playlists", map[string]any{"path": path}, "")

	var imp importPlaylistsOut
	callTool(t, cs, "import_playlists", map[string]any{"path": path}, &imp)
	if imp.Imported != 1 || len(imp.MissingMatches) != 1 || imp.MissingMatches[0] != "de_inferno, FUT vs OG, 2026-11-01, demo «fut-og.dem»" {
		t.Fatalf("import = %+v", imp)
	}
	toolFails(t, cs, "import_playlists", map[string]any{"path": filepath.Join(t.TempDir(), "x.txt")}, ".json")
}
