package tacticalplan

import (
	"net/url"
	"reflect"
	"testing"
)

// testDocument builds a small but complete match: two teams, a side swap after
// round 2, and one round of each interesting shape.
func testDocument() Document {
	d := NewDocument()
	d.Demo = Demo{Map: "de_mirage", Tickrate: 64, MaxRounds: 4}
	d.Teams = []Team{
		{Key: "ct-start", Name: "Alpha", StartSide: SideCT, Slots: []uint8{0, 1}},
		{Key: "t-start", Name: "Beta", StartSide: SideT, Slots: []uint8{2, 3}},
	}
	d.Players = []Player{
		{Slot: 0, SteamID64: "1", Name: "a1", TeamKey: "ct-start", StartSide: SideCT},
		{Slot: 1, SteamID64: "2", Name: "a2", TeamKey: "ct-start", StartSide: SideCT},
		{Slot: 2, SteamID64: "3", Name: "b1", TeamKey: "t-start", StartSide: SideT},
		{Slot: 3, SteamID64: "4", Name: "b2", TeamKey: "t-start", StartSide: SideT},
	}
	d.Rounds = []Round{
		{
			Number: 1, Half: 1, TickStart: 0, TickFreezeEnd: 640, TickEnd: 4480,
			Winner: SideT, EndReason: "bomb_exploded",
			Economy: Economy{CTBuy: BuyPistol, TBuy: BuyPistol, SampleTick: 1088},
			Bomb:    &Bomb{PlantTick: 3000, Site: SiteA},
			Class: Class{
				TSide: TFast, CTSide: CTRetake, Site: SiteA,
				OpeningSide: SideT, OpeningTick: 1500, FirstContactTick: 1500,
				Tags: []string{TagPistol, TagPostPlant}, Reasons: []string{},
			},
			Players: []PlayerRound{
				{Slot: 0, Side: SideCT, Deaths: 1, Damage: 40, OpeningDeath: true},
				{Slot: 1, Side: SideCT, Deaths: 1, Damage: 10},
				{Slot: 2, Side: SideT, Kills: 2, Damage: 190, Survived: true, OpeningKill: true},
				{Slot: 3, Side: SideT, Damage: 30, Survived: true},
			},
		},
		{
			Number: 2, Half: 1, TickStart: 5000, TickFreezeEnd: 5640, TickEnd: 9000,
			Winner: SideCT, EndReason: "ct_eliminated_t",
			Economy: Economy{CTBuy: BuyFull, TBuy: BuyEco, CTEquipValue: 22500, TEquipValue: 3000},
			Class: Class{
				TSide: TEcoRush, CTSide: CTHold, Site: SiteB,
				OpeningSide: SideCT, OpeningTick: 6000, FirstContactTick: 6000,
				Tags: []string{TagAntiEco}, Reasons: []string{},
			},
			Players: []PlayerRound{
				{Slot: 0, Side: SideCT, Kills: 2, Damage: 200, Survived: true, OpeningKill: true},
				{Slot: 1, Side: SideCT, Kills: 2, Damage: 180, Survived: true},
				{Slot: 2, Side: SideT, Deaths: 1, Damage: 20, OpeningDeath: true, Traded: false},
				{Slot: 3, Side: SideT, Deaths: 1},
			},
		},
		{
			// Second half: Alpha is now T, Beta is now CT.
			Number: 3, Half: 2, TickStart: 10000, TickFreezeEnd: 10640, TickEnd: 15000,
			Winner: SideT, EndReason: "t_eliminated_ct",
			Economy: Economy{CTBuy: BuySemi, TBuy: BuyFull, CTEquipValue: 12000, TEquipValue: 21000},
			Class: Class{
				TSide: TExecute, CTSide: CTStack, Site: SiteA,
				OpeningSide: SideT, OpeningTick: 12000, FirstContactTick: 12000, OpeningTraded: true,
				Tags: []string{TagOpeningTraded}, Reasons: []string{},
			},
			Players: []PlayerRound{
				{Slot: 0, Side: SideT, Kills: 2, Damage: 210, Survived: true, OpeningKill: true},
				{Slot: 1, Side: SideT, Kills: 1, Damage: 100},
				{Slot: 2, Side: SideCT, Deaths: 1, Damage: 50},
				{Slot: 3, Side: SideCT, Deaths: 1, Damage: 60, OpeningDeath: true},
			},
		},
		{
			Number: 4, Half: 2, TickStart: 16000, TickFreezeEnd: 16640, TickEnd: 20000,
			Winner: SideCT, EndReason: "time_expired",
			Economy: Economy{CTBuy: BuyFull, TBuy: BuyEco, CTEquipValue: 23000, TEquipValue: 4000},
			Class: Class{
				TSide: TSave, CTSide: CTHold, Site: SiteNone,
				Tags: []string{TagFullSave, TagTimeout}, Reasons: []string{},
			},
			Players: []PlayerRound{
				{Slot: 0, Side: SideT, Survived: true},
				{Slot: 1, Side: SideT, Survived: true},
				{Slot: 2, Side: SideCT, Kills: 0, Survived: true},
				{Slot: 3, Side: SideCT, Survived: true},
			},
		},
	}
	return d
}

