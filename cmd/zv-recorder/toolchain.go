package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/rechedev9/cliphub/internal/obs"
	"github.com/rechedev9/cliphub/internal/recording"
)

// The recorder's stderr reaches the diagnostic log channel through the media
// worker, which relays every "[cliphub-diagnostic]" line with the job's trace
// context. These records therefore carry job_id and attempt_id without the
// recorder knowing either.
const (
	toolchainTraceEvent   = "attempt.toolchain"
	consoleTailTraceEvent = "cs2.console_tail"

	unknownToolchainValue = "unknown"

	// cs2.console_tail carries the end of CS2's -condebug console.log.
	consoleTailMaxLines = 40
	consoleTailMaxBytes = 8 << 10
	// Only this much of the file end is read to find the last lines.
	consoleTailReadBytes = 64 << 10
	// Keeps the JSON-escaped trace line well below the relay's 48 KiB line
	// limit.
	consoleTailMaxLineBytes = 40 << 10
)

var (
	errCS2AlreadyRunning = errors.New("cs2.exe is already running")

	cs2ClientVersionPattern = regexp.MustCompile(`^[0-9]{1,12}$`)
	cs2PatchVersionPattern  = regexp.MustCompile(`^[0-9]{1,6}(\.[0-9]{1,6}){0,5}$`)
	hlaeVersionPattern      = regexp.MustCompile(`^[0-9]{1,4}\.[0-9]{1,4}\.[0-9]{1,4}([-+.][0-9A-Za-z][0-9A-Za-z.-]{0,31})?$`)
)

// captureIncompleteError marks a capture whose recorded video does not cover
// the scheduled windows. Its text is the wrapped coverage error unchanged.
type captureIncompleteError struct{ err error }

func (e *captureIncompleteError) Error() string { return e.err.Error() }
func (e *captureIncompleteError) Unwrap() error { return e.err }

// recorderFailureCode maps the recorder's known capture failures to their
// stable internal/obs code, or "" when the failure has none.
func recorderFailureCode(err error) obs.FailureCode {
	var hookErr *hookIncompatibleError
	var verificationErr *captureVerificationError
	var incompleteErr *captureIncompleteError
	switch {
	case errors.As(err, &hookErr):
		return obs.FailureHLAEHookIncompatible
	case errors.Is(err, errCS2AlreadyRunning):
		return obs.FailureCS2AlreadyRunning
	case errors.As(err, &verificationErr) && verificationErr.missingMarker:
		return obs.FailureCapturePOVUnverified
	case errors.As(err, &incompleteErr):
		return obs.FailureCaptureIncomplete
	default:
		return ""
	}
}

// stderrFailureLine is the final "error: ..." line the media worker condenses
// into the job failure reason, led by the failure-code prefix the alerter keys
// on when the failure is known.
func stderrFailureLine(err error) string {
	message := err.Error()
	if code := recorderFailureCode(err); code != "" {
		message = obs.Failure{Code: code, Substage: obs.SubstageCapture}.Prefix() + " " + message
	}
	return "error: " + message + "\n"
}

// captureToolchain is the capture environment of one recorder attempt.
type captureToolchain struct {
	CS2Build string
	CS2Patch string
	HLAE     string
	Encoder  string
}

// Message is the attempt.toolchain record body. Versions carry a "v" prefix so
// the diagnostic filter's IPv4 rule cannot mistake a 4-part version for an
// address; unknown values stay "unknown".
func (t captureToolchain) Message() string {
	return "cs2_build=" + t.CS2Build + " cs2_patch=" + versionLabel(t.CS2Patch) + " hlae=" + versionLabel(t.HLAE) + " encoder=" + t.Encoder
}

func versionLabel(version string) string {
	if version == unknownToolchainValue {
		return version
	}
	return "v" + version
}

func readCaptureToolchain(cs2Exe, hlaeExe, encoder string) captureToolchain {
	build, patch := readCS2SteamInf(cs2SteamInfPath(cs2Exe))
	return captureToolchain{
		CS2Build: build,
		CS2Patch: patch,
		HLAE:     readPinnedHLAEVersion(hlaeExe),
		Encoder:  captureEncoderLabel(encoder),
	}
}

func emitCaptureToolchain(toolchain captureToolchain) {
	obs.EmitTrace(context.Background(), obs.TraceEntry{Event: toolchainTraceEvent, Level: "info", Message: toolchain.Message()})
}

// cs2SteamInfPath resolves game/csgo/steam.inf next to game/bin/win64/cs2.exe,
// the same layout cs2ConsoleLogPath relies on.
func cs2SteamInfPath(cs2Exe string) string {
	gameDir := filepath.Dir(filepath.Dir(filepath.Dir(cs2Exe)))
	return filepath.Join(gameDir, "csgo", "steam.inf")
}

