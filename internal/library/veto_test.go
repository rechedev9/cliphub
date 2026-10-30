package library

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/tacticalplan"
)

func TestCleanSeriesChecksTeamsAndVetoSteps(t *testing.T) {
	a, b := uuid.New(), uuid.New()
	ok := Series{Format: FormatBO3, Event: "  IEM  ", TeamIDs: []uuid.UUID{a, b}, Vetoes: []VetoStep{
		{Team: "A", Action: "Ban", Map: " DE_Nuke "}, {Team: "b", Action: "pick", Map: "Mirage"}, {Action: "decider", Map: "de_inferno"},
	}}
	got, err := CleanSeries(ok)
	if err != nil || got.Event != "IEM" || got.Vetoes[0] != (VetoStep{Team: "a", Action: VetoBan, Map: "de_nuke"}) || got.Vetoes[1].Map != "de_mirage" {
		t.Fatalf("CleanSeries = %+v, %v", got, err)
	}
	for name, s := range map[string]Series{
		"format":        {Format: "bo2"},
		"one team":      {Format: FormatBO1, TeamIDs: []uuid.UUID{a}},
		"same team":     {Format: FormatBO1, TeamIDs: []uuid.UUID{a, a}},
		"no teams":      {Format: FormatBO1, Vetoes: []VetoStep{{Team: "a", Action: VetoBan, Map: "de_nuke"}}},
		"twice":         {Format: FormatBO1, TeamIDs: []uuid.UUID{a, b}, Vetoes: []VetoStep{{Team: "a", Action: VetoBan, Map: "de_nuke"}, {Team: "b", Action: VetoBan, Map: "de_nuke"}}},
		"decider team":  {Format: FormatBO1, Vetoes: []VetoStep{{Team: "a", Action: VetoDecider, Map: "de_nuke"}}},
		"decider early": {Format: FormatBO1, TeamIDs: []uuid.UUID{a, b}, Vetoes: []VetoStep{{Action: VetoDecider, Map: "de_nuke"}, {Team: "a", Action: VetoBan, Map: "de_mirage"}}},
		"action":        {Format: FormatBO1, TeamIDs: []uuid.UUID{a, b}, Vetoes: []VetoStep{{Team: "a", Action: "keep", Map: "de_nuke"}}},
		"map":           {Format: FormatBO1, TeamIDs: []uuid.UUID{a, b}, Vetoes: []VetoStep{{Team: "a", Action: VetoBan, Map: "de nuke"}}},
	} {
		if _, err := CleanSeries(s); err == nil {
			t.Errorf("%s: CleanSeries accepted %+v", name, s)
		}
	}
}

func TestVetoSequenceFollowsTheFormats(t *testing.T) {
	describe := func(steps []VetoPlanStep) string {
		parts := make([]string, len(steps))
		for i, s := range steps {
			parts[i] = s.Team + ":" + s.Action
		}
		return strings.Join(parts, " ")
	}
	cases := map[string]string{
		FormatBO1: "a:ban b:ban a:ban b:ban a:ban b:ban :decider",
		FormatBO3: "a:ban b:ban a:pick b:pick a:ban b:ban :decider",
		FormatBO5: "a:ban b:ban a:pick b:pick a:pick b:pick :decider",
	}
	for format, want := range cases {
		steps, err := VetoSequence(format, 7, SeriesTeamA)
		if err != nil || describe(steps) != want {
			t.Errorf("%s = %q, %v; want %q", format, describe(steps), err, want)
		}
	}
	if steps, _ := VetoSequence(FormatBO3, 7, SeriesTeamB); steps[0].Team != SeriesTeamB || steps[1].Team != SeriesTeamA {
		t.Fatalf("b first = %q", describe(steps))
	}
	if _, err := VetoSequence(FormatBO5, 4, SeriesTeamA); err == nil {
		t.Fatal("a bo5 over 4 maps must be refused")
	}
}

func mapRecord(name string, won, lost, bans, picks int) MapRecord {
	r := *newMapRecord(name)
	r.Matches, r.RoundsWon, r.RoundsLost = 1, won, lost
	for range bans {
		r.Veto.Banned.add(uuid.New())
	}
	for range picks {
		r.Veto.Picked.add(uuid.New())
	}
	return r
}

