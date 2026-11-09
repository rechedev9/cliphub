package main

import (
	"testing"

	"github.com/rechedev9/cliphub/internal/tacticalplan"
)

func TestReplayRoundsScopeReadsAsTheReplayURL(t *testing.T) {
	doc := tacticalplan.Document{Players: []tacticalplan.Player{{Slot: 4, SteamID64: "76561198000000001"}}}
	in := replayRoundsIn{TeamKey: "ct-start", Side: "t", Result: "won", Buy: []string{"full"}, OpponentBuy: []string{"eco", "semi"},
		Players: []string{"76561198000000001"}, RoundFrom: 3, RoundTo: 12}
	values, err := in.values(doc)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := values.Encode(), "buy=full&opponent_buy=eco&opponent_buy=semi&outcome=win&round_from=3&round_to=12&side=T&slot=4&team=ct-start"; got != want {
		t.Fatalf("query = %s, want %s", got, want)
	}
	f, err := tacticalplan.FilterFromValues(values)
	if err != nil {
		t.Fatal(err)
	}
	if f.TeamKey != "ct-start" || f.Side != tacticalplan.SideT || len(f.OpponentBuys) != 2 || len(f.Slots) != 1 || f.Slots[0] != 4 {
		t.Fatalf("filter = %+v", f)
	}
}

func TestReplayRoundsScopeRefusesWhatTheBarCannotSay(t *testing.T) {
	doc := tacticalplan.Document{}
	for name, in := range map[string]replayRoundsIn{
		"a result with no one to win it": {Result: "won"},
		"a team key that is not a side":  {TeamKey: "FUT"},
		"a player who did not play":      {Players: []string{"76561198000000001"}},
	} {
		if _, err := in.values(doc); err == nil {
			t.Errorf("%s: no error", name)
		}
	}
}
