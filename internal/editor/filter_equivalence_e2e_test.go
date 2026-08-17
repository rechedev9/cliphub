package editor

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// These tests run real FFmpeg over synthetic footage to answer the one question
// the string-level tests cannot: does the optimized filter chain decode to the
// same pixels as the chain it replaced? They skip when FFmpeg is absent, and
// they never touch HLAE, CS2, or any recorded demo.

func ffmpegForEquivalence(t *testing.T) string {
	t.Helper()
	path, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg not on PATH; skipping filter equivalence check")
	}
	return path
}

// normalizedSource renders the deterministic test pattern already conformed to
// the compilation output geometry and fps. That is exactly what the post-concat
// stage receives at render time, because the per-part chain normalized it first.
func normalizedSource(t *testing.T, ffmpeg, dir string, width, height, fps int) string {
	t.Helper()
	path := filepath.Join(dir, "normalized.mp4")
	cmd := exec.Command(ffmpeg,
		"-y", "-v", "error",
		"-f", "lavfi",
		"-i", fmt.Sprintf("testsrc2=size=%dx%d:rate=%d:duration=1", width, height, fps),
		"-c:v", "libx264", "-preset", "ultrafast", "-crf", "18",
		"-pix_fmt", "yuv420p",
		path,
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("render normalized source: %v: %s", err, out)
	}
	return path
}

// filteredFrameHash decodes input through filter and hashes the raw frames, so
// two chains match only when every pixel of every frame matches.
func filteredFrameHash(t *testing.T, ffmpeg, input, filter string) string {
	t.Helper()
	var stdout, stderr bytes.Buffer
	cmd := exec.Command(ffmpeg,
		"-y", "-v", "error",
		"-i", input,
		"-vf", filter,
		"-f", "rawvideo", "-pix_fmt", "yuv420p",
		"-",
	)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("filter %q: %v: %s", filter, err, stderr.String())
	}
	if stdout.Len() == 0 {
		t.Fatalf("filter %q produced no frames: %s", filter, stderr.String())
	}
	sum := sha256.Sum256(stdout.Bytes())
	return hex.EncodeToString(sum[:])
}

// TestPostConcatFilterPixelEquivalence is the truth table for B6: dropping the
// duplicated geometry/fps pass after the concat must not change a single pixel.
// The temporal-smoothing row is the documented exception — see its comment.
func TestPostConcatFilterPixelEquivalence(t *testing.T) {
	ffmpeg := ffmpegForEquivalence(t)
	dir := t.TempDir()

	tests := []struct {
		name string
		// short as it reaches the post-concat stage; Parts are irrelevant here
		// because that stage sees one already-concatenated stream.
		short ShortEdit
		// wantIdentical is the truth column: true means the optimized chain
		// must decode to byte-identical frames.
		wantIdentical bool
		why           string
	}{
		{
			name:          "no effects",
			short:         ShortEdit{Preset: PresetViral60Clean, DurationSeconds: 1, OutputFPS: 60},
			wantIdentical: true,
			why:           "scale/crop/setsar/fps over already-conformed frames is a no-op",
		},
		{
			name: "grade effect",
			short: ShortEdit{
				Preset: PresetViral60Clean, DurationSeconds: 1, OutputFPS: 60,
				Effects: []Effect{{Type: EffectGrade, Contrast: 1.1, Saturation: 1.05, Gamma: 1}},
			},
			wantIdentical: true,
			why:           "the grade still runs post-concat; only the redundant geometry is gone",
		},
		{
			name: "zoom effect falls back to the full chain",
			short: ShortEdit{
				Preset: PresetViral60Clean, DurationSeconds: 1, OutputFPS: 60,
				Effects: []Effect{{Type: EffectZoom, StartSeconds: 0.2, EndSeconds: 0.8, AtSeconds: 0.5, Scale: 1.2}},
			},
			wantIdentical: true,
			why:           "zoom bakes a timeline-dependent scale the per-part pass never saw",
		},
		{
			name: "temporal smoothing is applied once instead of twice",
			short: ShortEdit{
				Preset: PresetViral60Clean, DurationSeconds: 1, OutputFPS: 60,
				TemporalSmoothing: true,
			},
			// tmix is not idempotent: the old chain blended frames in the
			// per-part pass AND again after the concat. Dropping the second
			// pass is a deliberate output change, not a no-op.
			wantIdentical: false,
			why:           "tmix ran twice before this change and now runs once",
		},
	}

	source := normalizedSource(t, ffmpeg, dir, 1080, 1920, 60)

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			legacy := filteredFrameHash(t, ffmpeg, source, VideoFilter(tc.short))
			optimized := filteredFrameHash(t, ffmpeg, source, compilationPostConcatFilter(tc.short))
			identical := legacy == optimized
			if identical != tc.wantIdentical {
				t.Fatalf("pixel-identical = %v, want %v (%s)\n  legacy    %s -> %s\n  optimized %s -> %s",
					identical, tc.wantIdentical, tc.why,
					VideoFilter(tc.short), legacy,
					compilationPostConcatFilter(tc.short), optimized)
			}
		})
	}
}

