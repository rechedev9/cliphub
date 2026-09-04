package editor

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/rechedev9/cliphub/internal/demooverlay"
	"github.com/rechedev9/cliphub/internal/killplan"
	"github.com/rechedev9/cliphub/internal/mediafont"
	"github.com/rechedev9/cliphub/internal/parser"
	"github.com/rechedev9/cliphub/internal/recording"
	"github.com/rechedev9/cliphub/internal/rules"
)

func TestBuildManifestFullDemoAttachesIntroAndOutroOverlays(t *testing.T) {
	dir := t.TempDir()
	result := testRecordingResult(dir)
	result.Plan.Segments[0].TickEnd = result.Plan.Segments[0].TickStart + 64*40
	result.Plan.Segments[1].TickEnd = result.Plan.Segments[1].TickStart + 64*40
	for i := range result.Artifacts {
		result.Artifacts[i].DurationSeconds = 40
	}
	intro := filepath.Join(dir, "shorts", "full-demo-intro.png")
	outro := filepath.Join(dir, "shorts", "full-demo-outro.png")
	if err := os.MkdirAll(filepath.Dir(intro), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(intro, []byte("png"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(outro, []byte("png"), 0o600); err != nil {
		t.Fatal(err)
	}
	doc := demooverlay.Build(demooverlay.Roster{
		TargetSteamID64: "76561198148986856",
		Map:             "de_mirage",
		ScoreCT:         13,
		ScoreT:          8,
		Players: []demooverlay.RosterPlayer{
			{SteamID64: "76561198148986856", Name: "donk666", Team: "CT", Kills: 23, Deaths: 14, Assists: 4, ADR: 101.6, Rating: 1.35},
			{SteamID64: "9", Name: "KingwayO", Team: "T", Kills: 18, Deaths: 16},
		},
	}, nil)

	opts := testManifestOptions(dir, nil)
	opts.Preset = PresetGameplayPOV60
	opts.OutputFormat = OutputFormatLandscape16x9
	opts.CompileSegments = true
	opts.KillEffect = KillEffectClean
	opts.Transition = TransitionCut
	opts.FullDemoOverlay = &doc
	opts.FullDemoIntroImagePath = intro
	opts.FullDemoOutroImagePath = outro

	manifest := mustBuildManifest(t, result, opts)
	if len(manifest.Shorts) != 1 {
		t.Fatalf("shorts = %d", len(manifest.Shorts))
	}
	short := manifest.Shorts[0]
	if short.Title != "donk666 (23-14) Mirage | CS2 DEMO POV + VOICECOMMS" {
		t.Fatalf("title = %q", short.Title)
	}
	if strings.Contains(short.Title, "TOP #1") {
		t.Fatal("title invented FACEIT rank")
	}
	var introFx, outroFx *Effect
	for i := range short.Effects {
		switch short.Effects[i].Source {
		case "full-demo-intro":
			introFx = &short.Effects[i]
		case "full-demo-outro":
			outroFx = &short.Effects[i]
		}
	}
	if introFx == nil || outroFx == nil {
		t.Fatalf("effects = %#v, want intro and outro image overlays", short.Effects)
	}
	wantIntroStart, wantIntroEnd, wantOutroStart, wantOutroEnd := demooverlay.OverlayWindows(short.DurationSeconds)
	if introFx.Type != EffectImage || introFx.StartSeconds != wantIntroStart || introFx.EndSeconds != wantIntroEnd {
		t.Fatalf("intro effect = %#v, want %.1f-%.1f (after fade, before live)", introFx, wantIntroStart, wantIntroEnd)
	}
	if introFx.FadeInSeconds != demooverlay.IntroOverlaySlideSeconds {
		t.Fatalf("intro slide-in fade = %.2f, want %.2f", introFx.FadeInSeconds, demooverlay.IntroOverlaySlideSeconds)
	}
	if outroFx.Type != EffectImage || outroFx.StartSeconds != wantOutroStart || outroFx.EndSeconds != wantOutroEnd {
		t.Fatalf("outro effect = %#v, want %.1f-%.1f", outroFx, wantOutroStart, wantOutroEnd)
	}
	if wantIntroStart < demooverlay.FadeFromBlackSeconds+demooverlay.IntroOverlayAfterFadeSeconds-0.01 {
		t.Fatal("roster must wait until ~4s after the fade")
	}
	if wantIntroEnd >= float64(demooverlay.IntroFreezeSeconds) {
		t.Fatal("roster must leave before live action")
	}
	command := strings.Join(short.FFmpegCommand, " ")
	if !strings.Contains(command, intro) || !strings.Contains(command, outro) {
		t.Fatalf("ffmpeg missing overlay stills:\n%s", command)
	}
	if !strings.Contains(command, "fade=t=in:st=0") {
		t.Fatalf("ffmpeg missing fade from black:\n%s", command)
	}
	if !strings.Contains(command, "gblur=sigma=") {
		t.Fatalf("ffmpeg missing outro gameplay blur:\n%s", command)
	}
	if !strings.Contains(command, "crop=") || !strings.Contains(command, "pow(1-(t-") {
		t.Fatalf("ffmpeg missing intro column slide:\n%s", command)
	}
	if short.MusicPath != "" {
		t.Fatalf("full demo mixed a music bed: %q", short.MusicPath)
	}
	for _, effect := range short.Effects {
		if effect.Type == EffectZoom {
			t.Fatalf("full demo gained punch-in zoom: %#v", effect)
		}
		if strings.Contains(strings.ToLower(effect.Value), "subscribe") || strings.Contains(strings.ToLower(effect.Value), "suscríb") {
			t.Fatalf("full demo gained a subscribe CTA: %#v", effect)
		}
		if effect.Source == "edit-request" && (effect.Type == EffectText) {
			t.Fatalf("full demo gained a Shorts bookend/hook: %#v", effect)
		}
	}
}

func TestCompilationFilterFullDemoSlidesIntroAndBlursOutro(t *testing.T) {
	short := ShortEdit{
		Preset:          PresetGameplayPOV60,
		OutputFormat:    OutputFormatLandscape16x9,
		DurationSeconds: 24,
		Parts:           []ShortPart{{Input: "p1.mp4", DurationSeconds: 24, TickStart: 1000, TickEnd: 2536}},
		Tickrate:        64,
		Effects: []Effect{
			{
				Type:           EffectImage,
				Path:           "intro.png",
				Source:         "full-demo-intro",
				StartSeconds:   5,
				EndSeconds:     14,
				FadeInSeconds:  demooverlay.IntroOverlaySlideSeconds,
				FadeOutSeconds: 0.35,
				Width:          demooverlay.FrameWidth,
				Height:         demooverlay.FrameHeight,
			},
			{
				Type:         EffectImage,
				Path:         "outro.png",
				Source:       "full-demo-outro",
				StartSeconds: 16,
				EndSeconds:   24,
				Width:        demooverlay.FrameWidth,
				Height:       demooverlay.FrameHeight,
			},
		},
	}
	got := CompilationFilter(short)
	wantDim := fmt.Sprintf(
		"gblur=sigma=%.3f:enable='between(t\\,16.000\\,24.000)',eq=brightness=%.3f:enable='between(t\\,16.000\\,24.000)'",
		demooverlay.OutroBlurSigma, demooverlay.OutroEQBrightness,
	)
	for _, want := range []string{
		wantDim,
		"split=2[img0srcL][img0srcR]",
		"crop=",
		"pow(1-(t-",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("CompilationFilter missing %q:\n%s", want, got)
		}
	}
	for _, forbidden := range []string{"[vkeep]", "[vtail]", "[vblurred]", "split=2[vkeep]"} {
		if strings.Contains(got, forbidden) {
			t.Fatalf("outro dim must not use split/overlay buffering, found %q:\n%s", forbidden, got)
		}
	}
	if strings.Contains(got, "fade=t=in:st=5.000") {
		t.Fatalf("intro still faded in instead of sliding:\n%s", got)
	}
}

func TestBuildManifestShortsPathIgnoresFullDemoOverlay(t *testing.T) {
	dir := t.TempDir()
	result := testRecordingResult(dir)
	doc := demooverlay.Build(demooverlay.Roster{
		TargetSteamID64: "76561198148986856",
		Players:         []demooverlay.RosterPlayer{{SteamID64: "76561198148986856", Name: "donk666", Kills: 23, Deaths: 14}},
	}, nil)
	opts := testManifestOptions(dir, nil)
	opts.FullDemoOverlay = &doc
	opts.FullDemoIntroImagePath = filepath.Join(dir, "intro.png")
	opts.FullDemoOutroImagePath = filepath.Join(dir, "outro.png")

	manifest := mustBuildManifest(t, result, opts)
	if len(manifest.Shorts) == 0 {
		t.Fatal("no shorts")
	}
	for _, short := range manifest.Shorts {
		if strings.Contains(short.Title, "VOICECOMMS") {
			t.Fatalf("shorts title used Full Demo copy: %q", short.Title)
		}
		if short.OutputFormat != OutputFormatShort9x16 {
			t.Fatalf("shorts format = %q, want %q", short.OutputFormat, OutputFormatShort9x16)
		}
		command := strings.Join(short.FFmpegCommand, " ")
		if !strings.Contains(command, "crop=1080:1920") {
			t.Fatalf("shorts lost portrait crop:\n%s", command)
		}
		if strings.Contains(command, "fade=t=in:st=0") {
			t.Fatalf("shorts gained Full Demo fade-from-black:\n%s", command)
		}
		for _, effect := range short.Effects {
			if effect.Source == "full-demo-intro" || effect.Source == "full-demo-outro" {
				t.Fatalf("shorts gained a Full Demo overlay: %#v", effect)
			}
		}
	}
}

func TestFullDemoOverlayCompositesOntoFixtureCapture(t *testing.T) {
	ffmpeg := requireFFmpeg(t)
	dir := t.TempDir()
	const duration = 24.0
	fixture := filepath.Join(dir, "recording", "segments", "seg-001.mp4")
	writeLavfiCapture(t, ffmpeg, fixture, 1920, 1080, duration)

	result := testRecordingResult(dir)
	result.Plan.Segments = []recording.RecordingSegment{{
		ID:        "seg-001",
		Round:     1,
		TickStart: 1000,
		TickEnd:   1000 + int(duration*64),
		Kills:     []killplan.Kill{{Tick: 1064, Weapon: "ak47"}},
	}}
	result.Artifacts = []recording.RecordingArtifact{{
		SegmentID:       "seg-001",
		Role:            "segment",
		Type:            "video",
		Path:            fixture,
		SizeBytes:       1,
		DurationSeconds: duration,
		Codec:           "h264",
		Width:           1920,
		Height:          1080,
		FrameRate:       "60/1",
	}}

	doc := demooverlay.Build(demooverlay.Roster{
		TargetSteamID64: "76561198148986856",
		Map:             "de_mirage",
		ScoreCT:         13,
		ScoreT:          8,
		Players: []demooverlay.RosterPlayer{
			{SteamID64: "76561198148986856", Name: "donk666", Team: "CT", Kills: 23, Deaths: 14, Assists: 4},
			{SteamID64: "9", Name: "KingwayO", Team: "T", Kills: 18, Deaths: 16},
		},
	}, nil)
	opts := testManifestOptions(dir, nil)
	opts.Preset = PresetGameplayPOV60
	opts.OutputFormat = OutputFormatLandscape16x9
	opts.CompileSegments = true
	opts.KillEffect = KillEffectClean
	opts.Transition = TransitionCut
	opts.VideoPreset = "ultrafast"
	opts.VideoCRF = 28
	opts.CoversEnabled = false
	opts.FullDemoOverlay = &doc

	manifest := mustBuildManifest(t, result, opts)
	if len(manifest.Shorts) != 1 {
		t.Fatalf("compiled shorts = %d", len(manifest.Shorts))
	}
	short := manifest.Shorts[0]
	if short.MusicPath != "" {
		t.Fatalf("fixture Full Demo mixed music: %q", short.MusicPath)
	}
	if _, err := os.Stat(short.FullDemoIntroImagePath); err != nil {
		t.Fatalf("compositor did not write intro: %v", err)
	}
	if _, err := os.Stat(short.FullDemoOutroImagePath); err != nil {
		t.Fatalf("compositor did not write outro: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(short.Output), 0o750); err != nil {
		t.Fatal(err)
	}
	runFFmpegCommand(t, short.FFmpegCommand)
	if st, err := os.Stat(short.Output); err != nil || st.Size() < 1024 {
		t.Fatalf("compiled Full Demo output missing or tiny: %v", err)
	}
	assertPlayableMedia(t, ffmpeg, short.Output, short.DurationSeconds)
	w, h := probeVideoSize(t, ffmpeg, short.Output)
	if w != 1920 || h != 1080 {
		t.Fatalf("compiled Full Demo = %dx%d, want 1920x1080", w, h)
	}

	introFrame := extractFrame(t, ffmpeg, short.Output, 8, filepath.Join(dir, "frame-intro.png"))
	bodyFrame := extractFrame(t, ffmpeg, short.Output, 15, filepath.Join(dir, "frame-body.png"))
	outroFrame := extractFrame(t, ffmpeg, short.Output, 20, filepath.Join(dir, "frame-outro.png"))
	if filesEqual(t, introFrame, bodyFrame) {
		t.Fatal("intro roster frame matches the mid-round body; overlay did not land on the fixture")
	}
	if filesEqual(t, outroFrame, bodyFrame) {
		t.Fatal("outro scoreboard frame matches the mid-round body; overlay did not land on the fixture")
	}
	if dest := os.Getenv("FULL_DEMO_OVERLAY_OUT"); dest != "" {
		copyEvidence(t, dest, map[string]string{
			introFrame:                   "fixture-intro-on-capture.png",
			bodyFrame:                    "fixture-body-on-capture.png",
			outroFrame:                   "fixture-outro-on-capture.png",
			short.FullDemoIntroImagePath: "intro-roster.png",
			short.FullDemoOutroImagePath: "outro-scoreboard.png",
		})
	}
}

func TestShortsParsePlanPortraitSeam(t *testing.T) {
	ffmpeg := requireFFmpeg(t)
	dir := t.TempDir()
	segs := parser.Segment(
		[]parser.RawKill{
			{Tick: 10000, Round: 5, Weapon: "ak47"},
			{Tick: 20000, Round: 6, Weapon: "awp"},
		},
		nil,
		nil,
		rules.Default(),
		64,
	)
	if len(segs) != 2 {
		t.Fatalf("shorts parse segments = %d, want 2 kill bursts", len(segs))
	}
	plan := killplan.NewPlan()
	plan.Demo = killplan.Demo{
		Path:          filepath.Join(dir, "match.dem"),
		SHA256:        strings.Repeat("a", 64),
		Map:           "de_inferno",
		Tickrate:      64,
		DurationTicks: 40000,
	}
	plan.Target = killplan.Target{SteamID64: "76561198148986856", NameInDemo: "MartinezSa"}
	plan.Segments = segs

	recPlan, err := recording.NewPlanFromKillPlan(plan, plan.Demo.Path, filepath.Join(dir, "recording"), recording.DefaultStreamConfig())
	if err != nil {
		t.Fatalf("shorts recording plan: %v", err)
	}
	if len(recPlan.Segments) != 2 {
		t.Fatalf("recording segments = %d", len(recPlan.Segments))
	}

	result := recording.RecordingResult{Plan: recPlan}
	for _, seg := range recPlan.Segments {
		path := filepath.Join(dir, "recording", "segments", seg.ID+".mp4")
		writeLavfiCapture(t, ffmpeg, path, 1920, 1080, 2)
		result.Artifacts = append(result.Artifacts, recording.RecordingArtifact{
			SegmentID:       seg.ID,
			Role:            "segment",
			Type:            "video",
			Path:            path,
			SizeBytes:       1,
			DurationSeconds: 2,
			Codec:           "h264",
			Width:           1920,
			Height:          1080,
			FrameRate:       "60/1",
		})
	}

	opts := testManifestOptions(dir, &plan)
	opts.Preset = PresetViral60Clean
	opts.OutputFormat = OutputFormatShort9x16
	opts.VideoPreset = "ultrafast"
	opts.VideoCRF = 28
	opts.CoversEnabled = false
	doc := demooverlay.Build(demooverlay.Roster{
		TargetSteamID64: "76561198148986856",
		Players:         []demooverlay.RosterPlayer{{SteamID64: "76561198148986856", Name: "donk666", Kills: 2, Deaths: 0}},
	}, nil)
	opts.FullDemoOverlay = &doc

	manifest := mustBuildManifest(t, result, opts)
	if len(manifest.Shorts) != 2 {
		t.Fatalf("portrait shorts = %d, want 2", len(manifest.Shorts))
	}
	short := manifest.Shorts[0]
	if short.OutputFormat != OutputFormatShort9x16 {
		t.Fatalf("format = %q, want portrait", short.OutputFormat)
	}
	if err := os.MkdirAll(filepath.Dir(short.Output), 0o750); err != nil {
		t.Fatal(err)
	}
	runFFmpegCommand(t, short.FFmpegCommand)
	assertPlayableMedia(t, ffmpeg, short.Output, short.DurationSeconds)
	w, h := probeVideoSize(t, ffmpeg, short.Output)
	if w != 1080 || h != 1920 {
		t.Fatalf("shorts output = %dx%d, want 1080x1920 portrait", w, h)
	}
	command := strings.Join(short.FFmpegCommand, " ")
	if strings.Contains(command, "full-demo-intro") || strings.Contains(short.Title, "VOICECOMMS") {
		t.Fatal("shorts portrait picked up the Full Demo overlay path")
	}
}

func requireFFmpeg(t *testing.T) string {
	t.Helper()
	path, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg not on PATH; fixture capture compositor cannot run")
	}
	if _, err := mediafont.Materialize(); err != nil {
		t.Fatalf("font: %v", err)
	}
	return path
}

func writeLavfiCapture(t *testing.T, ffmpeg, path string, width, height int, seconds float64) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(ffmpeg, "-y", "-hide_banner", "-loglevel", "error",
		"-f", "lavfi", "-i", fmt.Sprintf("testsrc2=size=%dx%d:rate=60:duration=%.3f", width, height, seconds),
		"-f", "lavfi", "-i", fmt.Sprintf("sine=frequency=220:duration=%.3f", seconds),
		"-c:v", "libx264", "-preset", "ultrafast", "-crf", "28", "-pix_fmt", "yuv420p",
		"-c:a", "aac", "-ac", "2", "-shortest",
		path,
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("write lavfi capture %s: %v: %s", path, err, out)
	}
}

func runFFmpegCommand(t *testing.T, command []string) {
	t.Helper()
	if len(command) < 2 {
		t.Fatalf("empty ffmpeg command: %#v", command)
	}
	cmd := exec.Command(command[0], command[1:]...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("ffmpeg render: %v: %s", err, out)
	}
}

func probeVideoSize(t *testing.T, ffmpeg, path string) (int, int) {
	t.Helper()
	probe := strings.Replace(ffmpeg, "ffmpeg", "ffprobe", 1)
	if _, err := exec.LookPath(probe); err != nil {
		probe = "ffprobe"
	}
	cmd := exec.Command(probe, "-v", "error", "-select_streams", "v:0",
		"-show_entries", "stream=width,height", "-of", "csv=p=0", path)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("ffprobe %s: %v: %s", path, err, out)
	}
	var w, h int
	if _, err := fmt.Sscanf(strings.TrimSpace(string(out)), "%d,%d", &w, &h); err != nil {
		t.Fatalf("parse ffprobe %q: %v", out, err)
	}
	return w, h
}

