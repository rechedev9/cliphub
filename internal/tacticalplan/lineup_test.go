package tacticalplan

import (
	"reflect"
	"testing"
)

// subDocument is testDocument where Alpha fields a stand-in (SteamID 5, slot
// 4) instead of a2 in round 4.
func subDocument() Document {
	d := testDocument()
	d.Players = append(d.Players, Player{Slot: 4, SteamID64: "5", Name: "sub", TeamKey: "ct-start", StartSide: SideCT})
	d.Rounds[3].Players[1].Slot = 4
	return d
}

func keptRounds(d Document, f Filter) []int {
	out := []int{}
	for _, r := range f.Apply(d) {
		out = append(out, r.Number)
	}
	return out
}

func TestLineupKeepsARoundOnlyWhenEveryTeamPlayerInItIsChecked(t *testing.T) {
	d := subDocument()
	for _, tc := range []struct {
		name   string
		lineup Lineup
		want   []int
	}{
		{"no lineup keeps every round", nil, []int{1, 2, 3, 4}},
		{"the regulars drop the stand-in's round", Lineup{"1", "2"}, []int{1, 2, 3}},
		{"the stand-in checked keeps it too", Lineup{"1", "2", "5"}, []int{1, 2, 3, 4}},
		{"without a2 only the stand-in's round is left", Lineup{"1", "5"}, []int{4}},
		{"a lineup without the team keeps nothing", Lineup{"3", "4"}, []int{}},
	} {
		got := keptRounds(d, Filter{TeamKey: "ct-start", Lineup: tc.lineup})
		if !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%s: rounds = %v, want %v", tc.name, got, tc.want)
		}
	}
	// Beta's lineup ignores Alpha's stand-in: only Beta's own players count.
	if got := keptRounds(d, Filter{TeamKey: "t-start", Lineup: Lineup{"3", "4"}}); !reflect.DeepEqual(got, []int{1, 2, 3, 4}) {
		t.Errorf("rival lineup = %v, want every round", got)
	}
	if !(Lineup{"1", "5"}).PlayedIn(d, "ct-start") || (Lineup{"9"}).PlayedIn(d, "ct-start") {
		t.Error("PlayedIn must hold for a lineup that played a round and fail for one that played none")
	}
}

// The lineup narrows every number of the scout to the kept rounds, while the
// match score still counts the whole match.
func TestScoutWithALineupCountsOnlyItsRounds(t *testing.T) {
	m := ScoutMatch{MatchID: scoutMatchOne, TeamKey: "ct-start", Document: subDocument()}
	report := Scout([]ScoutMatch{m}, ScoutFilter{Lineup: Lineup{"1", "2"}})
	if report.Tendencies.RoundCount != 3 || len(report.Matches) != 1 || report.Matches[0].Rounds != 3 {
		t.Fatalf("scout = %d rounds, matches %+v; want the 3 rounds without the stand-in", report.Tendencies.RoundCount, report.Matches)
	}
	if s := report.Matches[0]; s.Score != 2 || s.OpponentScore != 2 {
		t.Fatalf("match score = %d-%d, want the full 2-2", s.Score, s.OpponentScore)
	}
	if len(report.Filter.Lineup) != 2 {
		t.Fatalf("report filter = %+v, want the lineup echoed", report.Filter)
	}
}
