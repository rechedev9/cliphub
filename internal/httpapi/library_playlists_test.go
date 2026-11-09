package httpapi

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/library"
)

func playlistRoundKeys(p library.Playlist) []string {
	out := []string{}
	for _, r := range p.Rounds {
		out = append(out, r.MatchID.String()[:4]+"/"+string(rune('0'+r.Round))+":"+r.Comment)
	}
	return out
}

func TestLibraryPlaylistsKeepRoundsInOrderWithComments(t *testing.T) {
	env := newLibraryTestEnv(t)
	a := env.seedLibraryMatch(strings.Repeat("a", 64), "Vitality", "MOUZ")
	b := env.seedLibraryMatch(strings.Repeat("b", 64), "FaZe", "G2")
	ka, kb := a.ID.String()[:4], b.ID.String()[:4]
	var p library.Playlist
	env.json(http.MethodPost, "/api/library/playlists", map[string]any{"name": " FUT T ", "folder": "Rivales", "author": "mopoz",
		"rounds": []map[string]any{{"match_id": a.ID, "round": 1, "comment": "humo"}, {"match_id": b.ID, "round": 2}}}, http.StatusCreated, &p)
	if p.Name != "FUT T" || len(p.Rounds) != 2 || p.Rounds[0].Map != "de_mirage" || p.Rounds[0].DemoSHA256 != strings.Repeat("a", 64) || p.Rounds[0].Comment != "humo" {
		t.Fatalf("created playlist = %+v", p)
	}
	for body, code := range map[string]string{
		`{"name":"x","rounds":[{"match_id":"` + uuid.NewString() + `","round":1}]}`: codeUnknownMatch,
		`{"name":"x","rounds":[{"match_id":"` + a.ID.String() + `","round":3}]}`:    "invalid_request",
		`{"name":""}`:               "invalid_request",
		`{"name":"x","maps":["a"]}`: "invalid_request",
	} {
		rw := env.do(http.MethodPost, "/api/library/playlists", []byte(body), "application/json")
		if rw.Code != http.StatusBadRequest || !strings.Contains(rw.Body.String(), `"code":"`+code+`"`) {
			t.Fatalf("POST %s = %d %s, want 400 %s", body, rw.Code, rw.Body.String(), code)
		}
	}
	var listed struct {
		Playlists []library.Playlist `json:"playlists"`
	}
	for query, want := range map[string]int{"": 1, "?map=de_mirage": 1, "?map=de_nuke": 0, "?folder=rivales": 1, "?no_folder=true": 0, "?q=MOPOZ": 1, "?favorite=true": 0} {
		env.json(http.MethodGet, "/api/library/playlists"+query, nil, http.StatusOK, &listed)
		if len(listed.Playlists) != want {
			t.Fatalf("GET playlists%s = %d, want %d", query, len(listed.Playlists), want)
		}
	}
	for _, query := range []string{"?favorite=maybe", "?map=Mirage!", "?limit=0"} {
		if rw := env.do(http.MethodGet, "/api/library/playlists"+query, nil, ""); rw.Code != http.StatusBadRequest {
			t.Fatalf("GET playlists%s = %d, want 400", query, rw.Code)
		}
	}
	path := "/api/library/playlists/" + p.ID.String()
	env.json(http.MethodPost, path+"/rounds", map[string]any{"position": 0, "rounds": []map[string]any{{"match_id": b.ID, "round": 2, "comment": "otra"}, {"match_id": a.ID, "round": 2}}}, http.StatusOK, &p)
	if got := strings.Join(playlistRoundKeys(p), " "); got != ka+"/2: "+ka+"/1:humo "+kb+"/2:" {
		t.Fatalf("after add = %s; want the new round first and the held one kept", got)
	}
	env.json(http.MethodPatch, path+"/rounds/"+a.ID.String()+"/1", map[string]any{"position": 2, "comment": "tarde"}, http.StatusOK, &p)
	env.json(http.MethodDelete, path+"/rounds/"+b.ID.String()+"/2", nil, http.StatusOK, &p)
	if got := strings.Join(playlistRoundKeys(p), " "); got != ka+"/2: "+ka+"/1:tarde" {
		t.Fatalf("after move and remove = %s", got)
	}
	env.json(http.MethodDelete, path+"/rounds/"+b.ID.String()+"/2", nil, http.StatusNotFound, nil)
	updated := p.UpdatedAt
	env.json(http.MethodPatch, path, map[string]any{"favorite": true}, http.StatusOK, &p)
	if !p.Favorite || len(p.Rounds) != 2 || !p.UpdatedAt.Equal(updated) {
		t.Fatalf("starred = %+v; want the rounds and updated_at kept", p)
	}
	env.json(http.MethodPatch, path, map[string]any{"rounds": []map[string]any{{"match_id": a.ID, "round": 1, "comment": "solo"}}}, http.StatusOK, &p)
	if got := strings.Join(playlistRoundKeys(p), " "); got != ka+"/1:solo" {
		t.Fatalf("after replace = %s", got)
	}
	env.json(http.MethodDelete, path, nil, http.StatusNoContent, nil)
	env.json(http.MethodGet, path, nil, http.StatusNotFound, nil)
}

