package editor

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rechedev9/cliphub/internal/demooverlay"
)

func TestImageOverlayFilterUsesActiveWindowNotFullDuration(t *testing.T) {
	short := ShortEdit{
		Preset:          PresetGameplayPOV60,
		OutputFormat:    OutputFormatLandscape16x9,
		DurationSeconds: 1800,
		OutputFPS:       60,
	}
	intro := Effect{
		Type:           EffectImage,
		Source:         "full-demo-intro",
		StartSeconds:   5,
		EndSeconds:     14,
		FadeInSeconds:  demooverlay.IntroOverlaySlideSeconds,
		FadeOutSeconds: 0.35,
		Width:          demooverlay.FrameWidth,
		Height:         demooverlay.FrameHeight,
	}
	got := imageOverlayFilter(intro, short)
	if strings.Contains(got, "trim=duration=1800") {
		t.Fatalf("intro overlay still trims to full short duration:\n%s", got)
	}
	if !strings.Contains(got, "trim=duration=9.017") {
		t.Fatalf("intro overlay missing active-window trim:\n%s", got)
	}
	if !strings.Contains(got, "setpts=PTS-STARTPTS+5.000/TB") {
		t.Fatalf("intro overlay missing PTS shift to window start:\n%s", got)
	}

	outro := Effect{
		Type:          EffectImage,
		Source:        "full-demo-outro",
		StartSeconds:  1792,
		EndSeconds:    1800,
		FadeInSeconds: demooverlay.IntroOverlaySlideSeconds,
		Width:         demooverlay.FrameWidth,
		Height:        demooverlay.FrameHeight,
	}
	got = imageOverlayFilter(outro, short)
	if strings.Contains(got, "trim=duration=1800") {
		t.Fatalf("outro overlay still trims to full short duration:\n%s", got)
	}
	if !strings.Contains(got, "trim=duration=8.017") {
		t.Fatalf("outro overlay missing active-window trim:\n%s", got)
	}
	if !strings.Contains(got, "setpts=PTS-STARTPTS+1792.000/TB") {
		t.Fatalf("outro overlay missing PTS shift to window start:\n%s", got)
	}
}

func TestImageOverlayFilterDegenerateWindowIsSafe(t *testing.T) {
	short := ShortEdit{DurationSeconds: 24, OutputFPS: 60}
	// A fade is what routes an image through the windowed loop/trim branch;
	// without it the zero-length guard is never reached.
	effect := Effect{
		Type:          EffectImage,
		Source:        "full-demo-outro",
		StartSeconds:  16,
		EndSeconds:    16,
		FadeInSeconds: demooverlay.IntroOverlaySlideSeconds,
	}
	got := imageOverlayFilter(effect, short)
	if strings.Contains(got, "trim=duration=") {
		t.Fatalf("zero-length window should not loop/trim:\n%s", got)
	}
}

func TestImageOverlayClauseUsesEOFPass(t *testing.T) {
	got := imageOverlayClause("vbase", "img0", "vout", "0", "0", "between(t\\,5.000\\,14.000)")
	for _, want := range []string{"eof_action=pass", "repeatlast=0", "enable='between(t\\,5.000\\,14.000)'"} {
		if !strings.Contains(got, want) {
			t.Fatalf("overlay clause missing %q:\n%s", want, got)
		}
	}
}

func TestFullDemoCompilationFilterOverlayWindowGraph(t *testing.T) {
	short := fullDemoOverlayFixtureShort()
	got := fullDemoCompilationFilter(short)
	fullTrims := []string{
		fmt.Sprintf("trim=duration=%.3f", short.DurationSeconds),
		fmt.Sprintf("trim=duration=%.3f", short.DurationSeconds+1.0/float64(short.OutputFPS)),
	}
	for _, clause := range strings.Split(got, ";") {
		if !strings.HasPrefix(clause, "[1:v]") && !strings.HasPrefix(clause, "[2:v]") {
			continue
		}
		for _, full := range fullTrims {
			if strings.Contains(clause, full) {
				t.Fatalf("overlay input still trims to the full %.0fs program (%s):\n%s", short.DurationSeconds, full, clause)
			}
		}
	}
	for _, want := range []string{
		"trim=duration=2.033",
		"setpts=PTS-STARTPTS+1.000/TB",
		"trim=duration=3.033",
		"setpts=PTS-STARTPTS+5.000/TB",
		"eof_action=pass:repeatlast=0",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("fullDemoCompilationFilter missing %q:\n%s", want, got)
		}
	}
}

