package main

import (
	"math"
	"net/http"
	"net/url"
	"slices"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/rechedev9/cliphub/internal/tacticalplan"
	"github.com/rechedev9/cliphub/internal/tuiclient"
)

func TestReplayRoundsScopeReadsAsTheReplayURL(t *testing.T) {
	doc := tacticalplan.Document{Players: []tacticalplan.Player{{Slot: 4, SteamID64: "76561198000000001"}}}
	in := replayScopeIn{TeamKey: "ct-start", Side: "t", Result: "won", Buy: []string{"full"}, OpponentBuy: []string{"eco", "semi"},
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
	for name, in := range map[string]replayScopeIn{
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
		{Tick: 100, Samples: []tacticalplan.Sample{{Slot: 4, X: 1, Y: 2, Z: 3}, {Slot: 5, X: 300, Y: 20, Z: -167.97, Yaw: 291.89, Pitch: 352.62}}},
		{Tick: 108, Samples: []tacticalplan.Sample{{Slot: 4, X: -1012.034, Y: 20, Z: -167.97, Yaw: 291.89, Pitch: 352.62}}},
	}
	frame, sample, ok := setposAt(frames, 4, 106)
	if !ok || frame.Tick != 108 {
		t.Fatalf("frame = %+v ok=%v, want tick 108", frame, ok)
	}
	line, ok := setposFor([3]float64{sample.X, sample.Y, sample.Z}, sample.Pitch, sample.Yaw)
	if want := "setpos -1012.03 20.00 -167.97;setang -7.38 -68.11 0"; !ok || line != want {
		t.Fatalf("setpos = %q, want %q", line, want)
	}
	if _, _, ok := setposAt(frames, 9, 106); ok {
		t.Fatal("a slot with no sample answered")
	}
}

// The web pins the same ties in setpos.test.ts: nearest sample, a tie takes the earlier one, never interpolated.
func TestSetposTickChoiceMatchesTheWeb(t *testing.T) {
	frames := []tacticalplan.Frame{
		{Tick: 1000, Samples: []tacticalplan.Sample{{Slot: 4, X: -1000}, {Slot: 5, X: 300}}},
		{Tick: 1008, Samples: []tacticalplan.Sample{{Slot: 4, X: -1012.03}}},
	}
	for _, tc := range []struct {
		slot uint8
		tick int
		want int
	}{{4, 1004, 1000}, {4, 1005, 1008}, {4, 900, 1000}, {4, 5000, 1008}, {5, 1007, 1000}} {
		if frame, _, ok := setposAt(frames, tc.slot, tc.tick); !ok || frame.Tick != tc.want {
			t.Errorf("slot %d at %d read tick %d (ok=%v), want %d", tc.slot, tc.tick, frame.Tick, ok, tc.want)
		}
	}
}

func TestSetposForRefusesWhatIsNotANumber(t *testing.T) {
	if _, ok := setposFor([3]float64{math.NaN(), 0, 0}, 0, 0); ok {
		t.Error("NaN origin accepted")
	}
	if _, ok := setposFor([3]float64{0, 0, 0}, 0, math.Inf(1)); ok {
		t.Error("infinite yaw accepted")
	}
	// The web pins the same line for the same raw throw floats in setpos.test.ts.
	got, ok := setposFor([3]float64{-383.62762451171875, -902.1229858398438, -415.96875}, -1.6500091552734375, -41.036346435546875)
	if want := "setpos -383.63 -902.12 -415.97;setang -1.65 -41.04 0"; !ok || got != want {
		t.Errorf("throw setpos = %q, want %q", got, want)
	}
	if got, _ := setposFor([3]float64{-1234567.891, 1000, -0.004}, 0, math.Copysign(0, -1)); got != "setpos -1234567.89 1000.00 0.00;setang 0.00 0.00 0" {
		t.Errorf("no thousands separator and no -0: %q", got)
	}
}

func TestPlayerThrowsListsTheThrowsOfOneSlotInOrder(t *testing.T) {
	slot, other := uint8(4), uint8(5)
	round := tacticalplan.Round{TickFreezeEnd: 960, Events: []tacticalplan.Event{
		{Kind: tacticalplan.EventFlash, Tick: 1500, ActorSlot: &slot, Place: "Outside", Throw: &tacticalplan.Throw{Tick: 1400, Pos: [3]float64{1, 2, 3}, Yaw: 291.89, Pitch: 352.62}},
		{Kind: tacticalplan.EventSmoke, Tick: 1300, ActorSlot: &slot, Throw: &tacticalplan.Throw{Tick: 1088, Pos: [3]float64{-383.628, -902.123, -415.969}, Yaw: -41.036, Pitch: -1.65}},
		{Kind: tacticalplan.EventSmoke, Tick: 1300, ActorSlot: &other, Throw: &tacticalplan.Throw{Tick: 1090}},
		{Kind: tacticalplan.EventHE, Tick: 1600, ActorSlot: &slot},
		{Kind: tacticalplan.EventKill, Tick: 1700, ActorSlot: &slot, Throw: &tacticalplan.Throw{Tick: 1700}},
		{Kind: tacticalplan.EventMolotov, Tick: 1800, ActorSlot: &slot, Throw: &tacticalplan.Throw{Tick: 1750, Pos: [3]float64{math.NaN(), 0, 0}}},
	}}
	got := playerThrows(round, slot, 64)
	want := []playerThrowOut{
		{Second: 2, Tick: 1088, Kind: "smoke", Setpos: "setpos -383.63 -902.12 -415.97;setang -1.65 -41.04 0"},
		{Second: 6.88, Tick: 1400, Kind: "flash", From: "Outside", Setpos: "setpos 1.00 2.00 3.00;setang -7.38 -68.11 0"},
	}
	if !slices.Equal(got, want) {
		t.Fatalf("throws = %+v, want %+v", got, want)
	}
	if len(playerThrows(round, 9, 64)) != 0 {
		t.Fatal("a slot that threw nothing got throws")
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

func TestReplayRoundsRefusesAFocusOutsideTheScopedTeam(t *testing.T) {
	doc := tacticalplan.Document{
		Players: []tacticalplan.Player{{Slot: 1, SteamID64: "76561198000000001"}, {Slot: 7, SteamID64: "76561198000000007"}},
		Teams:   []tacticalplan.Team{{Key: "ct-start", Slots: []uint8{1}}, {Key: "t-start", Slots: []uint8{7}}},
	}
	if _, err := (replayScopeIn{TeamKey: "ct-start", Focus: []string{"76561198000000007"}}).values(doc); err == nil {
		t.Fatal("a rival in focus was accepted: the overlay would draw no one")
	}
	values, err := (replayScopeIn{TeamKey: "t-start", Focus: []string{"76561198000000007"}}).values(doc)
	if err != nil || values.Get("subject") != "7" {
		t.Fatalf("own player in focus: %v, %v", values, err)
	}
	if _, err := (replayScopeIn{Focus: []string{"76561198000000007"}}).values(doc); err != nil {
		t.Fatalf("focus without a team: %v", err)
	}
}

func TestPlayerSetposReadsTheSampleAndListsTheThrowsOnRequest(t *testing.T) {
	rec := &recorder{seen: map[string]string{}}
	mux := http.NewServeMux()
	rec.handle(mux, "GET /api/library/matches/{id}/tactical", `{"demo":{"map":"de_nuke","tickrate":64},
		"players":[{"slot":4,"steamid64":"`+testSteamID+`","name":"mopoz","team_key":"ct-start"}],"rounds":[{"number":3,"tick_freeze_end":960}]}`)
	rec.handle(mux, "GET /api/library/matches/{id}/tactical/rounds/{round}", `{"round":{"number":3,"tick_freeze_end":960,"events":[
		{"tick":1300,"kind":"smoke","actor_slot":4,"place":"Outside","throw":{"tick":1088,"pos":[-383.62762451171875,-902.1229858398438,-415.96875],"yaw":-41.036346435546875,"pitch":-1.6500091552734375}},
		{"tick":1400,"kind":"kill","actor_slot":4,"target_slot":2}]},
		"frames":[{"tick":1000,"samples":[{"slot":4,"x":-1000,"y":20,"z":-167.97,"yaw":291.89,"pitch":-7.38,"flags":1}]},
		{"tick":1008,"samples":[{"slot":4,"x":-1012.03,"y":20,"z":-167.97,"yaw":291.89,"pitch":-7.38,"flags":1}]}]}`)
	cs := serveTools(t, mux, func(server *mcp.Server, s *studio) { registerReplayTools(server, s) })
	args := map[string]any{"match_id": testMatchID, "round": 3, "steamid": testSteamID, "second": 0.9}
	var plain playerSetposOut
	callTool(t, cs, "player_setpos", args, &plain)
	if plain.Tick != 1008 || plain.Setpos != "setpos -1012.03 20.00 -167.97;setang -7.38 -68.11 0" || len(plain.Throws) != 0 {
		t.Fatalf("setpos = %+v", plain)
	}
	args["grenades"] = true
	var withThrows playerSetposOut
	callTool(t, cs, "player_setpos", args, &withThrows)
	want := playerThrowOut{Second: 2, Tick: 1088, Kind: "smoke", From: "Outside", Setpos: "setpos -383.63 -902.12 -415.97;setang -1.65 -41.04 0"}
	if len(withThrows.Throws) != 1 || withThrows.Throws[0] != want {
		t.Fatalf("throws = %+v, want [%+v]", withThrows.Throws, want)
	}
}
