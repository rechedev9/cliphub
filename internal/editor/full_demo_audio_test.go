package editor

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rechedev9/cliphub/internal/recapplan"
)

func TestFullDemoLoudnessMeasurements(t *testing.T) {
	for _, tc := range []struct {
		name, body, status string
		invalid            bool
	}{
		{"finite", `{"input_i":"-21.50","input_tp":"-5.0","input_lra":"2.1","input_thresh":"-33","target_offset":"0.03"}`, "measured", false},
		{"silence", `{"input_i":"-inf","input_tp":"-inf","input_lra":"0","input_thresh":"-70","target_offset":"inf"}`, "silent", false},
		{"missing peak", `{"input_i":"-20"}`, "", true},
		{"infinite non-silence", `{"input_i":"-inf","input_tp":"-3"}`, "", true},
		{"NaN", `{"input_i":"NaN","input_tp":"-3"}`, "", true},
		{"no measurement", "decoder error", "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			measurement, err := parseLoudnessMeasurement("FFmpeg diagnostics\n" + tc.body)
			if (err != nil) != tc.invalid {
				t.Fatalf("measurement error: %v", err)
			}
			if !tc.invalid && measurement.Status != tc.status {
				t.Fatalf("status: %s", measurement.Status)
			}
			if _, err := json.Marshal(measurement); err != nil {
				t.Fatalf("measurement is not durable JSON: %v", err)
			}
		})
	}
}

func fullDemoTestFFmpeg(t *testing.T) string {
	t.Helper()
	path, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Fatal("FFmpeg is required for the Full Demo media canary:", err)
	}
	return path
}

// These short generated files prove FFmpeg mastering, not CS2/HLAE capture.
func TestFullDemoMasterDecodedAAC(t *testing.T) {
	ffmpeg := fullDemoTestFFmpeg(t)
	for _, tc := range []struct {
		name, audio               string
		silentApproved, wantError bool
		wantStatus                string
	}{
		{"steady program", "sine=frequency=440:duration=4:sample_rate=48000", false, false, "verified-decoded-aac"},
		{"transients", "aevalsrc=0.25*sin(2*PI*440*t)+0.55*sin(2*PI*2311*t)*lt(mod(t\\,1)\\,0.03):s=48000:d=4", false, false, "verified-decoded-aac"},
		{"approved mute", "anullsrc=r=48000:cl=stereo:d=4", true, false, "silent-approved"},
		{"unexpected silence", "anullsrc=r=48000:cl=stereo:d=4", false, true, "silent"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
			defer cancel()
			dir := t.TempDir()
			input, output := filepath.Join(dir, "program.nut"), filepath.Join(dir, "final.mp4")
			command := []string{ffmpeg, "-y", "-v", "error", "-f", "lavfi", "-i", "color=c=navy:s=160x90:r=60:d=4", "-f", "lavfi", "-i", tc.audio, "-map", "0:v", "-map", "1:a", "-c:v", "libx264", "-preset", "ultrafast", "-bf", "0", "-c:a", "pcm_f32le", "-ac", "2", "-t", "4", input}
			if _, err := runFFmpegOutput(ctx, command, "generate mastering canary"); err != nil {
				t.Fatal(err)
			}
			evidence, err := masterFullDemoProgram(ctx, ffmpeg, input, output, filepath.Join(dir, "logs"), recapplan.DefaultOptions().Audio.Loudness, tc.silentApproved)
			if (err != nil) != tc.wantError {
				t.Fatalf("master: %v; evidence: %+v", err, evidence)
			}
			if evidence.Status != tc.wantStatus {
				t.Fatalf("status: %s", evidence.Status)
			}
			if !tc.wantError {
				if _, err := runFFmpegOutput(ctx, []string{ffmpeg, "-v", "error", "-xerror", "-i", output, "-map", "0:v:0", "-map", "0:a:0", "-f", "null", "-"}, "decode delivered AAC/video"); err != nil {
					t.Fatal(err)
				}
			}
			if root := os.Getenv("FULL_DEMO_EVIDENCE_DIR"); root != "" {
				path := filepath.Join(root, strings.ReplaceAll(tc.name, " ", "-")+"-loudness.json")
				if err := os.MkdirAll(root, 0700); err != nil {
					t.Fatal(err)
				}
				b, err := json.MarshalIndent(evidence, "", "  ")
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, b, 0600); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}
