package editor

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/rechedev9/cliphub/internal/recapplan"
)

// The recovery chain run alongside the native masters must produce exactly the
// attempts of the serial chain, and a discarded speculation must leave no
// candidate or log behind, as if recovery had never started.
func TestFullDemoAACRecoverySpeculationMatchesSerialAndDiscardsCleanly(t *testing.T) {
	ffmpeg := fullDemoTestFFmpeg(t)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	if !hasMediaFoundationAAC(ctx, ffmpeg) {
		t.Skip("Windows Media Foundation AAC is required for this recovery canary")
	}
	dir := t.TempDir()
	input := filepath.Join(dir, "program-audio.nut")
	const duration = 1819.0 / recapplan.OutputFPS
	command := []string{ffmpeg, "-y", "-v", "error", "-f", "lavfi", "-i", "aevalsrc=(0.03+0.22*gte(mod(t\\,30)\\,15))*sin(2*PI*440*t)+0.05*sin(2*PI*2311*t)*lt(mod(t\\,1)\\,0.03):s=48000:d=" + decimal(duration), "-c:a", "pcm_f32le", "-ac", "2", input}
	if _, err := runFFmpegOutput(ctx, command, "generate AAC speculation canary"); err != nil {
		t.Fatal(err)
	}
	target := recapplan.DefaultOptions().Audio.Loudness
	first, err := measureLoudness(ctx, ffmpeg, input, target, "", duration, nil)
	if err != nil {
		t.Fatal(err)
	}
	attempts := func(result fullDemoAACRecoveryResult) []fullDemoAACRecoveryAttempt {
		t.Helper()
		if result.err != nil || result.unavailable || result.candidate == "" {
			t.Fatalf("recovery did not accept a candidate: %+v", result)
		}
		if _, err := os.Stat(result.candidate); err != nil {
			t.Fatalf("accepted candidate is missing: %v", err)
		}
		return result.attempts
	}
	serialDir := filepath.Join(dir, "serial")
	serial := runFullDemoAACRecovery(ctx, ffmpeg, input, filepath.Join(serialDir, "final.mp4"), filepath.Join(serialDir, "logs"), target, duration, first, nil)
	defer serial.release()
	speculativeDir := filepath.Join(dir, "speculative")
	speculative := startFullDemoAACRecovery(ctx, ffmpeg, input, filepath.Join(speculativeDir, "final.mp4"), filepath.Join(speculativeDir, "logs"), target, duration, first).wait()
	defer speculative.release()
	if want, got := attempts(serial), attempts(speculative); !reflect.DeepEqual(got, want) {
		t.Fatalf("speculative attempts = %+v, want %+v", got, want)
	}

	discardedDir := filepath.Join(dir, "discarded")
	discarded := startFullDemoAACRecovery(ctx, ffmpeg, input, filepath.Join(discardedDir, "final.mp4"), filepath.Join(discardedDir, "logs"), target, duration, first)
	discarded.discard()
	for _, pattern := range []string{"*", filepath.Join("logs", "*")} {
		left, err := filepath.Glob(filepath.Join(discardedDir, pattern))
		if err != nil {
			t.Fatal(err)
		}
		for _, path := range left {
			if info, err := os.Stat(path); err == nil && !info.IsDir() {
				t.Fatalf("discarded speculation left %s behind", path)
			}
		}
	}
}
