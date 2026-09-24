package recapplan

import (
	"encoding/binary"
	"testing"

	"github.com/rechedev9/cliphub/internal/sharecode"
)

const observedTestCode = "CSGO-WsnnD-eHaMw-QNDf9-oxuDh-ydOUD"

// observedPlan uses the current capture policy without voice, so crosshair
// evidence is the only source of blockers.
func observedPlan(t *testing.T, facts Facts, tailTrim bool) Document {
	t.Helper()
	o := DefaultOptions()
	o.Audio.Voice.Enabled = false
	o.Editorial.AllowSafeTailTrim = tailTrim
	d, err := Plan(facts, o, VoiceEvidence{Availability: "not_requested"}, nil, "facts")
	if err != nil {
		t.Fatal(err)
	}
	return d
}

// Studio 5.2.1, 2026-09-24: every round of a demo recorded after CS2's
// crosshair rework was blocked. The gate accepted only the version-1 payload
// that sharecode decodes, which only a provided code needs, so any other
// layout blocks every round. CS2 draws the observed code itself.
func TestObservedCrosshairAcceptsCodesClipHubCannotDecode(t *testing.T) {
	var b [18]byte
	copy(b[1:], []byte{2, 1, 30, 0, 200, 50, 50, 50, 1, 0, 0, 1, 5, 8, 3, 0, 0})
	for _, v := range b[1:] {
		b[0] += v
	}
	newVersion := sharecode.Encode(sharecode.Match{MatchID: binary.LittleEndian.Uint64(b[:8]), OutcomeID: binary.LittleEndian.Uint64(b[8:16]), TokenID: binary.LittleEndian.Uint16(b[16:])})
	for name, code := range map[string]string{
		"new payload version": newVersion,
		"longer code":         "CSGO-WsnnD-eHaMw-QNDf9-oxuDh-ydOUD-WsnnD-eHaMw",
	} {
		t.Run(name, func(t *testing.T) {
			if ValidCrosshairCode(code) {
				t.Fatalf("%s must be a code the version-1 decoder rejects", code)
			}
			facts := fixtureFacts()
			facts.Crosshairs = []CrosshairSample{{Tick: 0, Code: code}}
			if d := observedPlan(t, facts, true); len(d.Blockers) != 0 {
				t.Fatalf("observed code blocked the plan: %+v", d.Blockers)
			}
		})
	}
}

// The player leaving after the certified POV ended must not block the video:
// the capture trims that tail once the target is no longer observed.
func TestObservedCrosshairIgnoresTrimmableTail(t *testing.T) {
	// Measured on a local 64-tick demo: adamS died at 106542 in the last
	// round and his sample turned empty at 106688 (he left the server),
	// inside the three-second death tail.
	left := fixtureFacts()
	left.TickRate, left.EndTick = 64, 108535
	round := left.Rounds[1]
	death := 106542
	round.ID, round.Number, round.StartTick, round.FreezeEndTick, round.RoundEndTick, round.NextStartTick, round.DeathTick = "round-019", 19, 103578, 104858, 108145, 0, &death
	left.Rounds = []RoundFacts{round}
	left.Crosshairs = []CrosshairSample{{Tick: 2711, Code: "CSGO-bjS8N-BmWXz-RSJ55-LV7H3-jPD3J"}, {Tick: 106688, Code: ""}}

	// Alive at the final round end (19000), gone before the round tail ends.
	ended := fixtureFacts()
	ended.Crosshairs = []CrosshairSample{{Tick: 0, Code: observedTestCode}, {Tick: 19150, Code: ""}}

	for name, facts := range map[string]Facts{"left after dying": left, "left after the match": ended} {
		t.Run(name, func(t *testing.T) {
			d := observedPlan(t, facts, true)
			if len(d.Blockers) != 0 {
				t.Fatalf("trimmable tail blocked the plan: %+v", d.Blockers)
			}
			last := d.Rounds[len(d.Rounds)-1]
			gap := facts.Crosshairs[len(facts.Crosshairs)-1].Tick
			if gap <= last.LiveEndTick || gap >= last.RequestedEndTick {
				t.Fatalf("gap %d must fall in the requested tail (%d, %d)", gap, last.LiveEndTick, last.RequestedEndTick)
			}
			if d := observedPlan(t, facts, false); len(d.Blockers) != 1 {
				t.Fatalf("without tail trim the tail is captured and needs the crosshair: %+v", d.Blockers)
			}
		})
	}
}

func TestObservedCrosshairBlocksCertifiedGapsWithOneActionableNotice(t *testing.T) {
	const action = ". ClipHub no graba con otra mira: elige otro jugador o importa otra demo de la partida."
	for _, tc := range []struct {
		name    string
		samples []CrosshairSample
		want    string
	}{
		{"gap during live play", []CrosshairSample{{0, observedTestCode}, {12000, ""}, {12100, observedTestCode}}, "La demo no incluye la mira del jugador en la ronda 2" + action},
		{"gap at the death tick", []CrosshairSample{{0, observedTestCode}, {14000, ""}, {14100, observedTestCode}}, "La demo no incluye la mira del jugador en la ronda 2" + action},
		{"no code when the freeze lead-in starts", []CrosshairSample{{2400, observedTestCode}}, "La demo no incluye la mira del jugador en la ronda 1" + action},
		{"several rounds", []CrosshairSample{{0, ""}, {10000, observedTestCode}, {17500, ""}}, "La demo no incluye la mira del jugador en las rondas 1 y 25" + action},
		{"no code in the demo", nil, "La demo no incluye la mira del jugador" + action},
	} {
		t.Run(tc.name, func(t *testing.T) {
			facts := fixtureFacts()
			facts.Crosshairs = tc.samples
			d := observedPlan(t, facts, true)
			if len(d.Blockers) != 1 || d.Blockers[0].Code != ErrPOVContract || d.Blockers[0].Message != tc.want {
				t.Fatalf("blockers = %+v, want one %q", d.Blockers, tc.want)
			}
		})
	}

	facts := fixtureFacts()
	facts.Crosshairs = []CrosshairSample{{0, observedTestCode}, {14001, ""}, {14100, observedTestCode}}
	if d := observedPlan(t, facts, true); len(d.Blockers) != 0 {
		t.Fatalf("gap after the death tick blocked the plan: %+v", d.Blockers)
	}
}
