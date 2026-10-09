package recording

import (
	"context"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Generated 60 fps clips encoded like an HLAE libx264 capture. The frozen ones
// reproduce the 2026-10-09 capture whose frame count was exactly as planned.
func TestFrozenPictureDetection(t *testing.T) {
	ffmpeg := FindFFmpeg()
	if ffmpeg == "" {
		t.Fatal("FFmpeg is required for the frozen picture regression")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	dir := t.TempDir()
	encode := func(name, source string) string {
		t.Helper()
		path := filepath.Join(dir, name+".mp4")
		output, err := exec.CommandContext(ctx, ffmpeg, "-y", "-v", "error", "-f", "lavfi", "-i", source,
			"-c:v", "libx264", "-preset", "fast", "-crf", "18", "-pix_fmt", "yuv420p", path).CombinedOutput()
		if err != nil {
			t.Fatalf("FFmpeg: %v\n%s", err, output)
		}
		return path
	}
	const moving = "testsrc2=size=320x180:rate=60:duration=3"
	clips := map[string]string{
		"moving": encode("moving", moving),
		// The picture stops for the last second while the frame count stays 180.
		"frozen-tail": encode("frozen-tail", "testsrc2=size=320x180:rate=60:duration=2,tpad=stop_mode=clone:stop_duration=1"),
		// Upstream shape: frozen after the round ends, moving again at the respawn.
		"frozen-then-resumes": encode("frozen-then-resumes", moving+",loop=loop=45:size=1:start=90"),
		"short-hitch":         encode("short-hitch", moving+",loop=loop=6:size=1:start=90"),
		// A held angle: nothing moves except a 4x4 pixel marker.
		"low-motion": encode("low-motion", "color=c=gray:size=320x180:rate=60:duration=3[bg];color=c=white:size=4x4:rate=60:duration=3[dot];[bg][dot]overlay=x='mod(n,60)':y=20"),
	}

	for _, tc := range []struct {
		clip                   string
		wantStart, wantSeconds float64
	}{
		{clip: "moving"},
		{clip: "low-motion"},
		{clip: "short-hitch"},
		{clip: "frozen-tail", wantStart: 2.0, wantSeconds: 1.0},
		{clip: "frozen-then-resumes", wantStart: 1.5, wantSeconds: 0.75},
	} {
		t.Run(tc.clip, func(t *testing.T) {
			runs, err := DetectFrozenRuns(ctx, ffmpeg, clips[tc.clip])
			if err != nil {
				t.Fatal(err)
			}
			if tc.wantSeconds == 0 {
				if len(runs) != 0 {
					t.Fatalf("frozen runs = %+v, want none", runs)
				}
				return
			}
			if len(runs) != 1 {
				t.Fatalf("frozen runs = %+v, want exactly one", runs)
			}
			// The encoder keeps refining a still picture, which can hide its first frames.
			const refinement = 0.25
			run := runs[0]
			end := run.StartSeconds + run.Seconds
			if run.StartSeconds < tc.wantStart || run.StartSeconds > tc.wantStart+refinement ||
				math.Abs(end-(tc.wantStart+tc.wantSeconds)) > 0.05 || run.Frames != int(math.Round(run.Seconds*60)) {
				t.Fatalf("frozen run = %+v, want about %.2fs from %.2fs", run, tc.wantSeconds, tc.wantStart)
			}
		})
	}

	t.Run("warnings name frozen segment clips only", func(t *testing.T) {
		unreadable := filepath.Join(dir, "unreadable.mp4")
		if err := os.WriteFile(unreadable, []byte("not a video"), 0o600); err != nil {
			t.Fatal(err)
		}
		artifacts := []RecordingArtifact{
			{SegmentID: "seg-001", Role: "raw", Type: "video", Path: clips["frozen-tail"], SizeBytes: 1},
			{SegmentID: "seg-001", Role: "segment", Type: "video", Path: clips["frozen-tail"], SizeBytes: 1},
			{SegmentID: "seg-002", Role: "segment", Type: "video", Path: clips["moving"], SizeBytes: 1},
			{SegmentID: "seg-003", Role: "segment", Type: "video", Path: unreadable, SizeBytes: 1},
		}
		warnings := FrozenPictureWarnings(ctx, ffmpeg, artifacts)
		if len(warnings) != 2 || !strings.HasPrefix(warnings[0], "segment seg-001 picture frozen for ") ||
			!strings.HasPrefix(warnings[1], "segment seg-003 frozen picture check failed: ") {
			t.Fatalf("warnings = %q, want one frozen run for seg-001 and one failed check for seg-003", warnings)
		}
		if got := FrozenPictureWarnings(ctx, "", artifacts); len(got) != 1 || !strings.Contains(got[0], "ffmpeg not found") {
			t.Fatalf("warnings without FFmpeg = %q, want the skipped check reported", got)
		}
	})
}
