package tactical

import (
	"math"
	"testing"

	"github.com/rechedev9/fragforge/internal/tacticalplan"
)

const testTickrate = 64.0

// A synthetic map with two sites far enough apart that no position is
// ambiguous. Classification is tested against constructed rounds rather than a
// demo, so a rule change shows up as a failing expectation instead of a
// silently different label on someone's match.
func testSites() siteMap {
	return siteMap{
		centers: map[tacticalplan.Site][2]float64{
			tacticalplan.SiteA:   {0, 0},
			tacticalplan.SiteB:   {5000, 0},
			tacticalplan.SiteMid: {2500, 2500},
		},
		radius: siteRadius,
	}
}

func tick(seconds float64) int { return 640 + int(seconds*testTickrate) }

// attackers places n terrorists around a site centre at the given bearings.
func frameAt(atTick int, samples ...tacticalplan.Sample) tacticalplan.Frame {
	return tacticalplan.Frame{Tick: atTick, Samples: samples}
}

func alive(slot uint8, x, y float64) tacticalplan.Sample {
	return tacticalplan.Sample{Slot: slot, X: x, Y: y, Health: 100, Flags: tacticalplan.FlagAlive}
}

func utilityEvent(atTick int, side tacticalplan.Side) tacticalplan.Event {
	return tacticalplan.Event{Tick: atTick, Kind: tacticalplan.EventSmoke, Side: side}
}

// baseRound is a 5v5 round with no bomb, no kills and no positions; each test
// adds only what its rule needs.
func baseRound() tacticalplan.Round {
	r := tacticalplan.Round{
		Number: 5, Half: 1, TickStart: 0, TickFreezeEnd: 640, TickEnd: tick(100),
		Economy: tacticalplan.Economy{
			CTBuy: tacticalplan.BuyFull, TBuy: tacticalplan.BuyFull,
			CTEquipValue: 22500, TEquipValue: 21000, SampleTick: 1088,
		},
	}
	for slot := uint8(0); slot < 5; slot++ {
		r.Players = append(r.Players, tacticalplan.PlayerRound{Slot: slot, Side: tacticalplan.SideCT})
	}
	for slot := uint8(5); slot < 10; slot++ {
		r.Players = append(r.Players, tacticalplan.PlayerRound{Slot: slot, Side: tacticalplan.SideT})
	}
	return r
}

func TestClassifyEconomyPistolRounds(t *testing.T) {
	round := baseRound()
	round.Number = 1
	round.Economy.CTEquipValue = 3000
	round.Economy.TEquipValue = 3000

	ct, tt := classifyEconomy(round, nil)
	if ct != tacticalplan.BuyPistol || tt != tacticalplan.BuyPistol {
		t.Fatalf("first round of a half = %s/%s, want two pistol buys", ct, tt)
	}

	// The first round of an overtime half starts with $10,000, so calling it a
	// pistol round would misreport every overtime economy.
	otRound := round
	otRound.Overtime = 1
	otRound.Half = 3
	otRound.Economy.CTEquipValue = 22500
	otRound.Economy.TEquipValue = 21000
	ct, tt = classifyEconomy(otRound, nil)
	if ct != tacticalplan.BuyFull || tt != tacticalplan.BuyFull {
		t.Fatalf("first overtime round = %s/%s, want two full buys", ct, tt)
	}
}

func TestClassifyEconomyThresholds(t *testing.T) {
	previous := baseRound()
	previous.Number = 4
	previous.Half = 1
	previous.Winner = tacticalplan.SideCT

	tests := []struct {
		name    string
		ctValue int
		tValue  int
		ctMoney int
		tMoney  int
		wantCT  tacticalplan.BuyType
		wantT   tacticalplan.BuyType
	}{
		{"both eco at the ceiling", 5000, 5000, 20000, 20000, tacticalplan.BuyEco, tacticalplan.BuyEco},
		{"CT full needs more than T full", 22500, 20000, 9000, 9000, tacticalplan.BuyFull, tacticalplan.BuyFull},
		{"CT just under a full buy", 22499, 19999, 9000, 9000, tacticalplan.BuySemi, tacticalplan.BuySemi},
		{"broke after a loss is a force", 22499, 19999, 1000, 1000, tacticalplan.BuySemi, tacticalplan.BuyForce},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			round := baseRound()
			round.Economy = tacticalplan.Economy{
				CTEquipValue: tt.ctValue, TEquipValue: tt.tValue,
				CTMoney: tt.ctMoney, TMoney: tt.tMoney, SampleTick: 1088,
			}
			gotCT, gotT := classifyEconomy(round, &previous)
			if gotCT != tt.wantCT || gotT != tt.wantT {
				t.Fatalf("buys = %s/%s, want %s/%s", gotCT, gotT, tt.wantCT, tt.wantT)
			}
		})
	}
}

