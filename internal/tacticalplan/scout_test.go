package tacticalplan

import (
	"reflect"
	"testing"
	"time"

	"github.com/google/uuid"
)

func slotPtr(slot uint8) *uint8 { return &slot }

var (
	scoutMatchOne = uuid.MustParse("00000000-0000-0000-0000-000000000001")
	scoutMatchTwo = uuid.MustParse("00000000-0000-0000-0000-000000000002")
)

// scoutDocument is testDocument with a radar and the events the report reads:
// an A execute with two grenades and an AWP opening kill in round 3, and a
// B molotov in the CT setup of round 2.
func scoutDocument() Document {
	d := testDocument()
	d.Geometry = MapGeometry{Callouts: []Callout{
		{Name: "BombsiteA", Level: "default", Center: [2]float64{1000, 1000}},
		{Name: "BombsiteB", Level: "default", Center: [2]float64{-1000, -1000}},
		{Name: "Middle", Level: "default", Center: [2]float64{0, 0}},
	}}
	d.Rounds[1].Events = []Event{
		{Tick: 5700, Kind: EventMolotov, ActorSlot: slotPtr(1), Side: SideCT, Pos: [3]float64{-990, -1000, 0}},
	}
	d.Rounds[2].Events = []Event{
		{Tick: 11500, Kind: EventSmoke, ActorSlot: slotPtr(0), Side: SideT, Pos: [3]float64{990, 1000, 0}},
		{Tick: 11800, Kind: EventFlash, ActorSlot: slotPtr(1), Side: SideT, Pos: [3]float64{1010, 990, 0}},
		{Tick: 11900, Kind: EventSmoke, ActorSlot: slotPtr(2), Side: SideCT, Pos: [3]float64{0, 10, 0}},
		{
			Tick: 12000, Kind: EventKill, ActorSlot: slotPtr(0), TargetSlot: slotPtr(3), Side: SideT,
			Weapon: "awp", Place: "Window", Opening: true, TargetPos: [3]float64{900, 900, 0},
		},
	}
	return d
}

// swappedDocument is the same match seen from the other team: Alpha's players
// now sit in slots 2 and 3 under the t-start key, so a line keyed by slot
// would split one player in two.
func swappedDocument() Document {
	d := scoutDocument()
	d.Demo.Map = "de_inferno"
	d.Teams = []Team{
		{Key: "ct-start", Name: "Gamma", StartSide: SideCT, Slots: []uint8{0, 1}},
		{Key: "t-start", Name: "Alpha", StartSide: SideT, Slots: []uint8{2, 3}},
	}
	d.Players = []Player{
		{Slot: 0, SteamID64: "3", Name: "g1", TeamKey: "ct-start", StartSide: SideCT},
		{Slot: 1, SteamID64: "4", Name: "g2", TeamKey: "ct-start", StartSide: SideCT},
		{Slot: 2, SteamID64: "1", Name: "a1-new", TeamKey: "t-start", StartSide: SideT},
		{Slot: 3, SteamID64: "2", Name: "a2", TeamKey: "t-start", StartSide: SideT},
	}
	return d
}

func scoutMatches() []ScoutMatch {
	return []ScoutMatch{
		{
			MatchID: scoutMatchOne, TeamKey: "ct-start", OpponentID: "beta", Opponent: "Beta",
			PlayedAt: time.Date(2026, 10, 1, 18, 0, 0, 0, time.UTC), Document: scoutDocument(),
			Setup: map[int]Frame{2: {Tick: 6600, Samples: []Sample{
				{Slot: 0, X: 10, Y: -10, Flags: FlagAlive},
				{Slot: 1, X: -1000, Y: -1010, Flags: FlagAlive},
				{Slot: 2, X: 1000, Y: 1000, Flags: FlagAlive | FlagSideT},
			}}},
		},
		{
			MatchID: scoutMatchTwo, TeamKey: "t-start", OpponentID: "gamma", Opponent: "Gamma",
			PlayedAt: time.Date(2026, 10, 20, 18, 0, 0, 0, time.UTC), Document: swappedDocument(),
		},
	}
}

