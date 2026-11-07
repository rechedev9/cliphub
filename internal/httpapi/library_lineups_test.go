package httpapi

import (
	"bytes"
	"encoding/json"
	"math"
	"net/http"
	"testing"

	"github.com/rechedev9/cliphub/internal/artifacts"
	"github.com/rechedev9/cliphub/internal/library"
	"github.com/rechedev9/cliphub/internal/tacticalplan"
)

// seedThrowMatch gives round 1 one smoke with its throw, looking up as demos write it.
func seedThrowMatch(env libraryTestEnv) library.Match {
	env.t.Helper()
	m := env.seedLibraryMatch("lineups", "Vitality", "MOUZ")
	key := artifacts.LibraryTacticalIndexKey(m.DemoID)
	rc, err := env.storage.Open(key)
	if err != nil {
		env.t.Fatal(err)
	}
	var doc tacticalplan.Document
	if err := json.NewDecoder(rc).Decode(&doc); err != nil {
		env.t.Fatal(err)
	}
	_ = rc.Close()
	slot := uint8(0)
	doc.Rounds[0].Events = []tacticalplan.Event{{
		Tick: 132, Kind: tacticalplan.EventSmoke, ActorSlot: &slot, Side: tacticalplan.SideCT, Pos: [3]float64{510, 500, 0},
		Throw: &tacticalplan.Throw{Tick: 116, Pos: [3]float64{40, 0, 0}, Yaw: 270, Pitch: 330},
	}}
	b, _ := json.Marshal(doc)
	if err := env.storage.Put(key, bytes.NewReader(b)); err != nil {
		env.t.Fatal(err)
	}
	return m
}

func TestLibraryLineupByHandNamesItsPlace(t *testing.T) {
	env := newPlaysTestEnv(t)
	seedCalloutMatch(env)
	for _, tc := range []struct {
		body map[string]any
		want string
	}{
		{map[string]any{"map": "de_mirage", "origin": []float64{3, 4, 0}}, "Apartments"},
		{map[string]any{"map": "de_mirage", "origin": []float64{3, 4, 0}, "throw_callout": "BombsiteB"}, "BombsiteB"},
		{map[string]any{"map": "de_nuke", "origin": []float64{3, 4, 0}}, ""},
	} {
		body := map[string]any{"title": "Pop", "side": "T", "grenade": "flash", "pitch": -10, "yaw": 90}
		for k, v := range tc.body {
			body[k] = v
		}
		var got library.Lineup
		env.json(http.MethodPost, "/api/library/lineups", body, http.StatusCreated, &got)
		if got.ThrowCallout != tc.want {
			t.Fatalf("%v: throw callout = %q, want %q", tc.body, got.ThrowCallout, tc.want)
		}
	}
}

func TestLibraryLineupsFromAThrowAndByHand(t *testing.T) {
	env := newPlaysTestEnv(t)
	m := seedThrowMatch(env)

	var smoke library.Lineup
	env.json(http.MethodPost, "/api/library/lineups",
		map[string]any{"title": "Humo a ventana", "from": map[string]any{"match_id": m.ID, "round": 1, "throw_tick": 116}}, http.StatusCreated, &smoke)
	if smoke.Map != "de_mirage" || smoke.Side != "CT" || smoke.Grenade != "smoke" || smoke.Origin != [3]float64{40, 0, 0} ||
		smoke.Landing == nil || smoke.Source == nil || smoke.Source.Tick != 116 || !smoke.TechniqueEstimated {
		t.Fatalf("lineup from throw = %+v", smoke)
	}
	if math.Abs(smoke.Pitch+30) > 1e-9 || math.Abs(smoke.Yaw+90) > 1e-9 {
		t.Fatalf("angles = %v %v, want -30 and -90 for setang", smoke.Pitch, smoke.Yaw)
	}
	for body, code := range map[string]int{
		`{"title":"x","from":{"match_id":"` + m.ID.String() + `","round":1,"throw_tick":5}}`:                 http.StatusBadRequest,
		`{"title":"x","from":{"match_id":"` + m.ID.String() + `","round":9,"throw_tick":116}}`:               http.StatusBadRequest,
		`{"title":"x","map":"de_nuke","side":"T","grenade":"c4","origin":[0,0,0]}`:                           http.StatusBadRequest,
		`{"title":"Pop","map":"de_nuke","side":"T","grenade":"flash","origin":[1,2,3],"pitch":-10,"yaw":90}`: http.StatusCreated,
	} {
		if rw := env.do(http.MethodPost, "/api/library/lineups", []byte(body), "application/json"); rw.Code != code {
			t.Fatalf("POST %s = %d, want %d: %s", body, rw.Code, code, rw.Body.String())
		}
	}

	var patched library.Lineup
	env.json(http.MethodPatch, "/api/library/lineups/"+smoke.ID.String(), map[string]any{"jump": true, "click": "both", "status": "practice"}, http.StatusOK, &patched)
	if !patched.Jump || patched.Click != "both" || patched.Status != "practice" || patched.TechniqueEstimated {
		t.Fatalf("patched = %+v, want the technique set by hand", patched)
	}
	if rw := env.do(http.MethodPatch, "/api/library/lineups/"+smoke.ID.String(), []byte(`{"map":"de_nuke"}`), "application/json"); rw.Code != http.StatusBadRequest {
		t.Fatalf("PATCH map = %d, want 400", rw.Code)
	}

	var listed struct {
		Lineups []library.Lineup `json:"lineups"`
	}
	for query, want := range map[string]int{"": 2, "?map=de_mirage": 1, "?grenade=flash": 1, "?status=practice": 1, "?grenade=c4": -1} {
		if want < 0 {
			if rw := env.do(http.MethodGet, "/api/library/lineups"+query, nil, ""); rw.Code != http.StatusBadRequest {
				t.Fatalf("GET lineups%s = %d, want 400", query, rw.Code)
			}
			continue
		}
		env.json(http.MethodGet, "/api/library/lineups"+query, nil, http.StatusOK, &listed)
		if len(listed.Lineups) != want {
			t.Fatalf("GET lineups%s = %d, want %d", query, len(listed.Lineups), want)
		}
	}
	env.json(http.MethodDelete, "/api/library/lineups/"+smoke.ID.String(), nil, http.StatusNoContent, nil)
	env.json(http.MethodGet, "/api/library/lineups/"+smoke.ID.String(), nil, http.StatusNotFound, nil)
}
