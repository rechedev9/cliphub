package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/rechedev9/cliphub/internal/artifacts"
	"github.com/rechedev9/cliphub/internal/tacticalplan"
)

func TestLibraryTeamGrenadesListsWhereEachGrenadeCameFrom(t *testing.T) {
	env := newPlaysTestEnv(t)
	m := seedPlayMatch(env)
	// The smoke was thrown from a known spot; the flash comes from an older scan.
	key := artifacts.LibraryTacticalIndexKey(m.DemoID)
	rc, err := env.storage.Open(key)
	if err != nil {
		t.Fatal(err)
	}
	var doc tacticalplan.Document
	if err := json.NewDecoder(rc).Decode(&doc); err != nil {
		t.Fatal(err)
	}
	_ = rc.Close()
	doc.Rounds[0].Events[0].Throw = &tacticalplan.Throw{Tick: 100, Pos: [3]float64{-300, 40, 8}, Yaw: 90, Pitch: 10}
	b, _ := json.Marshal(doc)
	if err := env.storage.Put(key, bytes.NewReader(b)); err != nil {
		t.Fatal(err)
	}
	base := "/api/library/teams/" + teamOfMatch(t, m, "ct-start").TeamID.String() + "/grenades"

	var got struct {
		Grenades []map[string]any `json:"grenades"`
		Rounds   int              `json:"rounds"`
		Skipped  []scoutSkipped   `json:"skipped"`
	}
	env.json(http.MethodGet, base, nil, http.StatusOK, &got)
	if got.Rounds != 2 || len(got.Skipped) != 0 || len(got.Grenades) != 2 {
		t.Fatalf("grenades = %+v, want the smoke and the flash reported once, over 2 rounds", got)
	}
	smoke, flash := got.Grenades[0], got.Grenades[1]
	if smoke["kind"] != "smoke" || smoke["match_id"] != m.ID.String() || smoke["thrower_name"] != "one" || smoke["side"] != "CT" {
		t.Fatalf("smoke = %+v", smoke)
	}
	if origin, ok := smoke["origin"].([]any); !ok || len(origin) != 3 || origin[0] != -300.0 || smoke["throw_tick"] != 100.0 {
		t.Fatalf("smoke origin = %v at %v, want the throw spot", smoke["origin"], smoke["throw_tick"])
	}
	for _, field := range []string{"origin", "throw_tick", "view"} {
		if v, present := flash[field]; !present || v != nil {
			t.Fatalf("flash %s = %v (present %v), want null", field, v, present)
		}
	}

	env.json(http.MethodGet, base+"?kind=smoke&kind=he", nil, http.StatusOK, &got)
	if len(got.Grenades) != 1 || got.Grenades[0]["kind"] != "smoke" {
		t.Fatalf("smokes and HEs = %+v", got.Grenades)
	}
	env.json(http.MethodGet, base+"?map=de_nuke", nil, http.StatusOK, &got)
	if len(got.Grenades) != 0 || got.Rounds != 0 {
		t.Fatalf("another map = %+v, want nothing", got)
	}
	for _, bad := range []string{"?kind=incgrenade", "?side=mid", "?match_id=nope"} {
		if rw := env.do(http.MethodGet, base+bad, nil, ""); rw.Code != http.StatusBadRequest {
			t.Fatalf("GET %s = %d, want 400", bad, rw.Code)
		}
	}
}
