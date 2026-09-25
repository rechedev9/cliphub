package recapplan

import (
	"testing"

	"github.com/rechedev9/cliphub/internal/killplan"
)

// The final round keeps a second plus the full scoreboard after its last
// kill, so the outro never covers the final play nor flashes for a second.
func TestFinalRoundTailFitsTheScoreboardAfterTheLastKill(t *testing.T) {
	for _, tc := range []struct {
		name   string
		edit   func(*Facts, *Options)
		round  int
		end    int
		reason string
	}{
		{"late final kill extends the tail", func(*Facts, *Options) {}, 2, 19800, "scoreboard-tail"},
		{"early final kill keeps the round tail", func(f *Facts, _ *Options) { f.Rounds[2].Kills = f.Rounds[2].Kills[:1] }, 2, 19200, "round-tail"},
		{"demo end caps the extension", func(f *Facts, _ *Options) { f.EndTick = 19500 }, 2, 19500, "scoreboard-tail"},
		{"no scoreboard keeps the round tail", func(_ *Facts, o *Options) { o.Overlays.Scoreboard = false }, 2, 19200, "round-tail"},
		{"no safe tail trim keeps the round tail", func(_ *Facts, o *Options) { o.Editorial.AllowSafeTailTrim = false }, 2, 19200, "round-tail"},
		{"a dead POV keeps the death tail", func(f *Facts, _ *Options) { death := 18950; f.Rounds[2].DeathTick = &death }, 2, 19250, "death-tail-requires-certified-pov"},
		{"earlier rounds keep the round tail", func(f *Facts, _ *Options) {
			f.Rounds[0].Kills = []killplan.Kill{{Tick: 8400}}
		}, 0, 8700, "round-tail"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f, o := fixtureFacts(), fixtureOptions()
			o.Overlays.Scoreboard = true
			f.Rounds[2].Kills = []killplan.Kill{{Tick: 17800}, {Tick: 18900}}
			tc.edit(&f, &o)
			d, err := Plan(f, o, VoiceEvidence{Availability: "no_packets"}, nil, "facts")
			if err != nil {
				t.Fatal(err)
			}
			r := d.Rounds[tc.round]
			if r.RequestedEndTick != tc.end || r.CaptureEndTick != tc.end || r.EndReason != tc.reason {
				t.Fatalf("round %s end=%d reason=%q, want %d %q", r.ID, r.RequestedEndTick, r.EndReason, tc.end, tc.reason)
			}
		})
	}
}
