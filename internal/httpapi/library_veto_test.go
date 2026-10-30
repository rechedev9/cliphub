package httpapi

import (
	"net/http"
	"testing"

	"github.com/rechedev9/cliphub/internal/library"
)

func TestLibrarySeriesVetoMapsAndSimulator(t *testing.T) {
	f := newScoutFixture(t)
	env := f.env

	var s library.Series
	env.json(http.MethodPost, "/api/library/series", map[string]any{
		"format": "bo3", "event": "IEM", "team_ids": []string{f.vitality.String(), f.mouz.String()},
		"vetoes": []map[string]string{{"team": "b", "action": "ban", "map": "de_mirage"}, {"team": "a", "action": "pick", "map": "de_nuke"}},
	}, http.StatusCreated, &s)
	if len(s.Teams) != 2 || s.Teams[0].Name != "Vitality" || len(s.Vetoes) != 2 {
		t.Fatalf("created series = %+v", s)
	}
	if rw := env.do(http.MethodPost, "/api/library/series", []byte(`{"format":"bo3","vetoes":[{"team":"a","action":"ban","map":"de_nuke"}]}`), "application/json"); rw.Code != http.StatusBadRequest {
		t.Fatalf("veto without teams = %d, want 400", rw.Code)
	}
	env.json(http.MethodPatch, "/api/library/matches/"+f.home.ID.String(), map[string]any{"series_id": s.ID}, http.StatusOK, nil)

	var patched library.Series
	env.json(http.MethodPatch, "/api/library/series/"+s.ID.String(), map[string]any{"event": "Major"}, http.StatusOK, &patched)
	if patched.Event != "Major" || len(patched.Vetoes) != 2 || len(patched.MatchIDs) != 1 {
		t.Fatalf("patched = %+v, want the event changed and the rest kept", patched)
	}
	var listed struct {
		Series []library.Series `json:"series"`
	}
	env.json(http.MethodGet, "/api/library/series?team_id="+f.spirit.String(), nil, http.StatusOK, &listed)
	if len(listed.Series) != 0 {
		t.Fatalf("Spirit series = %d, want none", len(listed.Series))
	}

	var maps teamMapsResponse
	env.json(http.MethodGet, "/api/library/teams/"+f.vitality.String()+"/maps", nil, http.StatusOK, &maps)
	if maps.Matches != 2 || maps.Series != 1 || len(maps.Skipped) != 0 {
		t.Fatalf("maps = %d matches, %d series, skipped %v", maps.Matches, maps.Series, maps.Skipped)
	}
	byMap := map[string]library.MapRecord{}
	for _, r := range maps.Maps {
		byMap[r.Map] = r
	}
	played := byMap[f.home.Map]
	if played.Matches != 2 || played.Sides[0].Rounds+played.Sides[1].Rounds != 4 {
		t.Fatalf("%s record = %+v, want both matches and their 4 rounds", f.home.Map, played)
	}
	if byMap["de_nuke"].Veto.Picked.Count != 1 || byMap["de_mirage"].Veto.OpponentBanned.Count != 1 {
		t.Fatalf("veto counts = %+v / %+v", byMap["de_nuke"].Veto, byMap["de_mirage"].Veto)
	}

	var veto vetoResponse
	env.json(http.MethodGet, "/api/library/teams/"+f.vitality.String()+"/veto?opponent_id="+f.mouz.String()+"&format=bo1&done=de_train", nil, http.StatusOK, &veto)
	if veto.OpponentName != "MOUZ" || len(veto.Plan.Steps) != 7 || veto.Plan.Steps[0].Map != "de_train" || len(veto.Plan.Played) != 1 {
		t.Fatalf("veto = %+v", veto.Plan)
	}
	for _, path := range []string{
		"/veto?opponent_id=" + f.vitality.String(),
		"/veto?opponent_id=nope",
		"/veto?opponent_id=" + f.mouz.String() + "&format=bo5&pool=de_nuke&pool=de_mirage",
	} {
		if rw := env.do(http.MethodGet, "/api/library/teams/"+f.vitality.String()+path, nil, ""); rw.Code != http.StatusBadRequest {
			t.Fatalf("GET %s = %d, want 400", path, rw.Code)
		}
	}

	if rw := env.do(http.MethodDelete, "/api/library/series/"+s.ID.String(), nil, ""); rw.Code != http.StatusNoContent {
		t.Fatalf("delete = %d", rw.Code)
	}
	if rw := env.do(http.MethodDelete, "/api/library/series/"+s.ID.String(), nil, ""); rw.Code != http.StatusNotFound {
		t.Fatalf("delete twice = %d, want 404", rw.Code)
	}
}
