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
	path := "/api/library/teams/" + teamOfMatch(t, m, "ct-start").TeamID.String() + "/grenades"
	base := path + "?map=" + m.Map

	var got struct {
		Grenades  []map[string]any            `json:"grenades"`
		Rounds    int                         `json:"rounds"`
		RoundList []tacticalplan.GrenadeRound `json:"round_list"`
		Skipped   []scoutSkipped              `json:"skipped"`
	}
	env.json(http.MethodGet, base, nil, http.StatusOK, &got)
	if got.Rounds != 2 || len(got.RoundList) != 2 || len(got.Skipped) != 0 || len(got.Grenades) != 2 {
		t.Fatalf("grenades = %+v, want the smoke and the flash reported once, over 2 listed rounds", got)
	}
	if got.RoundList[0].Side != tacticalplan.SideCT || got.RoundList[0].Category == "" {
		t.Fatalf("first round = %+v, want the team's CT round with its category", got.RoundList[0])
	}
	if spot, ok := got.Grenades[0]["spot"].(string); !ok || spot == "" {
		t.Fatalf("smoke spot = %v, want a spot key", got.Grenades[0]["spot"])
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

	env.json(http.MethodGet, base+"&kind=smoke&kind=he", nil, http.StatusOK, &got)
	if len(got.Grenades) != 1 || got.Grenades[0]["kind"] != "smoke" {
		t.Fatalf("smokes and HEs = %+v", got.Grenades)
	}
	env.json(http.MethodGet, path+"?map=de_nuke", nil, http.StatusOK, &got)
	if len(got.Grenades) != 0 || got.Rounds != 0 {
		t.Fatalf("another map = %+v, want nothing", got)
	}
	// The rivals threw nothing; their rounds are the team's, seen from T.
	env.json(http.MethodGet, base+"&opponents=1", nil, http.StatusOK, &got)
	if len(got.Grenades) != 0 || got.Rounds != 2 || got.RoundList[0].Side != tacticalplan.SideT {
		t.Fatalf("opponents = %+v, want no grenades over the same 2 rounds on T", got)
	}
	for _, bad := range []string{"", "&kind=incgrenade", "&side=mid", "&match_id=nope", "&opponents=yes"} {
		url := base + bad
		if bad == "" {
			url = path
		}
		if rw := env.do(http.MethodGet, url, nil, ""); rw.Code != http.StatusBadRequest {
			t.Fatalf("GET %s = %d, want 400", url, rw.Code)
		}
	}
}
