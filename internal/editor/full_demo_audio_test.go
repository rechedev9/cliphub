package editor

import (
	"context"
	"encoding/json"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
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
			evidence, err := masterFullDemoProgram(ctx, ffmpeg, input, output, filepath.Join(dir, "logs"), recapplan.DefaultOptions().Audio.Loudness, tc.silentApproved, 4, nil)
			if (err != nil) != tc.wantError {
				t.Fatalf("master: %v; evidence: %+v", err, evidence)
			}
			if evidence.Status != tc.wantStatus {
				t.Fatalf("status: %s", evidence.Status)
			}
			if len(evidence.FallbackMasters) != 0 {
				t.Fatalf("previously passing canary used recovery: %+v", evidence.FallbackMasters)
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

func TestFullDemoAACRecoveryKeepsDecodedAcceptance(t *testing.T) {
	ffmpeg := fullDemoTestFFmpeg(t)
	ffprobe := fullDemoTestFFprobe(t)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	if !hasMediaFoundationAAC(ctx, ffmpeg) {
		t.Skip("Windows Media Foundation AAC is required for this recovery canary")
	}
	dir := t.TempDir()
	// Not a loudnorm analysis window nor an AAC packet boundary; delivery
	// verification needs the real 1080p program geometry.
	const frames = fullDemoAudioTestFrames
	input, duration := fullDemoAudioTestProgram(t, ctx, ffmpeg, dir, frames, fullDemoAudioTestTransient, "1920x1080")
	output := filepath.Join(dir, "final.mp4")
	target := recapplan.DefaultOptions().Audio.Loudness
	first, err := measureLoudness(ctx, ffmpeg, input, target, "", duration, nil)
	if err != nil {
		t.Fatal(err)
	}
	var last float64
	evidence, err := recoverFullDemoAAC(ctx, ffmpeg, input, committedFullDemoProgramVideo(input), output, filepath.Join(dir, "logs"), target, duration, ProgramLoudnessEvidence{Policy: target.PolicyVersion, Input: first, MasterTargets: []recapplan.LoudnessOptions{}, DecodedAAC: []LoudnessMeasurement{}, Status: "unverified"}, func(_ string, fraction float64) {
		if fraction < last || fraction >= 1 {
			t.Errorf("invalid recovery progress: %f after %f", fraction, last)
		}
		last = fraction
	})
	if err != nil {
		t.Fatalf("recovery: %v; evidence: %+v", err, evidence)
	}
	assertRecoveredAAC(t, evidence, target)
	if evidence.FinalMuxedAAC == nil {
		t.Fatal("recovery did not certify the final muxed AAC")
	}
	if _, err := verifyFullDemoDelivery(ctx, ffmpeg, ffprobe, output, frames, nil); err != nil {
		t.Fatal("recovered AAC broke delivery:", err)
	}
	if got := fullDemoTestVideoHash(t, ctx, ffmpeg, output); got != fullDemoTestVideoHash(t, ctx, ffmpeg, input) {
		t.Fatalf("recovery changed the copied video: %s", got)
	}
	if got := fullDemoTestVideoFrames(t, ctx, ffprobe, output); got != frames {
		t.Fatalf("recovery output has %d frames, want %d", got, frames)
	}
	packets := fullDemoTestAudioPackets(t, ctx, ffprobe, output)
	if _, end := fullDemoTestPacketClock(t, packets, "recovered"); end != int64(frames)*recapplan.SampleRate/recapplan.OutputFPS {
		t.Fatalf("AAC packet padding escaped the approved timeline: reaches sample %d", end)
	}
	for i, packet := range packets {
		if packet.Duration > 1024 {
			t.Fatalf("recovered AAC packet %d is longer than one access unit: %+v", i, packet)
		}
	}
	if final := packets[len(packets)-1]; final.Duration >= 1024 {
		t.Fatalf("recovered audio did not exercise a short final packet: %+v", final)
	}
	fullDemoTestNoTemporaryAudioFiles(t, dir)
}

func TestFullDemoAACRecoveryCorrectionIsBounded(t *testing.T) {
	target := recapplan.DefaultOptions().Audio.Loudness
	m := ProgramAACFallbackMaster{Encoder: "aac_mf", GainDB: 3, CeilingDBFS: -5}
	low, high := -30.0, 10.0
	decoded := LoudnessMeasurement{IntegratedLUFS: &low, TruePeakDBTP: &high}
	for range 20 {
		m = m.corrected(decoded, target)
	}
	if m.GainDB != 6 || m.CeilingDBFS != target.TargetTPDBTP-8 {
		t.Fatalf("unbounded correction: %+v", m)
	}
	decoded.IntegratedLUFS = &high
	for range 20 {
		m = m.corrected(decoded, target)
	}
	if m.GainDB != -3 {
		t.Fatalf("unbounded attenuation: %+v", m)
	}
}

// Regression for a 14-minute Windows program whose native AAC masters kept
// overshooting the true-peak target until the retargeting loop asked loudnorm
// for TP=-10.18, outside its [-9, 0] range, failing the render at 81 %.
func TestFullDemoMasterRetargetStaysWithinLoudnormRange(t *testing.T) {
	target := recapplan.DefaultOptions().Audio.Loudness
	current := aacHeadroomTarget(target)
	lufs, peak := target.TargetILUFS, 2.99
	decoded := LoudnessMeasurement{Status: "measured", IntegratedLUFS: &lufs, TruePeakDBTP: &peak}
	var changed bool
	for i := range 5 {
		current, changed = nextMasterTarget(current, target, decoded)
		if current.TargetTPDBTP < loudnormMinTPDBTP || current.TargetTPDBTP > loudnormMaxTPDBTP {
			t.Fatalf("attempt %d produced an out-of-range loudnorm target: %+v", i, current)
		}
		if !changed {
			if i == 0 {
				t.Fatal("first retarget must still have headroom")
			}
			break
		}
	}
	if changed {
		t.Fatalf("retargeting never reported exhausted headroom: %+v", current)
	}
	if current.TargetTPDBTP != loudnormMinTPDBTP {
		t.Fatalf("expected the true-peak floor once exhausted: %+v", current)
	}
	if current.PolicyVersion != target.PolicyVersion || current.TargetLRA != target.TargetLRA {
		t.Fatalf("retargeting altered unrelated policy fields: %+v", current)
	}

	quiet, inRange := -80.0, target.TargetTPDBTP-1
	current = target
	for range 80 {
		current, _ = nextMasterTarget(current, target, LoudnessMeasurement{Status: "measured", IntegratedLUFS: &quiet, TruePeakDBTP: &inRange})
	}
	if current.TargetILUFS != loudnormMaxILUFS || current.TargetTPDBTP != target.TargetTPDBTP {
		t.Fatalf("integrated loudness escaped the loudnorm range: %+v", current)
	}
}

func TestFullDemoAACRecoveryUnavailableDoesNotAcceptFailedAudio(t *testing.T) {
	dir := t.TempDir()
	low, peak := -19.81, -.26
	failed := ProgramLoudnessEvidence{Status: "unverified", DecodedAAC: []LoudnessMeasurement{{Status: "measured", IntegratedLUFS: &low, TruePeakDBTP: &peak}}}
	evidence, err := recoverFullDemoAAC(context.Background(), filepath.Join(dir, "missing-ffmpeg.exe"), "input.nut", committedFullDemoProgramVideo("input.nut"), "output.mp4", dir, recapplan.DefaultOptions().Audio.Loudness, 4, failed, nil)
	if err == nil || !strings.Contains(err.Error(), "audio_loudness_failed:") || !strings.Contains(err.Error(), "-19.81 LUFS / -0.26 dBTP") || evidence.Status != "unverified" || len(evidence.FallbackMasters) != 0 {
		t.Fatalf("unavailable encoder bypassed failure or lost measurements: %+v, %v", evidence, err)
	}
}

func assertRecoveredAAC(t *testing.T, evidence ProgramLoudnessEvidence, target recapplan.LoudnessOptions) {
	t.Helper()
	if evidence.Status != "verified-decoded-aac" || len(evidence.FallbackMasters) < 1 || len(evidence.FallbackMasters) > 3 || len(evidence.DecodedAAC) == 0 {
		t.Fatalf("missing bounded recovery evidence: %+v", evidence)
	}
	last := evidence.DecodedAAC[len(evidence.DecodedAAC)-1]
	if last.IntegratedLUFS == nil || last.TruePeakDBTP == nil || math.Abs(*last.IntegratedLUFS-target.TargetILUFS) > .5 || *last.TruePeakDBTP > target.TargetTPDBTP {
		t.Fatalf("recovery bypassed decoded acceptance: %+v", last)
	}
}

// An opt-in lossless A/V reproduction keeps private game/comms audio out of
// the repository while exercising the complete native-failure/recovery path.
func TestFullDemoAACRecoverySavedProgram(t *testing.T) {
	input := os.Getenv("FULL_DEMO_MASTER_REGRESSION_INPUT")
	if input == "" {
		t.Skip("set FULL_DEMO_MASTER_REGRESSION_INPUT to a lossless failing A/V program")
	}
	ffmpeg := fullDemoTestFFmpeg(t)
	ffprobe, err := exec.LookPath("ffprobe")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	probe, err := runFFmpegOutput(ctx, []string{ffprobe, "-v", "error", "-count_frames", "-select_streams", "v:0", "-show_entries", "stream=nb_read_frames", "-of", "default=nw=1:nk=1", input}, "regression frame clock")
	if err != nil {
		t.Fatal(err)
	}
	frames, err := strconv.ParseInt(strings.TrimSpace(probe), 10, 64)
	if err != nil {
		t.Fatal(err)
	}
	duration := float64(frames) / recapplan.OutputFPS
	dir := t.TempDir()
	output := filepath.Join(dir, "final.mp4")
	target := recapplan.DefaultOptions().Audio.Loudness
	evidence, err := masterFullDemoProgram(ctx, ffmpeg, input, output, filepath.Join(dir, "logs"), target, false, duration, nil)
	if err != nil {
		t.Fatalf("master: %v; evidence: %+v", err, evidence)
	}
	assertRecoveredAAC(t, evidence, target)
	if len(evidence.DecodedAAC) <= 3 {
		t.Fatal("regression did not reproduce the three native master failures")
	}
	var videoHash string
	for i, path := range []string{input, output} {
		digest, err := runFFmpegOutput(ctx, []string{ffmpeg, "-v", "error", "-i", path, "-map", "0:v:0", "-f", "hash", "-hash", "sha256", "-"}, "regression decoded video hash")
		if err != nil {
			t.Fatal(err)
		}
		if i == 0 {
			videoHash = digest
		} else if digest != videoHash {
			t.Fatal("audio recovery changed the video")
		}
	}
	probe, err = runFFmpegOutput(ctx, []string{ffprobe, "-v", "error", "-select_streams", "a:0", "-show_entries", "stream=duration", "-of", "default=nw=1:nk=1", output}, "recovered audio duration")
	if err != nil {
		t.Fatal(err)
	}
	gotDuration, err := strconv.ParseFloat(strings.TrimSpace(probe), 64)
	if err != nil || math.Abs(gotDuration-duration) > 1.0/60 {
		t.Fatalf("audio duration changed: %s versus %f", probe, duration)
	}
	if root := os.Getenv("FULL_DEMO_EVIDENCE_DIR"); root != "" {
		if err := os.MkdirAll(root, 0700); err != nil {
			t.Fatal(err)
		}
		body, err := json.MarshalIndent(evidence, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, "saved-program-recovery.json"), body, 0600); err != nil {
			t.Fatal(err)
		}
	}
}
