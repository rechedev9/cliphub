package obs

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

// alerterFailurePattern is the contract the remote alerter parses.
var alerterFailurePattern = regexp.MustCompile(`^failure_code=([a-z0-9_]+)( substage=([a-z0-9_]+))?;`)

func TestWithFailureRoundTrip(t *testing.T) {
	cause := fmt.Errorf("render: %w", context.Canceled)
	err := WithFailure(cause, FailureAudioMasterExhausted, SubstageAudioMaster)
	want := "failure_code=audio_master_exhausted substage=audio_master; render: context canceled"
	if err.Error() != want {
		t.Fatalf("Error() = %q, want %q", err.Error(), want)
	}
	failure, ok := FailureOf(fmt.Errorf("outer: %w", err))
	if !ok || failure != (Failure{Code: FailureAudioMasterExhausted, Substage: SubstageAudioMaster}) {
		t.Fatalf("FailureOf = %+v, %v", failure, ok)
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatal("the wrapper hides the original error from errors.Is")
	}
	if again := WithFailure(err, FailureFFmpegFailed, SubstageAACRecovery); again != err {
		t.Fatalf("a second classification replaced the first: %v", again)
	}
	if WithFailure(nil, FailureFFmpegFailed, SubstageAudioMaster) != nil {
		t.Fatal("nil error was classified")
	}
	if _, ok := FailureOf(cause); ok {
		t.Fatal("unclassified error reported a failure")
	}
	if got := WithFailure(errors.New("killed"), FailureRenderInterrupted, "").Error(); got != "failure_code=render_interrupted; killed" {
		t.Fatalf("no-substage prefix = %q", got)
	}
}

func TestFindFailureReadsTheChildFailureLine(t *testing.T) {
	stderr := strings.Join([]string{
		"2026-09-24T10:00:00+02:00 " + TracePrefix + `{"event":"process.stderr","message":"failure_code=ffmpeg_failed substage=concat; stale copy"}`,
		"2026-09-24T10:00:01+02:00 failure_code=loudnorm_param_out_of_range substage=audio_master; ffmpeg Full Demo program master: exit status 1: Result too large",
		"[Parsed_loudnorm_0 @ 000001] Value -10.180000 for parameter 'TP' out of range [-9 - 0]",
		"",
	}, "\n")
	failure, rest, ok := FindFailure(stderr)
	if !ok || failure != (Failure{Code: FailureLoudnormParamOutOfRange, Substage: SubstageAudioMaster}) {
		t.Fatalf("FindFailure = %+v, %v", failure, ok)
	}
	if rest != "ffmpeg Full Demo program master: exit status 1: Result too large" {
		t.Fatalf("rest = %q", rest)
	}
	failure, rest, ok = FindFailure("2026/09/24 10:00:00 render short 3: failure_code=capture_incomplete substage=capture; segment missing")
	if !ok || failure.Code != FailureCaptureIncomplete || rest != "render short 3: segment missing" {
		t.Fatalf("mid-line prefix = %+v %q %v", failure, rest, ok)
	}
	if _, _, ok := FindFailure("error: failure_code=Bad; nope\nplain failure"); ok {
		t.Fatal("matched a prefix that breaks the snake_case contract")
	}
}

func TestStripAndLeadWithFailure(t *testing.T) {
	if got := StripFailurePrefix("failure_code=capture_pov_unverified substage=capture; recorder failed: x"); got != "recorder failed: x" {
		t.Fatalf("strip = %q", got)
	}
	if got := StripFailurePrefix("recorder failed: failure_code=a; x"); got != "recorder failed: failure_code=a; x" {
		t.Fatalf("strip removed a prefix that does not lead: %q", got)
	}
	err := fmt.Errorf("reason: %w", WithFailure(errors.New("x"), FailureHLAEHookIncompatible, SubstageCapture))
	if got := LeadWithFailure(err, "recorder failed: hook"); got != "failure_code=hlae_hook_incompatible substage=capture; recorder failed: hook" {
		t.Fatalf("lead = %q", got)
	}
	if got := LeadWithFailure(err, "failure_code=hlae_hook_incompatible substage=capture; y"); strings.Count(got, "failure_code=") != 1 {
		t.Fatalf("lead duplicated the prefix: %q", got)
	}
	if got := LeadWithFailure(errors.New("x"), "plain"); got != "plain" {
		t.Fatalf("lead changed an unclassified message: %q", got)
	}
}

