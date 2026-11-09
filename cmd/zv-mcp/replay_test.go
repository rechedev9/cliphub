package main

import (
	"net/url"
	"testing"

	"github.com/rechedev9/cliphub/internal/tacticalplan"
	"github.com/rechedev9/cliphub/internal/tuiclient"
)

func TestReplayRoundsScopeReadsAsTheReplayURL(t *testing.T) {
	doc := tacticalplan.Document{Players: []tacticalplan.Player{{Slot: 4, SteamID64: "76561198000000001"}}}
	in := replayRoundsIn{TeamKey: "ct-start", Side: "t", Result: "won", Buy: []string{"full"}, OpponentBuy: []string{"eco", "semi"},
		Players: []string{"76561198000000001"}, Focus: []string{"76561198000000001"}, RoundFrom: 3, RoundTo: 12}
	values, err := in.values(doc)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := values.Encode(), "buy=full&opponent_buy=eco&opponent_buy=semi&outcome=win&round_from=3&round_to=12&side=T&slot=4&subject=4&team=ct-start"; got != want {
		t.Fatalf("query = %s, want %s", got, want)
	}
	f, err := tacticalplan.FilterFromValues(values)
	if err != nil {
		t.Fatal(err)
	}
	if f.TeamKey != "ct-start" || f.Side != tacticalplan.SideT || len(f.OpponentBuys) != 2 || len(f.Slots) != 1 || f.Slots[0] != 4 || len(f.Subjects) != 1 || f.Subjects[0] != 4 {
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

func TestSetposReadsTheNearestSampleAndNormalizesTheAngles(t *testing.T) {
	frames := []tacticalplan.Frame{
		{Tick: 100, Samples: []tacticalplan.Sample{{Slot: 4, X: 1, Y: 2, Z: 3}}},
		{Tick: 108, Samples: []tacticalplan.Sample{{Slot: 4, X: -1012.034, Y: 20, Z: -167.97, Yaw: 291.89, Pitch: 352.62}}},
	}
	frame, sample, ok := setposAt(frames, 4, 106)
	if !ok || frame.Tick != 108 {
		t.Fatalf("frame = %+v ok=%v, want tick 108", frame, ok)
	}
	if got, want := setposLine(sample), "setpos -1012.03 20.00 -167.97;setang -7.38 -68.11 0"; got != want {
		t.Fatalf("setpos = %q, want %q", got, want)
	}
	if _, _, ok := setposAt(frames, 9, 106); ok {
		t.Fatal("a slot with no sample answered")
	}
}

func TestPlaylistPathReadsAsTheReplayURL(t *testing.T) {
	const match = "82b2365b-3c63-48d4-8d9c-00342b71c5a1"
	got, err := playlistPath([]string{match + "/4", " " + match + "/11"}, "FUT T Ancient")
	if err != nil {
		t.Fatal(err)
	}
	if want := "/replay?r=" + match + "%3A4&r=" + match + "%3A11&t=FUT+T+Ancient"; got != want {
		t.Fatalf("path = %s, want %s", got, want)
	}
	for _, refs := range [][]string{nil, {"nope"}, {match + "/0"}, {"x/3"}} {
		if _, err := playlistPath(refs, ""); err == nil {
			t.Errorf("%v: no error", refs)
		}
	}
}

func TestOverlayNamesItsRoundsAndOpensThemAsRecorridos(t *testing.T) {
	got := tuiclient.MatchOverlay{Map: "de_mirage", Overlay: &tacticalplan.Overlay{Limit: 12, Total: 20, Rounds: []tacticalplan.OverlayRound{
		{Round: 3, Side: tacticalplan.SideT, Winner: tacticalplan.SideCT, DurationSeconds: 41.5, Tracks: []tacticalplan.OverlayTrack{{Name: "ana"}}},
	}}}
	out := overlayOut("m1", got, url.Values{"subject": {"4"}})
	if out.Superposed != 1 || out.Total != 20 || out.Cap != 12 || out.Rounds[0].Ref != "m1/3" || len(out.Rounds[0].Players) != 1 || out.Rounds[0].Players[0] != "ana" {
		t.Fatalf("overlay = %+v", out)
	}
	if want := "/partidas/m1?forma=recorridos&subject=4&tab=replay"; out.StudioPath != want {
		t.Fatalf("studio_path = %s, want %s", out.StudioPath, want)
	}
	if empty := overlayOut("m1", tuiclient.MatchOverlay{}, nil); empty.Rounds == nil || empty.Superposed != 0 {
		t.Fatalf("empty overlay = %+v", empty)
	}
}
