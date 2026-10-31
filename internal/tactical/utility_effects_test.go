package tactical

import (
	"testing"

	"github.com/rechedev9/cliphub/internal/tacticalplan"
)

func slotPtr(v uint8) *uint8 { return &v }

func TestAttachUtilityEffects(t *testing.T) {
	evs := []tacticalplan.Event{
		{Tick: 1000, Kind: tacticalplan.EventFlash, ActorSlot: slotPtr(1)},
		{Tick: 1000, Kind: tacticalplan.EventFlash, ActorSlot: slotPtr(1)}, // duplicated detonation
		{Tick: 1500, Kind: tacticalplan.EventFlash, ActorSlot: slotPtr(1)},
		{Tick: 2000, Kind: tacticalplan.EventHE, ActorSlot: slotPtr(2)},
		{Tick: 3000, Kind: tacticalplan.EventMolotov, ActorSlot: slotPtr(2)},
		{Tick: 3300, Kind: tacticalplan.EventMolotov, ActorSlot: slotPtr(2)},
	}
	hits := []utilityHit{
		{kind: tacticalplan.EventFlash, tick: 1001, attacker: 1, victim: 7, enemy: true, seconds: 2.5},
		{kind: tacticalplan.EventFlash, tick: 1490, attacker: 1, victim: 3, seconds: 1.1},
		{kind: tacticalplan.EventFlash, tick: 1000, attacker: 4, victim: 7, enemy: true, seconds: 3}, // no flash by slot 4
		{kind: tacticalplan.EventHE, tick: 2000, attacker: 2, victim: 8, enemy: true, healthLost: 40},
		{kind: tacticalplan.EventHE, tick: 2001, attacker: 2, victim: 3, healthLost: 12},
		{kind: tacticalplan.EventHE, tick: 2200, attacker: 2, victim: 8, enemy: true, healthLost: 5}, // too late
		{kind: tacticalplan.EventMolotov, tick: 3200, attacker: 2, victim: 9, enemy: true, healthLost: 8},
		{kind: tacticalplan.EventMolotov, tick: 3400, attacker: 2, victim: 9, enemy: true, healthLost: 6},
		{kind: tacticalplan.EventMolotov, tick: 2900, attacker: 2, victim: 9, enemy: true, healthLost: 6}, // before any fire
	}
	attachUtilityEffects(evs, hits, 64)

	if got := evs[0].Effect; got == nil || len(got.Blinded) != 1 || got.Blinded[0].Slot != 7 || got.Blinded[0].Seconds != 2.5 {
		t.Fatalf("first flash effect = %+v", got)
	}
	if evs[1].Effect != nil {
		t.Fatalf("duplicate flash got an effect: %+v", evs[1].Effect)
	}
	if got := evs[2].Effect; got == nil || len(got.Blinded) != 1 || got.Blinded[0].Enemy {
		t.Fatalf("team flash effect = %+v", got)
	}
	if got := evs[3].Effect; got == nil || got.EnemyDamage != 40 || got.TeamDamage != 12 {
		t.Fatalf("HE effect = %+v", got)
	}
	if got := evs[4].Effect; got == nil || got.EnemyDamage != 8 {
		t.Fatalf("first molotov effect = %+v", got)
	}
	if got := evs[5].Effect; got == nil || got.EnemyDamage != 6 {
		t.Fatalf("second molotov effect = %+v", got)
	}
}

func TestRemapEffect(t *testing.T) {
	effect := &tacticalplan.UtilityEffect{Blinded: []tacticalplan.BlindHit{{Slot: 3, Seconds: 1, Enemy: true}}, EnemyDamage: 4}
	got := remapEffect(effect, map[uint8]uint8{3: 0})
	if got.Blinded[0].Slot != 0 || effect.Blinded[0].Slot != 3 || got.EnemyDamage != 4 {
		t.Fatalf("remap = %+v (source %+v)", got, effect)
	}
	if remapEffect(nil, nil) != nil {
		t.Fatal("nil effect must stay nil")
	}
}
