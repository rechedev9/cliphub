package tacticalplan

import (
	"reflect"
	"slices"
	"testing"
)

func pbTactic(t *testing.T, pb Playbook, key string) PlaybookTactic {
	t.Helper()
	for _, tactic := range pb.Tactics {
		if tactic.Key == key {
			return tactic
		}
	}
	t.Fatalf("tactic %s missing from %+v", key, pb.Tactics)
	return PlaybookTactic{}
}

// pbOpenings gives round 2 an opening the team won in Middle and round 3 one
// it lost, its player dying on B.
func pbOpenings(m FramedMatch) FramedMatch {
	rounds := slices.Clone(m.Document.Rounds)
	rounds[1].Class.OpeningSide, rounds[1].Class.OpeningTick = SideT, pbTick(2, 10)
	rounds[1].Events = append(rounds[1].Events, Event{Tick: pbTick(2, 10), Kind: EventKill, ActorSlot: slotPtr(2), TargetSlot: slotPtr(0),
		Opening: true, Place: "Middle"})
	rounds[2].Class.OpeningSide, rounds[2].Class.OpeningTick = SideCT, pbTick(3, 12)
	rounds[2].Events = append(rounds[2].Events, Event{Tick: pbTick(3, 12), Kind: EventKill, ActorSlot: slotPtr(0), TargetSlot: slotPtr(3),
		Opening: true, TargetPos: [3]float64{pbB.x, pbB.y, 0}})
	m.Document.Rounds = rounds
	return m
}

func TestPlaybookTacticsListWhatEachRoundShows(t *testing.T) {
	pb := BuildPlaybook([]FramedMatch{pbOpenings(pbMatch(t))}, PlaybookOptions{Filter: ScoutFilter{Side: SideT}})
	byRound := map[int][]string{}
	for _, r := range pb.Rounds {
		byRound[r.Round] = r.Tactics
	}
	for round, want := range map[int][]string{
		2: {"category:execute", "site:A", "buy:full", "opponent_buy:full", "opening:Middle", "opening_result:won", "zone:Middle"},
		3: {"opening:BombsiteB", "opening_result:lost"},
		7: {"category:fake", "utility:smoke@BombsiteA"},
	} {
		for _, key := range want {
			if !slices.Contains(byRound[round], key) {
				t.Errorf("round %d tactics = %v, missing %s", round, byRound[round], key)
			}
		}
	}
	if smoke := pbTactic(t, pb, "utility:smoke@BombsiteA"); smoke.Kind != TacticUtility || smoke.Grenade != "smoke" ||
		smoke.Value != "BombsiteA" || smoke.Label != "Bombsite A" || smoke.Rounds != 1 {
		t.Errorf("smoke tactic = %+v, want one round counted once for its two smokes", smoke)
	}
	if got := refRounds(pbTactic(t, pb, "buy:full").Refs); !reflect.DeepEqual(got, []int{1, 2, 3, 4, 6, 7}) {
		t.Errorf("full buy rounds = %v", got)
	}
	if pb.Tactics[0].Kind != TacticCategory || pb.Tactics[len(pb.Tactics)-1].Kind != TacticUtility {
		t.Errorf("tactics are not in kind order: first %s, last %s", pb.Tactics[0].Key, pb.Tactics[len(pb.Tactics)-1].Key)
	}
}

func TestPlaybookComboNarrowsEveryCount(t *testing.T) {
	m := pbOpenings(pbMatch(t))
	pb := BuildPlaybook([]FramedMatch{m}, PlaybookOptions{
		Filter: ScoutFilter{Side: SideT},
		Combo:  PlaybookCombo{With: []string{"buy:full"}, Without: []string{"site:A"}},
	})
	if pb.RoundCount != 4 || pb.WinRate.Count != 3 || !reflect.DeepEqual(pb.Combo.With, []string{"buy:full"}) {
		t.Fatalf("combo kept %d rounds, %d won, combo %+v; want rounds 1, 3, 4 and 7", pb.RoundCount, pb.WinRate.Count, pb.Combo)
	}
	if fake := pbCategory(t, pb, PlaybookFake); fake.Rounds != 2 || fake.Share.Total != 4 {
		t.Errorf("fake share = %+v, want over the kept rounds", fake.PlaybookGroup)
	}
	for _, tactic := range pb.Tactics {
		if tactic.Key == "site:A" || tactic.Key == "category:save" {
			t.Errorf("tactic %s counted after the combo excluded its rounds", tactic.Key)
		}
	}
	none := BuildPlaybook([]FramedMatch{m}, PlaybookOptions{Filter: ScoutFilter{Side: SideT}, Combo: PlaybookCombo{With: []string{"zone:Nowhere"}}})
	if none.RoundCount != 0 || len(none.Tactics) != 0 || len(none.Rounds) != 0 {
		t.Errorf("an unseen tactic kept %d rounds", none.RoundCount)
	}
}

func TestPlaybookAWPSpotReadsTheCarrierAtTheStart(t *testing.T) {
	awp := &Gear{ActiveWeapon: EquipmentByName("knife"), Inventory: []InventoryItem{{Item: EquipmentByName("awp"), Count: 1}}}
	rifle := &Gear{ActiveWeapon: EquipmentByName("ak47")}
	doc := Document{Demo: Demo{Tickrate: pbRate}, Geometry: pbGeometry()}
	read := playbookRead{doc: doc, round: Round{Number: 1, TickFreezeEnd: pbFreezeEnd(1)}, team: map[uint8]bool{2: true, 3: true}}
	frames := []Frame{
		{Tick: pbTick(1, 5), Samples: []Sample{{Slot: 2, X: pbTSpawn.x, Y: pbTSpawn.y, Flags: FlagAlive, Gear: awp}}},
		{Tick: pbTick(1, AWPStartSeconds), Samples: []Sample{
			{Slot: 2, X: pbMid.x, Y: pbMid.y, Flags: FlagAlive, Gear: awp},
			{Slot: 3, X: pbA.x, Y: pbA.y, Flags: FlagAlive, Gear: rifle},
			{Slot: 0, X: pbB.x, Y: pbB.y, Flags: FlagAlive, Gear: awp},
		}},
		{Tick: pbTick(1, 20), Samples: []Sample{{Slot: 2, X: pbB.x, Y: pbB.y, Flags: FlagAlive, Gear: awp}}},
	}
	if got := read.awpSpots(frames); !reflect.DeepEqual(got, []string{"Middle"}) {
		t.Errorf("AWP spots = %v, want the team's carrier in Middle at %d s", got, AWPStartSeconds)
	}
	frames[1].Samples[0].Gear = nil
	if got := read.awpSpots(frames); len(got) != 0 {
		t.Errorf("a blob without gear read AWP spots %v", got)
	}
}

func TestCheckTacticKeys(t *testing.T) {
	if err := CheckTacticKeys("with", []string{"zone:Middle", "utility:smoke@BombsiteA", "opening_result:won"}); err != nil {
		t.Fatal(err)
	}
	for _, bad := range [][]string{{"zone"}, {"weapon:awp"}, {"zone:"}, {"zone:a b"}, make([]string, MaxTacticFilter+1)} {
		if CheckTacticKeys("with", bad) == nil {
			t.Errorf("CheckTacticKeys(%q) accepted it", bad)
		}
	}
}
