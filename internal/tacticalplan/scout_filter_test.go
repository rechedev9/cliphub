package tacticalplan

import (
	"reflect"
	"testing"

	"github.com/google/uuid"
)

func matchIDs(ms []ScoutMatch) []string {
	out := []string{}
	for _, m := range ms {
		out = append(out, m.MatchID.String())
	}
	return out
}

func TestScoutSelectByOpponentLevelCountsUnrated(t *testing.T) {
	ms := scoutMatches()
	ms[0].OpponentRank = 12 // the first match is against a top 15 team; the second rival is unranked

	kept, unrated := ScoutFilter{OpponentTop: 15}.SelectRated(ms)
	if !reflect.DeepEqual(matchIDs(kept), []string{scoutMatchOne.String()}) || !reflect.DeepEqual(matchIDs(unrated), []string{scoutMatchTwo.String()}) {
		t.Fatalf("top 15 kept %v and could not rate %v", matchIDs(kept), matchIDs(unrated))
	}
	if kept, unrated := (ScoutFilter{OpponentTop: 5}).SelectRated(ms); len(kept) != 0 || len(unrated) != 1 {
		t.Fatalf("top 5 kept %d and could not rate %d, want 0 and 1", len(kept), len(unrated))
	}
	if got := (ScoutFilter{}).Select(ms); len(got) != 2 {
		t.Fatalf("no level keeps %d matches, want both", len(got))
	}
	if _, err := ParseOpponentTop(10); err == nil {
		t.Fatal("ParseOpponentTop(10) accepted a band the filter does not offer")
	}
}

func TestScoutSelectByWhoseMatches(t *testing.T) {
	ms := scoutMatches()
	ms[1].OwnTeam = true // the second match was against the user's team: a scrim

	if got := matchIDs(ScoutFilter{Whose: WhoseOwn}.Select(ms)); !reflect.DeepEqual(got, []string{scoutMatchTwo.String()}) {
		t.Fatalf("own matches = %v", got)
	}
	if got := matchIDs(ScoutFilter{Whose: WhoseOthers}.Select(ms)); !reflect.DeepEqual(got, []string{scoutMatchOne.String()}) {
		t.Fatalf("other teams' matches = %v", got)
	}
	if _, err := ParseWhose("mine"); err == nil {
		t.Fatal("ParseWhose accepted an unknown value")
	}
}

func TestScoutRoundFilterCarriesTheRoundCriteria(t *testing.T) {
	ms := scoutMatches()
	f := ScoutFilter{Side: SideCT, Rounds: Filter{TeamKey: "ignored", Slots: []uint8{3}, Buys: []BuyType{BuyFull}, OpponentBuys: []BuyType{BuyEco}}}

	rf := f.RoundFilter(ms[0])
	if rf.TeamKey != "ct-start" || rf.Side != SideCT || rf.Slots != nil {
		t.Fatalf("RoundFilter = %+v, want the scouted team on CT without slots", rf)
	}
	// Alpha played CT in rounds 1 and 2; only round 2 was its full buy against an eco.
	if got := roundNumbers(f.rounds(ms[0])); !reflect.DeepEqual(got, []int{2}) {
		t.Fatalf("full against eco on CT = %v, want [2]", got)
	}
	if tend := AggregateMatches(ms[:1], f); tend.RoundCount != 1 {
		t.Fatalf("the tendencies count %d rounds, want the 1 the criteria keep", tend.RoundCount)
	}
}

func TestScoutSelectByTagsAndEvent(t *testing.T) {
	ms := scoutMatches()
	ms[0].Tags, ms[0].Event = []string{"Scrim"}, "IEM Beijing 2026"
	ms[1].Tags = []string{"oficial"}

	if got := matchIDs(ScoutFilter{Tags: []string{"scrim"}}.Select(ms)); !reflect.DeepEqual(got, []string{scoutMatchOne.String()}) {
		t.Fatalf("tag scrim keeps %v, want the first match whatever the case", got)
	}
	if got := matchIDs(ScoutFilter{ExcludeTags: []string{"Scrim"}}.Select(ms)); !reflect.DeepEqual(got, []string{scoutMatchTwo.String()}) {
		t.Fatalf("without scrims keeps %v", got)
	}
	if got := matchIDs(ScoutFilter{Events: []string{"iem beijing 2026"}}.Select(ms)); !reflect.DeepEqual(got, []string{scoutMatchOne.String()}) {
		t.Fatalf("event keeps %v; a match with no series is left out", got)
	}
}

func TestScoutRoundFilterSkipsExcludedTacticRounds(t *testing.T) {
	ms := scoutMatches()
	f := ScoutFilter{Side: SideCT, SkipRounds: map[uuid.UUID][]int{ms[0].MatchID: {2}}}
	// Alpha played CT in rounds 1 and 2; the excluded tactic took round 2.
	if got := roundNumbers(f.rounds(ms[0])); !reflect.DeepEqual(got, []int{1}) {
		t.Fatalf("CT rounds without the excluded tactic = %v, want [1]", got)
	}
}
