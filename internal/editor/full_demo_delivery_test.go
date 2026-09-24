package editor

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rechedev9/cliphub/internal/obs"
	"github.com/rechedev9/cliphub/internal/recapplan"
	"github.com/rechedev9/cliphub/internal/recording"
)

// The alerter's absolute floors for a 1080p60 Full Demo delivery.
const (
	deliveryQualityTestBitrateFloorKbps = 10000
	deliveryQualityTestLumaFloor        = 25
)

var deliveryQualityMessagePattern = regexp.MustCompile(`^preset=gameplay-pov-60 width=1920 height=1080 fps=60 bitrate_kbps=[0-9]+ yavg_mean=[0-9]+\.[0-9] black_ratio=[01]\.[0-9]{3}$`)

// writeDeliveryQualityFixture encodes a 1080p60 H.264 + stereo AAC delivery
// from one lavfi video graph. It runs FFmpeg directly with plain -i inputs, so
// it needs no -filter_complex_script (removed in FFmpeg 9).
func writeDeliveryQualityFixture(t *testing.T, ctx context.Context, path, video string, seconds float64, videoArgs ...string) {
	t.Helper()
	duration := strconv.FormatFloat(seconds, 'f', -1, 64)
	args := []string{"-y", "-v", "error",
		"-f", "lavfi", "-i", video,
		"-f", "lavfi", "-i", "sine=f=440:r=48000:d=" + duration,
		"-map", "0:v:0", "-map", "1:a:0", "-t", duration,
		"-c:v", "libx264", "-preset", "ultrafast", "-pix_fmt", "yuv420p"}
	args = append(args, videoArgs...)
	args = append(args, "-c:a", "aac", "-ac", "2", "-ar", "48000", path)
	// #nosec G204 -- fixed test arguments.
	if output, err := exec.CommandContext(ctx, fullDemoTestFFmpeg(t), args...).CombinedOutput(); err != nil {
		t.Fatalf("delivery fixture: %v\n%s", err, output)
	}
}

func deliveryQualityTestFFprobe(t *testing.T) string {
	t.Helper()
	ffprobe, err := exec.LookPath("ffprobe")
	if err != nil {
		t.Skip("ffprobe is required for the delivery quality probe")
	}
	return ffprobe
}

func assertNoDeliveryQualityScratch(t *testing.T, dir string) {
	t.Helper()
	leftovers, err := filepath.Glob(filepath.Join(dir, ".delivery-quality-*"))
	if err != nil || len(leftovers) != 0 {
		t.Fatalf("quality probe scratch was not removed: %v %v", leftovers, err)
	}
}

