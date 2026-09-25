package editor

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/rechedev9/cliphub/internal/filecommit"
	"github.com/rechedev9/cliphub/internal/recapplan"
)

// The recovery chain run alongside the native masters must produce exactly the
// attempts of the serial chain, its speculative delivery must certify and
// publish exactly what the serial delivery certifies and publishes, and a
// discarded speculation must leave no candidate, delivery attempt or log
// behind, as if recovery had never started.
func TestFullDemoAACRecoverySpeculationMatchesSerialAndDiscardsCleanly(t *testing.T) {
	ffmpeg := fullDemoTestFFmpeg(t)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	if !hasMediaFoundationAAC(ctx, ffmpeg) {
		t.Skip("Windows Media Foundation AAC is required for this recovery canary")
	}
	dir := t.TempDir()
	input := filepath.Join(dir, "program.nut")
	const frames = 1819 // Does not end on a loudnorm analysis/AAC packet boundary.
	const duration = float64(frames) / recapplan.OutputFPS
	// The program carries a video stream so the speculative delivery runs the
	// real final mux and the real final certification, not a reduced one.
	command := []string{ffmpeg, "-y", "-v", "error", "-f", "lavfi", "-i", "color=c=navy:s=320x180:r=60:d=" + decimal(duration), "-f", "lavfi", "-i", "aevalsrc=(0.03+0.22*gte(mod(t\\,30)\\,15))*sin(2*PI*440*t)+0.05*sin(2*PI*2311*t)*lt(mod(t\\,1)\\,0.03):s=48000:d=" + decimal(duration), "-map", "0:v", "-map", "1:a", "-c:v", "libx264", "-preset", "ultrafast", "-bf", "0", "-c:a", "pcm_f32le", "-ac", "2", "-t", decimal(duration), input}
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
	template := ProgramLoudnessEvidence{Policy: target.PolicyVersion, Input: first, DecodedAAC: []LoudnessMeasurement{}, MasterTargets: []recapplan.LoudnessOptions{}, Status: "unverified"}

	serialDir := filepath.Join(dir, "serial")
	serialOutput, serialLogs := filepath.Join(serialDir, "final.mp4"), filepath.Join(serialDir, "logs")
	serial := runFullDemoAACRecovery(ctx, ffmpeg, input, serialOutput, serialLogs, target, duration, first, nil)
	defer serial.release()
	speculativeDir := filepath.Join(dir, "speculative")
	speculativeOutput, speculativeLogs := filepath.Join(speculativeDir, "final.mp4"), filepath.Join(speculativeDir, "logs")
	// The speculation, its delivery included, must keep the NORMAL priority
	// class: when every native master is rejected it is the delivered master and
	// the critical path, so the context it resolves the program video with must
	// not carry the background scheduling mark.
	backgroundVideo := func(ctx context.Context) (string, error) {
		if backgroundProcessPriority(ctx) {
			t.Error("the speculative recovery is marked as background work; it is the critical path when recovery wins")
		}
		return input, nil
	}
	speculative := startFullDemoAACRecovery(ctx, ffmpeg, input, backgroundVideo, speculativeOutput, speculativeLogs, target, duration, first).wait()
	defer speculative.release()
	if want, got := attempts(serial), attempts(speculative); !reflect.DeepEqual(got, want) {
		t.Fatalf("speculative attempts = %+v, want %+v", got, want)
	}
	if speculative.deliveryErr != nil || speculative.delivery == nil {
		t.Fatalf("speculative delivery = %+v, error %v", speculative.delivery, speculative.deliveryErr)
	}
	if _, err := os.Stat(speculative.delivery.attempt); err != nil {
		t.Fatalf("speculative delivery attempt is missing: %v", err)
	}
	if _, err := os.Stat(speculativeOutput); !os.IsNotExist(err) {
		t.Fatalf("speculative delivery published before the native masters were exhausted: %v", err)
	}

	// Both paths must fold the same evidence and certify the same muxed AAC.
	serialEvidence, err := finishFullDemoAACRecovery(ctx, ffmpeg, committedFullDemoProgramVideo(input), serialOutput, serialLogs, target, duration, template, serial, nil)
	if err != nil {
		t.Fatalf("serial delivery: %v; evidence %+v", err, serialEvidence)
	}
	speculativeEvidence, err := finishFullDemoAACRecovery(ctx, ffmpeg, func(context.Context) (string, error) {
		t.Error("the committed program video was requested although the delivery was already muxed")
		return "", nil
	}, speculativeOutput, speculativeLogs, target, duration, template, speculative, nil)
	if err != nil {
		t.Fatalf("speculative delivery: %v; evidence %+v", err, speculativeEvidence)
	}
	if !reflect.DeepEqual(speculativeEvidence, serialEvidence) {
		t.Fatalf("speculative evidence = %+v, want %+v", speculativeEvidence, serialEvidence)
	}
	names := func(dir string) []string {
		t.Helper()
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatal(err)
		}
		var found []string
		for _, entry := range entries {
			found = append(found, entry.Name())
		}
		return found
	}
	// The evidence logs themselves quote their own attempt path, which is
	// unique per run; their loudnorm content is what the compared evidence
	// above was parsed from.
	if want, got := names(serialLogs), names(speculativeLogs); !reflect.DeepEqual(got, want) {
		t.Fatalf("speculative log file set = %v, want %v", got, want)
	}

	discardedDir := filepath.Join(dir, "discarded")
	discarded := startFullDemoAACRecovery(ctx, ffmpeg, input, committedFullDemoProgramVideo(input), filepath.Join(discardedDir, "final.mp4"), filepath.Join(discardedDir, "logs"), target, duration, first)
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

