package telemetryalert

import (
	"strings"
	"testing"

	"github.com/rechedev9/cliphub/internal/obs"
)

func TestParseFailureCode(t *testing.T) {
	tests := []struct{ message, code, substage string }{
		{"failure_code=loudnorm_param_out_of_range substage=audio_master; Value -10.18", "loudnorm_param_out_of_range", "audio_master"},
		{"failure_code=hlae_hook_incompatible; exited 6", "hlae_hook_incompatible", ""},
		{"exit status 1: failure_code=late_prefix substage=x; no", "", ""},
		{"failure_code=Bad-Code substage=x; no", "", ""},
		{"", "", ""},
	}
	for _, tt := range tests {
		code, substage := ParseFailureCode(tt.message)
		if code != tt.code || substage != tt.substage {
			t.Errorf("%q: got %q/%q", tt.message, code, substage)
		}
	}
}

func TestSignatureNormalizesVolatileTokens(t *testing.T) {
	tests := []struct{ message, want string }{
		{"exit status 1: 2026[path] 15:09:31 audio_loudness_failed: true peak -0.42 dBTP above -1.00 target after three masters",
			"exit status <n>: audio_loudness_failed: true peak -<n> dBTP above -<n> target after three masters"},
		{"starting\n[Parsed_loudnorm_0] Value -10.180000 for parameter 'TP' out of range [-9 - 0]\nError applying option",
			"[Parsed_loudnorm_0] Value -<n> for parameter 'TP' out of range [-<n> - <n>]"},
		{"2026-09-23T13:38:04.123Z job 4ae10001-0000-4000-8000-000000000003 failed after 5.2s (0xC0000142)",
			"job <n> failed after <n> (<n>)"},
		{"[path]: [path], [path] exit status 1", "[path] exit status <n>"},
		{"hash deadbeef00112233 differs from deadbeef", "hash <n> differs from deadbeef"},
		{"hlae=v2.192.2-cliphub.1 crashed in AfxHookSource2 on h264 x64", "hlae=v<n> crashed in AfxHookSource2 on h264 x64"},
		{"web terminó inesperadamente (código 1073807364)", "web terminó inesperadamente (código <n>)"},
		{"no keyword here\nsecond line", "no keyword here"},
	}
	for _, tt := range tests {
		if got := Signature(tt.message); got != tt.want {
			t.Errorf("Signature(%q)\n got %q\nwant %q", tt.message, got, tt.want)
		}
	}
	if got := Signature("failed " + strings.Repeat("x", 400)); len([]rune(got)) != maxSignatureRunes {
		t.Errorf("signature length = %d", len([]rune(got)))
	}
}

func TestIssueKeysSeparateTodaysRecordFailures(t *testing.T) {
	labels := Labels{"orchestrator", "pipeline.error", "worker", "record:demo"}
	key := func(message string) string {
		return newOccurrence(Record{Message: message}, labels, false).Key
	}
	hlae := key(`record:demo exited 6 after 5 s: HLAE hook crashed with a native error dialog ("Error - AfxHookSource2")`)
	running := key("cs2.exe is already running; close CS2 and retry")
	pov := key("capture POV verification failed: expected player 3, saw 7")
	if hlae == running || hlae == pov || running == pov {
		t.Fatalf("record:demo failure modes share a key: %s %s %s", hlae, running, pov)
	}
	if again := key(`record:demo exited 6 after 4 s: HLAE hook crashed with a native error dialog ("Error - AfxHookSource2")`); again != hlae {
		t.Fatalf("a reworded number changed the key")
	}
	if coded := key("failure_code=hlae_hook_incompatible substage=hook_start; anything"); coded != IssueKey(labels, "hlae_hook_incompatible") || len(coded) != 16 {
		t.Fatalf("coded key = %s", coded)
	}
	other := newOccurrence(Record{Message: "failure_code=hlae_hook_incompatible; x"}, Labels{"orchestrator", "pipeline.error", "worker", "render:variant"}, false).Key
	if other == IssueKey(labels, "hlae_hook_incompatible") {
		t.Fatalf("labels do not separate keys")
	}
}

func TestSignatureSkipsRelayedTraceLines(t *testing.T) {
	trace := obs.TracePrefix + `{"event":"cs2.console_tail","message":"lines=2\nNETWORK_DISCONNECT error"}`
	got := Signature("capture started\n" + trace + "\nrecord:demo: exit status 6: HLAE hook crashed")
	if want := Signature("record:demo: exit status 6: HLAE hook crashed"); got != want {
		t.Fatalf("Signature = %q, want %q", got, want)
	}
}
