package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/rechedev9/cliphub/internal/artifacts"
	"github.com/rechedev9/cliphub/internal/tacticalplan"
)

// seedPositionsMatch seeds the library fixture with a CT Spawn callout where
// player one stands in round 1, and an AWP kill of theirs in that round.
func seedPositionsMatch(env libraryTestEnv) {
	env.t.Helper()
	m := env.seedLibraryMatch("positions", "Vitality", "MOUZ")
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
	doc.Geometry.Callouts = []tacticalplan.Callout{{Name: "CTSpawn", Level: "default", Center: [2]float64{40, 20}}}
	slot := uint8(0)
	doc.Rounds[0].Events = []tacticalplan.Event{{Tick: 150, Kind: tacticalplan.EventKill, ActorSlot: &slot, Weapon: "awp"}}
	b, _ := json.Marshal(doc)
	if err := env.storage.Put(key, bytes.NewReader(b)); err != nil {
		env.t.Fatal(err)
	}
}

func TestLibraryPlayerPositions(t *testing.T) {
	env := newLibraryTestEnv(t)
	seedPositionsMatch(env)
	base := "/api/library/players/76561198000000001/positions"

	var resp positionsResponse
	env.json(http.MethodGet, base, nil, http.StatusOK, &resp)
	if resp.SteamID64 != "76561198000000001" || resp.MatchCount != 1 || len(resp.Maps) != 1 || len(resp.Skipped) != 0 {
		t.Fatalf("positions = %+v", resp)
	}
	ct := resp.Maps[0].Sides[0]
	// Round 2 has no frame of player one, so only round 1 is sampled.
	if ct.Side != tacticalplan.SideCT || ct.Rounds != 2 || ct.Sampled != 1 || ct.Role != tacticalplan.PositionRotator {
		t.Fatalf("CT = %+v", ct)
	}
	if len(ct.Places) != 1 || ct.Places[0].Label != "CT Spawn" || ct.Places[0].Rounds.Count != 1 || ct.AWPKills.Count != 1 {
		t.Fatalf("CT places = %+v, AWP %+v", ct.Places, ct.AWPKills)
	}

	env.json(http.MethodGet, base+"?map=de_nuke", nil, http.StatusOK, &resp)
	if resp.MatchCount != 0 || len(resp.Maps) != 0 {
		t.Fatalf("positions on a map never played = %+v", resp)
	}
	for _, bad := range []string{"?map=Mirage!", "?from=ayer", "?to=luego"} {
		if rw := env.do(http.MethodGet, base+bad, nil, ""); rw.Code != http.StatusBadRequest {
			t.Fatalf("positions%s = %d, want 400", bad, rw.Code)
		}
	}
	if rw := env.do(http.MethodGet, "/api/library/players/76561198000000999/positions", nil, ""); rw.Code != http.StatusNotFound {
		t.Fatalf("positions of an unknown player = %d, want 404", rw.Code)
	}
}