func TestScoutSelectFiltersAndOrdersNewestFirst(t *testing.T) {
	ms := scoutMatches()
	all := ScoutFilter{}.Select(ms)
	if len(all) != 2 || all[0].MatchID != scoutMatchTwo {
		t.Fatalf("select = %v, want both matches with the newest first", ids(all))
	}
	if got := (ScoutFilter{Maps: []string{"de_mirage"}}).Select(ms); len(got) != 1 || got[0].MatchID != scoutMatchOne {
		t.Fatalf("map filter = %v, want the mirage match", ids(got))
	}
	if got := (ScoutFilter{Opponents: []string{"gamma"}}).Select(ms); len(got) != 1 || got[0].MatchID != scoutMatchTwo {
		t.Fatalf("rival filter = %v, want the match against gamma", ids(got))
	}
	from := time.Date(2026, 10, 10, 0, 0, 0, 0, time.UTC)
	if got := (ScoutFilter{From: &from}).Select(ms); len(got) != 1 || got[0].MatchID != scoutMatchTwo {
		t.Fatalf("date filter = %v, want the later match", ids(got))
	}
	to := time.Date(2026, 10, 1, 18, 0, 0, 0, time.UTC)
	if got := (ScoutFilter{To: &to}).Select(ms); len(got) != 1 || got[0].MatchID != scoutMatchOne {
		t.Fatalf("inclusive upper bound = %v, want the match played at the bound", ids(got))
	}
}

func ids(ms []ScoutMatch) []uuid.UUID {
	out := make([]uuid.UUID, len(ms))
	for i, m := range ms {
		out[i] = m.MatchID
	}
	return out
}

// Aggregating several matches must equal summing them, and one player must
// stay one line even when the slot changes between demos.
func TestAggregateMatchesSumsMatchesAndKeysPlayersBySteamID(t *testing.T) {
	ms := scoutMatches()
	tend := AggregateMatches(ms, ScoutFilter{})

	one := Aggregate(ms[0].Document, Filter{TeamKey: "ct-start"})
	two := Aggregate(ms[1].Document, Filter{TeamKey: "t-start"})
	if tend.MatchCount != 2 || tend.RoundCount != one.RoundCount+two.RoundCount {
		t.Fatalf("matches/rounds = %d/%d, want 2/%d", tend.MatchCount, tend.RoundCount, one.RoundCount+two.RoundCount)
	}
	if tend.Wins != one.Wins+two.Wins || tend.Wins != 4 {
		t.Fatalf("wins = %d, want %d+%d = 4", tend.Wins, one.Wins, two.Wins)
	}
	if len(tend.Players) != 2 {
		t.Fatalf("player lines = %+v, want Alpha's 2 players", tend.Players)
	}
	top := tend.Players[0]
	if top.SteamID64 != "1" || top.Rounds != 8 || top.Kills != 6 {
		t.Fatalf("top line = %+v, want steamid 1 over 8 rounds with 6 kills", top)
	}
	// The newest match names the player.
	if top.Name != "a1-new" {
		t.Fatalf("name = %q, want the most recent match's nick", top.Name)
	}
	if tend.Openings.Rounds != one.Openings.Rounds+two.Openings.Rounds {
		t.Fatalf("opening duels = %d, want the sum of both matches", tend.Openings.Rounds)
	}
}

func TestAggregateMatchesSideFilterKeepsOneSide(t *testing.T) {
	tend := AggregateMatches(scoutMatches(), ScoutFilter{Side: SideT})
	if tend.RoundCount != 4 {
		t.Fatalf("T rounds = %d, want 2 per match", tend.RoundCount)
	}
	if len(tend.CTPatterns) != 0 {
		t.Fatalf("CT patterns = %+v, want none with the T side fixed", tend.CTPatterns)
	}
	for _, p := range tend.TPatterns {
		if p.Share.Total != 4 {
			t.Fatalf("T pattern %q denominator = %d, want 4", p.Pattern, p.Share.Total)
		}
	}
}

