package workers

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/editor"
	"github.com/rechedev9/cliphub/internal/job"
	"github.com/rechedev9/cliphub/internal/obs"
	"github.com/rechedev9/cliphub/internal/recording"
	"github.com/rechedev9/cliphub/internal/rules"
	"github.com/rechedev9/cliphub/internal/tasks"
)

// A child that failed with a classified error, the way zv-editor reports a
// Full Demo audio failure: a timestamped log.Fatal line, followed by a late
// diagnostic trace line that must not be mistaken for the cause.
func TestCodedCommandHelper(t *testing.T) {
	if os.Getenv("CLIPHUB_CODED_COMMAND_HELPER") != "1" {
		return
	}
	_, _ = os.Stderr.WriteString("2026-09-24T10:00:00+02:00 " + obs.TracePrefix + `{"event":"tool.finished","message":"ffmpeg Full Demo program master"}` + "\n")
	_, _ = os.Stderr.WriteString("2026-09-24T10:00:01+02:00 failure_code=audio_master_exhausted substage=audio_master; audio_loudness_failed: Media Foundation AAC recovery is unavailable after three masters\n")
	_, _ = os.Stderr.WriteString("2026-09-24T10:00:01+02:00 " + obs.TracePrefix + `{"event":"process.output_gap","message":"late"}` + "\n")
	os.Exit(1)
}

func TestCommandFailureIsConciseAndLeadsWithChildFailureCode(t *testing.T) {
	t.Setenv("CLIPHUB_CODED_COMMAND_HELPER", "1")
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	_, err = (execCommandRunner{}).Run(context.Background(), executable, "-test.run=^TestCodedCommandHelper$")
	if err == nil {
		t.Fatal("the helper unexpectedly succeeded")
	}
	tool := filepath.Base(executable)
	want := "failure_code=audio_master_exhausted substage=audio_master; " + tool + ": exit status 1: audio_loudness_failed: Media Foundation AAC recovery is unavailable after three masters"
	if err.Error() != want {
		t.Fatalf("Error() = %q\nwant      %q", err.Error(), want)
	}
	if strings.Contains(err.Error(), filepath.Dir(executable)) {
		t.Fatalf("the executable path leaked into the error: %q", err.Error())
	}
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) || exitErr.ExitCode() != 1 {
		t.Fatalf("exit error lost from the chain: %v", err)
	}

	id := uuid.New()
	repo := newFakeJobRepo(job.Job{ID: id, Status: job.StatusRecorded})
	if err := recordTaskFailure(context.Background(), repo, id, tasks.TypeRenderVariant, err); err != nil {
		t.Fatal(err)
	}
	events, selectErr := obs.Default().SelectErrors(id.String(), tasks.TypeRenderVariant)
	if selectErr != nil || len(events) != 1 {
		t.Fatalf("journal events = %#v, %v", events, selectErr)
	}
	if !strings.HasPrefix(events[0].Message, "failure_code=audio_master_exhausted substage=audio_master; ") {
		t.Fatalf("journal message does not lead with the code: %q", events[0].Message)
	}
}

func TestCommandFailureWithoutCodeKeepsToolExitAndLastStderrLine(t *testing.T) {
	t.Setenv("CLIPHUB_DIAGNOSTIC_COMMAND_HELPER", "1")
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	_, err = (execCommandRunner{}).Run(context.Background(), executable, "-test.run=^TestDiagnosticCommandHelper$")
	want := filepath.Base(executable) + ": exit status 17: CANARY_CAUSE: encoder device unavailable"
	if err == nil || err.Error() != want {
		t.Fatalf("Error() = %v, want %q", err, want)
	}
	if _, ok := obs.FailureOf(err); ok {
		t.Fatal("an unclassified subprocess failure was given a code")
	}
	if text := commandText(err); !strings.Contains(text, "Preparing the encoder") || !strings.Contains(text, "stdout result must remain") {
		t.Fatalf("complete output is not available to failure parsers: %q", text)
	}
}