// A normal gameplay-like delivery stays above every floor, and the probe's
// file output survives a directory whose name needs filtergraph quoting.
func TestFullDemoDeliveryQualityOnNormalFixture(t *testing.T) {
	ffprobe := deliveryQualityTestFFprobe(t)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	dir := filepath.Join(t.TempDir(), "q di'r, [x];y")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(dir, "delivery.mp4")
	writeDeliveryQualityFixture(t, ctx, file, "testsrc2=s=1920x1080:r=60:d=2,noise=alls=20:allf=t", 2, "-crf", "12")

	outcome, err := verifyFullDemoDeliveryWithDiagnostics(ctx, fullDemoTestFFmpeg(t), ffprobe, file, 120, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	quality := outcome.Quality
	if quality == nil {
		t.Fatal("delivery decode produced no quality probe result")
	}
	info, err := os.Stat(file)
	if err != nil {
		t.Fatal(err)
	}
	if want := int64(math.Round(float64(info.Size()) * 8 / 2 / 1000)); quality.BitrateKbps != want {
		t.Fatalf("bitrate_kbps = %d, want size*8/duration/1000 = %d", quality.BitrateKbps, want)
	}
	if quality.Width != 1920 || quality.Height != 1080 || quality.FPS != 60 || quality.BitrateKbps < deliveryQualityTestBitrateFloorKbps || quality.YAVGMean < deliveryQualityTestLumaFloor || quality.YAVGMean > 235 || quality.BlackRatio != 0 {
		t.Fatalf("normal delivery quality = %+v", *quality)
	}
	if message := quality.message(PresetGameplayPOV60); !deliveryQualityMessagePattern.MatchString(message) {
		t.Fatalf("delivery.quality message = %q", message)
	}
	if outcome.QualityLog != "" || outcome.QualityWarnings != nil {
		t.Fatalf("the probe alone must not produce QC output: %q %v", outcome.QualityLog, outcome.QualityWarnings)
	}
	assertNoDeliveryQualityScratch(t, dir)
}

// The 5.0.0 incident: a black world with only the HUD and crosshair drawn.
// The HUD keeps the picture under the QC checker's black threshold, so the
// mean luma and the bitrate must flag it.
func TestFullDemoDeliveryQualityFlagsBlackCaptureWithHUD(t *testing.T) {
	ffprobe := deliveryQualityTestFFprobe(t)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	hud := "color=c=black:s=1920x1080:r=60:d=3" +
		",drawbox=x=40:y=1000:w=360:h=36:c=white@0.8:t=fill" +
		",drawbox=x=1520:y=1000:w=360:h=36:c=white@0.8:t=fill" +
		",drawbox=x=840:y=16:w=240:h=48:c=gray:t=fill" +
		",drawbox=x=954:y=534:w=12:h=12:c=lime:t=fill"
	if font := filepath.Join(os.Getenv("WINDIR"), "Fonts", "arial.ttf"); os.Getenv("WINDIR") != "" {
		if _, err := os.Stat(font); err == nil {
			hud += ",drawtext=fontfile=" + ffmpegQuotedFilterPath(font) + ":text=100:x=60:y=940:fontsize=40:fontcolor=white"
		}
	}
	dir := t.TempDir()
	file := filepath.Join(dir, "black.mp4")
	writeDeliveryQualityFixture(t, ctx, file, hud, 3)

	outcome, err := verifyFullDemoDeliveryWithDiagnostics(ctx, fullDemoTestFFmpeg(t), ffprobe, file, 180, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	quality := outcome.Quality
	if quality == nil {
		t.Fatal("delivery decode produced no quality probe result")
	}
	if quality.YAVGMean >= deliveryQualityTestLumaFloor || quality.BitrateKbps >= deliveryQualityTestBitrateFloorKbps {
		t.Fatalf("black capture with HUD passed the floors: %+v", *quality)
	}
	assertNoDeliveryQualityScratch(t, dir)
}

// Black runs pair start/end markers, drop runs shorter than 0.5 s and extend a
// run still open at the end of the stream to the end.
func TestFullDemoDeliveryQualityBlackRatio(t *testing.T) {
	ffprobe := deliveryQualityTestFFprobe(t)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	file := filepath.Join(t.TempDir(), "runs.mp4")
	writeDeliveryQualityFixture(t, ctx, file, "testsrc2=s=1920x1080:r=60:d=4,drawbox=c=black:t=fill:enable='lt(t,1)+between(t,2,2.2)+gte(t,3)'", 4)

	outcome, err := verifyFullDemoDeliveryWithDiagnostics(ctx, fullDemoTestFFmpeg(t), ffprobe, file, 240, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if outcome.Quality == nil || math.Abs(outcome.Quality.BlackRatio-.5) > .02 {
		t.Fatalf("black_ratio = %+v, want 0.5 (1 s + 1 s open run of 4 s, 0.2 s flash dropped)", outcome.Quality)
	}
}

func TestDeliveryBlackRatioPairsRuns(t *testing.T) {
	for _, tc := range []struct {
		name         string
		starts, ends []float64
		want         float64
	}{
		{"none", nil, nil, 0},
		{"closed run", []float64{1}, []float64{3}, .2},
		{"short flash dropped", []float64{1}, []float64{1.4}, 0},
		{"open run to the end", []float64{8}, nil, .2},
		{"stray end ignored", []float64{5}, []float64{2, 6}, .1},
		{"whole stream", []float64{0}, nil, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := deliveryBlackRatio(tc.starts, tc.ends, 10); math.Abs(got-tc.want) > 1e-9 {
				t.Fatalf("black ratio = %v, want %v", got, tc.want)
			}
		})
	}
}

// The probe's own blackdetect instance must not turn into a QC warning.
func TestQualityWarningsIgnoreDeliveryQualityProbe(t *testing.T) {
	probeOnly := "[blackdetect@cliphub_quality @ 000001] black_start:0 black_end:1 black_duration:1\n"
	if warnings := QualityWarningsFromFFmpegLog("seg", probeOnly); len(warnings) != 0 {
		t.Fatalf("probe line produced QC warnings: %v", warnings)
	}
	both := probeOnly + "[Parsed_blackdetect_0 @ 000002] black_start:0 black_end:1 black_duration:1\n"
	if warnings := QualityWarningsFromFFmpegLog("seg", both); len(warnings) != 1 || !strings.Contains(warnings[0], "black frames") {
		t.Fatalf("QC blackdetect warning lost: %v", warnings)
	}
}

func TestFFmpegQuotedFilterPath(t *testing.T) {
	if got, want := ffmpegQuotedFilterPath(`C:\Out\q di'r, [x];y\a.txt`), `'C\:/Out/q di\'\''r, [x];y/a.txt'`; filepath.Separator == '\\' && got != want {
		t.Fatalf("quoted path = %s, want %s", got, want)
	}
}

func fullDemoProfileShort(options recapplan.Options) ShortEdit {
	return ShortEdit{Preset: PresetGameplayPOV60, FullDemo: &FullDemoRenderEvidence{
		Effective:       recapplan.Document{Options: options},
		ProgramLoudness: &ProgramLoudnessEvidence{Status: "verified-decoded-aac"},
	}}
}

func TestFullDemoRenderProfile(t *testing.T) {
	faceit := fullDemoProfileShort(recapplan.Options{SourceKind: "faceit", Overlays: recapplan.OverlayOptions{Source: "demo"}})
	if got, want := fullDemoRenderProfile(faceit), "source_kind=faceit overlay_source=faceit hud=native fps=60 resolution=1080p encoder=x264 aac_path=native capture_tail_pads=0"; got != want {
		t.Fatalf("faceit profile = %q, want %q", got, want)
	}

	custom := fullDemoProfileShort(recapplan.Options{SourceKind: "demo", Overlays: recapplan.OverlayOptions{Source: "demo", HUDTheme: "circuit"}})
	custom.VideoEncoder = VideoEncoderNVENC
	custom.FullDemo.ProgramLoudness.FallbackMasters = []ProgramAACFallbackMaster{{Encoder: "aac_mf"}}
	custom.FullDemo.CaptureTailPads = []recording.FullDemoTailPad{{SegmentID: "round-001", ClipFrames: 120, WindowFrames: 121, PaddedFrames: 1}}
	if got, want := fullDemoRenderProfile(custom), "source_kind=demo overlay_source=demo hud=circuit fps=60 resolution=1080p encoder=nvenc aac_path=mf_recovery capture_tail_pads=1"; got != want {
		t.Fatalf("custom HUD profile = %q, want %q", got, want)
	}

	enriched := fullDemoProfileShort(recapplan.Options{SourceKind: "demo", Overlays: recapplan.OverlayOptions{Source: "faceit", HUDTheme: "retired-theme"}})
	if got := fullDemoRenderProfile(enriched); !strings.Contains(got, " overlay_source=faceit hud=custom ") {
		t.Fatalf("enriched demo profile = %q", got)
	}
}

func captureEditorTraces(t *testing.T) func() []obs.TraceEntry {
	t.Helper()
	var (
		output bytes.Buffer
		mu     sync.Mutex
	)
	previous := log.Writer()
	log.SetOutput(writerFunc(func(p []byte) (int, error) {
		mu.Lock()
		defer mu.Unlock()
		return output.Write(p)
	}))
	t.Cleanup(func() { log.SetOutput(previous) })
	return func() []obs.TraceEntry {
		mu.Lock()
		defer mu.Unlock()
		var entries []obs.TraceEntry
		for _, line := range strings.Split(output.String(), "\n") {
			_, body, ok := strings.Cut(line, obs.TracePrefix)
			if !ok {
				continue
			}
			var entry obs.TraceEntry
			if err := json.Unmarshal([]byte(body), &entry); err != nil {
				t.Fatalf("trace line %q: %v", line, err)
			}
			entries = append(entries, entry)
		}
		return entries
	}
}

type writerFunc func([]byte) (int, error)

func (f writerFunc) Write(p []byte) (int, error) { return f(p) }

func tracesOf(entries []obs.TraceEntry, event string) []string {
	var messages []string
	for _, entry := range entries {
		if entry.Event == event {
			messages = append(messages, entry.Level+" "+entry.Message)
		}
	}
	return messages
}

func TestFullDemoDeliveryQualityAndProfileTraces(t *testing.T) {
	traces := captureEditorTraces(t)
	ctx := context.Background()
	emitFullDemoDeliveryQuality(ctx, PresetGameplayPOV60, &fullDemoDeliveryQuality{Width: 1920, Height: 1080, FPS: 60, BitrateKbps: 2400, YAVGMean: 17.25, BlackRatio: .4})
	emitFullDemoDeliveryQuality(ctx, PresetGameplayPOV60, nil)
	emitFullDemoRenderProfile(ctx, fullDemoProfileShort(recapplan.Options{SourceKind: "faceit", Overlays: recapplan.OverlayOptions{Source: "demo"}}))
	emitFullDemoRenderProfile(ctx, ShortEdit{})
	entries := traces()
	if got := tracesOf(entries, "delivery.quality"); len(got) != 1 || got[0] != "info preset=gameplay-pov-60 width=1920 height=1080 fps=60 bitrate_kbps=2400 yavg_mean=17.2 black_ratio=0.400" {
		t.Fatalf("delivery.quality traces = %q", got)
	}
	if got := tracesOf(entries, "render.profile"); len(got) != 1 || !strings.HasPrefix(got[0], "info source_kind=faceit overlay_source=faceit ") {
		t.Fatalf("render.profile traces = %q", got)
	}
}

func TestFullDemoStageBreadcrumbs(t *testing.T) {
	traces := captureEditorTraces(t)
	base, _ := withFullDemoTimingCollector(context.Background())
	base = withFullDemoStageBreadcrumbs(base)
	render := fullDemoTimingScope(base, "full_demo", 0, -1, 10)
	master := func(tp string) []string {
		return []string{"ffmpeg", "-i", "in.wav", "-af", "loudnorm=I=-14.000000:TP=" + tp + ":LRA=11.000000:measured_I=-20.1:measured_TP=-3.5:measured_LRA=4:measured_thresh=-30:offset=0:linear=true", "out.m4a"}
	}
	for _, step := range []struct {
		ctx     context.Context
		command []string
	}{
		{fullDemoTimingScope(base, "items", 3, -1, 2), []string{"ffmpeg"}},
		{fullDemoTimingScope(base, "items", 4, -1, 2), []string{"ffmpeg"}},
		{render, []string{"ffmpeg", "-encoders"}},
		{fullDemoTimingStage(render, "audio_input_analysis", -1), master("-1.000000")},
		{fullDemoTimingStageVariant(render, "audio_candidate_encode", 0, "aac", "native"), master("-1.300000")},
		{fullDemoTimingStageVariant(render, "audio_candidate_analysis", 0, "aac", "native"), master("-1.000000")},
		{fullDemoTimingStageVariant(render, "audio_candidate_encode", 0, "aac_mf", "recovery"), []string{"ffmpeg", "-c:a", "aac_mf"}},
		{fullDemoTimingStage(render, "audio_input_analysis", 2), master("-10.180000")},
		{fullDemoTimingScope(base, "delivery", 0, -1, 10), []string{"ffprobe"}},
		{fullDemoTimingScope(base, "delivery", 0, -1, 10), []string{"ffmpeg"}},
	} {
		enterFullDemoStage(step.ctx, step.command)
	}
	// A render without breadcrumbs (every non-Full-Demo render) emits nothing.
	plain, _ := withFullDemoTimingCollector(context.Background())
	enterFullDemoStage(fullDemoTimingScope(plain, "delivery", 0, -1, 10), []string{"ffmpeg"})

	want := []string{
		"info stage=video_items attempt=1",
		"info stage=audio_analysis attempt=1",
		"info stage=audio_master attempt=1 target_tp=-1.3",
		"info stage=aac_recovery attempt=1",
		"info stage=audio_master attempt=3 target_tp=-10.18",
		"info stage=delivery_verify attempt=1",
	}
	if got := tracesOf(traces(), "stage.entered"); strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("stage.entered traces:\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

// The breadcrumb fires from the real FFmpeg runner before the process starts,
// and reads TP from the original command even when it is a filter graph.
func TestFullDemoStageBreadcrumbFromFFmpegRun(t *testing.T) {
	ffmpeg := fullDemoTestFFmpeg(t)
	traces := captureEditorTraces(t)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	base, _ := withFullDemoTimingCollector(ctx)
	base = withFullDemoStageBreadcrumbs(base)
	stageCtx := fullDemoTimingStage(fullDemoTimingScope(base, "full_demo", 0, -1, 1), "audio_input_analysis", 1)
	command := []string{ffmpeg, "-hide_banner", "-nostats", "-v", "info", "-f", "lavfi", "-i", "sine=f=440:r=48000:d=1", "-af", "loudnorm=I=-14:TP=-2.5:LRA=11:print_format=json", "-f", "null", "-"}
	if _, err := runFFmpegOutputProgress(stageCtx, command, "breadcrumb canary", 1, func(float64) {}); err != nil {
		t.Fatal(err)
	}
	entries := traces()
	crumb, started := -1, -1
	for i, entry := range entries {
		switch {
		case entry.Event == "stage.entered" && entry.Message == "stage=audio_master attempt=2 target_tp=-2.5":
			crumb = i
		case entry.Event == "tool.started" && started < 0:
			started = i
		}
	}
	if crumb < 0 || started < 0 || crumb > started {
		t.Fatalf("breadcrumb must precede tool.started: crumb=%d started=%d %+v", crumb, started, entries)
	}
}
