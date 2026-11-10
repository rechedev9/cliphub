package tacticalplan

import (
	"net/url"
	"reflect"
	"testing"

	"github.com/google/uuid"
)

func slot(n uint8) *uint8 { return &n }

// weaponDocument adds to testDocument a Scout kill by Alpha in round 3, a lone
// AK-47 buy by Beta in round 2 and a lone AWP by Alpha in round 4.
func weaponDocument() Document {
	d := testDocument()
	d.Rounds[2].Events = []Event{{Tick: 12000, Kind: EventKill, ActorSlot: slot(0), TargetSlot: slot(2), Weapon: scoutWeapon}}
	d.Rounds[1].Hits = []Hit{{Tick: 6000, ActorSlot: 2, TargetSlot: 0, Damage: 27, Weapon: akWeapon}}
	d.Rounds[1].Players[2].EquipValue = 3700
	d.Rounds[1].Players[3].EquipValue = 500
	d.Rounds[3].Players[0].EquipValue, d.Rounds[3].Players[0].AWP = 5750, true
	d.Rounds[3].Players[1].EquipValue = 200
	return d
}

func TestFilterRoundCriteria(t *testing.T) {
	cases := []struct {
		name   string
		filter Filter
		want   []int
	}{
		{"second round of each half", Filter{SecondRound: true}, []int{2, 4}},
		{"won by elimination", Filter{EndReasons: []EndReason{EndElimination}}, []int{2, 3}},
		{"bomb or time", Filter{EndReasons: []EndReason{EndExploded, EndTime}}, []int{1, 4}},
		{"T side spent 1000 to 2000 a player", Filter{Side: SideT, Equip: EquipRange{From: 1000, To: 2000}}, []int{2, 4}},
		{"Alpha's rival spent 5000 or more a player", Filter{TeamKey: "ct-start", OpponentEquip: EquipRange{From: 5000}}, []int{3, 4}},
		{"either side at 10000 or more", Filter{Equip: EquipRange{From: 10000}}, []int{2, 3, 4}},
		{"Alpha used a Scout", Filter{TeamKey: "ct-start", Weapon: WeaponScout}, []int{3}},
		{"Alpha used no Scout", Filter{TeamKey: "ct-start", Weapon: WeaponNoScout}, []int{1, 2, 4}},
		{"Beta's hero AK", Filter{TeamKey: "t-start", Weapon: WeaponHeroAK}, []int{2}},
		{"Alpha's hero AWP", Filter{TeamKey: "ct-start", Weapon: WeaponHeroAWP}, []int{4}},
		{"rival of Beta had a hero AWP", Filter{TeamKey: "t-start", OpponentWeapon: WeaponHeroAWP}, []int{4}},
		{"only listed numbers", Filter{Numbers: []int{1, 3}}, []int{1, 3}},
		{"an empty list keeps none", Filter{Numbers: []int{}}, []int{}},
	}
	d := weaponDocument()
	for _, tc := range cases {
		if got := roundNumbers(tc.filter.Apply(d)); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%s: rounds %v, want %v", tc.name, got, tc.want)
		}
		if tc.filter.Empty() {
			t.Errorf("%s: filter reports empty", tc.name)
		}
	}
}

func TestHeroNeedsCheapTeammates(t *testing.T) {
	d := weaponDocument()
	d.Rounds[3].Players[1].EquipValue = HeroTeammateMaxEquip
	if got := roundNumbers(Filter{TeamKey: "ct-start", Weapon: WeaponHeroAWP}.Apply(d)); len(got) != 0 {
		t.Fatalf("a teammate at the eco line breaks the hero round, got %v", got)
	}
}

