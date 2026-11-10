package httpapi

import (
	"context"
	"net/http"
	"strconv"
	"testing"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/library"
)

func TestLibraryPatternDetectionIsStoredAndFollowsEdits(t *testing.T) {
	f := newScoutFixture(t)
	ctx := context.Background()
	play := &library.Play{Title: "Ida a A", Map: "de_mirage", Side: "T", Kind: "execute", Site: "A", Origin: "manual"}
	if err := f.env.repo.CreatePlay(ctx, play); err != nil {
		t.Fatal(err)
	}
	var saved library.Pattern
	body := map[string]any{"name": "Juegan", "map": "de_mirage", "category": " Ejecuciones ", "play_id": play.ID.String(),
		"pattern": map[string]any{"side": "CT", "blocks": []map[string]any{{"kind": "player", "from": 0}}}}
	f.env.json(http.MethodPost, "/api/library/patterns", body, http.StatusCreated, &saved)
	if saved.Category != "Ejecuciones" || saved.PlayID == nil || *saved.PlayID != play.ID {
		t.Fatalf("saved = %+v", saved)
	}
	body["play_id"] = f.vitality.String()
	if rw := f.env.do(http.MethodPost, "/api/library/patterns", mustJSON(t, body), "application/json"); rw.Code != http.StatusNotFound {
		t.Fatalf("a pattern linked to a missing play = %d, want 404", rw.Code)
	}

	var listed struct {
		Patterns []patternListItem `json:"patterns"`
	}
	f.env.json(http.MethodGet, "/api/library/patterns?category=Ejecuciones&play_id="+play.ID.String(), nil, http.StatusOK, &listed)
	if len(listed.Patterns) != 1 || listed.Patterns[0].ID != saved.ID {
		t.Fatalf("by category and play = %+v", listed.Patterns)
	}
	f.env.json(http.MethodGet, "/api/library/patterns?category=Rush", nil, http.StatusOK, &listed)
	if len(listed.Patterns) != 0 {
		t.Fatalf("another category = %+v", listed.Patterns)
	}

	var rounds patternRunResponse
	f.env.json(http.MethodGet, "/api/library/patterns/"+saved.ID.String()+"/rounds", nil, http.StatusOK, &rounds)
	if rounds.Result.Matched == 0 {
		t.Fatalf("library rounds = %+v", rounds.Result)
	}
	rows, err := f.env.repo.ListPatternMatches(ctx, library.PatternMatchFilter{PatternIDs: []uuid.UUID{saved.ID}})
	if err != nil || len(rows) != 6 || rows[0].Revision != library.PatternRevision(saved) {
		t.Fatalf("stored rows = %d %+v, %v; want two teams of three matches", len(rows), rows, err)
	}

	var hits matchPatternsResponse
	f.env.json(http.MethodGet, "/api/library/matches/"+f.home.ID.String()+"/patterns", nil, http.StatusOK, &hits)
	if hits.Total != 1 || len(hits.Patterns) != 1 || hits.Patterns[0].Pattern.ID != saved.ID || hits.Patterns[0].Team != "Vitality" || len(hits.Patterns[0].Rounds) == 0 {
		t.Fatalf("match patterns = %+v, want it from the CT side, where it holds", hits)
	}
	round := hits.Patterns[0].Rounds[0].Round
	f.env.json(http.MethodGet, "/api/library/matches/"+f.home.ID.String()+"/patterns?round="+strconv.Itoa(round), nil, http.StatusOK, &hits)
	for _, hit := range hits.Patterns {
		if len(hit.Rounds) != 1 || hit.Rounds[0].Round != round {
			t.Fatalf("round %d = %+v", round, hit.Rounds)
		}
	}

	// An edit that changes what the pattern reads makes its stored rows stale.
	var edited library.Pattern
	f.env.json(http.MethodPatch, "/api/library/patterns/"+saved.ID.String(), map[string]any{"clear_play": true,
		"pattern": map[string]any{"side": "CT", "blocks": []map[string]any{{"kind": "player", "from": 0, "invert": true}}}}, http.StatusOK, &edited)
	if edited.PlayID != nil || edited.Category != "Ejecuciones" {
		t.Fatalf("edited = %+v", edited)
	}
	var after patternRunResponse
	f.env.json(http.MethodGet, "/api/library/patterns/"+saved.ID.String()+"/rounds", nil, http.StatusOK, &after)
	if after.Result.Rounds != rounds.Result.Rounds || after.Result.Matched+rounds.Result.Matched != rounds.Result.Rounds {
		t.Fatalf("inverted after the edit = %d of %d, before %d", after.Result.Matched, after.Result.Rounds, rounds.Result.Matched)
	}
	var edits matchPatternsResponse
	f.env.json(http.MethodGet, "/api/library/matches/"+f.home.ID.String()+"/patterns", nil, http.StatusOK, &edits)
	for _, hit := range edits.Patterns {
		for _, r := range hit.Rounds {
			if hit.TeamKey == "ct-start" && r.Round == round {
				t.Fatalf("round %d still matches after the edit: %+v", round, edits)
			}
		}
	}
	// A team's pattern only shows from that team's side.
	var mine library.Pattern
	team := map[string]any{"name": "Vitality CT", "map": "de_mirage", "team_id": f.vitality.String(),
		"pattern": map[string]any{"side": "CT", "blocks": []map[string]any{{"kind": "player", "from": 0}}}}
	f.env.json(http.MethodPost, "/api/library/patterns", team, http.StatusCreated, &mine)
	var other matchPatternsResponse
	f.env.json(http.MethodGet, "/api/library/matches/"+f.other.ID.String()+"/patterns", nil, http.StatusOK, &other)
	// Nor does it count in the total of a match the team did not play.
	if other.Total != 1 {
		t.Fatalf("a match Vitality did not play counts %d patterns, want 1", other.Total)
	}
	for _, hit := range other.Patterns {
		if hit.Pattern.ID == mine.ID {
			t.Fatalf("a match Vitality did not play = %+v", other)
		}
	}
	var home matchPatternsResponse
	f.env.json(http.MethodGet, "/api/library/matches/"+f.home.ID.String()+"/patterns", nil, http.StatusOK, &home)
	found := false
	for _, hit := range home.Patterns {
		found = found || (hit.Pattern.ID == mine.ID && hit.Team == "Vitality")
	}
	if !found || home.Total != 2 {
		t.Fatalf("Vitality's own pattern in its match = %+v", home)
	}
}