func loudnessLevel(value float64) *float64 { return &value }

// speculatedRecovery builds a completed recovery whose accepted candidate was
// already muxed and certified, without running FFmpeg. delivered is what the
// final certification measured from the attempt.
func speculatedRecovery(t *testing.T, output string, delivered LoudnessMeasurement) (fullDemoAACRecoveryResult, ProgramLoudnessEvidence, []byte) {
	t.Helper()
	target := recapplan.DefaultOptions().Audio.Loudness
	candidate, candidateCleanup, err := fullDemoAudioCandidatePath(output)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(candidate, []byte("accepted recovery candidate"), 0o600); err != nil {
		t.Fatal(err)
	}
	attempt, attemptCleanup, err := filecommit.Attempt(output)
	if err != nil {
		t.Fatal(err)
	}
	body := []byte("muxed and certified delivery")
	if err := os.WriteFile(attempt, body, 0o600); err != nil {
		t.Fatal(err)
	}
	rejected := LoudnessMeasurement{Status: "measured", IntegratedLUFS: loudnessLevel(-18.25), TruePeakDBTP: loudnessLevel(-3.75), LRA: loudnessLevel(9.5), Threshold: loudnessLevel(-30.5), Offset: loudnessLevel(1.25)}
	result := fullDemoAACRecoveryResult{
		attempts: []fullDemoAACRecoveryAttempt{
			{master: ProgramAACFallbackMaster{Encoder: "aac_mf", GainDB: 3, CeilingDBFS: target.TargetTPDBTP - 3.5}, decoded: &rejected},
			{master: ProgramAACFallbackMaster{Encoder: "aac_mf", GainDB: 2.25, CeilingDBFS: target.TargetTPDBTP - 4.5}, decoded: &delivered},
		},
		candidate: candidate,
		cleanup:   candidateCleanup,
		delivery:  &fullDemoAACDelivery{attempt: attempt, cleanup: attemptCleanup, measurement: delivered},
	}
	template := ProgramLoudnessEvidence{Policy: target.PolicyVersion, DecodedAAC: []LoudnessMeasurement{}, MasterTargets: []recapplan.LoudnessOptions{}, Status: "unverified"}
	return result, template, body
}

func refusedProgramVideo(t *testing.T) fullDemoProgramVideo {
	t.Helper()
	return func(context.Context) (string, error) {
		t.Error("the committed program video was requested although the delivery was already muxed")
		return "", nil
	}
}

