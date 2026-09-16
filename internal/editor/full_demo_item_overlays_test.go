package editor

import (
	"math"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rechedev9/cliphub/internal/demooverlay"
	"github.com/rechedev9/cliphub/internal/recapplan"
	"github.com/rechedev9/cliphub/internal/recording"
)

func fullDemoItemOverlayFixtureEffects(dir string) []Effect {
	return []Effect{
		{
			Type: EffectImage, Path: filepath.Join(dir, "intro.png"), Source: "full-demo-intro",
			X: "0", Y: "0", Width: demooverlay.FrameWidth, Height: demooverlay.FrameHeight,
			StartSeconds: 0, EndSeconds: 5,
			FadeInSeconds: demooverlay.IntroOverlaySlideSeconds, FadeOutSeconds: 0.35,
		},
		{
			Type: EffectImage, Path: filepath.Join(dir, "outro.png"), Source: "full-demo-outro",
			X: "0", Y: "0", Width: demooverlay.FrameWidth, Height: demooverlay.FrameHeight,
			StartSeconds: 52, EndSeconds: 60,
			FadeInSeconds: demooverlay.IntroOverlaySlideSeconds,
		},
	}
}

func fullDemoItemOverlayFixtureShort(dir string) ShortEdit {
	options := recapplan.DefaultOptions()
	options.Capture.HUDProfile = "native-clean-spectator"
	options.Overlays.HUDTheme = ""
	return ShortEdit{
		Preset:          PresetGameplayPOV60,
		OutputFormat:    OutputFormatLandscape16x9,
		OutputFPS:       60,
		Output:          filepath.Join(dir, "program.nut"),
		DurationSeconds: 60,
		Parts: []ShortPart{
			{SegmentID: "round-001", Input: filepath.Join(dir, "game.nut"), DurationSeconds: 60, TickStart: 64, TickEnd: 64 + 64*60},
		},
		Effects: fullDemoItemOverlayFixtureEffects(dir),
		FullDemo: &FullDemoRenderEvidence{
			SchemaVersion: "1.0",
			Effective:     recapplan.Document{Clock: recapplan.Clock{TickRate: 64}, Options: options},
		},
		fullDemo: &fullDemoRenderContext{
			ffmpeg:    "ffmpeg",
			workDir:   filepath.Join(dir, "prepared"),
			recording: recording.RecordingResult{Plan: recording.RecordingPlan{Tickrate: 64, DemoDurationTicks: 64 * 60, Segments: []recording.RecordingSegment{{ID: "round-001", TickStart: 64}}}},
		},
	}
}

func TestFullDemoItemOverlayEligibility(t *testing.T) {
	dir := t.TempDir()
	supported := fullDemoItemOverlayFixtureShort(dir)
	if !fullDemoItemOverlayEligible(supported) {
		t.Fatal("intro+outro image overlays should be eligible")
	}

	none := supported
	none.Effects = nil
	if !fullDemoItemOverlayEligible(none) {
		t.Fatal("a render without effects should take the copy path")
	}

	unsupported := supported
	unsupported.Effects = append(append([]Effect{}, supported.Effects...), Effect{Type: EffectKillfeed, StartSeconds: 1, EndSeconds: 2})
	if fullDemoItemOverlayEligible(unsupported) {
		t.Fatal("a killfeed effect must keep the legacy global pass")
	}

	missingPath := supported
	missingPath.Effects = fullDemoItemOverlayFixtureEffects(dir)
	missingPath.Effects[0].Path = ""
	if fullDemoItemOverlayEligible(missingPath) {
		t.Fatal("an overlay without a materialized still must fall back")
	}

	// The cover first-frame freeze is a program-global n-based clause; the
	// item graph cannot express it, so it must fall back even with valid stills.
	cover := supported
	cover.CoverFirstFrame = true
	if fullDemoItemOverlayEligible(cover) {
		t.Fatal("cover first frame must keep the legacy global pass")
	}

	for _, bad := range []float64{math.NaN(), math.Inf(1), math.Inf(-1)} {
		nonfinite := supported
		nonfinite.Effects = fullDemoItemOverlayFixtureEffects(dir)
		nonfinite.Effects[0].StartSeconds = bad
		if fullDemoItemOverlayEligible(nonfinite) {
			t.Fatalf("non-finite window %v must fall back", bad)
		}
	}

	// Negative and zero-width windows are handled exactly by the reused legacy
	// clamping (buildImageOverlayFilter / betweenExpression), so they stay
	// eligible instead of silently changing behavior.
	negative := supported
	negative.Effects = fullDemoItemOverlayFixtureEffects(dir)
	negative.Effects[0].StartSeconds = -1
	negative.Effects[1].StartSeconds, negative.Effects[1].EndSeconds = 52, 52
	if !fullDemoItemOverlayEligible(negative) {
		t.Fatal("negative/zero-width windows are clamped by the reused legacy graph and should stay eligible")
	}

	// Effect order is preserved by reusing appendCompilationProgramVideo, so a
	// reversed order stays eligible instead of being silently normalized.
	reversed := supported
	reversed.Effects = []Effect{supported.Effects[1], supported.Effects[0]}
	if !fullDemoItemOverlayEligible(reversed) {
		t.Fatal("reversed global overlays should stay eligible and keep their order")
	}

	portrait := supported
	portrait.OutputFormat = OutputFormatShort9x16
	if fullDemoItemOverlayEligible(portrait) {
		t.Fatal("non-landscape output must keep the legacy path")
	}

	noFullDemo := supported
	noFullDemo.FullDemo = nil
	if fullDemoItemOverlayEligible(noFullDemo) {
		t.Fatal("a non-FullDemo short must never use item overlays")
	}
}