func TestFilterTeamAndSideTogether(t *testing.T) {
	d := testDocument()
	rounds := Filter{TeamKey: "ct-start", Side: SideCT}.Apply(d)
	if len(rounds) != 2 || rounds[0].Number != 1 || rounds[1].Number != 2 {
		t.Fatalf("rounds = %+v, want Alpha's first-half CT rounds", rounds)
	}
}

func TestScoutPistolsPerSideWithEvidence(t *testing.T) {
	report := Scout(scoutMatches(), ScoutFilter{})
	p := report.Pistols
	if len(p.Sides) != 2 || p.Sides[0].Side != SideT || p.Sides[0].Rounds != 1 || p.Sides[0].WinRate.Count != 1 {
		t.Fatalf("pistol sides = %+v, want 1 T pistol won and 1 CT pistol", p.Sides)
	}
	if p.Sides[1].Side != SideCT || p.Sides[1].WinRate.Count != 0 {
		t.Fatalf("CT pistol = %+v, want lost", p.Sides[1])
	}
	if len(p.Rows) != 2 {
		t.Fatalf("pistol rows = %+v, want one per side", p.Rows)
	}
	tRow := p.Rows[0]
	if tRow.Side != SideT || tRow.Pattern != string(TFast) || tRow.Site != SiteA {
		t.Fatalf("T pistol row = %+v, want a fast A", tRow)
	}
	want := Evidence{MatchID: scoutMatchTwo, Round: 1, Tick: 640}
	if len(tRow.Examples) != 1 || tRow.Examples[0] != want {
		t.Fatalf("T pistol evidence = %+v, want %+v", tRow.Examples, want)
	}
}

func TestScoutExecutesCarryGrenadesAndTimings(t *testing.T) {
	report := Scout(scoutMatches(), ScoutFilter{})
	if len(report.Executes) != 1 {
		t.Fatalf("executes = %+v, want the A execute only", report.Executes)
	}
	ex := report.Executes[0]
	if ex.Site != SiteA || ex.Rounds != 1 || ex.WinRate.Count != 1 || ex.Plants.Count != 0 {
		t.Fatalf("A execute = %+v, want 1 won round without a plant", ex)
	}
	// The opponent's smoke in the same round is not Alpha's execute.
	if len(ex.Utility) != 2 {
		t.Fatalf("execute utility = %+v, want Alpha's smoke and flash", ex.Utility)
	}
	smoke := ex.Utility[0]
	if smoke.Kind != EventSmoke || smoke.Callout != "BombsiteA" || smoke.Share.Count != 1 || smoke.Share.Total != 1 {
		t.Fatalf("smoke row = %+v, want an A smoke in 1 of 1 executes", smoke)
	}
	// 11500 - 10640 ticks at 64 tick = 13.4 s.
	if smoke.MedianSeconds != 13.4 {
		t.Fatalf("smoke time = %v, want 13.4", smoke.MedianSeconds)
	}
	if len(smoke.Throwers) != 1 || smoke.Throwers[0].SteamID64 != "1" {
		t.Fatalf("smoke thrower = %+v, want steamid 1", smoke.Throwers)
	}
	if smoke.Examples[0] != (Evidence{MatchID: scoutMatchOne, Round: 3, Tick: 11500}) {
		t.Fatalf("smoke evidence = %+v, want match one round 3 tick 11500", smoke.Examples[0])
	}
	if ex.MeanUtility[EventSmoke] != 1 || ex.MeanUtility[EventFlash] != 1 || ex.MeanUtility[EventMolotov] != 0 {
		t.Fatalf("mean utility = %+v, want one smoke and one flash", ex.MeanUtility)
	}
}

