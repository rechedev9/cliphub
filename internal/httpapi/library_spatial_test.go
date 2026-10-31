package httpapi

import (
	"net/http"
	"testing"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/library"
	"github.com/rechedev9/cliphub/internal/tacticalplan"
)

func TestLibraryTeamHeatmapAndOverlayFollowTheTeam(t *testing.T) {
	f := newScoutFixture(t)
	base := "/api/library/teams/" + f.vitality.String()

	var heat spatialResponse
	f.env.json(http.MethodGet, base+"/heatmap?kind=position", nil, http.StatusOK, &heat)
	if heat.Map != "de_mirage" || len(heat.Maps) != 1 || heat.Maps[0].Matches != 2 || len(heat.Matches) != 2 {
		t.Fatalf("heatmap read %q over %+v, want both Vitality matches on de_mirage", heat.Map, heat.Matches)
	}
	if heat.Geometry == nil || heat.ReferenceMatchID == nil || heat.Heatmap == nil || heat.Overlay != nil {
		t.Fatalf("heatmap response = %+v, want a geometry, a reference and only the heatmap", heat)
	}
	// Only the home match has a Vitality player with positions: slot 0 at freeze end.
	cells := heat.Heatmap.Cells
	if len(cells) != 1 || cells[0].Side != tacticalplan.SideCT || cells[0].Examples[0].MatchID != f.home.ID {
		t.Fatalf("cells = %+v, want the home CT sample", cells)
	}

	var overlay spatialResponse
	f.env.json(http.MethodGet, base+"/overlay?limit=3", nil, http.StatusOK, &overlay)
	if overlay.Overlay == nil || overlay.Overlay.Total != 4 || len(overlay.Overlay.Rounds) != 3 {
		t.Fatalf("overlay = %+v, want 4 rounds matched and 3 drawn", overlay.Overlay)
	}
	var first tacticalplan.OverlayRound
	for _, r := range overlay.Overlay.Rounds {
		if r.MatchID == f.home.ID && r.Round == 1 {
			first = r
		}
	}
	if len(first.Tracks) != 1 || first.Tracks[0].Name != "one" || first.Side != tacticalplan.SideCT {
		t.Fatalf("first overlaid round = %+v, want the home round 1 track of slot 0", first)
	}

	var ct spatialResponse
	f.env.json(http.MethodGet, base+"/overlay?side=T", nil, http.StatusOK, &ct)
	for _, r := range ct.Overlay.Rounds {
		if r.Side != tacticalplan.SideT {
			t.Fatalf("T overlay has a %s round: %+v", r.Side, r)
		}
	}
	var nuke spatialResponse
	f.env.json(http.MethodGet, base+"/heatmap?map=de_nuke", nil, http.StatusOK, &nuke)
	if len(nuke.Matches) != 0 || nuke.Geometry != nil || len(nuke.Heatmap.Cells) != 0 {
		t.Fatalf("nuke heatmap = %+v, want nothing read", nuke)
	}

	for _, bad := range []string{
		"/heatmap?kind=smell", "/heatmap?t_from=20&t_to=10", "/heatmap?grenade=decoy", "/heatmap?map=de_nuke&map=de_mirage",
		"/overlay?limit=99", "/heatmap?buy=lots",
	} {
		if rw := f.env.do(http.MethodGet, base+bad, nil, ""); rw.Code != http.StatusBadRequest {
			t.Fatalf("GET %s = %d, want 400", bad, rw.Code)
		}
	}
	if rw := f.env.do(http.MethodGet, "/api/library/teams/"+uuid.NewString()+"/heatmap", nil, ""); rw.Code != http.StatusNotFound {
		t.Fatalf("heatmap of an unknown team = %d, want 404", rw.Code)
	}
}

func TestLibraryMatchHeatmapReadsTheTacticalFilter(t *testing.T) {
	f := newScoutFixture(t)
	base := "/api/library/matches/" + f.home.ID.String() + "/tactical"

	var overlay spatialResponse
	f.env.json(http.MethodGet, base+"/overlay?team=ct-start&buy=pistol", nil, http.StatusOK, &overlay)
	if len(overlay.Overlay.Rounds) != 1 || overlay.Overlay.Rounds[0].Round != 1 || overlay.Matches[0].TeamKey != "ct-start" {
		t.Fatalf("pistol overlay = %+v, want round 1 of ct-start", overlay.Overlay)
	}
	var deaths spatialResponse
	f.env.json(http.MethodGet, base+"/heatmap?kind=death", nil, http.StatusOK, &deaths)
	if deaths.Heatmap.Kind != tacticalplan.HeatDeath || deaths.Heatmap.Rounds != 2 || len(deaths.Heatmap.Cells) != 0 {
		t.Fatalf("death heatmap = %+v, want 2 rounds and no kill events", deaths.Heatmap)
	}
	if rw := f.env.do(http.MethodGet, base+"/heatmap?side=mid", nil, ""); rw.Code != http.StatusBadRequest {
		t.Fatalf("heatmap?side=mid = %d, want 400", rw.Code)
	}
	if rw := f.env.do(http.MethodGet, "/api/library/matches/"+uuid.NewString()+"/tactical/overlay", nil, ""); rw.Code != http.StatusNotFound {
		t.Fatalf("overlay of an unknown match = %d, want 404", rw.Code)
	}
}

