package tactical

import (
	"testing"

	"github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs/common"

	"github.com/rechedev9/cliphub/internal/tacticalplan"
)

func TestThrowLogMatchesEachDetonationToItsThrow(t *testing.T) {
	var log throwLog
	throwAt := func(tick int) tacticalplan.Throw {
		return tacticalplan.Throw{Tick: tick, Pos: [3]float64{float64(tick), 0, 0}}
	}
	log.add(pendingThrow{entityID: 10, steamID: 1, kind: tacticalplan.EventFlash, throw: throwAt(100)})
	log.add(pendingThrow{entityID: 11, steamID: 1, kind: tacticalplan.EventFlash, throw: throwAt(110)})
	log.add(pendingThrow{entityID: 12, steamID: 2, kind: tacticalplan.EventMolotov, throw: throwAt(120)})
	log.add(pendingThrow{entityID: 13, steamID: 2, kind: tacticalplan.EventSmoke, throw: throwAt(130)})

	// The second flash pops first: its entity picks it, not the older one.
	if got, ok := log.claim(detonation{entityID: 11, steamID: 1, kind: tacticalplan.EventFlash, tick: 200}, 0); !ok || got.Tick != 110 {
		t.Fatalf("flash by entity = %+v, %v; want the throw at 110", got, ok)
	}
	// A repeated report of that detonation gets the same throw and leaves the other flash.
	if got, ok := log.claim(detonation{entityID: 11, steamID: 1, kind: tacticalplan.EventFlash, tick: 200}, 0); !ok || got.Tick != 110 {
		t.Fatalf("repeated flash = %+v, %v; want the throw at 110 again", got, ok)
	}
	// A molotov's fire is a new entity, so the thrower and kind find its throw.
	if got, ok := log.claim(detonation{steamID: 2, kind: tacticalplan.EventMolotov, tick: 250}, 0); !ok || got.Tick != 120 {
		t.Fatalf("molotov = %+v, %v; want the throw at 120", got, ok)
	}
	// A throw older than the flight window is not a candidate.
	if got, ok := log.claim(detonation{steamID: 1, kind: tacticalplan.EventFlash, tick: 3000}, 1000); ok {
		t.Fatalf("stale flash claimed %+v", got)
	}
	// Nothing of that kind by that player is in flight.
	if _, ok := log.claim(detonation{steamID: 1, kind: tacticalplan.EventSmoke, tick: 300}, 0); ok {
		t.Fatal("player 1 had no smoke in flight")
	}
	if len(log.pending) != 2 {
		t.Fatalf("pending = %+v, want the first flash and the smoke", log.pending)
	}
	log.reset()
	if len(log.pending) != 0 {
		t.Fatal("reset kept throws of the last round")
	}
}

func TestGrenadeKindCoversEveryGrenade(t *testing.T) {
	cases := map[common.EquipmentType]tacticalplan.EventKind{
		common.EqSmoke: tacticalplan.EventSmoke, common.EqFlash: tacticalplan.EventFlash, common.EqHE: tacticalplan.EventHE,
		common.EqMolotov: tacticalplan.EventMolotov, common.EqIncendiary: tacticalplan.EventMolotov, common.EqDecoy: tacticalplan.EventDecoy,
	}
	for eq, want := range cases {
		if got, ok := grenadeKind(&common.Equipment{Type: eq}); !ok || got != want {
			t.Errorf("grenadeKind(%v) = %q, %v; want %q", eq, got, ok, want)
		}
	}
	if _, ok := grenadeKind(&common.Equipment{Type: common.EqAK47}); ok {
		t.Error("a rifle is not a grenade")
	}
}
