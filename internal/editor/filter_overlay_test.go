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
	"time"

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
	effect := Effect{
		Type:         EffectImage,
		Source:       "full-demo-outro",
		StartSeconds: 16,
		EndSeconds:   16,
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

func TestCompilationFilterFullDemoOverlayWindowGraph(t *testing.T) {
	short := fullDemoOverlayFixtureShort()
	got := CompilationFilter(short)
	if strings.Contains(got, "trim=duration=24.000") && strings.Contains(got, "full-demo") {
		t.Fatalf("compilation filter still trims overlay streams to full short duration:\n%s", got)
	}
	for _, want := range []string{
		"trim=duration=2.033",
		"setpts=PTS-STARTPTS+1.000/TB",
		"trim=duration=3.033",
		"setpts=PTS-STARTPTS+5.000/TB",
		"eof_action=pass:repeatlast=0",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("CompilationFilter missing %q:\n%s", want, got)
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

func overlayEquivalenceFilter(short ShortEdit, overlayFn func(Effect, ShortEdit) string) string {
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
	slide, next := introSlideOverlayClausesWithFilter(current, 1, "img0", "vintro", *intro, short, overlayFn)
	clauses = append(clauses, slide...)
	current = next
	clauses = append(clauses,
		fmt.Sprintf("[2:v]%s[outroimg]", overlayFn(*outro, short)),
		imageOverlayClause(current, "outroimg", "vout", "0", "0", betweenExpression(outro.StartSeconds, outro.EndSeconds)),
	)
	return strings.Join(clauses, ";") + ";[vout]format=yuv420p[vfinal]"
}

func introSlideOverlayClausesWithFilter(current string, imageInput int, imageLabel, next string, effect Effect, short ShortEdit, overlayFn func(Effect, ShortEdit) string) ([]string, string) {
	l := demooverlay.DefaultLayout()
	leftW := l.Intro.LeftPanelX + l.Intro.PanelWidth + 16
	rightX := l.Intro.RightPanelX - 16
	rightW := demooverlay.FrameWidth - rightX
	enable := betweenExpression(effect.StartSeconds, effect.EndSeconds)
	slideOut := demooverlay.IntroOverlaySlideOutSeconds
	outStart := effect.EndSeconds - slideOut
	if outStart < effect.StartSeconds+effect.FadeInSeconds {
		outStart = effect.StartSeconds + effect.FadeInSeconds
	}
	mid := imageLabel + "L"
	clauses := []string{
		fmt.Sprintf("[%d:v]%s[%s]", imageInput, overlayFn(effect, short), imageLabel),
		fmt.Sprintf("[%s]split=2[%ssrcL][%ssrcR]", imageLabel, imageLabel, imageLabel),
		fmt.Sprintf("[%ssrcL]crop=%d:%d:0:0[%sL]", imageLabel, leftW, demooverlay.FrameHeight, imageLabel),
		fmt.Sprintf("[%ssrcR]crop=%d:%d:%d:0[%sR]", imageLabel, rightW, demooverlay.FrameHeight, rightX, imageLabel),
		fmt.Sprintf("[%s][%sL]overlay=x='%s':y=0:format=auto:eof_action=pass:repeatlast=0:enable='%s'[%s]",
			current, imageLabel,
			introSlideX(effect.StartSeconds, effect.FadeInSeconds, outStart, effect.EndSeconds, -leftW, 0),
			enable, mid),
		fmt.Sprintf("[%s][%sR]overlay=x='%s':y=0:format=auto:eof_action=pass:repeatlast=0:enable='%s'[%s]",
			mid, imageLabel,
			introSlideX(effect.StartSeconds, effect.FadeInSeconds, outStart, effect.EndSeconds, demooverlay.FrameWidth, rightX),
			enable, next),
	}
	return clauses, next
}

func renderOverlayStreamHash(t *testing.T, ffmpeg, dir string, short ShortEdit, overlayFn func(Effect, ShortEdit) string, startSeconds, durationSeconds float64) string {
	t.Helper()
	filter := overlayEquivalenceFilter(short, overlayFn) +
		fmt.Sprintf(";[vfinal]trim=start=%.3f:duration=%.3f,setpts=PTS-STARTPTS[vtrim]", startSeconds, durationSeconds)
	cmd := exec.Command(ffmpeg, "-y", "-v", "error",
		"-i", short.Parts[0].Input,
		"-loop", "1", "-i", short.Effects[0].Path,
		"-loop", "1", "-i", short.Effects[1].Path,
		"-filter_complex", filter,
		"-map", "[vtrim]",
		"-f", "framemd5",
		"-",
	)
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("framemd5 overlay fixture: %v: %s\nfilter=%s", err, stderr.String(), filter)
	}
	return stdout.String()
}

func renderFullDemoOverlayFixture(t *testing.T, ffmpeg, dir string, short ShortEdit, overlayFn func(Effect, ShortEdit) string) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, fmt.Sprintf("render-%p.mp4", overlayFn))
	filter := overlayEquivalenceFilter(short, overlayFn)
	cmd := exec.Command(ffmpeg, "-y", "-v", "error",
		"-i", short.Parts[0].Input,
		"-loop", "1", "-i", short.Effects[0].Path,
		"-loop", "1", "-i", short.Effects[1].Path,
		"-filter_complex", filter,
		"-map", "[vfinal]",
		"-t", fmt.Sprintf("%.3f", short.DurationSeconds),
		"-c:v", "libx264", "-preset", "ultrafast", "-crf", "18", "-pix_fmt", "yuv420p",
		out,
	)
	if runOut, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("render overlay fixture: %v: %s\nfilter=%s", err, runOut, filter)
	}
	return out
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