func TestLibraryAnnotationsLifecycle(t *testing.T) {
	f := newScoutFixture(t)
	env := f.env
	stroke := map[string]any{"color": "accent", "width": 3, "arrow": true, "points": [][2]float64{{10, 20}, {300, -40}}}

	var note library.Annotation
	env.json(http.MethodPost, "/api/library/annotations", map[string]any{
		"match_id": f.home.ID, "round": 1, "tick": 150, "text": "  entran por palacio ", "strokes": []any{stroke},
	}, http.StatusCreated, &note)
	if note.Text != "entran por palacio" || len(note.Strokes) != 1 || !note.Strokes[0].Arrow || note.MatchID != f.home.ID {
		t.Fatalf("created note = %+v", note)
	}
	env.json(http.MethodPost, "/api/library/annotations", map[string]any{
		"match_id": f.away.ID, "round": 2, "tick": 600, "text": "rotación",
	}, http.StatusCreated, nil)

	for _, bad := range []map[string]any{
		{"match_id": f.home.ID, "round": 1, "tick": 150},
		{"match_id": f.home.ID, "round": 99, "tick": 150, "text": "x"},
		{"match_id": f.home.ID, "round": 1, "text": "x"},
		{"match_id": f.home.ID, "round": 1, "tick": 1, "strokes": []any{map[string]any{"color": "red", "width": 2, "points": [][2]float64{{0, 0}, {1, 1}}}}},
		{"match_id": f.home.ID, "round": 1, "tick": 1, "text": "x", "extra": true},
	} {
		if rw := env.do(http.MethodPost, "/api/library/annotations", mustJSON(t, bad), "application/json"); rw.Code != http.StatusBadRequest {
			t.Fatalf("POST %v = %d, want 400", bad, rw.Code)
		}
	}
	if rw := env.do(http.MethodPost, "/api/library/annotations", mustJSON(t, map[string]any{
		"match_id": uuid.New(), "round": 1, "tick": 1, "text": "x",
	}), "application/json"); rw.Code != http.StatusNotFound {
		t.Fatalf("POST on an unknown match = %d, want 404", rw.Code)
	}

	var list struct {
		Annotations []library.Annotation `json:"annotations"`
	}
	env.json(http.MethodGet, "/api/library/annotations?match_id="+f.home.ID.String()+"&match_id="+f.away.ID.String(), nil, http.StatusOK, &list)
	if len(list.Annotations) != 2 {
		t.Fatalf("annotations = %+v, want both notes", list.Annotations)
	}
	env.json(http.MethodGet, "/api/library/annotations?match_id="+f.home.ID.String()+"&round=1", nil, http.StatusOK, &list)
	if len(list.Annotations) != 1 || list.Annotations[0].ID != note.ID {
		t.Fatalf("round 1 annotations = %+v", list.Annotations)
	}
	for _, bad := range []string{"", "?match_id=nope", "?match_id=" + f.home.ID.String() + "&round=0"} {
		if rw := env.do(http.MethodGet, "/api/library/annotations"+bad, nil, ""); rw.Code != http.StatusBadRequest {
			t.Fatalf("GET annotations%s = %d, want 400", bad, rw.Code)
		}
	}

	var patched library.Annotation
	env.json(http.MethodPatch, "/api/library/annotations/"+note.ID.String(), map[string]any{"text": "palacio y rampa", "tick": 170}, http.StatusOK, &patched)
	if patched.Text != "palacio y rampa" || patched.Tick != 170 || len(patched.Strokes) != 1 {
		t.Fatalf("patched note = %+v, want the drawing kept", patched)
	}
	if rw := env.do(http.MethodPatch, "/api/library/annotations/"+note.ID.String(), mustJSON(t, map[string]any{"round": 2}), "application/json"); rw.Code != http.StatusBadRequest {
		t.Fatalf("PATCH round = %d, want 400", rw.Code)
	}
	if rw := env.do(http.MethodPatch, "/api/library/annotations/"+note.ID.String(), mustJSON(t, map[string]any{"text": "", "strokes": []any{}}), "application/json"); rw.Code != http.StatusBadRequest {
		t.Fatalf("PATCH that empties the note = %d, want 400", rw.Code)
	}
	if rw := env.do(http.MethodDelete, "/api/library/annotations/"+note.ID.String(), nil, ""); rw.Code != http.StatusNoContent {
		t.Fatalf("DELETE = %d, want 204", rw.Code)
	}
	if rw := env.do(http.MethodDelete, "/api/library/annotations/"+note.ID.String(), nil, ""); rw.Code != http.StatusNotFound {
		t.Fatalf("second DELETE = %d, want 404", rw.Code)
	}
}