// A force buy is a half buy made while broke after a loss, not a value band:
// the same equipment value after a WIN is a planned half buy.
func TestForceBuyRequiresLosingThePreviousRound(t *testing.T) {
	round := baseRound()
	round.Economy = tacticalplan.Economy{
		CTEquipValue: 12000, TEquipValue: 12000,
		CTMoney: 500, TMoney: 500, SampleTick: 1088,
	}
	won := baseRound()
	won.Number = 4
	won.Winner = tacticalplan.SideT

	_, gotT := classifyEconomy(round, &won)
	if gotT != tacticalplan.BuySemi {
		t.Fatalf("T buy after a win = %s, want %s", gotT, tacticalplan.BuySemi)
	}

	lost := won
	lost.Winner = tacticalplan.SideCT
	_, gotT = classifyEconomy(round, &lost)
	if gotT != tacticalplan.BuyForce {
		t.Fatalf("T buy after a loss while broke = %s, want %s", gotT, tacticalplan.BuyForce)
	}
}

func TestClassifyRoundExecute(t *testing.T) {
	round := baseRound()
	round.Winner = tacticalplan.SideT
	acc := &roundAcc{
		frames: []tacticalplan.Frame{
			frameAt(tick(35), alive(5, 100, 100), alive(6, 150, 120), alive(7, 120, 80)),
		},
		events: []tacticalplan.Event{
			utilityEvent(tick(30), tacticalplan.SideT),
			utilityEvent(tick(31), tacticalplan.SideT),
			utilityEvent(tick(32), tacticalplan.SideT),
		},
	}
	class := classifyRound(classifyInput{round: round, acc: acc, tickrate: testTickrate, sites: testSites()})

	if class.TSide != tacticalplan.TExecute {
		t.Fatalf("T pattern = %s, want %s (reasons: %v)", class.TSide, tacticalplan.TExecute, class.Reasons)
	}
	if class.Site != tacticalplan.SiteA {
		t.Fatalf("site = %s, want A", class.Site)
	}
	if len(class.Reasons) == 0 {
		t.Fatal("every classification must record why it landed there")
	}
}

func TestClassifyRoundSplit(t *testing.T) {
	round := baseRound()
	acc := &roundAcc{
		frames: []tacticalplan.Frame{
			// Two pairs approaching the same site from opposite sides.
			frameAt(tick(40),
				alive(5, 300, 0), alive(6, 280, 40),
				alive(7, -300, 0), alive(8, -280, -40)),
		},
	}
	class := classifyRound(classifyInput{round: round, acc: acc, tickrate: testTickrate, sites: testSites()})
	if class.TSide != tacticalplan.TSplit {
		t.Fatalf("T pattern = %s, want %s (reasons: %v)", class.TSide, tacticalplan.TSplit, class.Reasons)
	}
}

func TestClassifyRoundEcoRush(t *testing.T) {
	round := baseRound()
	round.Economy.TBuy = tacticalplan.BuyEco
	acc := &roundAcc{
		frames: []tacticalplan.Frame{
			frameAt(tick(12), alive(5, 100, 0), alive(6, 120, 30), alive(7, 90, -20)),
		},
	}
	class := classifyRound(classifyInput{round: round, acc: acc, tickrate: testTickrate, sites: testSites()})
	if class.TSide != tacticalplan.TEcoRush {
		t.Fatalf("T pattern = %s, want %s (reasons: %v)", class.TSide, tacticalplan.TEcoRush, class.Reasons)
	}
}

func TestClassifyRoundFastAndDefaultDifferOnlyInTiming(t *testing.T) {
	fast := classifyRound(classifyInput{
		round:    baseRound(),
		acc:      &roundAcc{frames: []tacticalplan.Frame{frameAt(tick(15), alive(5, 100, 0), alive(6, 120, 30), alive(7, 90, -20))}},
		tickrate: testTickrate,
		sites:    testSites(),
	})
	if fast.TSide != tacticalplan.TFast {
		t.Fatalf("a commit at 15s = %s, want %s (reasons: %v)", fast.TSide, tacticalplan.TFast, fast.Reasons)
	}

	slow := classifyRound(classifyInput{
		round:    baseRound(),
		acc:      &roundAcc{frames: []tacticalplan.Frame{frameAt(tick(55), alive(5, 100, 0), alive(6, 120, 30), alive(7, 90, -20))}},
		tickrate: testTickrate,
		sites:    testSites(),
	})
	if slow.TSide != tacticalplan.TDefault {
		t.Fatalf("a commit at 55s = %s, want %s (reasons: %v)", slow.TSide, tacticalplan.TDefault, slow.Reasons)
	}
}

