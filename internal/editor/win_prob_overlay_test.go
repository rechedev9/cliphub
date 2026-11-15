package editor

import (
	"math"
	"strings"
	"testing"

	"github.com/rechedev9/cliphub/internal/killplan"
	"github.com/rechedev9/cliphub/internal/recording"
)

func TestWinProbabilityFollowsEachPartClock(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	fixture := replayFixture(dir)
	fixture.Plan.Segments[0].WinProb = []killplan.WinProbSpan{{TickStart: 14300, TickEnd: 14400, CTPct: 40}, {TickStart: 14400, TickEnd: 14460, CTPct: 35}}
	fixture.Plan.Segments[1].WinProb = []killplan.WinProbSpan{{TickStart: 22300, TickEnd: 22406, CTPct: 70}}
	opts := testManifestOptions(dir, nil)
	opts.CompileSegments, opts.VictimReplay = true, true
	opts.Overlays = ShortOverlays{WinProbability: winProbCues(fixture.Plan)}
	short := mustBuildManifest(t, fixture, opts).Shorts[0]
	start := recording.EffectiveRecordStartTick(fixture.Plan.Segments[0], 64)
	if got := short.Overlays.WinProbability["seg-001"]; len(got) != 2 || math.Abs(got[0].StartSeconds-float64(14300-start)/64) > 1e-9 {
		t.Fatalf("seg-001 cues = %+v, want them on the clip clock from tick %d", got, start)
	}
	effects := generatedWinProbEffects(short)
	values := map[string]Effect{}
	for _, e := range effects {
		values[e.Value] = e
	}
	if values["CT 40 %"].FadeInSeconds == 0 || values["CT 35 %"].FadeInSeconds != 0 {
		t.Errorf("only the first value of a run should fade in: %+v / %+v", values["CT 40 %"], values["CT 35 %"])
	}
	seg2, ok := values["30 % T"]
	if !ok {
		t.Fatalf("seg-002 has no T plate: %+v", effects)
	}
	part := short.Parts[2]
	segStart := recording.EffectiveRecordStartTick(fixture.Plan.Segments[1], 64)
	if want := part.TimelineStartSeconds + float64(22300-segStart)/64; math.Abs(seg2.StartSeconds-want) > 1e-9 || seg2.EndSeconds > part.TimelineStartSeconds+part.DurationSeconds+1e-9 {
		t.Errorf("seg-002 plate %v..%v, want from %v inside its part", seg2.StartSeconds, seg2.EndSeconds, want)
	}
	short.Effects = effects
	if command := strings.Join(BuildFFmpegCommand("ffmpeg", short), " "); !strings.Contains(command, "w/2-24-text_w") || !strings.Contains(command, "expansion=none") {
		t.Errorf("render command has no win chance plates:\n%s", command)
	}
	for _, e := range effects {
		if e.StartSeconds >= short.Parts[1].TimelineStartSeconds && e.EndSeconds <= part.TimelineStartSeconds {
			t.Errorf("win chance drawn over the victim replay: %+v", e)
		}
	}
}

func TestWinProbabilityWaitsForTheHookAndIsOffByDefault(t *testing.T) {
	t.Parallel()
	short := ShortEdit{
		SegmentID: "seg-001", OutputFormat: OutputFormatShort9x16, DurationSeconds: 10, HookText: true,
		Overlays: ShortOverlays{WinProbability: map[string][]WinProbCue{"seg-001": {{StartSeconds: 0.5, EndSeconds: 6, CTPct: 52}}}},
	}
	effects := generatedWinProbEffects(short)
	if len(effects) != 3 || effects[0].StartSeconds != winProbHookSeconds || effects[0].Value != "PROB. DE VICTORIA" {
		t.Fatalf("effects = %+v, want the label and two plates after the hook", effects)
	}
	short.Overlays.WinProbability = nil
	if effects := generatedWinProbEffects(short); effects != nil {
		t.Fatalf("effects without the option = %+v", effects)
	}
}