func TestScoutDefaultsAndSetups(t *testing.T) {
	report := Scout(scoutMatches(), ScoutFilter{})

	d := report.TDefaults
	if d.Rounds != 1 || d.Examples[0].Round != 3 {
		t.Fatalf("T defaults = %+v, want round 3 of match one, the only full-buy T round", d.ScoutRounds)
	}
	if len(d.Utility) != 2 {
		t.Fatalf("default utility = %+v, want the two early grenades", d.Utility)
	}
	if len(d.OpeningPlaces) != 1 || d.OpeningPlaces[0].Place != "Window" || d.OpeningPlaces[0].Won.Count != 1 {
		t.Fatalf("opening places = %+v, want a won duel at Window", d.OpeningPlaces)
	}

	s := report.CTSetups
	if s.Rounds != 2 || s.Sampled != 1 {
		t.Fatalf("CT setups = %d rounds, %d sampled; want 2 and 1", s.Rounds, s.Sampled)
	}
	if len(s.Utility) != 1 || s.Utility[0].Kind != EventMolotov || s.Utility[0].Callout != "BombsiteB" {
		t.Fatalf("setup utility = %+v, want a B molotov", s.Utility)
	}
	if len(s.Positions) != 2 {
		t.Fatalf("setup positions = %+v, want Alpha's 2 CTs and not the T", s.Positions)
	}
	byID := map[string]ScoutPositionRow{}
	for _, p := range s.Positions {
		byID[p.SteamID64] = p
	}
	if spot := byID["1"].Spots; len(spot) != 1 || spot[0].Callout != "Middle" {
		t.Fatalf("steamid 1 spots = %+v, want Middle", spot)
	}
	if spot := byID["2"].Spots; len(spot) != 1 || spot[0].Callout != "BombsiteB" || spot[0].Examples[0].Tick != 6600 {
		t.Fatalf("steamid 2 spots = %+v, want BombsiteB at tick 6600", spot)
	}
}

func TestScoutEconomyAfterLoss(t *testing.T) {
	e := Scout(scoutMatches(), ScoutFilter{}).Economy
	if len(e.Contexts) != 2 {
		t.Fatalf("contexts = %+v, want one after the pistol loss and one after a loss", e.Contexts)
	}
	pistol := e.Contexts[0]
	if pistol.Context != ScoutAfterPistolLoss || pistol.Side != SideCT || pistol.Rounds != 1 {
		t.Fatalf("pistol context = %+v, want 1 CT round after the lost pistol", pistol)
	}
	if pistol.Examples[0] != (Evidence{MatchID: scoutMatchOne, Round: 2, Tick: 5640}) {
		t.Fatalf("pistol evidence = %+v, want match one round 2", pistol.Examples[0])
	}
	loss := e.Contexts[1]
	if loss.Context != ScoutAfterLoss || loss.Examples[0].MatchID != scoutMatchTwo || loss.Examples[0].Round != 4 {
		t.Fatalf("loss context = %+v, want match two round 4", loss)
	}
	for _, row := range e.Rows {
		if row.Buy != BuyFull || row.Share.Total != 1 {
			t.Fatalf("economy row = %+v, want a full buy out of its context", row)
		}
	}
	// The round after a half switch is a pistol, never a reaction.
	for _, c := range e.Contexts {
		for _, ex := range c.Examples {
			if ex.MatchID == scoutMatchOne && ex.Round == 3 {
				t.Fatal("round 3 starts a half and must not count as after a loss")
			}
		}
	}
}

func TestScoutAWPAndOpenings(t *testing.T) {
	report := Scout(scoutMatches(), ScoutFilter{})

	if len(report.AWP.Rows) != 1 {
		t.Fatalf("AWP rows = %+v, want Alpha's Window kill only", report.AWP.Rows)
	}
	row := report.AWP.Rows[0]
	if row.Side != SideT || row.Place != "Window" || row.Kills != 1 || row.OpeningKills != 1 || row.Share.Total != 1 {
		t.Fatalf("AWP row = %+v, want 1 opening kill from Window as T", row)
	}
	tSide := report.AWP.Sides[0]
	if tSide.Side != SideT || tSide.Rounds != 4 || tSide.KillRounds.Count != 1 {
		t.Fatalf("AWP T side = %+v, want 1 of 4 rounds with an AWP kill", tSide)
	}

	var tLine *ScoutOpening
	for i := range report.Openings {
		if report.Openings[i].SteamID64 == "1" && report.Openings[i].Side == SideT {
			tLine = &report.Openings[i]
		}
	}
	if tLine == nil {
		t.Fatalf("openings = %+v, want a T line for steamid 1", report.Openings)
	}
	if tLine.Rounds != 4 || tLine.Attempts.Count != 3 || tLine.Won.Count != 2 {
		t.Fatalf("T openings of steamid 1 = %+v, want 3 duels in 4 rounds, 2 won", tLine)
	}
	// A lost duel is placed where the team's player fell, not where the killer stood.
	foundCT := false
	for _, o := range report.Openings {
		if o.SteamID64 == "2" && o.Side == SideCT {
			foundCT = true
			if len(o.Places) != 1 || o.Places[0].Place != "BombsiteA" {
				t.Fatalf("CT duel places of steamid 2 = %+v, want BombsiteA where it died", o.Places)
			}
		}
	}
	if !foundCT {
		t.Fatal("no CT opening line for steamid 2")
	}
	for i := 1; i < len(report.Openings); i++ {
		prev, cur := report.Openings[i-1], report.Openings[i]
		if prev.Side == cur.Side && prev.Attempts.Count < cur.Attempts.Count {
			t.Fatalf("openings out of order: %+v before %+v", prev, cur)
		}
	}
}

