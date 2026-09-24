package editor

import (
	"context"
	"errors"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/rechedev9/cliphub/internal/obs"
	"github.com/rechedev9/cliphub/internal/recapplan"
)

// fullDemoFailureFFmpegSource is a stand-in FFmpeg for the master loop. It
// answers loudnorm measurements with the values in FAKE_LOUDNORM_I/TP, writes
// candidates and can reproduce the failures the loop must classify.
const fullDemoFailureFFmpegSource = `package main

import (
	"fmt"
	"os"
	"strings"
)

func main() {
	args := os.Args[1:]
	for _, arg := range args {
		if arg == "-encoders" {
			fmt.Println(" A....D aac                  AAC (Advanced Audio Coding)")
			if os.Getenv("FAKE_AAC_MF") == "1" {
				fmt.Println(" A....D aac_mf               AAC via MediaFoundation (codec aac)")
			}
			return
		}
	}
	command := strings.Join(args, " ")
	out := args[len(args)-1]
	if out == "-" {
		fmt.Fprintf(os.Stderr, "[Parsed_loudnorm_0 @ 0000000000000001]\n{\n\t\"input_i\" : \"%s\",\n\t\"input_tp\" : \"%s\",\n\t\"input_lra\" : \"1.00\",\n\t\"input_thresh\" : \"-24.00\",\n\t\"output_i\" : \"-14.00\",\n\t\"target_offset\" : \"0.00\"\n}\n", os.Getenv("FAKE_LOUDNORM_I"), os.Getenv("FAKE_LOUDNORM_TP"))
		return
	}
	if os.Getenv("FAKE_REJECT_LOUDNORM") == "1" && strings.Contains(command, "loudnorm=") {
		fmt.Fprintln(os.Stderr, "[Parsed_loudnorm_0 @ 0000000000000001] Value -10.180000 for parameter 'TP' out of range [-9 - 0]")
		fmt.Fprintln(os.Stderr, "Error applying option 'TP' to filter 'loudnorm': Result too large")
		fmt.Fprintln(os.Stderr, "Error opening output files: Result too large")
		os.Exit(1)
	}
	if os.Getenv("FAKE_FAIL_AAC_MF") == "1" && strings.Contains(command, "aac_mf") {
		fmt.Fprintln(os.Stderr, "[aac_mf @ 0000000000000001] could not set the desired input type")
		fmt.Fprintln(os.Stderr, "Error while opening encoder for output stream #0:0")
		os.Exit(1)
	}
	if err := os.WriteFile(out, []byte("fake"), 0o600); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
`

func fullDemoFailureFFmpeg(t *testing.T) string {
	t.Helper()
	goExe, err := exec.LookPath("go")
	if err != nil {
		t.Skip("the go toolchain is required to build the fake FFmpeg:", err)
	}
	dir := t.TempDir()
	src := filepath.Join(dir, "fake-ffmpeg.go")
	if err := os.WriteFile(src, []byte(fullDemoFailureFFmpegSource), 0o600); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "ffmpeg")
	if runtime.GOOS == "windows" {
		path += ".exe"
	}
	if out, err := exec.Command(goExe, "build", "-o", path, src).CombinedOutput(); err != nil {
		t.Fatalf("build fake FFmpeg: %v: %s", err, out)
	}
	return path
}

// runFailingFullDemoMaster drives the real master loop against the fake
// FFmpeg with the decoded values of the Studio 3.0.0 incident: every native
// AAC candidate lands on the integrated target but overshoots the true peak
// by 2.99 dBTP, so the loop retargets until loudnorm's floor and hands over.
func runFailingFullDemoMaster(t *testing.T, ctx context.Context, env map[string]string) (ProgramLoudnessEvidence, error) {
	t.Helper()
	ffmpeg := fullDemoFailureFFmpeg(t)
	target := recapplan.DefaultOptions().Audio.Loudness
	t.Setenv("FAKE_LOUDNORM_I", decimal(target.TargetILUFS))
	t.Setenv("FAKE_LOUDNORM_TP", "2.99")
	for key, value := range env {
		t.Setenv(key, value)
	}
	prior := log.Writer()
	log.SetOutput(io.Discard)
	t.Cleanup(func() { log.SetOutput(prior) })
	dir := t.TempDir()
	return masterFullDemoProgram(ctx, ffmpeg, filepath.Join(dir, "program.nut"), filepath.Join(dir, "final.mp4"), filepath.Join(dir, "logs"), target, false, 4, nil)
}

