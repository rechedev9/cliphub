package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/rechedev9/cliphub/internal/obs"
	"github.com/rechedev9/cliphub/internal/recording"
)

// failureCodePattern is the alerter's parser for the failure-code prefix.
var failureCodePattern = regexp.MustCompile(`^failure_code=([a-z0-9_]+)( substage=([a-z0-9_]+))?;`)

func writeCS2Install(t *testing.T, steamInf string) string {
	t.Helper()
	root := t.TempDir()
	cs2 := filepath.Join(root, "game", "bin", "win64", "cs2.exe")
	if err := os.MkdirAll(filepath.Dir(cs2), 0o750); err != nil {
		t.Fatal(err)
	}
	if steamInf != "" {
		inf := filepath.Join(root, "game", "csgo", "steam.inf")
		if err := os.MkdirAll(filepath.Dir(inf), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(inf, []byte(steamInf), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return cs2
}

func writeHLAEPin(t *testing.T, marker string) string {
	t.Helper()
	dir := t.TempDir()
	if marker != "" {
		if err := os.WriteFile(filepath.Join(dir, hlaeInstallMarker), []byte(marker), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return filepath.Join(dir, "HLAE.exe")
}

func TestReadCS2SteamInfParsesBuildAndPatch(t *testing.T) {
	cs2 := writeCS2Install(t, "ClientVersion=2000915\r\nServerVersion=2000915\r\nPatchVersion=1.41.8.3\r\nProductName=cs2\r\nappID=730\r\nSourceRevision=11030201\r\nVersionDate=Sep 23 2026\r\n")
	if got := cs2SteamInfPath(cs2); got != filepath.Join(filepath.Dir(filepath.Dir(filepath.Dir(cs2))), "csgo", "steam.inf") {
		t.Fatalf("steam.inf path = %q", got)
	}
	build, patch := readCS2SteamInf(cs2SteamInfPath(cs2))
	if build != "2000915" || patch != "1.41.8.3" {
		t.Fatalf("steam.inf = build %q patch %q", build, patch)
	}
}

func TestReadCS2SteamInfMissingOrMalformedIsUnknown(t *testing.T) {
	build, patch := readCS2SteamInf(cs2SteamInfPath(writeCS2Install(t, "")))
	if build != "unknown" || patch != "unknown" {
		t.Fatalf("missing steam.inf = build %q patch %q, want unknown", build, patch)
	}
	build, patch = readCS2SteamInf(cs2SteamInfPath(writeCS2Install(t, "ClientVersion=20009 15\nPatchVersion=1.41 beta\n")))
	if build != "unknown" || patch != "unknown" {
		t.Fatalf("malformed steam.inf = build %q patch %q, want unknown", build, patch)
	}
}

func TestReadPinnedHLAEVersion(t *testing.T) {
	for _, tc := range []struct {
		name, marker, want string
	}{
		{"official pin", `{"files":[],"schemaVersion":2,"sourceSha256":"x","version":"2.192.3"}`, "2.192.3"},
		{"interim pin", `{"version":"2.192.2-cliphub.1"}`, "2.192.2-cliphub.1"},
		{"no marker", "", "unknown"},
		{"corrupt marker", `{"version":`, "unknown"},
		{"unsafe version", `{"version":"2.192.3 C:\\tools"}`, "unknown"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := readPinnedHLAEVersion(writeHLAEPin(t, tc.marker)); got != tc.want {
				t.Fatalf("HLAE version = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestCaptureToolchainMessage(t *testing.T) {
	cs2 := writeCS2Install(t, "ClientVersion=2000915\nPatchVersion=1.41.8.3\n")
	hlae := writeHLAEPin(t, `{"version":"2.192.3"}`)
	if got, want := readCaptureToolchain(cs2, hlae, recording.EncoderNVENC).Message(), "cs2_build=2000915 cs2_patch=v1.41.8.3 hlae=v2.192.3 encoder=nvenc"; got != want {
		t.Fatalf("toolchain = %q, want %q", got, want)
	}
	missing := readCaptureToolchain(writeCS2Install(t, ""), writeHLAEPin(t, ""), recording.EncoderDefault)
	if got, want := missing.Message(), "cs2_build=unknown cs2_patch=unknown hlae=unknown encoder=x264"; got != want {
		t.Fatalf("toolchain without evidence = %q, want %q", got, want)
	}
	for encoder, want := range map[string]string{
		recording.EncoderLibx264: "x264",
		recording.EncoderAMF:     "amf",
		recording.EncoderQSV:     "qsv",
		"av1-future":             "other",
	} {
		if got := captureEncoderLabel(encoder); got != want {
			t.Fatalf("encoder %q = %q, want %q", encoder, got, want)
		}
	}
}

func captureLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var output bytes.Buffer
	previous := log.Writer()
	log.SetOutput(&output)
	t.Cleanup(func() { log.SetOutput(previous) })
	return &output
}

func decodeTraceLine(t *testing.T, line string) obs.TraceEntry {
	t.Helper()
	_, body, ok := strings.Cut(strings.TrimSpace(line), obs.TracePrefix)
	if !ok {
		t.Fatalf("line lacks the trace prefix: %q", line)
	}
	var entry obs.TraceEntry
	if err := json.Unmarshal([]byte(body), &entry); err != nil {
		t.Fatalf("decode trace %q: %v", body, err)
	}
	return entry
}

func TestEmitCaptureToolchainWritesTraceLine(t *testing.T) {
	output := captureLog(t)
	emitCaptureToolchain(captureToolchain{CS2Build: "2000915", CS2Patch: "1.41.8.3", HLAE: "2.192.3", Encoder: "x264"})
	entry := decodeTraceLine(t, output.String())
	if entry.Event != "attempt.toolchain" || entry.Level != "info" || entry.Message != "cs2_build=2000915 cs2_patch=v1.41.8.3 hlae=v2.192.3 encoder=x264" {
		t.Fatalf("toolchain trace = %+v", entry)
	}
}

func TestStderrFailureLineCarriesFailureCodes(t *testing.T) {
	hook := stopProcessAfterWaitFailure("cs2.exe", &hookIncompatibleError{windowTitle: "Error - AfxHookSource2"}, true, func(string) error {
		return errors.New("taskkill failed")
	})
	for _, tc := range []struct {
		name string
		err  error
		code string
	}{
		{"exit 6 hook crash", hook, "hlae_hook_incompatible"},
		{"cs2 already running", fmt.Errorf("%w; close it before recording", errCS2AlreadyRunning), "cs2_already_running"},
		{"missing POV marker", (&cs2ConsoleLogMonitor{path: "console.log"}).requireCaptureVerified(), "capture_pov_unverified"},
		{"coverage", errors.Join(&captureIncompleteError{err: errors.New("capture truncated: segment seg-001 is 1.000s short")}, errors.New("restore settings")), "capture_incomplete"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			line := stderrFailureLine(tc.err)
			rest, ok := strings.CutPrefix(line, "error: ")
			if !ok || !strings.HasSuffix(line, "\n") {
				t.Fatalf("failure line shape = %q", line)
			}
			match := failureCodePattern.FindStringSubmatch(rest)
			if match == nil || match[1] != tc.code || match[3] != "capture" {
				t.Fatalf("failure line = %q, want code %s substage capture", line, tc.code)
			}
			if !strings.HasSuffix(strings.TrimSpace(rest), strings.TrimSpace(tc.err.Error())) {
				t.Fatalf("failure line lost the original message: %q", line)
			}
			if want := "error: failure_code=" + tc.code + " substage=capture; " + tc.err.Error() + "\n"; line != want {
				t.Fatalf("failure line = %q\nwant           %q", line, want)
			}
		})
	}
	if !strings.Contains(stderrFailureLine(hook), "HLAE hook crashed with a native error dialog") {
		t.Fatalf("exit-6 failure line lost the hook explanation: %q", stderrFailureLine(hook))
	}
}

func TestStderrFailureLineLeavesOtherFailuresUncoded(t *testing.T) {
	for _, err := range []error{
		&demoParseError{path: "console.log"},
		&unplayableStartError{path: "console.log"},
		&captureVerificationError{path: "console.log", reason: "observer target drifted from 7656 to 7657"},
		errors.New("HLAE not found"),
	} {
		if line := stderrFailureLine(err); line != "error: "+err.Error()+"\n" {
			t.Fatalf("uncoded failure line = %q", line)
		}
	}
}

func TestValidateCaptureResultMarksCoverageAsIncomplete(t *testing.T) {
	var incomplete *captureIncompleteError
	if err := validateCaptureResult(recording.RecordingResult{}, filepath.FromSlash("C:/Steam/game/bin/win64/cs2.exe")); errors.As(err, &incomplete) {
		t.Fatalf("upload validation failure was coded as capture_incomplete: %v", err)
	}
}

func writeConsoleLog(t *testing.T, lines []string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "console.log")
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\r\n")+"\r\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestCS2ConsoleTailKeepsLastFortyLinesAsPlainJSON(t *testing.T) {
	var lines []string
	for i := 0; i < 50; i++ {
		lines = append(lines, fmt.Sprintf("09/23 23:22:%02d [Client] line %d", i, i))
	}
	markers := []string{
		"ResetBreakpadAppId",
		"NETWORK_DISCONNECT_MESSAGE_PARSE_ERROR",
		"[zackvideo] capture_failed: demo playback ended before every protected segment completed",
		"unplayable_start: probe",
		"CS2 exited without the completed POV verification marker",
	}
	lines = append(lines[:len(lines)-len(markers)], markers...)
	tail, count := readCS2ConsoleTail(writeConsoleLog(t, lines))
	if count != consoleTailMaxLines || strings.Contains(tail, "line 9\n") || !strings.HasPrefix(tail, "09/23 23:22:10 ") {
		t.Fatalf("tail kept %d lines:\n%s", count, tail)
	}
	if strings.Contains(tail, "\r") {
		t.Fatalf("tail kept CRLF line endings: %q", tail)
	}
	line, err := consoleTailTraceLine(tail, count, time.Date(2026, 9, 24, 10, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	// The media worker's marker parsers skip trace lines, so the console text
	// needs no escaping beyond JSON's own.
	if strings.Contains(line, `\u`) || strings.Count(line, "\n") != 0 || !strings.Contains(line, "NETWORK_DISCONNECT_MESSAGE_PARSE_ERROR") {
		t.Fatalf("console tail trace is not plain single-line JSON: %s", line)
	}
	entry := decodeTraceLine(t, line)
	if entry.Event != "cs2.console_tail" || entry.Level != "warn" || !entry.Time.Equal(time.Date(2026, 9, 24, 10, 0, 0, 0, time.UTC)) {
		t.Fatalf("console tail trace = %+v", entry)
	}
	if entry.Message != "lines=40\n"+tail {
		t.Fatalf("decoded console tail differs:\n%q\nwant\n%q", entry.Message, "lines=40\n"+tail)
	}
	for _, marker := range markers {
		if !strings.Contains(entry.Message, marker) {
			t.Fatalf("decoded tail lost %q", marker)
		}
	}
}

func TestCS2ConsoleTailIsCappedAtEightKiB(t *testing.T) {
	var lines []string
	for i := 0; i < 40; i++ {
		lines = append(lines, fmt.Sprintf("%02d %s", i, strings.Repeat("Ü", 400)))
	}
	tail, count := readCS2ConsoleTail(writeConsoleLog(t, lines))
	if len(tail) > consoleTailMaxBytes || count >= 40 || count != strings.Count(tail, "\n")+1 {
		t.Fatalf("tail = %d bytes, %d lines", len(tail), count)
	}
	one, oneCount := boundConsoleTail([]string{strings.Repeat("é", consoleTailMaxBytes)})
	if len(one) > consoleTailMaxBytes || oneCount != 1 || !strings.HasPrefix(one, "é") {
		t.Fatalf("oversized single line = %d bytes, %d lines", len(one), oneCount)
	}
	// Control bytes are escaped six bytes each, so a hostile console tail
	// still has to be trimmed to fit the relay's line limit.
	control := strings.TrimSuffix(strings.Repeat(strings.Repeat("\x01", 200)+"\n", 40), "\n")
	line, err := consoleTailTraceLine(control, 40, time.Now())
	if err != nil || len(line) > consoleTailMaxLineBytes {
		t.Fatalf("escaped trace line = %d bytes, %v", len(line), err)
	}
	entry := decodeTraceLine(t, line)
	kept, _, _ := strings.Cut(entry.Message, "\n")
	if kept == "lines=40" || strings.Count(entry.Message, "\n") != mustLineCount(t, kept) {
		t.Fatalf("trimmed trace line count disagrees with its body: %q", kept)
	}
}

func mustLineCount(t *testing.T, header string) int {
	t.Helper()
	var n int
	if _, err := fmt.Sscanf(header, "lines=%d", &n); err != nil {
		t.Fatalf("header %q: %v", header, err)
	}
	return n
}

func TestEmitCS2ConsoleTailSkipsMissingOrEmptyLog(t *testing.T) {
	output := captureLog(t)
	emitCS2ConsoleTail(filepath.Join(t.TempDir(), "missing.log"))
	empty := filepath.Join(t.TempDir(), "console.log")
	if err := os.WriteFile(empty, []byte("\r\n\r\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	emitCS2ConsoleTail(empty)
	if output.Len() != 0 {
		t.Fatalf("empty console produced a trace: %q", output.String())
	}
	emitCS2ConsoleTail(writeConsoleLog(t, []string{"Host activate: Quitting"}))
	if entry := decodeTraceLine(t, output.String()); entry.Message != "lines=1\nHost activate: Quitting" {
		t.Fatalf("console tail = %q", entry.Message)
	}
}