// editorFFmpegFailureStderr is zv-editor's stderr for an unclassified FFmpeg
// failure: trace records, then log.Fatal's record whose first line is the
// canonical "ffmpeg <label>: exit status N: <cause>" and whose following lines
// are the complete FFmpeg 8.1 stderr, ending with FFmpeg's generic trailers.
var editorFFmpegFailureStderr = strings.Join([]string{
	"2026-09-24T10:00:00+02:00 " + obs.TracePrefix + `{"event":"tool.started","message":"ffmpeg HUD transition composition"}`,
	"2026-09-24T10:00:01+02:00 " + obs.TracePrefix + `{"event":"tool.finished","message":"ffmpeg HUD transition composition: exit status 1","outcome":"error"}`,
	"2026-09-24T10:00:01+02:00 ffmpeg HUD transition composition: exit status 0xfffffffe: [in#1 @ 000001a7d1e7a340] Error opening input: No such file or directory",
	"Input #0, matroska,webm, from 'hud-base.mkv':",
	"  Duration: 00:00:02.00, start: 0.000000, bitrate: 181 kb/s",
	"  Stream #0:0: Video: ffv1, bgra, 1920x1080, 60 fps, 60 tbr, 1k tbn",
	"[in#1 @ 000001a7d1e7a340] Error opening input: No such file or directory",
	"Error opening input file hud-transition.mkv.",
	"Error opening input files: No such file or directory",
	"2026-09-24T10:00:01+02:00 " + obs.TracePrefix + `{"event":"process.output_gap","message":"late"}`,
	"",
}, "\n")

func TestEditorFFmpegFailureCauseNamesTheFailingCommand(t *testing.T) {
	runErr := newCommandError("zv-editor.exe", errors.New("exit status 1"), editorFFmpegFailureStderr, editorFFmpegFailureStderr)
	want := "zv-editor.exe: exit status 1: ffmpeg HUD transition composition: exit status 0xfffffffe: [in#1 @ 000001a7d1e7a340] Error opening input: No such file or directory"
	if runErr.Error() != want {
		t.Fatalf("Error() = %q\nwant      %q", runErr.Error(), want)
	}
	if _, ok := obs.FailureOf(runErr); ok {
		t.Fatal("an unclassified editor failure was given a code")
	}
	if text := commandText(runErr); !strings.Contains(text, "Error opening input files: No such file or directory") {
		t.Fatalf("complete output is not available to failure parsers: %q", text)
	}

	id := uuid.New()
	repo := newFakeJobRepo(job.Job{ID: id, Status: job.StatusRecorded})
	if err := recordTaskFailure(context.Background(), repo, id, tasks.TypeRenderVariant, runErr); err != nil {
		t.Fatal(err)
	}
	events, selectErr := obs.Default().SelectErrors(id.String(), tasks.TypeRenderVariant)
	if selectErr != nil || len(events) != 1 {
		t.Fatalf("journal events = %#v, %v", events, selectErr)
	}
	if first, _, _ := strings.Cut(events[0].Message, "\n"); first != want {
		t.Fatalf("pipeline.error message = %q", events[0].Message)
	}
}

func TestCommandFailureCauseIgnoresEarlierExecFailures(t *testing.T) {
	for _, tc := range []struct {
		name, stderr, want string
	}{
		{
			// A master attempt that failed and was recovered is not the cause
			// of a later, different final failure.
			name: "recovered FFmpeg failure before the final one",
			stderr: "2026-09-24T10:00:00+02:00 full demo master attempt 1: ffmpeg Full Demo program master: exit status 1: [fc#0 @ 00000207057a5dc0] Error applying option 'TP' to filter 'loudnorm': Result too large\n" +
				"[fc#0 @ 00000207057a5dc0] Error applying option 'TP' to filter 'loudnorm': Result too large\n" +
				"Error : Result too large\n" +
				"2026-09-24T10:05:00+02:00 full_demo_output_invalid: complete decode did not certify 50400 frames (got 50399)\n",
			want: "zv-editor.exe: exit status 1: full_demo_output_invalid: complete decode did not certify 50400 frames (got 50399)",
		},
		{
			name: "recorder error line after a logged helper failure",
			stderr: "2026/09/24 10:00:00 capture job cleanup: taskkill: exit status 128\n" +
				"error: HLAE not found\n",
			want: "zv-editor.exe: exit status 1: error: HLAE not found",
		},
		{
			name:   "exec failure without output",
			stderr: "2026-09-24T10:00:00+02:00 ffmpeg Full Demo delivery probe: exit status 1\n",
			want:   "zv-editor.exe: exit status 1: ffmpeg Full Demo delivery probe: exit status 1",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := newCommandError("zv-editor.exe", errors.New("exit status 1"), tc.stderr, tc.stderr).Error(); got != tc.want {
				t.Fatalf("Error() = %q\nwant      %q", got, tc.want)
			}
		})
	}
}