func TestClassifyRoundSave(t *testing.T) {
	round := baseRound()
	round.Winner = tacticalplan.SideCT
	for i := range round.Players {
		if round.Players[i].Side == tacticalplan.SideT && round.Players[i].Slot >= 8 {
			round.Players[i].Survived = true
		}
	}
	acc := &roundAcc{}
	class := classifyRound(classifyInput{round: round, acc: acc, tickrate: testTickrate, sites: testSites()})

	if class.TSide != tacticalplan.TSave {
		t.Fatalf("T pattern = %s, want %s (reasons: %v)", class.TSide, tacticalplan.TSave, class.Reasons)
	}
	if class.Site != tacticalplan.SiteNone {
		t.Fatalf("a save has no site, got %s", class.Site)
	}
	if !hasTag(class.Tags, tacticalplan.TagFullSave) {
		t.Fatalf("tags = %v, want %s", class.Tags, tacticalplan.TagFullSave)
	}
}

func TestClassifyRoundRetake(t *testing.T) {
	round := baseRound()
	round.Winner = tacticalplan.SideCT
	round.Bomb = &tacticalplan.Bomb{PlantTick: tick(45), Site: tacticalplan.SiteB, DefuseTick: tick(70)}
	acc := &roundAcc{
		frames: []tacticalplan.Frame{
			frameAt(tick(44), alive(0, 5000, 100), alive(1, 4900, 0)),
		},
	}
	class := classifyRound(classifyInput{round: round, acc: acc, tickrate: testTickrate, sites: testSites()})

	if class.CTSide != tacticalplan.CTRetake {
		t.Fatalf("CT pattern = %s, want %s (reasons: %v)", class.CTSide, tacticalplan.CTRetake, class.Reasons)
	}
	if class.Site != tacticalplan.SiteB {
		t.Fatalf("site = %s, want B (the plant is proof)", class.Site)
	}
	if !hasTag(class.Tags, tacticalplan.TagPostPlant) || !hasTag(class.Tags, tacticalplan.TagRetakeWon) {
		t.Fatalf("tags = %v, want postplant and retake_won", class.Tags)
	}
}

func TestClassifyRoundStack(t *testing.T) {
	round := baseRound()
	acc := &roundAcc{
		frames: []tacticalplan.Frame{
			frameAt(tick(20), alive(0, 100, 0), alive(1, 150, 50), alive(2, 90, -50)),
		},
	}
	class := classifyRound(classifyInput{round: round, acc: acc, tickrate: testTickrate, sites: testSites()})
	if class.CTSide != tacticalplan.CTStack {
		t.Fatalf("CT pattern = %s, want %s (reasons: %v)", class.CTSide, tacticalplan.CTStack, class.Reasons)
	}
}

func TestClassifyRoundAggression(t *testing.T) {
	round := baseRound()
	killer := uint8(0)
	acc := &roundAcc{
		kills: []killRecord{{
			tick:       tick(10),
			killerSlot: &killer,
			killerSide: tacticalplan.SideCT,
			victimSlot: 5,
			victimSide: tacticalplan.SideT,
			// Far from every site: this defender went hunting.
			pos: [3]float64{2500, -3000, 0},
		}},
	}
	class := classifyRound(classifyInput{round: round, acc: acc, tickrate: testTickrate, sites: testSites()})
	if class.CTSide != tacticalplan.CTAggression {
		t.Fatalf("CT pattern = %s, want %s (reasons: %v)", class.CTSide, tacticalplan.CTAggression, class.Reasons)
	}
	if class.FirstContactTick != tick(10) {
		t.Fatalf("first contact = %d, want %d", class.FirstContactTick, tick(10))
	}
}