// readCS2SteamInf returns the installed CS2 ClientVersion and PatchVersion,
// each "unknown" when the file or the key is missing or malformed.
func readCS2SteamInf(path string) (build, patch string) {
	build, patch = unknownToolchainValue, unknownToolchainValue
	// #nosec G304 -- path is derived from the explicit local cs2.exe path.
	file, err := os.Open(path)
	if err != nil {
		return build, patch
	}
	defer file.Close()
	scanner := bufio.NewScanner(io.LimitReader(file, 64<<10))
	for scanner.Scan() {
		key, value, ok := strings.Cut(scanner.Text(), "=")
		if !ok {
			continue
		}
		value = strings.TrimSpace(value)
		switch strings.TrimSpace(key) {
		case "ClientVersion":
			if cs2ClientVersionPattern.MatchString(value) {
				build = value
			}
		case "PatchVersion":
			if cs2PatchVersionPattern.MatchString(value) {
				patch = value
			}
		}
	}
	return build, patch
}

// hlaeInstallMarker is the part of Studio's pinned runtime-tool marker
// (.cliphub-install.json next to HLAE.exe) that names the installed version.
const hlaeInstallMarker = ".cliphub-install.json"

// readPinnedHLAEVersion reads the pinned HLAE version Studio installed at the
// ZV_HLAE_PATH the recorder was given. An HLAE without that marker is not the
// Studio pin, so its version is reported as unknown.
func readPinnedHLAEVersion(hlaeExe string) string {
	// #nosec G304 -- the marker sits next to the explicit local HLAE.exe path.
	file, err := os.Open(filepath.Join(filepath.Dir(hlaeExe), hlaeInstallMarker))
	if err != nil {
		return unknownToolchainValue
	}
	defer file.Close()
	var marker struct {
		Version string `json:"version"`
	}
	if json.NewDecoder(io.LimitReader(file, 4<<20)).Decode(&marker) != nil || !hlaeVersionPattern.MatchString(marker.Version) {
		return unknownToolchainValue
	}
	return marker.Version
}

func captureEncoderLabel(encoder string) string {
	switch encoder {
	case recording.EncoderNVENC:
		return "nvenc"
	case recording.EncoderAMF:
		return "amf"
	case recording.EncoderQSV:
		return "qsv"
	case recording.EncoderDefault, recording.EncoderLibx264:
		return "x264"
	default:
		return "other"
	}
}

// emitCS2ConsoleTail forwards the end of this run's CS2 console log after a
// capture failure. The diagnostic filters apply downstream.
func emitCS2ConsoleTail(path string) {
	tail, lines := readCS2ConsoleTail(path)
	if lines == 0 {
		return
	}
	line, err := consoleTailTraceLine(tail, lines, time.Now())
	if err != nil {
		log.Printf("console tail trace encoding failed: %v", err)
		return
	}
	log.Print(line)
}

// readCS2ConsoleTail returns at most the last consoleTailMaxLines lines and
// consoleTailMaxBytes bytes of the console log, plus the line count kept.
func readCS2ConsoleTail(path string) (string, int) {
	// #nosec G304 -- path is the recorder's own CS2 -condebug console log.
	file, err := os.Open(path)
	if err != nil {
		return "", 0
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return "", 0
	}
	offset := max(info.Size()-consoleTailReadBytes, 0)
	data, err := io.ReadAll(io.NewSectionReader(file, offset, info.Size()-offset))
	if err != nil {
		return "", 0
	}
	text := strings.ReplaceAll(string(data), "\r\n", "\n")
	lines := strings.Split(strings.TrimRight(text, "\n"), "\n")
	if offset > 0 && len(lines) > 1 {
		lines = lines[1:] // the first line was cut by the read window
	}
	return boundConsoleTail(lines)
}

func boundConsoleTail(lines []string) (string, int) {
	if len(lines) > consoleTailMaxLines {
		lines = lines[len(lines)-consoleTailMaxLines:]
	}
	for len(lines) > 1 && len(strings.Join(lines, "\n")) > consoleTailMaxBytes {
		lines = lines[1:]
	}
	tail := strings.Join(lines, "\n")
	if len(tail) > consoleTailMaxBytes {
		tail = tail[len(tail)-consoleTailMaxBytes:]
		for len(tail) > 0 && !utf8.RuneStart(tail[0]) {
			tail = tail[1:]
		}
	}
	if strings.TrimSpace(tail) == "" {
		return "", 0
	}
	return tail, len(lines)
}

// consoleTailTraceLine encodes the cs2.console_tail trace as one JSON line,
// dropping the oldest console lines until it fits consoleTailMaxLineBytes. The
// console text stays readable: the media worker's failure marker parsers skip
// diagnostic trace lines, so CS2 console output never classifies a failure.
func consoleTailTraceLine(tail string, lines int, now time.Time) (string, error) {
	parts := strings.Split(tail, "\n")
	for {
		encoded, err := json.Marshal(obs.TraceEntry{
			Time:    now.UTC(),
			Event:   consoleTailTraceEvent,
			Level:   "warn",
			Message: fmt.Sprintf("lines=%d\n%s", lines, strings.Join(parts, "\n")),
		})
		if err != nil {
			return "", err
		}
		line := obs.TracePrefix + string(encoded)
		if len(line) <= consoleTailMaxLineBytes || len(parts) <= 1 {
			return line, nil
		}
		parts, lines = parts[1:], lines-1
	}
}