func TestClassOfIgnoresFailurePrefix(t *testing.T) {
	if got := ClassOf("failure_code=capture_pov_unverified substage=capture; demo_incompatible: cs2 cannot replay this demo"); got != ClassDemoIncompatible {
		t.Fatalf("ClassOf = %q", got)
	}
	for _, class := range []string{ClassMissingPlate, ClassCaptureFlake, ClassRecordingNotReusable, ClassFaceitRosterIncomplete} {
		found := false
		for _, known := range Classes() {
			found = found || known == class
		}
		if !found {
			t.Fatalf("Classes() misses %s", class)
		}
	}
}

func TestLastDiagnosticLine(t *testing.T) {
	stderr := "Preparing\n2026/09/24 10:00:00 CANARY_CAUSE: encoder device unavailable\n2026-09-24T10:00:00Z " + TracePrefix + `{"event":"tool.finished"}` + "\n\n"
	if got := LastDiagnosticLine(stderr); got != "CANARY_CAUSE: encoder device unavailable" {
		t.Fatalf("LastDiagnosticLine = %q", got)
	}
	if got := LastDiagnosticLine("\n \n"); got != "" {
		t.Fatalf("empty stderr = %q", got)
	}
}

func TestWithoutTraceLines(t *testing.T) {
	text := "Preparing\n2026/09/24 10:00:03 " + TracePrefix + `{"event":"cs2.console_tail","message":"lines=1\nNETWORK_DISCONNECT_MESSAGE_PARSE_ERROR"}` + "\nerror: HLAE not found\n"
	if got := WithoutTraceLines(text); got != "Preparing\nerror: HLAE not found\n" {
		t.Fatalf("WithoutTraceLines = %q", got)
	}
	if got := WithoutTraceLines("plain\n"); got != "plain\n" {
		t.Fatalf("text without traces changed: %q", got)
	}
	if !IsTraceLine("2026-09-24T10:00:00Z "+TracePrefix+"{}") || IsTraceLine("error: cliphub-diagnostic") {
		t.Fatal("IsTraceLine misclassified a line")
	}
}

// FFmpeg 8.1 stderr captured from real failing runs.
const (
	ffmpegLoudnormFilterStderr = `Input #0, lavfi, from 'sine=f=440:d=1':
  Duration: N/A, start: 0.000000, bitrate: 705 kb/s
  Stream #0:0: Audio: pcm_s16le, 44100 Hz, mono, s16, 705 kb/s
[Parsed_loudnorm_0 @ 0000021405cd0340] Value -10.180000 for parameter 'TP' out of range [-9 - 0]
[fc#-1 @ 0000021407baa900] Error applying option 'TP' to filter 'loudnorm': Result too large
Error opening output file master.m4a.
Error opening output files: Result too large
`
	ffmpegLoudnormComplexStderr = `[Parsed_loudnorm_0 @ 0000020705831a00] Value -10.180000 for parameter 'TP' out of range [-9 - 0]
[fc#0 @ 00000207057a5dc0] Error applying option 'TP' to filter 'loudnorm': Result too large
Error : Result too large
`
	ffmpegMissingInputStderr = `[in#0 @ 000001a7d1e7a340] Error opening input: No such file or directory
Error opening input file missing-input.wav.
Error opening input files: No such file or directory
`
	ffmpegEncoderRuntimeStderr = `[libx264 @ 000001e7b6f583c0] width not divisible by 2 (15x15)
[vost#0:0/libx264 @ 000001e7b6f53080] [enc:libx264 @ 000001e7b6f54400] Error while opening encoder - maybe incorrect parameters such as bit_rate, rate, width or height.
[vost#0:0/libx264 @ 000001e7b6f53080] Terminating thread with return code -22 (Invalid argument)
[out#0/null @ 000001e7b6ec1600] Nothing was written into output file, because at least one of its streams received no packets.
frame=    0 fps=0.0 q=0.0 Lsize=       0KiB time=N/A bitrate=N/A speed=N/A elapsed=0:00:00.00
Conversion failed!
`
)