func extractFrame(t *testing.T, ffmpeg, input string, at float64, dest string) string {
	t.Helper()
	cmd := exec.Command(ffmpeg, "-y", "-hide_banner", "-loglevel", "error",
		"-ss", fmt.Sprintf("%.3f", at), "-i", input, "-frames:v", "1", dest)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("extract frame at %.1fs: %v: %s", at, err, out)
	}
	return dest
}

func filesEqual(t *testing.T, a, b string) bool {
	t.Helper()
	left, err := os.ReadFile(a)
	if err != nil {
		t.Fatal(err)
	}
	right, err := os.ReadFile(b)
	if err != nil {
		t.Fatal(err)
	}
	return string(left) == string(right)
}

func copyEvidence(t *testing.T, dest string, files map[string]string) {
	t.Helper()
	if err := os.MkdirAll(dest, 0o750); err != nil {
		t.Fatal(err)
	}
	for src, name := range files {
		raw, err := os.ReadFile(src)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dest, name), raw, 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

// Check the encoded artifact, including both streams, rather than only its argv.
func assertPlayableMedia(t *testing.T, ffmpeg, path string, wantDuration float64) {
	t.Helper()
	out, err := exec.Command("ffprobe", "-v", "error", "-show_streams", "-of", "json", path).CombinedOutput()
	if err != nil {
		t.Fatalf("probe media: %v: %s", err, out)
	}
	var probe struct {
		Streams []struct {
			CodecType string `json:"codec_type"`
			CodecName string `json:"codec_name"`
			Duration  string `json:"duration"`
			FrameRate string `json:"avg_frame_rate"`
			Channels  int    `json:"channels"`
		} `json:"streams"`
	}
	if err := json.Unmarshal(out, &probe); err != nil {
		t.Fatal(err)
	}
	video, audio := 0, 0
	for _, stream := range probe.Streams {
		if stream.CodecType != "video" && stream.CodecType != "audio" {
			continue
		}
		duration, err := strconv.ParseFloat(stream.Duration, 64)
		if err != nil || math.IsNaN(duration) || math.Abs(duration-wantDuration) > 0.25 {
			t.Errorf("%s duration = %q, want %.3f ± 0.25s", stream.CodecType, stream.Duration, wantDuration)
		}
		switch stream.CodecType {
		case "video":
			video++
			if stream.CodecName != "h264" || stream.FrameRate != "60/1" {
				t.Errorf("video = %s at %s, want h264 at 60 fps", stream.CodecName, stream.FrameRate)
			}
		case "audio":
			audio++
			if stream.CodecName != "aac" || stream.Channels != 2 {
				t.Errorf("audio = %s, %d channels, want stereo AAC", stream.CodecName, stream.Channels)
			}
		}
	}
	if video != 1 || audio != 1 {
		t.Fatalf("streams: video=%d audio=%d, want one each", video, audio)
	}
	if out, err := exec.Command(ffmpeg, "-v", "error", "-xerror", "-i", path, "-map", "0:v:0", "-map", "0:a:0", "-f", "null", "-").CombinedOutput(); err != nil {
		t.Fatalf("decode complete media: %v: %s", err, out)
	}
}