func TestFullDemoProgramCommandCopiesOnlyPreparedItemOverlays(t *testing.T) {
	dir := t.TempDir()
	short := fullDemoItemOverlayFixtureShort(dir)

	// Before preparation the concat list still names raw parts; copying them
	// would drop the overlays, so the command must keep the legacy global pass.
	unprepared := BuildFFmpegCommand("ffmpeg", short)
	unpreparedJoined := strings.Join(unprepared, " ")
	if strings.Contains(unpreparedJoined, "-c:v copy") {
		t.Fatalf("unprepared program command copied raw parts:\n%s", unpreparedJoined)
	}
	for _, want := range []string{"-filter_complex", "gblur=sigma=", "pow(1-(t-", filepath.Join(dir, "intro.png"), filepath.Join(dir, "outro.png")} {
		if !strings.Contains(unpreparedJoined, want) {
			t.Fatalf("legacy program command missing %q:\n%s", want, unpreparedJoined)
		}
	}

	short.fullDemo.preparedInputs = []string{filepath.Join(dir, "item-000.nut")}
	prepared := BuildFFmpegCommand("ffmpeg", short)
	preparedJoined := strings.Join(prepared, " ")
	if !strings.Contains(preparedJoined, "-c:v copy") || strings.Contains(preparedJoined, "-filter_complex") {
		t.Fatalf("prepared program command did not copy the composed items:\n%s", preparedJoined)
	}
	if strings.Contains(preparedJoined, filepath.Join(dir, "intro.png")) || strings.Contains(preparedJoined, filepath.Join(dir, "outro.png")) {
		t.Fatalf("prepared program command still opened overlay stills:\n%s", preparedJoined)
	}
	if !strings.Contains(preparedJoined, "-c:a pcm_f32le") {
		t.Fatalf("prepared program command lost lossless PCM audio:\n%s", preparedJoined)
	}
	if !containsArg(prepared, fullDemoProgramPath(short)) {
		t.Fatalf("prepared program command output = %v", prepared)
	}

	unsupported := short
	unsupported.Effects = append(append([]Effect{}, short.Effects...), Effect{Type: EffectText, Value: "x", StartSeconds: 1, EndSeconds: 2})
	fallback := BuildFFmpegCommand("ffmpeg", unsupported)
	if !strings.Contains(strings.Join(fallback, " "), "-filter_complex") {
		t.Fatalf("unsupported overlays lost the legacy global pass:\n%v", fallback)
	}
}

// The item graph must run the original global expressions on the global frame
// clock: the base is shifted to global PTS, the legacy graph is reused
// unchanged, and the output is shifted back to item-local PTS.
func TestFullDemoItemVideoClausesUseGlobalClock(t *testing.T) {
	dir := t.TempDir()
	short := fullDemoItemOverlayFixtureShort(dir)
	item := recapplan.TimelineItem{StartFrame: 137, EndFrame: 137 + 300}
	clauses, output := fullDemoItemVideoClauses(short, item, "[0:v]format=yuv420p", imageEffects(short.Effects), 1)
	filter := strings.Join(clauses, ";")
	if output != "[vout]" {
		t.Fatalf("output label = %q, want [vout]", output)
	}
	if !strings.Contains(filter, "[0:v]format=yuv420p[vlocal]") {
		t.Fatalf("item base was not labeled for the global shift:\n%s", filter)
	}
	// Frame 137 is not a millisecond value; the integer clock keeps it exact.
	if !strings.Contains(filter, "[vlocal]settb=expr=1/60,setpts=N+137[vg0]") {
		t.Fatalf("item base was not shifted onto the global integer frame clock:\n%s", filter)
	}
	if !strings.Contains(filter, "[v]settb=expr=1/60,setpts=N[vout]") {
		t.Fatalf("composed video was not restored to the canonical item-local clock:\n%s", filter)
	}
	// Windows must be the original global expressions, never item-local shifts.
	for _, want := range []string{"[vg0]format=yuv420p[vbase]", "between(t\\,0.000\\,5.000)", "between(t\\,52.000\\,60.000)", "setpts=PTS-STARTPTS+0.000/TB"} {
		if !strings.Contains(filter, want) {
			t.Fatalf("item graph missing global expression %q:\n%s", want, filter)
		}
	}
	if strings.Contains(filter, "setpts=PTS-STARTPTS+137/60/TB[gimg") {
		t.Fatalf("still image was shifted to an item-local window:\n%s", filter)
	}
}