func TestLastFFmpegCauseLineSkipsGenericTrailers(t *testing.T) {
	for _, tc := range []struct {
		name, stderr, want string
	}{
		{"loudnorm option in -af", ffmpegLoudnormFilterStderr, "[fc#-1 @ 0000021407baa900] Error applying option 'TP' to filter 'loudnorm': Result too large"},
		{"loudnorm option in -filter_complex", ffmpegLoudnormComplexStderr, "[fc#0 @ 00000207057a5dc0] Error applying option 'TP' to filter 'loudnorm': Result too large"},
		{"missing input", ffmpegMissingInputStderr, "[in#0 @ 000001a7d1e7a340] Error opening input: No such file or directory"},
		{"runtime encoder failure", ffmpegEncoderRuntimeStderr, "[out#0/null @ 000001e7b6ec1600] Nothing was written into output file, because at least one of its streams received no packets."},
		{"late trace line", ffmpegMissingInputStderr + "2026-09-24T10:00:00Z " + TracePrefix + `{"event":"tool.finished"}` + "\n", "[in#0 @ 000001a7d1e7a340] Error opening input: No such file or directory"},
		{"only trailers", "Conversion failed!\n", "Conversion failed!"},
		{"no trailer", "[aac_mf @ 01] could not set the desired input type\nError while opening encoder for output stream #0:0\n", "Error while opening encoder for output stream #0:0"},
		{"empty", "\n \n", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := LastFFmpegCauseLine(tc.stderr); got != tc.want {
				t.Fatalf("LastFFmpegCauseLine = %q\nwant                  %q", got, tc.want)
			}
		})
	}
}

func TestLastExecFailureLine(t *testing.T) {
	head := "ffmpeg Full Demo program master: exit status 1: [fc#-1 @ 0000021407baa900] Error applying option 'TP' to filter 'loudnorm': Result too large"
	editorFatal := "2026-09-24T10:00:00+02:00 " + TracePrefix + `{"event":"tool.finished","message":"ffmpeg Full Demo program master: exit status 1"}` + "\n" +
		"2026-09-24T10:00:01+02:00 " + head + "\n" + ffmpegLoudnormFilterStderr +
		"2026-09-24T10:00:01+02:00 " + TracePrefix + `{"event":"process.output_gap","message":"late"}` + "\n"
	for _, tc := range []struct {
		name, stderr, want string
	}{
		{"editor log.Fatal with the FFmpeg output", editorFatal, head},
		{"wrapped label", "2026/09/24 10:00:01 render short 3: " + head + "\n" + ffmpegLoudnormFilterStderr, "render short 3: " + head},
		{"last line", "Preparing\nffmpeg probe: exit status 3\n", "ffmpeg probe: exit status 3"},
		{"Windows hex exit status", "2026-09-24T10:00:01+02:00 " + strings.Replace(head, "exit status 1", "exit status 0xffffffde", 1) + "\r\n" + strings.ReplaceAll(ffmpegLoudnormFilterStderr, "\n", "\r\n"), strings.Replace(head, "exit status 1", "exit status 0xffffffde", 1)},
		{"recovered failure before a different final one", "2026-09-24T10:00:01+02:00 master attempt 1: " + head + "\n" + ffmpegLoudnormFilterStderr + "2026-09-24T10:05:00+02:00 full_demo_output_invalid: complete decode did not certify 50400 frames\n", ""},
		{"logged helper failure before a plain error line", "2026/09/24 10:00:00 capture job cleanup: taskkill: exit status 128\nerror: HLAE not found\n", ""},
		{"cause not in the following output", "tool: exit status 1: device lost\nunrelated line\n", ""},
		{"no exec failure", "Preparing\nerror: HLAE not found\n", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := LastExecFailureLine(tc.stderr); got != tc.want {
				t.Fatalf("LastExecFailureLine = %q\nwant                  %q", got, tc.want)
			}
		})
	}
}

func TestRFC3339LogWriter(t *testing.T) {
	var out bytes.Buffer
	at := time.Date(2026, 9, 24, 10, 0, 0, 0, time.FixedZone("CEST", 2*60*60))
	writer := RFC3339LogWriter{Out: &out, Now: func() time.Time { return at }}
	if n, err := writer.Write([]byte("worker job=1 transition=failed\n")); err != nil || n != len("worker job=1 transition=failed\n") {
		t.Fatalf("Write = %d, %v", n, err)
	}
	if got := out.String(); got != "2026-09-24T10:00:00+02:00 worker job=1 transition=failed\n" {
		t.Fatalf("line = %q", got)
	}
}