func TestRecorderFailureCodeStaysOutOfReasonButLeadsJournal(t *testing.T) {
	output := "windowed capture: patched cs2_video.txt\n" +
		"error: failure_code=hlae_hook_incompatible substage=capture; HLAE hook crashed with a native error dialog\n"
	runErr := newCommandError("zv-recorder.exe", errors.New("exit status 6"), output, output)
	if got := runErr.Error(); got != "failure_code=hlae_hook_incompatible substage=capture; zv-recorder.exe: exit status 6: error: HLAE hook crashed with a native error dialog" {
		t.Fatalf("recorder command error = %q", got)
	}
	failure := newRecordFailure(runErr, recording.RecordingResult{}, nil)
	if failure.Error() != "recorder failed: HLAE hook crashed with a native error dialog" {
		t.Fatalf("user-facing reason = %q", failure.Error())
	}
	message := workerDiagnosticMessage(failure)
	if !strings.HasPrefix(message, "failure_code=hlae_hook_incompatible substage=capture; recorder failed: ") || !strings.Contains(message, "windowed capture") {
		t.Fatalf("journal message = %q", message)
	}
}

func TestRecorderMarkersAreFoundInTheCompleteOutput(t *testing.T) {
	// The marker is not on the last stderr line, so it is absent from the
	// concise Error() and must still classify the failure.
	output := "error: capture POV verification failed: " + networkDisconnectMarker + "\nCS2 closed\n"
	runErr := newCommandError("zv-recorder.exe", errors.New("exit status 1"), output, output)
	if strings.Contains(runErr.Error(), networkDisconnectMarker) {
		t.Fatalf("test premise: marker is on the concise line: %q", runErr.Error())
	}
	if reason := recordFailureReason(runErr, recording.RecordingResult{}, nil); !strings.HasPrefix(reason, demoIncompatiblePrefix) {
		t.Fatalf("reason = %q", reason)
	}
	crash := newCommandError("zv-recorder.exe", errors.New("exit status 1"), missingCaptureAttestationMarker+"\nCS2 closed\n", missingCaptureAttestationMarker+"\nCS2 closed\n")
	if !retryableCaptureCrash(crash, recording.RecordingResult{}) {
		t.Fatal("a transient CS2 crash marker before the last line is no longer retried")
	}
}

func TestRenderTimeoutIsClassifiedAsInterrupted(t *testing.T) {
	repo := newFakeRepo()
	store := newFakeStorage()
	id := uuid.New()
	plan := minimalKillPlan()
	repo.jobs[id] = &job.Job{ID: id, Status: job.StatusRecorded, Rules: rules.Default(), KillPlan: &plan}
	putJSON(t, store, recording.ResultArtifactKey(id), recordingResultWithSegment("", "C:/stale/seg-001.mp4"))
	_ = store.Put(mustSegmentClipKey(t, id, "seg-001"), bytes.NewReader([]byte("clip")))
	w := NewRenderWorker(repo, store, RenderWorkerConfig{WorkDir: t.TempDir(), EditorPath: "zv-editor", Timeout: "50ms"})
	w.runner = &fakeRunner{fn: func(ctx context.Context, _ string, _ ...string) ([]byte, error) {
		<-ctx.Done()
		return nil, newCommandError("zv-editor.exe", errors.New("exit status 1"), "", "")
	}}
	err := w.HandleRenderVariant(context.Background(), renderTask(t, id, editor.PresetViral60Clean))
	failure, ok := obs.FailureOf(err)
	if !ok || failure.Code != obs.FailureRenderInterrupted || !strings.HasPrefix(err.Error(), "failure_code=render_interrupted; zv-editor.exe: exit status 1") {
		t.Fatalf("timed-out render = %v", err)
	}
}
