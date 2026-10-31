package tacticalplan

import "testing"

func TestDropRepeatedUtilityKeepsTheFirstOfARepeatedDetonation(t *testing.T) {
	one, two := uint8(1), uint8(2)
	effect := &UtilityEffect{EnemyDamage: 40}
	events := []Event{
		{Kind: EventFlash, Tick: 100, ActorSlot: &one, Effect: effect},
		{Kind: EventFlash, Tick: 100, ActorSlot: &one},
		{Kind: EventFlash, Tick: 100, ActorSlot: &two},
		{Kind: EventHE, Tick: 100, ActorSlot: &one},
		{Kind: EventFlash, Tick: 101, ActorSlot: &one},
		{Kind: EventKill, Tick: 100, ActorSlot: &one},
		{Kind: EventKill, Tick: 100, ActorSlot: &one},
		{Kind: EventSmoke, Tick: 100},
		{Kind: EventSmoke, Tick: 100},
	}
	got := DropRepeatedUtility(events)
	if len(got) != len(events)-1 {
		t.Fatalf("kept %d events, want %d: %+v", len(got), len(events)-1, got)
	}
	if got[0].Effect != effect || got[1].ActorSlot != &two {
		t.Fatalf("the first detonation must keep its effect and the order must hold: %+v", got[:2])
	}
	if len(events) != 9 || events[1].Kind != EventFlash || events[1].Effect != nil {
		t.Fatal("the input slice must not be modified")
	}

	doc := Document{Rounds: []Round{{Number: 1, Events: events}}}
	doc.DropRepeatedUtility()
	if len(doc.Rounds[0].Events) != len(got) {
		t.Fatalf("document round keeps %d events, want %d", len(doc.Rounds[0].Events), len(got))
	}
}
