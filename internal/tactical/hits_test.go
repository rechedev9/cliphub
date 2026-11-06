package tactical

import (
	"testing"

	"github.com/rechedev9/cliphub/internal/tacticalplan"
)

func TestMergeHitSumsASprayButNotALaterBurst(t *testing.T) {
	var hits []tacticalplan.Hit
	add := func(tick int, actor, target uint8, damage int, weapon string) {
		hits = mergeHit(hits, tacticalplan.Hit{Tick: tick, ActorSlot: actor, TargetSlot: target, Damage: damage, Weapon: weapon}, 16)
	}
	add(100, 1, 7, 27, "ak47")
	add(104, 2, 8, 30, "m4a1")
	add(108, 1, 7, 27, "ak47")
	add(110, 1, 7, 20, "hegrenade")
	add(130, 1, 7, 27, "ak47")

	want := []tacticalplan.Hit{
		{Tick: 100, ActorSlot: 1, TargetSlot: 7, Damage: 54, Weapon: "ak47"},
		{Tick: 104, ActorSlot: 2, TargetSlot: 8, Damage: 30, Weapon: "m4a1"},
		{Tick: 110, ActorSlot: 1, TargetSlot: 7, Damage: 20, Weapon: "hegrenade"},
		{Tick: 130, ActorSlot: 1, TargetSlot: 7, Damage: 27, Weapon: "ak47"},
	}
	if len(hits) != len(want) {
		t.Fatalf("got %d hits, want %d: %+v", len(hits), len(want), hits)
	}
	for i := range want {
		if hits[i] != want[i] {
			t.Fatalf("hit %d = %+v, want %+v", i, hits[i], want[i])
		}
	}
}

func TestRemapHitsUsesFinalSlots(t *testing.T) {
	got := remapHits([]tacticalplan.Hit{{Tick: 5, ActorSlot: 0, TargetSlot: 3, Damage: 10}}, map[uint8]uint8{0: 4, 3: 1})
	if len(got) != 1 || got[0].ActorSlot != 4 || got[0].TargetSlot != 1 {
		t.Fatalf("remapped = %+v", got)
	}
	if remapHits(nil, nil) != nil {
		t.Fatal("no hits must stay nil so the document omits the field")
	}
}