func TestFullDemoOverlayWindowBenchmark(t *testing.T) {
	if os.Getenv("FULL_DEMO_OVERLAY_BENCH") != "1" {
		t.Skip("set FULL_DEMO_OVERLAY_BENCH=1 for local wall-time comparison")
	}
	ffmpeg := ffmpegForEquivalence(t)
	dir := t.TempDir()
	const duration = 30.0 * 60
	const fps = 60
	base := filepath.Join(dir, "base-long.mp4")
	writeLavfiCaptureAtFPS(t, ffmpeg, base, 1920, 1080, fps, duration)
	introPNG := writeSolidPNG(t, ffmpeg, dir, "intro.png", "red", demooverlay.FrameWidth, demooverlay.FrameHeight)
	outroPNG := writeSolidPNG(t, ffmpeg, dir, "outro.png", "blue", demooverlay.FrameWidth, demooverlay.FrameHeight)

	short := fullDemoOverlayFixtureShort()
	short.DurationSeconds = duration
	short.Parts = []ShortPart{{Input: base, DurationSeconds: duration}}
	short.Effects[0].Path = introPNG
	introStart, introEnd, outroStart, outroEnd := demooverlay.OverlayWindows(duration)
	short.Effects[0].StartSeconds = introStart
	short.Effects[0].EndSeconds = introEnd
	short.Effects[1].Path = outroPNG
	short.Effects[1].StartSeconds = outroStart
	short.Effects[1].EndSeconds = outroEnd

	measure := func(label string, overlayFn func(Effect, ShortEdit) string) time.Duration {
		start := time.Now()
		renderFullDemoOverlayFixture(t, ffmpeg, filepath.Join(dir, label), short, overlayFn)
		return time.Since(start)
	}
	legacyDur := measure("legacy", legacyImageOverlayFilter)
	optDur := measure("optimized", buildImageOverlayFilter)
	t.Logf("30min 1080p60 overlay composite wall: legacy=%s optimized=%s delta=%.1f%%",
		legacyDur, optDur, (1-float64(optDur)/float64(legacyDur))*100)
}