// A speculative delivery is folded into the evidence in exactly the order the
// serial chain records it, and only then published.
func TestFinishFullDemoAACRecoveryPublishesSpeculativeDeliveryInOrder(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	dir := t.TempDir()
	output := filepath.Join(dir, "final.mp4")
	target := recapplan.DefaultOptions().Audio.Loudness
	delivered := LoudnessMeasurement{Status: "measured", IntegratedLUFS: loudnessLevel(target.TargetILUFS), TruePeakDBTP: loudnessLevel(target.TargetTPDBTP - 1.85), LRA: loudnessLevel(8.75), Threshold: loudnessLevel(-29.25), Offset: loudnessLevel(0.5)}
	result, template, body := speculatedRecovery(t, output, delivered)
	attempt := result.delivery.attempt

	evidence, err := finishFullDemoAACRecovery(ctx, "", refusedProgramVideo(t), output, filepath.Join(dir, "logs"), target, 30, template, result, nil)
	if err != nil {
		t.Fatalf("finish: %v; evidence %+v", err, evidence)
	}
	recovered := aacHeadroomTarget(target)
	if want := []recapplan.LoudnessOptions{recovered, recovered}; !reflect.DeepEqual(evidence.MasterTargets, want) {
		t.Fatalf("master targets = %+v, want %+v", evidence.MasterTargets, want)
	}
	if want := []ProgramAACFallbackMaster{result.attempts[0].master, result.attempts[1].master}; !reflect.DeepEqual(evidence.FallbackMasters, want) {
		t.Fatalf("fallback masters = %+v, want %+v", evidence.FallbackMasters, want)
	}
	if want := []LoudnessMeasurement{*result.attempts[0].decoded, delivered}; !reflect.DeepEqual(evidence.DecodedAAC, want) {
		t.Fatalf("decoded AAC = %+v, want %+v", evidence.DecodedAAC, want)
	}
	if evidence.FinalMuxedAAC == nil || !reflect.DeepEqual(*evidence.FinalMuxedAAC, delivered) {
		t.Fatalf("final muxed AAC = %+v, want %+v", evidence.FinalMuxedAAC, delivered)
	}
	if evidence.Status != "verified-decoded-aac" {
		t.Fatalf("status = %q", evidence.Status)
	}
	published, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	if string(published) != string(body) {
		t.Fatalf("published output = %q, want the speculative delivery", published)
	}
	if _, err := os.Stat(attempt); !os.IsNotExist(err) {
		t.Fatalf("delivery attempt remains after publication: %v", err)
	}
	if _, err := os.Stat(result.candidate); !os.IsNotExist(err) {
		t.Fatalf("recovery candidate remains after publication: %v", err)
	}
}

// Nothing a speculative delivery produced may reach output when the render was
// cancelled or when the certified measurement misses its approved targets.
func TestFinishFullDemoAACRecoveryNeverPublishesUnapprovedSpeculativeDelivery(t *testing.T) {
	target := recapplan.DefaultOptions().Audio.Loudness
	approved := LoudnessMeasurement{Status: "measured", IntegratedLUFS: loudnessLevel(target.TargetILUFS), TruePeakDBTP: loudnessLevel(target.TargetTPDBTP - 1.85), LRA: loudnessLevel(8.75), Threshold: loudnessLevel(-29.25), Offset: loudnessLevel(0.5)}
	missed := approved
	missed.IntegratedLUFS = loudnessLevel(target.TargetILUFS - 4.5)
	for _, tc := range []struct {
		name      string
		delivered LoudnessMeasurement
		cancelled bool
	}{
		{"cancelled before the commit", approved, true},
		{"certified measurement misses its targets", missed, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			dir := t.TempDir()
			output := filepath.Join(dir, "final.mp4")
			if err := os.WriteFile(output, []byte("previous good output"), 0o600); err != nil {
				t.Fatal(err)
			}
			result, template, _ := speculatedRecovery(t, output, tc.delivered)
			attempt := result.delivery.attempt
			if tc.cancelled {
				cancel()
			}
			evidence, err := finishFullDemoAACRecovery(ctx, "", refusedProgramVideo(t), output, filepath.Join(dir, "logs"), target, 30, template, result, nil)
			if err == nil {
				t.Fatalf("finish error = nil; evidence %+v", evidence)
			}
			if evidence.FinalMuxedAAC != nil || evidence.Status != "unverified" {
				t.Fatalf("rejected delivery reached the evidence: %+v", evidence)
			}
			if len(evidence.DecodedAAC) != 2 || len(evidence.FallbackMasters) != 2 {
				t.Fatalf("attempts were not folded before the failure: %+v", evidence)
			}
			kept, err := os.ReadFile(output)
			if err != nil {
				t.Fatal(err)
			}
			if string(kept) != "previous good output" {
				t.Fatalf("output = %q, want the previous good output", kept)
			}
			if _, err := os.Stat(attempt); !os.IsNotExist(err) {
				t.Fatalf("delivery attempt remains: %v", err)
			}
		})
	}
}