func TestLibraryPlaylistFileFindsTheMatchByItsDemoOnAnotherPC(t *testing.T) {
	sha := strings.Repeat("c", 64)
	src := newLibraryTestEnv(t)
	m := src.seedLibraryMatch(sha, "Vitality", "MOUZ")
	var p library.Playlist
	src.json(http.MethodPost, "/api/library/playlists", map[string]any{"name": "Antis", "rounds": []map[string]any{{"match_id": m.ID, "round": 2, "comment": "fake B"}}}, http.StatusCreated, &p)
	var file playlistFile
	src.json(http.MethodGet, "/api/library/playlists/export?id="+p.ID.String(), nil, http.StatusOK, &file)
	if file.Format != playlistFormat || len(file.Playlists) != 1 || len(file.Matches) != 1 || file.Matches[0].DemoSHA256 != sha ||
		strings.Join(file.Matches[0].Teams, ",") != "Vitality,MOUZ" || file.Matches[0].FileName != sha+".dem" {
		t.Fatalf("playlist file = %+v", file)
	}
	raw, _ := json.Marshal(file)

	dst := newLibraryTestEnv(t)
	var resp playlistImportResponse
	dst.json(http.MethodPost, "/api/library/playlists/import", json.RawMessage(raw), http.StatusOK, &resp)
	if resp.Imported != 1 || !resp.Playlists[0].Rounds[0].Missing || len(resp.MissingMatches) != 1 || resp.MissingMatches[0].FileName != sha+".dem" {
		t.Fatalf("import = %+v; want the round kept as missing and its demo named", resp)
	}
	local := dst.seedLibraryMatch(sha, "Vitality", "MOUZ")
	var got library.Playlist
	dst.json(http.MethodGet, "/api/library/playlists/"+p.ID.String(), nil, http.StatusOK, &got)
	if r := got.Rounds[0]; r.Missing || r.MatchID != local.ID || r.Comment != "fake B" {
		t.Fatalf("after the demo came = %+v; want the round on the local match", got.Rounds)
	}
	dst.json(http.MethodPost, "/api/library/playlists/import", json.RawMessage(raw), http.StatusOK, &resp)
	if resp.Imported != 0 || resp.Duplicates != 1 || len(resp.MissingMatches) != 0 {
		t.Fatalf("reimport = %+v; want one duplicate", resp)
	}
	rw := dst.do(http.MethodPost, "/api/library/playlists/import", []byte(`{"format":"cliphub-stratbook","version":1}`), "application/json")
	if rw.Code != http.StatusBadRequest || !strings.Contains(rw.Body.String(), codePlaylistFormat) {
		t.Fatalf("import of another file = %d %s", rw.Code, rw.Body.String())
	}
}