func TestHeroAWPEstimatedFromKills(t *testing.T) {
	d := weaponDocument()
	d.AnalysisVersion = AWPFlagVersion - 1
	d.Rounds[3].Players[0].AWP = false
	if got := roundNumbers(Filter{TeamKey: "ct-start", Weapon: WeaponHeroAWP}.Apply(d)); len(got) != 0 {
		t.Fatalf("without an AWP kill an old analysis has no hero AWP, got %v", got)
	}
	d.Rounds[3].Hits = []Hit{{Tick: 17000, ActorSlot: 0, TargetSlot: 2, Damage: 100, Weapon: awpWeapon}}
	if got := roundNumbers(Filter{TeamKey: "ct-start", Weapon: WeaponHeroAWP}.Apply(d)); !reflect.DeepEqual(got, []int{4}) {
		t.Fatalf("an AWP hit marks the hero in an old analysis, got %v", got)
	}
	// The views flag such matches as awp_estimated, so a hero AWP must count as an AWP read.
	for _, f := range []Filter{{Weapon: WeaponHeroAWP}, {OpponentWeapon: WeaponHeroAWP}} {
		if !f.ReadsAWP() {
			t.Fatalf("%+v must read the AWP", f)
		}
	}
	if (Filter{Weapon: WeaponHeroAK}).ReadsAWP() {
		t.Fatal("a hero AK does not read the AWP")
	}
}

func TestSecondRoundSkipsOvertime(t *testing.T) {
	d := testDocument()
	d.Rounds = append(d.Rounds,
		Round{Number: 5, Half: 3, Overtime: 1},
		Round{Number: 6, Half: 3, Overtime: 1},
	)
	if got := roundNumbers(Filter{SecondRound: true}.Apply(d)); !reflect.DeepEqual(got, []int{2, 4}) {
		t.Fatalf("second rounds = %v, want [2 4]", got)
	}
}

func TestFilterFromValuesRoundCriteria(t *testing.T) {
	q, _ := url.ParseQuery("second_round=true&end_reason=elimination,time_expired&weapon=hero_awp&opponent_weapon=SCOUT" +
		"&equip_from=1000&equip_to=3250&opponent_equip_from=4000")
	f, err := FilterFromValues(q)
	if err != nil {
		t.Fatal(err)
	}
	want := Filter{SecondRound: true, EndReasons: []EndReason{EndElimination, EndTime}, Weapon: WeaponHeroAWP, OpponentWeapon: WeaponScout,
		Equip: EquipRange{From: 1000, To: 3250}, OpponentEquip: EquipRange{From: 4000}}
	if !reflect.DeepEqual(f, want) {
		t.Fatalf("parsed %+v, want %+v", f, want)
	}
	for _, bad := range []string{"weapon=knife", "end_reason=surrender", "equip_to=0", "equip_from=4000&equip_to=1000",
		"opponent_equip_from=-1", "equip_to=25000", "second_round=maybe"} {
		v, _ := url.ParseQuery(bad)
		if _, err := FilterFromValues(v); err == nil {
			t.Errorf("%s: parsed without an error", bad)
		}
	}
}

func TestScoutFilterPlayRounds(t *testing.T) {
	inPlay, outside := uuid.New(), uuid.New()
	d := testDocument()
	matches := []ScoutMatch{{MatchID: inPlay, TeamKey: "ct-start", Document: d}, {MatchID: outside, TeamKey: "ct-start", Document: d}}
	f := ScoutFilter{PlayRounds: map[uuid.UUID][]int{inPlay: {2, 3}}}
	kept := f.Select(matches)
	if len(kept) != 1 || kept[0].MatchID != inPlay {
		t.Fatalf("kept %d matches, want only the play's", len(kept))
	}
	if got := roundNumbers(f.rounds(kept[0])); !reflect.DeepEqual(got, []int{2, 3}) {
		t.Fatalf("play rounds = %v, want [2 3]", got)
	}
	if got := roundNumbers(f.RoundFilter(matches[1]).Apply(d)); len(got) != 0 {
		t.Fatalf("a match outside the play keeps %v", got)
	}
}