// A reversed effect order must stay reversed, exactly like the legacy pass.
func TestFullDemoItemVideoClausesPreserveEffectOrder(t *testing.T) {
	dir := t.TempDir()
	short := fullDemoItemOverlayFixtureShort(dir)
	short.Effects = []Effect{short.Effects[1], short.Effects[0]} // outro, then intro
	item := recapplan.TimelineItem{StartFrame: 0, EndFrame: 300}
	clauses, _ := fullDemoItemVideoClauses(short, item, "[0:v]format=yuv420p", imageEffects(short.Effects), 1)
	filter := strings.Join(clauses, ";")
	outro := strings.Index(filter, "[1:v]format=rgba")
	intro := strings.Index(filter, "[2:v]format=rgba")
	if outro < 0 || intro < 0 || outro > intro {
		t.Fatalf("effect order was not preserved (outro=%d intro=%d):\n%s", outro, intro, filter)
	}
}

func TestFullDemoItemCommandComposesOverlaysAfterBase(t *testing.T) {
	dir := t.TempDir()
	short := fullDemoItemOverlayFixtureShort(dir)
	item := recapplan.TimelineItem{Role: "round", SourceRef: "round-001", SourceStartTick: 64, StartFrame: 4 * 60, EndFrame: 6 * 60, EndSample: 48000 * 120, StartSample: 0}
	command, err := fullDemoItemCommand(short, item, filepath.Join(dir, "item.nut"))
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(command, " ")
	filter := command[argIndex(command, "-filter_complex")+1]
	if !strings.Contains(joined, filepath.Join(dir, "intro.png")) || !strings.Contains(joined, filepath.Join(dir, "outro.png")) {
		t.Fatalf("item command missing overlay stills:\n%s", joined)
	}
	if !strings.Contains(filter, "format=yuv420p,hud") && !strings.Contains(filter, "format=yuv420p[vlocal]") {
		t.Fatalf("item base/HUD did not precede the global shift:\n%s", filter)
	}
	if !strings.Contains(filter, "setpts=N+240[vg0]") {
		t.Fatalf("item base was not shifted onto the global integer clock:\n%s", filter)
	}
	if !strings.Contains(filter, "[v]settb=expr=1/60,setpts=N[vout]") {
		t.Fatalf("item output was not restored to the canonical local clock:\n%s", filter)
	}
	if argIndex(command, "[vout]") < 0 {
		t.Fatalf("item command did not map the composed output: %v", command)
	}
}

func TestFullDemoItemCommandFallsBackForUnsupportedEffects(t *testing.T) {
	dir := t.TempDir()
	short := fullDemoItemOverlayFixtureShort(dir)
	short.Effects = append(short.Effects, Effect{Type: EffectText, Value: "sponsor", StartSeconds: 1, EndSeconds: 2})
	item := recapplan.TimelineItem{Role: "round", SourceRef: "round-001", SourceStartTick: 64, StartFrame: 0, EndFrame: 60, EndSample: 48000, StartSample: 0}
	command, err := fullDemoItemCommand(short, item, filepath.Join(dir, "item.nut"))
	if err != nil {
		t.Fatal(err)
	}
	filter := command[argIndex(command, "-filter_complex")+1]
	if strings.Contains(filter, "gblur=sigma=") || strings.Contains(filter, filepath.Join(dir, "intro.png")) {
		t.Fatalf("unsupported item gained an overlay:\n%s", filter)
	}
	if !strings.Contains(filter, "format=yuv420p[v];") {
		t.Fatalf("legacy item chain changed:\n%s", filter)
	}
	if argIndex(command, "[vout]") >= 0 {
		t.Fatalf("unsupported item used the composed output: %v", command)
	}
}

func argIndex(command []string, flag string) int {
	for i, arg := range command {
		if arg == flag {
			return i
		}
	}
	return -1
}
