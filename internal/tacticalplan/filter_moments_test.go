package tacticalplan

import (
	"net/url"
	"reflect"
	"testing"
)

func TestFilterAWPReadsTheFlagAndOlderKills(t *testing.T) {
	d := scoutDocument() // round 3: slot 0 (Alpha, T) kills with an AWP
	d.Rounds[0].Players[2].AWP = true // round 1: slot 2 (Beta, T) bought one
	alpha := "ct-start"

	if got := roundNumbers(Filter{TeamKey: alpha, AWP: PresenceWith}.Apply(d)); !reflect.DeepEqual(got, []int{3}) {
		t.Fatalf("Alpha with an AWP = %v, want [3] from the kill", got)
	}
	if got := roundNumbers(Filter{TeamKey: alpha, OpponentAWP: PresenceWith}.Apply(d)); !reflect.DeepEqual(got, []int{1}) {
		t.Fatalf("rival with an AWP = %v, want [1] from the flag", got)
	}
	if got := roundNumbers(Filter{TeamKey: alpha, AWP: PresenceWithout, OpponentAWP: PresenceWithout}.Apply(d)); !reflect.DeepEqual(got, []int{2, 4}) {
		t.Fatalf("no AWP on either side = %v, want [2 4]", got)
	}
	if got := roundNumbers(Filter{AWP: PresenceWith}.Apply(d)); !reflect.DeepEqual(got, []int{1, 3}) {
		t.Fatalf("side-agnostic AWP = %v, want [1 3]", got)
	}
}

func TestFilterAdvantageFollowsTheAliveCount(t *testing.T) {
	d := testDocument()
	// Round 2: Beta (T) loses slot 2, then slot 3, after the round ended.
	d.Rounds[1].Events = []Event{
		{Tick: 6000, Kind: EventKill, ActorSlot: slotPtr(0), TargetSlot: slotPtr(2)},
		{Tick: 9500, Kind: EventKill, ActorSlot: slotPtr(0), TargetSlot: slotPtr(3)},
	}
	alpha, beta := "ct-start", "t-start"
	two := []Advantage{{Own: 2, Rival: 1}}

	if got := roundNumbers(Filter{TeamKey: alpha, Advantages: two}.Apply(d)); !reflect.DeepEqual(got, []int{2}) {
		t.Fatalf("Alpha 2v1 = %v, want [2]", got)
	}
	if got := roundNumbers(Filter{TeamKey: beta, Advantages: two}.Apply(d)); len(got) != 0 {
		t.Fatalf("Beta 2v1 = %v, want none: Beta was the one down", got)
	}
	if got := roundNumbers(Filter{TeamKey: beta, Advantages: []Advantage{{Own: 1, Rival: 2}}}.Apply(d)); !reflect.DeepEqual(got, []int{2}) {
		t.Fatalf("Beta 1v2 = %v, want [2]", got)
	}
	if got := roundNumbers(Filter{Advantages: []Advantage{{Own: 1, Rival: 2}}}.Apply(d)); !reflect.DeepEqual(got, []int{2}) {
		t.Fatalf("side-agnostic 1v2 = %v, want [2]", got)
	}
	if got := roundNumbers(Filter{TeamKey: alpha, Advantages: []Advantage{{Own: 2, Rival: 0}}}.Apply(d)); len(got) != 0 {
		t.Fatalf("2v0 = %v, want none: the second kill came after the round ended", got)
	}
}

func TestParseAdvantage(t *testing.T) {
	if a, err := ParseAdvantage(" 4V5 "); err != nil || a != (Advantage{Own: 4, Rival: 5}) || a.String() != "4v5" {
		t.Fatalf("ParseAdvantage(4V5) = %v, %v", a, err)
	}
	for _, bad := range []string{"", "45", "6v5", "4v-1", "0v0", "av5"} {
		if _, err := ParseAdvantage(bad); err == nil {
			t.Errorf("ParseAdvantage(%q) accepted", bad)
		}
	}
}

func TestFilterFromValuesReadsAWPAndAdvantage(t *testing.T) {
	f, err := FilterFromValues(url.Values{"awp": {"with"}, "opponent_awp": {"without"}, "advantage": {"4v5,5v4"}})
	if err != nil {
		t.Fatal(err)
	}
	want := Filter{AWP: PresenceWith, OpponentAWP: PresenceWithout, Advantages: []Advantage{{4, 5}, {5, 4}}}
	if !reflect.DeepEqual(f, want) || f.Empty() {
		t.Fatalf("FilterFromValues = %+v, want %+v", f, want)
	}
	for _, bad := range []url.Values{{"awp": {"yes"}}, {"opponent_awp": {"1"}}, {"advantage": {"9v9"}}} {
		if _, err := FilterFromValues(bad); err == nil {
			t.Errorf("FilterFromValues(%v) accepted", bad)
		}
	}
}