// legacyKillfeedCropFilter reproduces the pre-B2 ordering: every frame from t=0
// was scaled, cropped and lanczos-rescaled before the trim discarded all but
// the sampled one.
func legacyKillfeedCropFilter(effect Effect, short ShortEdit, sampleSeconds float64) string {
	cropWidth := effect.CropWidth
	if cropWidth == 0 {
		cropWidth = 360
	}
	cropHeight := effect.CropHeight
	if cropHeight == 0 {
		cropHeight = 110
	}
	filters := []string{
		scaleFilter("1080", short),
		fmt.Sprintf("crop=%d:%d:%d:%d", cropWidth, cropHeight, effect.CropX, effect.CropY),
		sourceCropScaleFilter(effect),
		fmt.Sprintf("trim=start=%.3f:duration=0.050", sampleSeconds),
		"loop=loop=-1:size=1:start=0",
		fmt.Sprintf("setpts=N/%d/TB", outputFPS(short)),
	}
	if short.DurationSeconds > 0 {
		filters = append(filters, fmt.Sprintf("trim=duration=%.3f", short.DurationSeconds))
	}
	filters = append(filters, gradeFilters(short.Effects)...)
	filters = append(filters, "curves=all='0/0 0.35/0.08 1/1'")
	filters = append(filters, "format=rgba")
	return strings.Join(filters, ",")
}

// TestKillfeedCropFilterPixelEquivalence is the truth table for B2: moving the
// trim ahead of the per-frame filters must freeze the same sampled frame.
func TestKillfeedCropFilterPixelEquivalence(t *testing.T) {
	ffmpeg := ffmpegForEquivalence(t)
	dir := t.TempDir()
	// A 16:9 source, like the HLAE capture the killfeed is cropped from.
	source := normalizedSource(t, ffmpeg, dir, 1920, 1080, 60)

	tests := []struct {
		name          string
		effect        Effect
		sampleSeconds float64
	}{
		{
			name:          "static crop defaults",
			effect:        Effect{Type: EffectKillfeed, CropX: 1558, CropY: 64, CropWidth: 360, CropHeight: 110},
			sampleSeconds: 0.500,
		},
		{
			name:          "probe-refined crop",
			effect:        Effect{Type: EffectKillfeed, CropX: 1602, CropY: 71, CropWidth: 288, CropHeight: 41, Width: 344},
			sampleSeconds: 0.750,
		},
		{
			name:          "sample at the very start",
			effect:        Effect{Type: EffectKillfeed, CropX: 1558, CropY: 64, CropWidth: 360, CropHeight: 110},
			sampleSeconds: 0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			short := ShortEdit{Preset: PresetViral60Clean, DurationSeconds: 0.5, OutputFPS: 60}
			legacy := filteredFrameHash(t, ffmpeg, source, legacyKillfeedCropFilter(tc.effect, short, tc.sampleSeconds))
			optimized := filteredFrameHash(t, ffmpeg, source, killfeedCropFilter(tc.effect, short, tc.sampleSeconds))
			if legacy != optimized {
				t.Fatalf("killfeed crop frames differ after moving trim first\n  legacy    %s -> %s\n  optimized %s -> %s",
					legacyKillfeedCropFilter(tc.effect, short, tc.sampleSeconds), legacy,
					killfeedCropFilter(tc.effect, short, tc.sampleSeconds), optimized)
			}
		})
	}
}

func TestViralMotionFiltersRenderWithFFmpeg(t *testing.T) {
	ffmpeg := ffmpegForEquivalence(t)
	source := normalizedSource(t, ffmpeg, t.TempDir(), 1080, 1920, 60)
	cases := []struct {
		name  string
		short ShortEdit
	}{
		{
			name: "shake kill",
			short: ShortEdit{
				Preset: PresetViral60Clean, DurationSeconds: 1, OutputFPS: 60,
				KillEffect: KillEffectShake,
				Kills:      []KillCue{{TimeSeconds: 0.4}},
			},
		},
		{
			name: "glitch kill",
			short: ShortEdit{
				Preset: PresetViral60Clean, DurationSeconds: 1, OutputFPS: 60,
				KillEffect: KillEffectGlitch,
				Kills:      []KillCue{{TimeSeconds: 0.4}},
			},
		},
		{
			name: "glitch transition",
			short: ShortEdit{
				Preset: PresetViral60Clean, DurationSeconds: 1, OutputFPS: 60,
				Transition: TransitionGlitch,
			},
		},
		{
			name: "zoom-whip transition",
			short: ShortEdit{
				Preset: PresetViral60Clean, DurationSeconds: 1, OutputFPS: 60,
				Transition: TransitionZoomWhip,
			},
		},
		{
			name: "aggressive headshot chroma",
			short: ShortEdit{
				Preset: PresetViralAggressive60, DurationSeconds: 1, OutputFPS: 60,
				Effects: []Effect{
					{Type: EffectGrade, Contrast: 1.25, Saturation: 1.45, Gamma: 1.04},
					{Type: EffectChroma, StartSeconds: 0.3, EndSeconds: 0.5, Amplitude: 10},
					{Type: EffectFlash, StartSeconds: 0.3, EndSeconds: 0.36, Color: "#00ffff", Opacity: 0.18},
				},
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if len(tc.short.Effects) == 0 {
				tc.short.Effects = generatedEditEffects(tc.short)
			}
			filter := VideoFilter(tc.short)
			if filter == "" {
				t.Fatal("empty video filter")
			}
			if hash := filteredFrameHash(t, ffmpeg, source, filter); hash == "" {
				t.Fatal("ffmpeg produced an empty hash")
			}
		})
	}
}
