package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rechedev9/cliphub/internal/faceit"
	"github.com/rechedev9/cliphub/internal/library"
	"github.com/rechedev9/cliphub/internal/store"
)

const faceitRoom = "1-63c8199e-efe6-44b2-b2ba-be93c6570a90"

func uploadMatch(t *testing.T, env libraryTestEnv, sha string, fields map[string]string) uploadResult {
	t.Helper()
	document, positions := uploadFixture(t, sha)
	body, contentType := uploadBody(t, fields, map[string][]byte{"document": document, "positions": positions})
	rw := env.do(http.MethodPost, "/api/library/uploads", body.Bytes(), contentType)
	if rw.Code != http.StatusCreated && rw.Code != http.StatusOK {
		t.Fatalf("upload = %d, body=%s", rw.Code, rw.Body.String())
	}
	var res uploadResult
	if err := json.Unmarshal(rw.Body.Bytes(), &res); err != nil {
		t.Fatal(err)
	}
	return res
}

func TestUploadLibraryAnalysisDatesTheMatchByTheDemo(t *testing.T) {
	played := time.Now().UTC().Add(-26 * time.Hour).Truncate(time.Second)
	cases := []struct {
		name     string
		playedAt string
		want     func(res uploadResult) time.Time
	}{
		{"the date the demo shows", played.Format(time.RFC3339), func(uploadResult) time.Time { return played }},
		{"no date keeps the import time", "", func(res uploadResult) time.Time { return res.Demo.ImportedAt }},
		{"a date after the import is not the match's", time.Now().Add(time.Hour).UTC().Format(time.RFC3339),
			func(res uploadResult) time.Time { return res.Demo.ImportedAt }},
		{"an unreadable date is no date", "ayer", func(res uploadResult) time.Time { return res.Demo.ImportedAt }},
	}
	for i, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			env := newUploadEnv(t)
			res := uploadMatch(t, env, strings.Repeat(fmt.Sprintf("%02x", i), 32),
				map[string]string{"file_name": "match.dem", "played_at": tc.playedAt})
			if want := tc.want(res); !res.Match.PlayedAt.Equal(want) {
				t.Fatalf("played_at = %v, want %v", res.Match.PlayedAt, want)
			}
		})
	}
}

func TestUploadLibraryAnalysisDatesAMatchAnEarlierUploadLeftOnItsImportDay(t *testing.T) {
	env := newUploadEnv(t)
	sha := strings.Repeat("cd", 32)
	first := uploadMatch(t, env, sha, map[string]string{"file_name": "match.dem"})
	if !first.Match.PlayedAt.Equal(first.Demo.ImportedAt) {
		t.Fatalf("first upload played_at = %v, want the import time", first.Match.PlayedAt)
	}
	played := first.Demo.ImportedAt.Add(-20 * time.Hour).Truncate(time.Second)
	again := uploadMatch(t, env, sha, map[string]string{"file_name": "match.dem", "played_at": played.Format(time.RFC3339)})
	if again.Created || again.Match.ID != first.Match.ID || !again.Match.PlayedAt.Equal(played) {
		t.Fatalf("second upload = created %v, match %s at %v; want match %s at %v",
			again.Created, again.Match.ID, again.Match.PlayedAt, first.Match.ID, played)
	}

	other := played.Add(-time.Hour)
	third := uploadMatch(t, env, sha, map[string]string{"file_name": "match.dem", "played_at": other.Format(time.RFC3339)})
	if !third.Match.PlayedAt.Equal(played) {
		t.Fatalf("a match no longer on its import day changed to %v, want %v", third.Match.PlayedAt, played)
	}
}

func TestUploadLibraryAnalysisDatesAFaceitDemoByItsRoom(t *testing.T) {
	started := time.Now().UTC().Add(-30 * time.Hour).Truncate(time.Second)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/matches/"+faceitRoom {
			http.NotFound(w, r)
			return
		}
		_, _ = fmt.Fprintf(w, `{"match_id":%q,"started_at":%d}`, faceitRoom, started.Unix())
	}))
	defer server.Close()
	client, err := faceit.New(faceit.Options{APIKey: "test", BaseURL: server.URL, HTTPClient: server.Client()})
	if err != nil {
		t.Fatal(err)
	}
	repo, files := store.NewMemoryLibraryRepository(), newFakeStorage()
	h := NewHandlers(newFakeRepo(), files, &fakeQueue{}, WithLibrary(repo),
		WithLibraryUploads(uploadWorkspace), WithFaceit(client, nil))
	env := libraryTestEnv{t: t, repo: repo, storage: files, h: h, router: Routes(h)}

	room := uploadMatch(t, env, strings.Repeat("ef", 32), map[string]string{"file_name": faceitRoom + "-1-1.dem"})
	if !room.Match.PlayedAt.Equal(started) || room.Demo.Source != library.SourceFaceit {
		t.Fatalf("FACEIT upload = %v from %q, want %v from faceit", room.Match.PlayedAt, room.Demo.Source, started)
	}
	plain := uploadMatch(t, env, strings.Repeat("12", 32), map[string]string{"file_name": "scrim.dem"})
	if !plain.Match.PlayedAt.Equal(plain.Demo.ImportedAt) {
		t.Fatalf("a demo not named after a room = %v, want its import time", plain.Match.PlayedAt)
	}
}
