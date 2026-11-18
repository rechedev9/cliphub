package httpapi

import (
	"net/http"
	"strings"
	"testing"

	"github.com/rechedev9/cliphub/internal/library"
)

func TestLibraryDemoTitleIsSavedReadAndCleared(t *testing.T) {
	env := newLibraryTestEnv(t)
	m := env.seedLibraryMatch("titled", "Vitality", "MOUZ")
	path := "/api/library/demos/" + m.DemoID.String()

	var patched library.Demo
	env.json(http.MethodPatch, path, map[string]any{"title": "  Final ESL contra MOUZ  "}, http.StatusOK, &patched)
	if patched.Title != "Final ESL contra MOUZ" {
		t.Fatalf("patched title = %q, want the title without the outer spaces", patched.Title)
	}
	var read library.Demo
	env.json(http.MethodGet, path, nil, http.StatusOK, &read)
	var listed struct {
		Matches []library.Match `json:"matches"`
	}
	env.json(http.MethodGet, "/api/library/matches", nil, http.StatusOK, &listed)
	var match library.Match
	env.json(http.MethodGet, "/api/library/matches/"+m.ID.String(), nil, http.StatusOK, &match)
	if read.Title != patched.Title || len(listed.Matches) != 1 || listed.Matches[0].DemoTitle != patched.Title || match.DemoTitle != patched.Title {
		t.Fatalf("title read back as demo %q, listed %+v, match %q", read.Title, listed.Matches, match.DemoTitle)
	}

	env.json(http.MethodPatch, path, map[string]any{"tags": []string{"ESL"}}, http.StatusOK, &patched)
	if patched.Title != "Final ESL contra MOUZ" {
		t.Fatalf("title after a tag edit = %q, want it kept", patched.Title)
	}
	env.json(http.MethodPatch, path, map[string]any{"title": strings.Repeat("é", 80)}, http.StatusOK, &patched)
	for _, refused := range []string{strings.Repeat("é", 81), "Final\nESL", "Final\rESL"} {
		if rw := env.do(http.MethodPatch, path, []byte(`{"title":"`+refused+`"}`), "application/json"); rw.Code != http.StatusBadRequest {
			t.Fatalf("title %q = %d, want 400", refused, rw.Code)
		}
	}
	env.json(http.MethodGet, path, nil, http.StatusOK, &read)
	if read.Title != strings.Repeat("é", 80) {
		t.Fatalf("title after refused edits = %q, want the last saved one", read.Title)
	}

	var cleared library.Demo
	env.json(http.MethodPatch, path, map[string]any{"title": "   "}, http.StatusOK, &cleared)
	var untitled library.Match
	env.json(http.MethodGet, "/api/library/matches/"+m.ID.String(), nil, http.StatusOK, &untitled)
	if cleared.Title != "" || untitled.DemoTitle != "" {
		t.Fatalf("after a blank title: demo %q, match %q; want no title", cleared.Title, untitled.DemoTitle)
	}
}

func TestSearchLibraryFindsAMatchByItsDemoTitle(t *testing.T) {
	env := newPlaysTestEnv(t)
	m := env.seedLibraryMatch("search", "Vitality", "MOUZ")
	env.json(http.MethodPatch, "/api/library/demos/"+m.DemoID.String(), map[string]any{"title": "Final del Clásico"}, http.StatusOK, nil)
	var resp searchResponse
	env.json(http.MethodGet, "/api/library/search?q=final+clasico", nil, http.StatusOK, &resp)
	if len(resp.Results) != 1 || resp.Results[0].Kind != searchMatch || resp.Results[0].Title != "Final del Clásico" {
		t.Fatalf("search by title = %+v, want the match under its title", resp.Results)
	}
}
