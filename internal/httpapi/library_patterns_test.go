package httpapi

import (
	"net/http"
	"net/url"
	"testing"

	"github.com/rechedev9/cliphub/internal/library"
)

func TestLibraryTeamPatternRunsSavesAndDetectsAcrossTheLibrary(t *testing.T) {
	f := newScoutFixture(t)
	base := "/api/library/teams/" + f.vitality.String() + "/pattern?pattern="
	anyone := url.QueryEscape(`{"side":"CT","blocks":[{"kind":"player","from":0}]}`)

	var run patternRunResponse
	f.env.json(http.MethodGet, base+anyone, nil, http.StatusOK, &run)
	if run.Map != "de_mirage" || run.Result.Rounds == 0 || run.Geometry == nil || run.Overlay == nil || run.Heatmap == nil {
		t.Fatalf("run = map %q, result %+v", run.Map, run.Result)
	}
	for _, m := range run.Result.Matches {
		for _, r := range m.Matched {
			if r.Side != "CT" {
				t.Fatalf("a CT pattern kept a %s round", r.Side)
			}
		}
	}
	var inverted patternRunResponse
	f.env.json(http.MethodGet, base+url.QueryEscape(`{"side":"CT","blocks":[{"kind":"player","from":0,"invert":true}]}`), nil, http.StatusOK, &inverted)
	if inverted.Result.Rounds != run.Result.Rounds || inverted.Result.Matched+run.Result.Matched != run.Result.Rounds {
		t.Fatalf("invert kept %d of %d, the block %d", inverted.Result.Matched, inverted.Result.Rounds, run.Result.Matched)
	}
	for _, bad := range []string{"", url.QueryEscape(`{"blocks":[]}`), url.QueryEscape(`{"blocks":[{"kind":"kill","from":0}]}`), anyone + "&max_matches=99"} {
		if rw := f.env.do(http.MethodGet, base+bad, nil, ""); rw.Code != http.StatusBadRequest {
			t.Fatalf("GET pattern %s = %d, want 400", bad, rw.Code)
		}
	}

	var saved library.Pattern
	body := map[string]any{"name": " Todos ", "map": "de_mirage", "team_id": f.vitality.String(), "pattern": map[string]any{"side": "CT", "blocks": []map[string]any{{"kind": "player", "from": 0}}}}
	f.env.json(http.MethodPost, "/api/library/patterns", body, http.StatusCreated, &saved)
	if saved.Name != "Todos" || saved.TeamID == nil {
		t.Fatalf("saved = %+v", saved)
	}
	wide := map[string]any{"name": "Biblioteca", "map": "de_mirage", "pattern": map[string]any{"side": "T", "blocks": []map[string]any{{"kind": "player", "from": 0}}}}
	f.env.json(http.MethodPost, "/api/library/patterns", wide, http.StatusCreated, nil)
	if rw := f.env.do(http.MethodPost, "/api/library/patterns", []byte(`{"name":"x","map":"de_mirage","pattern":{"blocks":[{"kind":"player","from":0}]}}`), "application/json"); rw.Code != http.StatusBadRequest {
		t.Fatalf("a pattern without a side = %d, want 400", rw.Code)
	}

	var listed struct {
		Patterns []patternListItem `json:"patterns"`
	}
	f.env.json(http.MethodGet, "/api/library/patterns?map=de_mirage&stats=1&team_id="+f.vitality.String(), nil, http.StatusOK, &listed)
	if len(listed.Patterns) != 2 || listed.Patterns[0].Stats == nil || listed.Patterns[1].Stats == nil {
		t.Fatalf("team list = %+v", listed.Patterns)
	}
	f.env.json(http.MethodGet, "/api/library/patterns?map=de_mirage&stats=1", nil, http.StatusOK, &listed)
	for _, p := range listed.Patterns {
		if p.Stats == nil || p.Stats.Rounds == 0 {
			t.Fatalf("library-wide figures of %q = %+v", p.Name, p.Stats)
		}
	}

	// The cached figures are the ones a fresh pass gives.
	first := listed.Patterns
	f.env.json(http.MethodGet, "/api/library/patterns?map=de_mirage&stats=1", nil, http.StatusOK, &listed)
	for i, p := range listed.Patterns {
		if *p.Stats != *first[i].Stats {
			t.Fatalf("cached figures of %q = %+v, first pass %+v", p.Name, p.Stats, first[i].Stats)
		}
	}

	var rounds patternRunResponse
	f.env.json(http.MethodGet, "/api/library/patterns/"+saved.ID.String()+"/rounds", nil, http.StatusOK, &rounds)
	if rounds.Result.Matched != run.Result.Matched {
		t.Fatalf("saved pattern over its team matched %d, the run %d", rounds.Result.Matched, run.Result.Matched)
	}

	// A pattern's own scope applies to its figures and to its rounds alike.
	var newest library.Pattern
	body["name"], body["scope"] = "Última", map[string]any{"matches": 1}
	f.env.json(http.MethodPost, "/api/library/patterns", body, http.StatusCreated, &newest)
	f.env.json(http.MethodGet, "/api/library/patterns?map=de_mirage&stats=1&team_id="+f.vitality.String(), nil, http.StatusOK, &listed)
	f.env.json(http.MethodGet, "/api/library/patterns/"+newest.ID.String()+"/rounds", nil, http.StatusOK, &rounds)
	for _, p := range listed.Patterns {
		if p.ID == newest.ID && (p.Stats.Rounds != rounds.Result.Rounds || len(rounds.Result.Matches) > 1) {
			t.Fatalf("one-match pattern: list %+v, rounds %d of the run's %d over %d matches", p.Stats, rounds.Result.Rounds, run.Result.Rounds, len(rounds.Result.Matches))
		}
	}
	// A half buy is semi or force, as the scope bar holds it.
	var half library.Pattern
	body["name"], body["scope"] = "Media", map[string]any{"buy": "half"}
	f.env.json(http.MethodPost, "/api/library/patterns", body, http.StatusCreated, &half)
	f.env.json(http.MethodGet, "/api/library/patterns/"+half.ID.String()+"/rounds", nil, http.StatusOK, &rounds)
	for _, m := range rounds.Result.Matches {
		for _, r := range m.Matched {
			if r.Buy != "semi" && r.Buy != "force" {
				t.Fatalf("a half buy pattern kept a %s round", r.Buy)
			}
		}
	}
	f.env.json(http.MethodDelete, "/api/library/patterns/"+saved.ID.String(), nil, http.StatusNoContent, nil)
	f.env.json(http.MethodGet, "/api/library/patterns/"+saved.ID.String(), nil, http.StatusNotFound, nil)
}