func roundNumbers(rounds []Round) []int {
	out := make([]int, 0, len(rounds))
	for _, r := range rounds {
		out = append(out, r.Number)
	}
	return out
}

func TestFilterEmptySelectsEverything(t *testing.T) {
	d := testDocument()
	var f Filter
	if !f.Empty() {
		t.Fatal("zero filter must report empty")
	}
	if got := roundNumbers(f.Apply(d)); !reflect.DeepEqual(got, []int{1, 2, 3, 4}) {
		t.Fatalf("selected %v, want every round", got)
	}
}

func TestFilterBySide(t *testing.T) {
	d := testDocument()
	f := Filter{Side: SideT, Buys: []BuyType{BuyEco}}
	if got := roundNumbers(f.Apply(d)); !reflect.DeepEqual(got, []int{2, 4}) {
		t.Fatalf("T eco rounds = %v, want [2 4]", got)
	}
}

// A team filter must follow the team across the side swap: Alpha starts CT, so
// its full buys are round 2 (as CT) and round 3 (as T).
func TestFilterByTeamFollowsSideSwap(t *testing.T) {
	d := testDocument()
	f := Filter{TeamKey: "ct-start", Buys: []BuyType{BuyFull}}
	if got := roundNumbers(f.Apply(d)); !reflect.DeepEqual(got, []int{2, 3}) {
		t.Fatalf("Alpha full buys = %v, want [2 3]", got)
	}

	wins := Filter{TeamKey: "ct-start", Outcome: OutcomeWin}
	if got := roundNumbers(wins.Apply(d)); !reflect.DeepEqual(got, []int{2, 3}) {
		t.Fatalf("Alpha wins = %v, want [2 3]", got)
	}
	losses := Filter{TeamKey: "ct-start", Outcome: OutcomeLoss}
	if got := roundNumbers(losses.Apply(d)); !reflect.DeepEqual(got, []int{1, 4}) {
		t.Fatalf("Alpha losses = %v, want [1 4]", got)
	}
}

func TestFilterByOpponentBuyIsTheAntiEcoQuestion(t *testing.T) {
	d := testDocument()
	f := Filter{Side: SideCT, Buys: []BuyType{BuyFull}, OpponentBuys: []BuyType{BuyEco}}
	if got := roundNumbers(f.Apply(d)); !reflect.DeepEqual(got, []int{2, 4}) {
		t.Fatalf("CT full vs T eco = %v, want [2 4]", got)
	}
}

func TestFilterCombinationsAndTags(t *testing.T) {
	d := testDocument()
	tests := []struct {
		name   string
		filter Filter
		want   []int
	}{
		{"site", Filter{Sites: []Site{SiteA}}, []int{1, 3}},
		{"t pattern", Filter{TPatterns: []TPattern{TExecute, TFast}}, []int{1, 3}},
		{"ct pattern", Filter{CTPatterns: []CTPattern{CTHold}}, []int{2, 4}},
		{"tag", Filter{Tags: []string{TagPostPlant}}, []int{1}},
		{"two tags are an AND", Filter{Tags: []string{TagFullSave, TagTimeout}}, []int{4}},
		{"round range", Filter{RoundFrom: 2, RoundTo: 3}, []int{2, 3}},
		{"phase regulation", Filter{Phase: PhaseRegulation}, []int{1, 2, 3, 4}},
		{"slot on a side", Filter{Side: SideT, Slots: []uint8{2}}, []int{1, 2}},
		{"site and outcome", Filter{Side: SideT, Sites: []Site{SiteA}, Outcome: OutcomeWin}, []int{1, 3}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := roundNumbers(tt.filter.Apply(d))
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("selected %v, want %v", got, tt.want)
			}
		})
	}
}

