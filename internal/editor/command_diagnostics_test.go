package editor

import (
	"bytes"
	"context"
	"io"
	"log"
	"os/exec"
	"regexp"
	"strings"
	"testing"
)

// The Studio 3.0.0 incident: loudnorm refuses a TP target below -9 dBTP. The
// first error line must name the command and the rejected option, not FFmpeg's
// generic "Error opening output files" trailer.
func TestFFmpegFailureOfRejectedLoudnormNamesTheOption(t *testing.T) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg unavailable")
	}
	previous := log.Writer()
	log.SetOutput(io.Discard)
	defer log.SetOutput(previous)
	command := []string{ffmpeg, "-hide_banner", "-nostdin", "-f", "lavfi", "-i", "sine=f=440:d=1", "-af", "loudnorm=I=-14:TP=-10.18:LRA=11", "-f", "null", "-"}
	output, failure := runFFmpegOutput(context.Background(), command, "Full Demo program master")
	if failure == nil || !strings.Contains(output, "Value -10.180000 for parameter 'TP' out of range") {
		t.Fatalf("FFmpeg accepted the out-of-range target: %v\n%s", failure, output)
	}
	first, rest, _ := strings.Cut(failure.Error(), "\n")
	cause := regexp.MustCompile(`^ffmpeg Full Demo program master: exit status (?:0x[0-9a-f]+|\d+): \[[^\]]+\] (?:Error applying option 'TP' to filter 'loudnorm': Result too large|Value -10\.180000 for parameter 'TP' out of range \[-9 - 0\])$`)
	if !cause.MatchString(first) {
		t.Fatalf("first line = %q\ncomplete stderr:\n%s", first, rest)
	}
	if rest != strings.TrimSpace(output) {
		t.Fatalf("complete stderr no longer follows the first line: %q", rest)
	}
}

func TestFFmpegDiagnosticsPreserveCausalOutputWithAndWithoutProgress(t *testing.T) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg unavailable")
	}
	for _, progress := range []bool{false, true} {
		t.Run(map[bool]string{false: "buffered", true: "progress"}[progress], func(t *testing.T) {
			var logs bytes.Buffer
			previous := log.Writer()
			log.SetOutput(&logs)
			defer log.SetOutput(previous)
			command := []string{ffmpeg, "-hide_banner", "-f", "lavfi", "-i", "color=s=16x16:d=0.01", "-c:v", "cliphub_missing_encoder", "-f", "null", "-"}
			var output string
			var failure error
			if progress {
				output, failure = runFFmpegOutputProgress(context.Background(), command, "diagnostic canary", 1, func(float64) {})
			} else {
				output, failure = runFFmpegOutput(context.Background(), command, "diagnostic canary")
			}
			if failure == nil || !strings.Contains(output, "Unknown encoder") {
				t.Fatalf("original output lost: %s %v", output, failure)
			}
			text := logs.String()
			for _, expected := range []string{"tool.started", "process.stderr", "Unknown encoder", "tool.finished", `"outcome":"error"`, `"exit_code":`} {
				if !strings.Contains(text, expected) {
					t.Fatalf("missing %s: %s", expected, text)
				}
			}
			if strings.Index(text, "Unknown encoder") > strings.Index(text, "tool.finished") {
				t.Fatal("causal output arrived after terminal event")
			}
		})
	}
}