func legacyImageOverlayFilter(effect Effect, short ShortEdit) string {
	filters := []string{
		"format=rgba",
		imageScaleFilter(effect),
	}
	if hasEffectFade(effect) || effect.Source == "full-demo-intro" {
		duration := short.DurationSeconds
		if duration <= 0 {
			duration = effect.EndSeconds
		}
		filters = append(filters,
			"loop=loop=-1:size=1:start=0",
			fmt.Sprintf("setpts=N/%d/TB", outputFPS(short)),
		)
		if duration > 0 {
			filters = append(filters, fmt.Sprintf("trim=duration=%.3f", duration))
		}
		fadeIn, fadeOut := normalizedFadeDurations(effect)
		if effect.Source == "full-demo-intro" {
			fadeIn = 0
		}
		faded := effect
		faded.FadeInSeconds = fadeIn
		faded.FadeOutSeconds = fadeOut
		filters = append(filters, overlayFadeFilters(faded)...)
	}
	return strings.Join(filters, ",")
}

func fullDemoOverlayFixtureShort() ShortEdit {
	return ShortEdit{
		Preset:          PresetGameplayPOV60,
		OutputFormat:    OutputFormatLandscape16x9,
		DurationSeconds: 8,
		OutputFPS:       30,
		Tickrate:        64,
		Parts:           []ShortPart{{Input: "p1.mp4", DurationSeconds: 8, TickStart: 1000, TickEnd: 1512}},
		Effects: []Effect{
			{
				Type:           EffectImage,
				Path:           "intro.png",
				Source:         "full-demo-intro",
				StartSeconds:   1,
				EndSeconds:     3,
				FadeInSeconds:  demooverlay.IntroOverlaySlideSeconds,
				FadeOutSeconds: 0.35,
				Width:          demooverlay.FrameWidth,
				Height:         demooverlay.FrameHeight,
			},
			{
				Type:          EffectImage,
				Path:          "outro.png",
				Source:        "full-demo-outro",
				StartSeconds:  5,
				EndSeconds:    8,
				FadeInSeconds: demooverlay.IntroOverlaySlideSeconds,
				Width:         demooverlay.FrameWidth,
				Height:        demooverlay.FrameHeight,
			},
		},
	}
}

func TestFullDemoOverlayWindowPixelEquivalence(t *testing.T) {
	ffmpeg := ffmpegForEquivalence(t)
	dir := t.TempDir()
	const duration = 8.0
	const fps = 30

	base := filepath.Join(dir, "base.mp4")
	writeLavfiCaptureAtFPS(t, ffmpeg, base, 1920, 1080, fps, duration)
	introPNG := writeSolidPNG(t, ffmpeg, dir, "intro.png", "red", demooverlay.FrameWidth, demooverlay.FrameHeight)
	outroPNG := writeSolidPNG(t, ffmpeg, dir, "outro.png", "blue", demooverlay.FrameWidth, demooverlay.FrameHeight)

	short := fullDemoOverlayFixtureShort()
	short.Parts[0].Input = base
	short.Effects[0].Path = introPNG
	short.Effects[1].Path = outroPNG

	regions := []struct {
		name  string
		start float64
		end   float64
	}{
		{name: "pre-intro body", start: 0.5, end: 0.9},
		{name: "intro slide", start: 1.4, end: 1.5},
		{name: "between bookends", start: 3.5, end: 4.0},
		{name: "outro hold", start: 6.0, end: 6.5},
	}
	for _, region := range regions {
		legacy := overlayCompositeRawHash(t, ffmpeg, short, legacyImageOverlayFilter, region.start, region.end-region.start)
		optimized := overlayCompositeRawHash(t, ffmpeg, short, buildImageOverlayFilter, region.start, region.end-region.start)
		if legacy != optimized {
			t.Fatalf("%s raw hash mismatch\n  legacy    %s\n  optimized %s", region.name, legacy, optimized)
		}
	}
}