// Asking for a win with no perspective cannot be answered, and answering
// "everything" would quietly report the wrong number.
func TestOutcomeWithoutPerspectiveSelectsNothing(t *testing.T) {
	d := testDocument()
	f := Filter{Outcome: OutcomeWin}
	if got := f.Apply(d); len(got) != 0 {
		t.Fatalf("selected %v rounds without a perspective", roundNumbers(got))
	}
}

func TestFilterFromValues(t *testing.T) {
	values := url.Values{
		"side":         {"t"},
		"buy":          {"eco,force"},
		"opponent_buy": {"full"},
		"site":         {"a", "b"},
		"outcome":      {"win"},
		"t_pattern":    {"execute"},
		"ct_pattern":   {"retake"},
		"tag":          {"postplant"},
		"slot":         {"3", "1"},
		"round_from":   {"2"},
		"round_to":     {"12"},
		"phase":        {"overtime"},
		"team":         {"ct-start"},
	}
	got, err := FilterFromValues(values)
	if err != nil {
		t.Fatalf("FilterFromValues: %v", err)
	}
	want := Filter{
		TeamKey:      "ct-start",
		Side:         SideT,
		Buys:         []BuyType{BuyEco, BuyForce},
		OpponentBuys: []BuyType{BuyFull},
		Sites:        []Site{SiteA, SiteB},
		Outcome:      OutcomeWin,
		TPatterns:    []TPattern{TExecute},
		CTPatterns:   []CTPattern{CTRetake},
		Tags:         []string{"postplant"},
		Slots:        []uint8{1, 3},
		RoundFrom:    2,
		RoundTo:      12,
		Phase:        PhaseOvertime,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parsed %+v, want %+v", got, want)
	}
}

func TestFilterFromValuesRejectsUnknownVocabulary(t *testing.T) {
	tests := []struct {
		name   string
		values url.Values
	}{
		{"side", url.Values{"side": {"ct2"}}},
		{"buy", url.Values{"buy": {"halfbuy"}}},
		{"site", url.Values{"site": {"c"}}},
		{"outcome", url.Values{"outcome": {"draw"}}},
		{"t pattern", url.Values{"t_pattern": {"rush"}}},
		{"ct pattern", url.Values{"ct_pattern": {"passive"}}},
		{"slot", url.Values{"slot": {"99"}}},
		{"slot not a number", url.Values{"slot": {"x"}}},
		{"round bound", url.Values{"round_from": {"0"}}},
		{"phase", url.Values{"phase": {"warmup"}}},
		{"inverted range", url.Values{"round_from": {"9"}, "round_to": {"3"}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := FilterFromValues(tt.values); err == nil {
				t.Fatal("want an error")
			}
		})
	}
}

func TestFilterFromValuesEmpty(t *testing.T) {
	f, err := FilterFromValues(url.Values{})
	if err != nil {
		t.Fatalf("FilterFromValues: %v", err)
	}
	if !f.Empty() {
		t.Fatalf("no query parameters must yield an empty filter, got %+v", f)
	}
}

func TestTeamSideFollowsHalf(t *testing.T) {
	d := testDocument()
	if got := d.TeamSide("ct-start", d.Rounds[0]); got != SideCT {
		t.Fatalf("first half side = %q, want CT", got)
	}
	if got := d.TeamSide("ct-start", d.Rounds[2]); got != SideT {
		t.Fatalf("second half side = %q, want T", got)
	}
	if got := d.TeamSide("nobody", d.Rounds[0]); got != "" {
		t.Fatalf("unknown team must have no side, got %q", got)
	}
}
