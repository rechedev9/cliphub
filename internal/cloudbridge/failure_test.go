package cloudbridge

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestLocalJobFailureMapsEachLocalClass(t *testing.T) {
	const hookCrash = `zv-recorder.exe failed: exit status 6: HLAE hook crashed with a native error dialog ("Error - AfxHookSource2")`
	cases := []struct {
		name string
		job  LocalJob
		want string
	}{
		{name: "HLAE hook crash", job: LocalJob{FailureCode: "capture_incompatible", FailureReason: hookCrash}, want: codeCaptureIncompatible},
		{name: "capture flake", job: LocalJob{FailureCode: "capture_flake", FailureReason: "observer target 5 drifted from 3"}, want: codeCaptureFlake},
		{name: "interrupted", job: LocalJob{FailureCode: "interrupted", FailureReason: "interrupted: orchestrator restarted during recording"}, want: codeInterrupted},
		{name: "demo incompatible", job: LocalJob{FailureCode: "demo_incompatible", FailureReason: "demo_incompatible: old build"}, want: codeDemoIncompatible},
		{name: "unplayable start", job: LocalJob{FailureCode: "unplayable_start", FailureReason: "unplayable_start: crash at tick 0"}, want: codeUnplayableStart},
		{name: "target not found", job: LocalJob{FailureCode: "target_not_found", FailureReason: "steamid 7656 not found in demo"}, want: codeTargetNotFound},
		{name: "a local class with no cloud code", job: LocalJob{FailureCode: "missing_plate", FailureReason: "the plate is missing"}, want: codeJobFailed},
		{name: "an unclassified failure", job: LocalJob{FailureReason: "cs2.exe exited with status 1"}, want: codeJobFailed},
		// An orchestrator that sends no failure_code still gets the class from the reason.
		{name: "no code, classified from the reason", job: LocalJob{FailureReason: hookCrash}, want: codeCaptureIncompatible},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tc.job.Status = localJobFailed
			failure := localJobFailure(tc.job)
			if failure.Code != tc.want {
				t.Fatalf("code = %q, want %q", failure.Code, tc.want)
			}
			if failure.Detail != tc.job.FailureReason {
				t.Fatalf("detail = %q, want the untouched local reason %q", failure.Detail, tc.job.FailureReason)
			}
		})
	}
}

func TestRenderFailureIsTerminalAndKeepsTheRenderError(t *testing.T) {
	failure := renderFailure(LocalVariant{Status: localVariantFailed, Error: "ffmpeg exited with status 1: C:\\work\\out.mp4"})
	if failure.Code != codeRenderFailed || failure.Detail != "ffmpeg exited with status 1: C:\\work\\out.mp4" {
		t.Fatalf("failure = %+v, want render_failed with the render error as detail", failure)
	}
}

// The user sees the message; it must exist for every code of the contract,
// fit the portal's limit and never carry the raw cause.
func TestEveryFailureCodeHasAUserMessage(t *testing.T) {
	codes := []string{
		"capture_flake", "interrupted", "worker_lost", "demo_download_failed", "internal",
		"demo_incompatible", "unplayable_start", "target_not_found", "spec_mismatch",
		"invalid_spec", "render_failed", "job_failed", "timeout", "upload_failed",
		"capture_incompatible", "steam_unavailable", "disk_full", "tools_missing", "canceled",
	}
	const rawCause = `C:\Users\worker\zv-recorder.exe failed: exit status 1`
	for _, code := range codes {
		request := jobFailure{Code: code, Detail: rawCause}.request(42)
		switch {
		case request.Code != code || request.Detail != rawCause || request.MachineSeconds != 42:
			t.Errorf("%s: request = %+v, want the code, detail and machine time passed through", code, request)
		case request.Message == "" || utf8.RuneCountInString(request.Message) > maxFailureMessageRunes:
			t.Errorf("%s: message %q is empty or over %d characters", code, request.Message, maxFailureMessageRunes)
		case strings.Contains(request.Message, "zv-recorder") || strings.ContainsAny(request.Message, "\u2013\u2014"):
			t.Errorf("%s: message %q carries raw output or a dash the product copy does not use", code, request.Message)
		}
	}
	if got := (jobFailure{Code: "interrupted"}).request(-5).MachineSeconds; got != 0 {
		t.Errorf("machine seconds = %d for a negative duration, want 0", got)
	}
}
