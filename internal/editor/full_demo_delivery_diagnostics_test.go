package editor

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"
)

// The combined delivery decode must produce exactly the same optional quality
// warnings the legacy separate black/freeze checker produced, while keeping the
// strict frame evidence. Positive black AND freeze events are required: clean
// media alone cannot catch a broken filter graph.
func TestFullDemoDeliveryCombinedDiagnosticsMatchLegacyChecker(t *testing.T) {
	ffmpeg := fullDemoTestFFmpeg(t)
	ffprobe, err := exec.LookPath("ffprobe")
	if err != nil {
		t.Skip("ffprobe is required for the delivery diagnostics canary")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	file := filepath.Join(t.TempDir(), "events.mp4")
	command := []string{ffmpeg, "-y", "-v", "error",
		"-f", "lavfi", "-i", "color=c=black:s=1920x1080:r=60:d=1.5",
		"-f", "lavfi", "-i", "color=c=gray:s=1920x1080:r=60:d=1.5",
		"-f", "lavfi", "-i", "testsrc2=s=1920x1080:r=60:d=3",
		"-f", "lavfi", "-i", "anullsrc=r=48000:cl=stereo:d=6",
		"-filter_complex", "[0:v][1:v][2:v]concat=n=3:v=1:a=0[v]",
		"-map", "[v]", "-map", "3:a", "-c:v", "libx264", "-preset", "ultrafast", "-pix_fmt", "yuv420p", "-c:a", "aac", "-t", "6", file}
	if _, err := runFFmpegOutput(ctx, command, "delivery diagnostics fixture"); err != nil {
		t.Fatal(err)
	}
	short := ShortEdit{Output: file, Preset: PresetGameplayPOV60}
	legacyLog, err := runFFmpegOutput(ctx, BuildQualityCheckFFmpegCommand(ffmpeg, short), "quality check")
	if err != nil {
		t.Fatal(err)
	}
	legacy := QualityWarningsFromFFmpegLog(short.SegmentID, legacyLog)
	if len(legacy) < 2 || !containsAny(legacy, "black frames") || !containsAny(legacy, "frozen frames") {
		t.Fatalf("legacy checker did not see the positive events: %v", legacy)
	}

	outcome, err := verifyFullDemoDeliveryWithDiagnostics(ctx, ffmpeg, ffprobe, file, 360, nil, &fullDemoDeliveryDiagnostics{
		SegmentID: short.SegmentID,
		Filters:   qualityCheckFilters(short),
	})
	if err != nil {
		t.Fatal(err)
	}
	if outcome.Evidence.FrameCount != 360 || !outcome.Evidence.FullDecode {
		t.Fatalf("combined decode lost strict frame evidence: %+v", outcome.Evidence)
	}
	if !reflect.DeepEqual(outcome.QualityWarnings, legacy) {
		t.Fatalf("combined quality warnings = %v, want legacy %v", outcome.QualityWarnings, legacy)
	}
	if !strings.Contains(outcome.QualityLog, "black_start:") || !strings.Contains(outcome.QualityLog, "freeze_start:") {
		t.Fatalf("combined diagnostics log lacks positive events:\n%s", outcome.QualityLog)
	}
}

// A clean real FFmpeg complete decode must keep the canonical count, stay
// unmodified, and report no quality events while progress only moves forward.
func TestFullDemoDeliveryCombinedDiagnosticsCleanMedia(t *testing.T) {
	ffmpeg := fullDemoTestFFmpeg(t)
	ffprobe, err := exec.LookPath("ffprobe")
	if err != nil {
		t.Skip("ffprobe is required for the delivery diagnostics canary")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	file := fullDemoCleanDeliveryFixture(t, ctx, ffmpeg, 1)
	var fractions []float64
	progress := func(stage string, fraction float64) {
		if stage == "Verificando fotogramas, vídeo y audio" {
			fractions = append(fractions, fraction)
		}
	}
	outcome, err := verifyFullDemoDeliveryWithDiagnostics(ctx, ffmpeg, ffprobe, file, 60, progress, &fullDemoDeliveryDiagnostics{
		SegmentID: "demo-compilation",
		Filters:   qualityCheckFilters(ShortEdit{Preset: PresetGameplayPOV60}),
	})
	if err != nil {
		t.Fatal(err)
	}
	if outcome.Evidence.FrameCount != 60 || len(outcome.Evidence.ContentSHA256) != 64 {
		t.Fatalf("unexpected clean evidence: %+v", outcome.Evidence)
	}
	if len(outcome.QualityWarnings) != 0 {
		t.Fatalf("clean media reported quality warnings: %v", outcome.QualityWarnings)
	}
	if outline := QualityWarningsFromFFmpegLog("demo-compilation", outcome.QualityLog); len(outline) != 0 {
		t.Fatalf("clean diagnostics parsed events: %v", outline)
	}
	for i := 1; i < len(fractions); i++ {
		if fractions[i] < fractions[i-1] {
			t.Fatalf("non-monotonic decode progress: %v", fractions)
		}
	}
	if len(fractions) < 2 || fractions[len(fractions)-1] <= fractions[0] {
		t.Fatalf("complete decode did not report media progress: %v", fractions)
	}
}

// Disabling the optional diagnostics must not change the mandatory delivery
// evidence; and truncated media must still fail the combined decode, never be
// rescued by the diagnostic fallback.
func TestFullDemoDeliveryCombinedDiagnosticsDisabledAndTruncated(t *testing.T) {
	ffmpeg := fullDemoTestFFmpeg(t)
	ffprobe, err := exec.LookPath("ffprobe")
	if err != nil {
		t.Skip("ffprobe is required for the delivery diagnostics canary")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	file := fullDemoCleanDeliveryFixture(t, ctx, ffmpeg, 1)
	plain, err := verifyFullDemoDeliveryWithDiagnostics(ctx, ffmpeg, ffprobe, file, 60, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if plain.QualityLog != "" || len(plain.QualityWarnings) != 0 {
		t.Fatalf("disabled diagnostics produced quality data: %+v", plain)
	}
	if !plain.Evidence.FullDecode || plain.Evidence.FrameCount != 60 {
		t.Fatalf("disabled diagnostics lost strict frame evidence: %+v", plain.Evidence)
	}
	if _, err := verifyFullDemoDeliveryWithDiagnostics(ctx, ffmpeg, ffprobe, file, 59, nil, nil); err == nil {
		t.Fatal("wrong canonical frame count passed delivery")
	}
	combined, err := verifyFullDemoDeliveryWithDiagnostics(ctx, ffmpeg, ffprobe, file, 60, nil, &fullDemoDeliveryDiagnostics{
		SegmentID: "demo-compilation",
		Filters:   qualityCheckFilters(ShortEdit{Preset: PresetGameplayPOV60}),
	})
	if err != nil {
		t.Fatal(err)
	}
	if plain.Evidence.ContentSHA256 != combined.Evidence.ContentSHA256 || plain.Evidence.FrameCount != combined.Evidence.FrameCount {
		t.Fatalf("diagnostics altered delivery evidence: %+v vs %+v", plain.Evidence, combined.Evidence)
	}

	fast := filepath.Join(t.TempDir(), "truncated.mp4")
	if _, err := runFFmpegOutput(ctx, []string{ffmpeg, "-v", "error", "-i", file, "-c", "copy", "-movflags", "+faststart", fast}, "faststart fixture"); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(fast)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Truncate(fast, info.Size()-500); err != nil {
		t.Fatal(err)
	}
	if _, err := verifyFullDemoDeliveryWithDiagnostics(ctx, ffmpeg, ffprobe, fast, 60, nil, &fullDemoDeliveryDiagnostics{
		SegmentID: "demo-compilation",
		Filters:   qualityCheckFilters(ShortEdit{Preset: PresetGameplayPOV60}),
	}); err == nil {
		t.Fatal("truncated media passed the combined decode")
	}

	// A corrupt container must fail the probe/decode instead of being rescued
	// by the diagnostic fallback.
	corrupt := filepath.Join(t.TempDir(), "corrupt.mp4")
	body, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 64 && i < len(body); i++ {
		body[i] = 0xFF
	}
	if err := os.WriteFile(corrupt, body, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := verifyFullDemoDeliveryWithDiagnostics(ctx, ffmpeg, ffprobe, corrupt, 60, nil, &fullDemoDeliveryDiagnostics{
		SegmentID: "demo-compilation",
		Filters:   qualityCheckFilters(ShortEdit{Preset: PresetGameplayPOV60}),
	}); err == nil {
		t.Fatal("corrupt media passed the combined decode")
	}
}

// An optional diagnostic filter that cannot be configured must not turn the
// optional check into a fatal error: strict delivery still validates and the
// setup problem becomes a warning.
func TestFullDemoDeliveryDiagnosticSetupFailureKeepsStrictDelivery(t *testing.T) {
	ffmpeg := fullDemoTestFFmpeg(t)
	ffprobe, err := exec.LookPath("ffprobe")
	if err != nil {
		t.Skip("ffprobe is required for the delivery diagnostics canary")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	file := fullDemoCleanDeliveryFixture(t, ctx, ffmpeg, 1)
	outcome, err := verifyFullDemoDeliveryWithDiagnostics(ctx, ffmpeg, ffprobe, file, 60, nil, &fullDemoDeliveryDiagnostics{
		SegmentID: "demo-compilation",
		Filters:   []string{"no_such_filter"},
	})
	if err != nil {
		t.Fatalf("optional diagnostic setup failure became fatal: %v", err)
	}
	if !outcome.Evidence.FullDecode || outcome.Evidence.FrameCount != 60 {
		t.Fatalf("strict delivery was weakened: %+v", outcome.Evidence)
	}
	if len(outcome.QualityWarnings) != 1 || !strings.Contains(outcome.QualityWarnings[0], "quality check demo-compilation:") {
		t.Fatalf("setup failure did not become a quality warning: %v", outcome.QualityWarnings)
	}
	// The original setup stderr must survive the strict retry instead of being
	// overwritten by it.
	if !strings.Contains(outcome.QualityLog, "No such filter") {
		t.Fatalf("original missing-filter stderr was lost:\n%s", outcome.QualityLog)
	}
}

// Malformed audio must be rejected on its own path, separately from corrupted
// or truncated video.
func TestFullDemoDeliveryRejectsMalformedAudio(t *testing.T) {
	ffmpeg := fullDemoTestFFmpeg(t)
	ffprobe, err := exec.LookPath("ffprobe")
	if err != nil {
		t.Skip("ffprobe is required for the delivery diagnostics canary")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	dir := t.TempDir()
	for _, tc := range []struct {
		name     string
		audio    []string
		probeErr string
	}{
		{"wrong sample rate and channels", []string{"-c:a", "aac", "-ar", "44100", "-ac", "1"}, "stereo AAC at 48 kHz"},
		{"no audio stream", nil, "missing video or audio"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			file := filepath.Join(dir, strings.ReplaceAll(tc.name, " ", "-")+".mp4")
			command := []string{ffmpeg, "-y", "-v", "error", "-f", "lavfi", "-i", "testsrc2=s=1920x1080:r=60:d=1"}
			if tc.audio != nil {
				command = append(command, "-f", "lavfi", "-i", "anullsrc=r=48000:cl=stereo")
			}
			command = append(command, "-t", "1", "-c:v", "libx264", "-preset", "ultrafast", "-pix_fmt", "yuv420p")
			if tc.audio != nil {
				command = append(command, "-map", "0:v:0", "-map", "1:a:0")
				command = append(command, tc.audio...)
			}
			command = append(command, file)
			if _, err := runFFmpegOutput(ctx, command, "malformed audio fixture"); err != nil {
				t.Fatal(err)
			}
			_, err := verifyFullDemoDeliveryWithDiagnostics(ctx, ffmpeg, ffprobe, file, 60, nil, &fullDemoDeliveryDiagnostics{
				SegmentID: "demo-compilation",
				Filters:   qualityCheckFilters(ShortEdit{Preset: PresetGameplayPOV60}),
			})
			if err == nil || !strings.Contains(err.Error(), tc.probeErr) {
				t.Fatalf("malformed audio was not rejected with %q: %v", tc.probeErr, err)
			}
		})
	}
}

func TestFullDemoDeliveryDiagnosticSetupErrorClassification(t *testing.T) {
	for _, message := range []string{
		"No such filter: 'definitely_not_a_real_filter'",
		"Error initializing filter 'freezedetect'",
		"Failed to configure output pad on Parsed_blackdetect_0",
		"Error parsing filterchain 'definitely_not_a_real_filter=1' around:",
	} {
		if !fullDemoDeliveryDiagnosticSetupError(errors.New(message)) {
			t.Fatalf("setup error not classified: %s", message)
		}
	}
	for _, message := range []string{
		"Invalid data found when processing input",
		"Error while decoding stream #0:0: Invalid data",
		"moov atom not found",
	} {
		if fullDemoDeliveryDiagnosticSetupError(errors.New(message)) {
			t.Fatalf("media error misclassified as a filter setup failure: %s", message)
		}
	}
	if fullDemoDeliveryDiagnosticSetupError(nil) {
		t.Fatal("nil error classified as a setup failure")
	}
}

func TestFullDemoDeliveryCombinedDiagnosticsCancellation(t *testing.T) {
	ffmpeg := fullDemoTestFFmpeg(t)
	ffprobe, err := exec.LookPath("ffprobe")
	if err != nil {
		t.Skip("ffprobe is required for the delivery diagnostics canary")
	}
	file := fullDemoCleanDeliveryFixture(t, context.Background(), ffmpeg, 1)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := verifyFullDemoDeliveryWithDiagnostics(ctx, ffmpeg, ffprobe, file, 60, nil, &fullDemoDeliveryDiagnostics{
		SegmentID: "demo-compilation",
		Filters:   qualityCheckFilters(ShortEdit{Preset: PresetGameplayPOV60}),
	}); err == nil {
		t.Fatal("cancelled delivery decode succeeded")
	}
}

func fullDemoCleanDeliveryFixture(t *testing.T, ctx context.Context, ffmpeg string, seconds int) string {
	t.Helper()
	file := filepath.Join(t.TempDir(), "delivery.mp4")
	command := []string{ffmpeg, "-y", "-v", "error", "-f", "lavfi", "-i", "testsrc2=s=1920x1080:r=60:d=" + strconv.Itoa(seconds), "-f", "lavfi", "-i", "anullsrc=r=48000:cl=stereo", "-t", strconv.Itoa(seconds), "-c:v", "libx264", "-preset", "ultrafast", "-pix_fmt", "yuv420p", "-c:a", "aac", file}
	if _, err := runFFmpegOutput(ctx, command, "delivery fixture"); err != nil {
		t.Fatal(err)
	}
	return file
}

func containsAny(values []string, needle string) bool {
	for _, value := range values {
		if strings.Contains(value, needle) {
			return true
		}
	}
	return false
}