func TestScoutSummariesAndDeterminism(t *testing.T) {
	ms := scoutMatches()
	report := Scout(ms, ScoutFilter{})
	if len(report.Matches) != 2 || report.Matches[0].MatchID != scoutMatchTwo {
		t.Fatalf("matches = %+v, want both with the newest first", report.Matches)
	}
	if m := report.Matches[1]; m.Score != 2 || m.OpponentScore != 2 || m.Opponent != "Beta" {
		t.Fatalf("match one summary = %+v, want 2-2 against Beta", m)
	}
	if len(report.Maps) != 2 || report.Maps[0].Map != "de_inferno" || report.Maps[0].Rounds != 4 {
		t.Fatalf("maps = %+v, want inferno and mirage with 4 rounds each", report.Maps)
	}

	reversed := []ScoutMatch{ms[1], ms[0]}
	if again := Scout(reversed, ScoutFilter{}); !reflect.DeepEqual(report, again) {
		t.Fatal("the report must not depend on the order matches are passed in")
	}
}

func TestSetupFramesPicksTheFrameAtSetupTime(t *testing.T) {
	sample := func(tick int) Frame {
		return Frame{Tick: tick, Samples: []Sample{{Slot: 0, X: float64(tick), Y: 1, Flags: FlagAlive}}}
	}
	blob, err := EncodePositions([]RoundFrames{
		{Round: 1, Frames: []Frame{sample(1000), sample(1600), sample(2000)}},
		{Round: 2, Frames: []Frame{sample(5000)}},
	}, 8, 64)
	if err != nil {
		t.Fatalf("EncodePositions: %v", err)
	}
	doc := NewDocument()
	doc.Demo.Tickrate = 64
	doc.Positions = blob.Descriptor
	doc.Rounds = []Round{
		{Number: 1, TickFreezeEnd: 640, TickEnd: 4000},
		// Round 2 ends before its setup time: there is no setup to read.
		{Number: 2, TickFreezeEnd: 4500, TickEnd: 5100},
	}
	frames, err := SetupFrames(blob.Data, doc)
	if err != nil {
		t.Fatalf("SetupFrames: %v", err)
	}
	if len(frames) != 1 || frames[1].Tick != 1600 {
		t.Fatalf("frames = %+v, want round 1 at tick 1600 (640 + 15 s)", frames)
	}
	if _, err := SetupFrames([]byte("nope"), doc); err == nil {
		t.Fatal("a blob without a header must be rejected")
	}
}

func TestNearestCalloutPrefersTheSameLevel(t *testing.T) {
	lower := -100.0
	g := MapGeometry{Callouts: []Callout{
		{Name: "Upper", Level: "default", Center: [2]float64{0, 0}},
		{Name: "Lower", Level: "lower", Center: [2]float64{500, 0}},
	}}
	g.Calibration.LowerAltitudeMax = &lower
	if got := NearestCallout(g, [3]float64{10, 0, -200}); got != "Lower" {
		t.Fatalf("callout = %q, want the lower level's even though Upper is closer", got)
	}
	if got := NearestCallout(MapGeometry{}, [3]float64{}); got != "" {
		t.Fatalf("callout without geometry = %q, want empty", got)
	}
}