func TestSimulateVetoRecommendsByEdgeAndOpponentHabits(t *testing.T) {
	pool := []string{"de_nuke", "de_mirage", "de_inferno", "de_ancient", "de_anubis", "de_dust2", "de_train"}
	ours := []MapRecord{mapRecord("de_mirage", 26, 10, 0, 0), mapRecord("de_nuke", 8, 26, 0, 0), mapRecord("de_inferno", 20, 16, 0, 0)}
	theirs := []MapRecord{mapRecord("de_mirage", 13, 13, 0, 0), mapRecord("de_ancient", 26, 6, 0, 0)}
	req := VetoRequest{Format: FormatBO3, Pool: pool, First: SeriesTeamA, Ours: ours, Theirs: theirs}
	plan, err := SimulateVeto(req)
	if err != nil {
		t.Fatal(err)
	}
	// We ban Ancient, where they are strongest; they ban our best, Mirage; we
	// pick Inferno; they pick Nuke, our weakest.
	got := []string{}
	for _, s := range plan.Steps[:4] {
		got = append(got, s.Map+"/"+s.Basis)
	}
	want := []string{"de_ancient/edge", "de_mirage/edge", "de_inferno/edge", "de_nuke/edge"}
	if !slices.Equal(got, want) {
		t.Fatalf("first steps = %v, want %v", got, want)
	}
	if last := plan.Steps[len(plan.Steps)-1]; last.Action != VetoDecider || last.Basis != VetoBasisLeftover || len(plan.Played) != 3 {
		t.Fatalf("decider = %+v, played %v", last, plan.Played)
	}

	// Shuffled pool and records give the same plan.
	shuffled := req
	shuffled.Pool = []string{"de_train", "de_dust2", "de_anubis", "de_ancient", "de_inferno", "de_mirage", "de_nuke"}
	shuffled.Ours = []MapRecord{ours[2], ours[0], ours[1]}
	again, err := SimulateVeto(shuffled)
	if err != nil || !slices.EqualFunc(again.Steps, plan.Steps, func(a, b VetoPlanStep) bool { return a == b }) {
		t.Fatalf("plan depends on input order: %+v", again.Steps)
	}

	// With known habits the opponent bans what it always bans.
	habits := req
	habits.Theirs = append([]MapRecord{mapRecord("de_dust2", 0, 0, 3, 0)}, theirs...)
	withHabit, err := SimulateVeto(habits)
	if err != nil || withHabit.Steps[1].Map != "de_dust2" || withHabit.Steps[1].Basis != VetoBasisHistory || withHabit.OpponentHistory != 3 {
		t.Fatalf("habit step = %+v (history %d), %v", withHabit.Steps[1], withHabit.OpponentHistory, err)
	}

	// A given step keeps its map and the rest follows from it.
	chosen := req
	chosen.Done = []string{"de_train"}
	forced, err := SimulateVeto(chosen)
	if err != nil || forced.Steps[0].Map != "de_train" || forced.Steps[0].Basis != VetoBasisChosen || forced.Steps[2].Map != "de_inferno" {
		t.Fatalf("forced = %+v, %v", forced.Steps, err)
	}
	chosen.Done = []string{"de_train", "de_train"}
	if _, err := SimulateVeto(chosen); err == nil {
		t.Fatal("a map taken twice must be refused")
	}
}

func TestTeamMapHistoryCountsResultsSidesAndVetoes(t *testing.T) {
	team, rival := uuid.New(), uuid.New()
	played := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	match := func(mapName string, day, own, opp int) Match {
		return Match{ID: uuid.New(), Map: mapName, PlayedAt: played.AddDate(0, 0, day), Teams: []MatchTeam{
			{Key: "ct-start", TeamID: &team, Score: own}, {Key: "t-start", TeamID: &rival, Score: opp},
		}}
	}
	doc := tacticalplan.Document{Teams: []tacticalplan.Team{{Key: "ct-start", StartSide: tacticalplan.SideCT}, {Key: "t-start", StartSide: tacticalplan.SideT}}}
	doc.Rounds = []tacticalplan.Round{
		{Number: 1, Half: 1, Winner: tacticalplan.SideCT, TickFreezeEnd: 100, Class: tacticalplan.Class{Tags: []string{tacticalplan.TagPistol}}},
		{Number: 2, Half: 1, Winner: tacticalplan.SideT, TickFreezeEnd: 200},
		{Number: 13, Half: 2, Winner: tacticalplan.SideT, TickFreezeEnd: 1300, Class: tacticalplan.Class{Tags: []string{tacticalplan.TagPistol}}},
	}
	early, late := match("de_mirage", 0, 13, 9), match("de_mirage", 5, 10, 13)
	nuke := match("de_nuke", 2, 13, 13)
	series := Series{ID: uuid.New(), Teams: []SeriesTeam{{Slot: "a", TeamID: rival}, {Slot: "b", TeamID: team}}, Vetoes: []VetoStep{
		{Team: "a", Action: VetoBan, Map: "de_anubis"}, {Team: "b", Action: VetoBan, Map: "de_train"},
		{Team: "a", Action: VetoPick, Map: "de_nuke"}, {Team: "b", Action: VetoPick, Map: "mirage"}, {Action: VetoDecider, Map: "de_inferno"},
	}}
	got := TeamMapHistory(team, []MapHistoryMatch{
		{Match: early, TeamKey: "ct-start", Document: &doc}, {Match: nuke, TeamKey: "ct-start"}, {Match: late, TeamKey: "ct-start", Document: &doc},
	}, []Series{series})
	names := []string{}
	for _, r := range got {
		names = append(names, r.Map)
	}
	if !slices.Equal(names, []string{"de_mirage", "de_nuke", "de_anubis", "de_inferno", "de_train"}) {
		t.Fatalf("maps = %v", names)
	}
	mirage := got[0]
	if mirage.Wins != 1 || mirage.Losses != 1 || mirage.RoundsWon != 23 || mirage.RoundsLost != 22 || mirage.MatchIDs[0] != late.ID || !mirage.LastPlayedAt.Equal(late.PlayedAt) {
		t.Fatalf("mirage = %+v", mirage)
	}
	ct, tSide := mirage.Sides[0], mirage.Sides[1]
	if ct.Rounds != 4 || ct.Won.Count != 2 || ct.PistolWon.Count != 2 || ct.Won.Examples[0].MatchID != late.ID || ct.Won.Examples[0].Tick != 100 {
		t.Fatalf("ct side = %+v", ct)
	}
	if tSide.Rounds != 2 || tSide.Won.Count != 2 || tSide.PistolWon.Count != 2 {
		t.Fatalf("t side = %+v", tSide)
	}
	if mirage.Veto.Picked.Count != 1 || got[1].Ties != 1 || got[1].MeasuredMatches != 0 || got[1].Veto.OpponentPicked.Count != 1 {
		t.Fatalf("vetoes = %+v / nuke %+v", mirage.Veto, got[1])
	}
	if got[2].Veto.OpponentBanned.Count != 1 || got[3].Veto.Decider.Count != 1 || got[4].Veto.Banned.SeriesIDs[0] != series.ID {
		t.Fatalf("ban and decider counts = %+v %+v %+v", got[2].Veto, got[3].Veto, got[4].Veto)
	}
}