func TestClassifyRoundTags(t *testing.T) {
	round := baseRound()
	round.Overtime = 2
	round.EndReason = "time_expired"
	round.Economy.TBuy = tacticalplan.BuyEco
	round.Players[0].Kills = 5
	round.Class.OpeningTraded = true

	class := classifyRound(classifyInput{round: round, acc: &roundAcc{}, tickrate: testTickrate, sites: testSites()})
	for _, want := range []string{
		tacticalplan.TagOvertime,
		tacticalplan.TagTimeout,
		tacticalplan.TagAce,
		tacticalplan.TagAntiEco,
		tacticalplan.TagOpeningTraded,
	} {
		if !hasTag(class.Tags, want) {
			t.Fatalf("tags = %v, want %s among them", class.Tags, want)
		}
	}
}

func TestClassifyRoundKeepsOpeningFacts(t *testing.T) {
	round := baseRound()
	slot := uint8(6)
	round.Class = tacticalplan.Class{
		OpeningSlot: &slot,
		OpeningSide: tacticalplan.SideT,
		OpeningTick: tick(9),
	}
	class := classifyRound(classifyInput{round: round, acc: &roundAcc{}, tickrate: testTickrate, sites: testSites()})
	if class.OpeningSlot == nil || *class.OpeningSlot != slot || class.OpeningSide != tacticalplan.SideT {
		t.Fatalf("the opening duel is a fact and must survive classification, got %+v", class)
	}
}

func TestSiteFromPlace(t *testing.T) {
	tests := []struct {
		place string
		want  tacticalplan.Site
	}{
		{"BombsiteA", tacticalplan.SiteA},
		{"Bombsite A", tacticalplan.SiteA},
		{"BombsiteB", tacticalplan.SiteB},
		{"Middle", tacticalplan.SiteMid},
		{"Mid", tacticalplan.SiteMid},
		{"Apartments", tacticalplan.SiteNone},
		{"", tacticalplan.SiteNone},
	}
	for _, tt := range tests {
		if got := siteFromPlace(tt.place); got != tt.want {
			t.Fatalf("siteFromPlace(%q) = %s, want %s", tt.place, got, tt.want)
		}
	}
}

func TestBuildSiteMapPrefersPlants(t *testing.T) {
	rounds := []*roundAcc{
		{events: []tacticalplan.Event{
			{Kind: tacticalplan.EventPlant, Site: tacticalplan.SiteA, Pos: [3]float64{100, 100, 0}},
		}},
		{events: []tacticalplan.Event{
			{Kind: tacticalplan.EventPlant, Site: tacticalplan.SiteA, Pos: [3]float64{300, 300, 0}},
		}},
	}
	geo := tacticalplan.MapGeometry{Callouts: []tacticalplan.Callout{
		{Name: "BombsiteA", Center: [2]float64{9999, 9999}, Samples: 500},
		{Name: "BombsiteB", Center: [2]float64{5000, 0}, Samples: 400},
	}}

	m := buildSiteMap(rounds, geo)
	center, ok := m.center(tacticalplan.SiteA)
	if !ok {
		t.Fatal("site A must be located")
	}
	if center != [2]float64{200, 200} {
		t.Fatalf("A centre = %v, want the mean of the two plants", center)
	}
	// B was never planted on, so the place name fills it in.
	if center, ok := m.center(tacticalplan.SiteB); !ok || center != [2]float64{5000, 0} {
		t.Fatalf("B centre = %v (%t), want the callout centroid", center, ok)
	}
	if got, _ := m.nearest(210, 190); got != tacticalplan.SiteA {
		t.Fatalf("nearest site = %s, want A", got)
	}
}

func hasTag(tags []string, want string) bool {
	for _, tag := range tags {
		if tag == want {
			return true
		}
	}
	return false
}

// The seam at +/-pi is where a naive quadrant grid reports a phantom split, so
// it gets its own table.
func TestApproachGroups(t *testing.T) {
	const deg = math.Pi / 180
	tests := []struct {
		name     string
		bearings []float64
		want     int
	}{
		{"one tight group", []float64{0, 10 * deg, -10 * deg}, 1},
		{"one group across the seam", []float64{179 * deg, -179 * deg, 175 * deg}, 1},
		{"two opposite groups", []float64{0, 15 * deg, 180 * deg, -170 * deg}, 2},
		{"two groups across the seam", []float64{170 * deg, -175 * deg, 10 * deg, 0}, 2},
		{"a lone straggler is not a group", []float64{0, 10 * deg, 120 * deg}, 1},
		{"nobody", nil, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := approachGroups(tt.bearings); got != tt.want {
				t.Fatalf("approachGroups(%v) = %d, want %d", tt.bearings, got, tt.want)
			}
		})
	}
}