// A speculative delivery that failed (mux, certification or the committed
// program video itself) fails the recovery exactly as the serial delivery
// would have: the attempts are still folded, nothing is retried against the
// program video, and the previous output is untouched.
func TestFinishFullDemoAACRecoveryReportsSpeculativeDeliveryFailure(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	dir := t.TempDir()
	output := filepath.Join(dir, "final.mp4")
	if err := os.WriteFile(output, []byte("previous good output"), 0o600); err != nil {
		t.Fatal(err)
	}
	target := recapplan.DefaultOptions().Audio.Loudness
	delivered := LoudnessMeasurement{Status: "measured", IntegratedLUFS: loudnessLevel(target.TargetILUFS), TruePeakDBTP: loudnessLevel(target.TargetTPDBTP - 1.85), LRA: loudnessLevel(8.75), Threshold: loudnessLevel(-29.25), Offset: loudnessLevel(0.5)}
	result, template, _ := speculatedRecovery(t, output, delivered)
	// The failed speculation released its own attempt before reporting.
	result.delivery.release()
	result.delivery = nil
	failure := errors.New("ffmpeg Full Demo final audio mux: exit status 1")
	result.deliveryErr = failure
	evidence, err := finishFullDemoAACRecovery(ctx, "", refusedProgramVideo(t), output, filepath.Join(dir, "logs"), target, 30, template, result, nil)
	if !errors.Is(err, failure) {
		t.Fatalf("finish error = %v, want the speculative delivery failure", err)
	}
	if evidence.FinalMuxedAAC != nil || evidence.Status != "unverified" {
		t.Fatalf("failed delivery reached the evidence: %+v", evidence)
	}
	if len(evidence.DecodedAAC) != 2 || len(evidence.FallbackMasters) != 2 || len(evidence.MasterTargets) != 2 {
		t.Fatalf("attempts were not folded before the failure: %+v", evidence)
	}
	kept, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	if string(kept) != "previous good output" {
		t.Fatalf("output = %q, want the previous good output", kept)
	}
	if _, err := os.Stat(result.candidate); !os.IsNotExist(err) {
		t.Fatalf("recovery candidate remains after the failure: %v", err)
	}
}

// A native master that passes discards the speculation: its candidate, its
// unpublished delivery attempt and its logs must all disappear, and the output
// the native delivery is about to write must be untouched.
func TestFullDemoAACRecoverySpeculationDiscardsUnpublishedDelivery(t *testing.T) {
	dir := t.TempDir()
	output := filepath.Join(dir, "final.mp4")
	if err := os.WriteFile(output, []byte("previous good output"), 0o600); err != nil {
		t.Fatal(err)
	}
	target := recapplan.DefaultOptions().Audio.Loudness
	delivered := LoudnessMeasurement{Status: "measured", IntegratedLUFS: loudnessLevel(target.TargetILUFS), TruePeakDBTP: loudnessLevel(target.TargetTPDBTP - 1.85), LRA: loudnessLevel(8.75), Threshold: loudnessLevel(-29.25), Offset: loudnessLevel(0.5)}
	result, _, _ := speculatedRecovery(t, output, delivered)
	logs := filepath.Join(dir, "logs")
	if err := os.MkdirAll(logs, 0o700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"program-aac-recovery-0.txt", fullDemoFinalMuxLog, fullDemoFinalAnalysisLog} {
		path := filepath.Join(logs, name)
		if err := os.WriteFile(path, []byte(name), 0o600); err != nil {
			t.Fatal(err)
		}
		result.logs = append(result.logs, path)
	}
	candidate, attempt := result.candidate, result.delivery.attempt
	done := make(chan struct{})
	close(done)
	speculation := &fullDemoAACRecoverySpeculation{cancel: func() {}, done: done, result: result}
	speculation.discard()
	for _, path := range append([]string{candidate, attempt}, result.logs...) {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("discarded speculation left %s behind: %v", path, err)
		}
	}
	if speculation.result.delivery != nil {
		t.Fatal("discarded speculation still owns a delivery")
	}
	speculation.discard()
	kept, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	if string(kept) != "previous good output" {
		t.Fatalf("output = %q, want the previous good output", kept)
	}
}