func TestFullDemoMasterExhaustionIsClassifiedWhenHandOverFails(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	// No aac_mf encoder: the hand-over to Media Foundation recovery fails too.
	evidence, err := runFailingFullDemoMaster(t, ctx, nil)
	if err == nil {
		t.Fatal("a master that never met its targets was accepted")
	}
	if !strings.HasPrefix(err.Error(), "failure_code=audio_master_exhausted substage=audio_master; audio_loudness_failed:") {
		t.Fatalf("exhausted master error = %q", err.Error())
	}
	if failure, ok := obs.FailureOf(err); !ok || failure.Code != obs.FailureAudioMasterExhausted || failure.Substage != obs.SubstageAudioMaster {
		t.Fatalf("FailureOf = %+v, %v", failure, ok)
	}
	if len(evidence.MasterTargets) == 0 || len(evidence.MasterTargets) > 3 || len(evidence.DecodedAAC) != len(evidence.MasterTargets) {
		t.Fatalf("native attempts = %d targets / %d decoded", len(evidence.MasterTargets), len(evidence.DecodedAAC))
	}
	for i, attempt := range evidence.MasterTargets {
		if attempt.TargetTPDBTP < loudnormMinTPDBTP || attempt.TargetTPDBTP > loudnormMaxTPDBTP {
			t.Fatalf("attempt %d asked loudnorm for an out-of-range target: %+v", i, attempt)
		}
	}
}

func TestFullDemoMasterLoudnormRejectionIsClassified(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	_, err := runFailingFullDemoMaster(t, ctx, map[string]string{"FAKE_REJECT_LOUDNORM": "1"})
	if err == nil || !strings.HasPrefix(err.Error(), "failure_code=loudnorm_param_out_of_range substage=audio_master; ffmpeg Full Demo program master: exit status 1: Error opening output files: Result too large") {
		t.Fatalf("rejected loudnorm option = %v", err)
	}
	if !strings.Contains(err.Error(), "Value -10.180000 for parameter 'TP' out of range [-9 - 0]") {
		t.Fatalf("the complete FFmpeg stderr is no longer in the error: %q", err.Error())
	}
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatal("the FFmpeg exit error is no longer in the chain")
	}
}

func TestFullDemoAACRecoveryFailureIsClassified(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Media Foundation AAC recovery only runs on Windows")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	_, err := runFailingFullDemoMaster(t, ctx, map[string]string{"FAKE_AAC_MF": "1", "FAKE_FAIL_AAC_MF": "1"})
	if err == nil || !strings.HasPrefix(err.Error(), "failure_code=aac_recovery_failed substage=aac_recovery; audio_loudness_failed: AAC recovery: ffmpeg Full Demo AAC recovery: exit status 1: Error while opening encoder") {
		t.Fatalf("failed recovery = %v", err)
	}
}

func TestFullDemoMasterCancellationIsClassifiedAsInterrupted(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := runFailingFullDemoMaster(t, ctx, nil)
	if failure, ok := obs.FailureOf(err); !ok || failure.Code != obs.FailureRenderInterrupted || failure.Substage != obs.SubstageAudioMaster {
		t.Fatalf("cancelled master = %v", err)
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation lost from the chain: %v", err)
	}
}

func TestFFmpegFailureNamesTheFinalCauseFirst(t *testing.T) {
	stderr := "Input #0, nut, from 'program.nut':\n[Parsed_loudnorm_0 @ 01] Value -10.180000 for parameter 'TP' out of range [-9 - 0]\nError opening output files: Result too large\n"
	err := ffmpegFailure("Full Demo program master", errors.New("exit status 1"), stderr)
	first, rest, _ := strings.Cut(err.Error(), "\n")
	if first != "ffmpeg Full Demo program master: exit status 1: Error opening output files: Result too large" {
		t.Fatalf("first line = %q", first)
	}
	if rest != strings.TrimSpace(stderr) {
		t.Fatalf("complete stderr = %q", rest)
	}
	if got := ffmpegFailure("probe", errors.New("exit status 1"), "only line\n").Error(); got != "ffmpeg probe: exit status 1: only line" {
		t.Fatalf("single-line stderr = %q", got)
	}
	if got := ffmpegFailure("probe", errors.New("exit status 1"), "").Error(); got != "ffmpeg probe: exit status 1" {
		t.Fatalf("empty stderr = %q", got)
	}
}

func TestLoudnormRangeRejectionMatchesIncidentStderr(t *testing.T) {
	for _, text := range []string{
		"[Parsed_loudnorm_0] Value -10.180000 for parameter 'TP' out of range [-9 - 0]",
		"Error applying option 'TP' to filter 'loudnorm': Result too large",
	} {
		if !loudnormRangeRejection.MatchString(text) {
			t.Fatalf("incident stderr not recognised: %q", text)
		}
	}
	if loudnormRangeRejection.MatchString("[aresample @ 01] Value 7 for parameter 'osr' out of range") {
		t.Fatal("a non-loudnorm range error was attributed to loudnorm")
	}
}