func overlayCompositeRawHash(t *testing.T, ffmpeg string, short ShortEdit, overlayFn func(Effect, ShortEdit) string, start, duration float64) string {
	t.Helper()
	filter := overlayEquivalenceFilter(short, overlayFn) +
		fmt.Sprintf(";[vfinal]trim=start=%.3f:duration=%.6f,format=yuv420p[vout]", start, duration)
	var stdout bytes.Buffer
	var stderr strings.Builder
	cmd := exec.Command(ffmpeg, "-y", "-v", "error",
		"-i", short.Parts[0].Input,
		"-loop", "1", "-i", short.Effects[0].Path,
		"-loop", "1", "-i", short.Effects[1].Path,
		"-filter_complex", filter,
		"-map", "[vout]",
		"-f", "rawvideo",
		"-",
	)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("overlay raw hash filter: %v: %s\n%s", err, stderr.String(), filter)
	}
	if stdout.Len() == 0 {
		t.Fatalf("overlay raw hash produced no frames")
	}
	sum := sha256.Sum256(stdout.Bytes())
	return hex.EncodeToString(sum[:])
}

// overlayEquivalenceFilter renders the production intro slide and outro
// overlay clauses with overlayFn swapped in as the image overlay filter.
func overlayEquivalenceFilter(short ShortEdit, overlayFn func(Effect, ShortEdit) string) string {
	previous := imageOverlayFilterFunc
	imageOverlayFilterFunc = overlayFn
	defer func() { imageOverlayFilterFunc = previous }()
	var intro, outro *Effect
	for i := range short.Effects {
		switch short.Effects[i].Source {
		case "full-demo-intro":
			intro = &short.Effects[i]
		case "full-demo-outro":
			outro = &short.Effects[i]
		}
	}
	if intro == nil || outro == nil {
		return "null"
	}
	clauses := []string{"[0:v]null[vbase]"}
	current := "vbase"
	if dim, dimOut, ok := fullDemoOutroDimClauses(short, current, "vdim"); ok {
		clauses = append(clauses, dim...)
		current = dimOut
	}
	slide, next := introSlideOverlayClauses(current, 1, "img0", "vintro", *intro, short)
	clauses = append(clauses, slide...)
	current = next
	clauses = append(clauses,
		fmt.Sprintf("[2:v]%s[outroimg]", imageOverlayFilter(*outro, short)),
		imageOverlayClause(current, "outroimg", "vout", "0", "0", betweenExpression(outro.StartSeconds, outro.EndSeconds)),
	)
	return strings.Join(clauses, ";") + ";[vout]format=yuv420p[vfinal]"
}

func writeLavfiCaptureAtFPS(t *testing.T, ffmpeg, path string, width, height, fps int, seconds float64) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(ffmpeg, "-y", "-hide_banner", "-loglevel", "error",
		"-f", "lavfi", "-i", fmt.Sprintf("testsrc2=size=%dx%d:rate=%d:duration=%.3f", width, height, fps, seconds),
		"-c:v", "libx264", "-preset", "ultrafast", "-crf", "28", "-pix_fmt", "yuv420p",
		path,
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("write lavfi capture %s: %v: %s", path, err, out)
	}
}

func writeSolidPNG(t *testing.T, ffmpeg, dir, name, color string, width, height int) string {
	t.Helper()
	path := filepath.Join(dir, name)
	cmd := exec.Command(ffmpeg, "-y", "-v", "error",
		"-f", "lavfi", "-i", fmt.Sprintf("color=c=%s:s=%dx%d", color, width, height),
		"-frames:v", "1", path,
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("write png %s: %v: %s", path, err, out)
	}
	return path
}