// The desktop filter runs before upload and must leave the prefix parseable.
// It is TypeScript, so this runs it through Node (24 strips types natively).
func TestDesktopDiagnosticFilterKeepsFailurePrefix(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node is required to run the desktop diagnostic filter")
	}
	filter, err := filepath.Abs(filepath.Join("..", "..", "desktop", "src", "diagnostic-message.ts"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filter); err != nil {
		t.Skip("desktop sources are not available:", err)
	}
	type sample struct {
		failure Failure
		message string
	}
	samples := []sample{
		{Failure{FailureAudioMasterExhausted, SubstageAudioMaster}, "zv-editor.exe: exit status 1: audio_loudness_failed: approved targets remain unmet after three masters and three recovery attempts; decoded AAC: #1 -14.02 LUFS / 1.19 dBTP"},
		{Failure{FailureLoudnormParamOutOfRange, SubstageAudioMaster}, "zv-editor.exe: exit status 1: ffmpeg Full Demo program master: exit status 1: [Parsed_loudnorm_0 @ 000001d2c3] Value -10.180000 for parameter 'TP' out of range [-9 - 0]"},
		{Failure{FailureAACRecoveryFailed, SubstageAACRecovery}, `audio_loudness_failed: AAC recovery: ffmpeg Full Demo AAC recovery: exit status 1: could not open C:\Users\someone\AppData\Local\ClipHub\work\candidate.m4a`},
		{Failure{FailureDeliveryVerifyFailed, SubstageDeliveryVerify}, "full_demo_output_invalid: complete decode did not certify 50400 frames (got 50399)"},
		{Failure{FailureFFmpegFailed, SubstageConcat}, "ffmpeg concat: exit status 3221225477: /tmp/work/list.txt"},
		{Failure{FailureCaptureIncomplete, SubstageCapture}, "recorder failed: round 7 ended 3 frames early"},
		{Failure{FailureHLAEHookIncompatible, SubstageCapture}, `HLAE hook crashed with a native error dialog ("Error - AfxHookSource2") cs2_build=14182 hlae=v2.192.3`},
		{Failure{FailureCS2AlreadyRunning, SubstageCapture}, "cs2.exe is already running"},
		{Failure{FailureCapturePOVUnverified, SubstageCapture}, "CS2 exited without the completed POV verification marker"},
		{Failure{FailureRenderInterrupted, SubstageOverlay}, "context deadline exceeded"},
		{Failure{Code: FailureRenderInterrupted}, "zv-editor.exe: exit status 1"},
	}
	inputs := make([]string, len(samples))
	for i, s := range samples {
		inputs[i] = WithFailure(errors.New(s.message), s.failure.Code, s.failure.Substage).Error()
	}
	body, err := json.Marshal(inputs)
	if err != nil {
		t.Fatal(err)
	}
	script := `import { pathToFileURL } from 'node:url';
const { diagnosticMessage } = await import(pathToFileURL(process.env.CLIPHUB_DIAGNOSTIC_FILTER).href);
let input = '';
for await (const chunk of process.stdin) input += chunk;
process.stdout.write(JSON.stringify(JSON.parse(input).map((message) => diagnosticMessage(message))));`
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, node, "--no-warnings", "--input-type=module", "-e", script)
	cmd.Env = append(os.Environ(), "CLIPHUB_DIAGNOSTIC_FILTER="+filter)
	cmd.Stdin = bytes.NewReader(body)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("run desktop filter: %v: %s", err, stderr.String())
	}
	var filtered []string
	if err := json.Unmarshal(out, &filtered); err != nil || len(filtered) != len(samples) {
		t.Fatalf("filter output %q: %v", out, err)
	}
	for i, s := range samples {
		match := alerterFailurePattern.FindStringSubmatch(filtered[i])
		if match == nil || match[1] != string(s.failure.Code) || match[3] != string(s.failure.Substage) {
			t.Errorf("prefix lost for %s: %q", s.failure.Code, filtered[i])
		}
	}
	if strings.Contains(filtered[2], `C:\Users`) {
		t.Fatalf("the filter kept a Windows path: %q", filtered[2])
	}
}
