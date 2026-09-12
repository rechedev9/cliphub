package editor

import (
	"bytes"
	"context"
	"log"
	"os/exec"
	"strings"
	"testing"
)

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
