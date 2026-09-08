package recording

import (
	"context"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// A real B-frame stream and a slightly shorter HLAE-style WAV reproduce the
// frame loss caused by stream copy with -shortest. These are generated signals.
func TestMuxPreservesEveryVideoFrame(t *testing.T) {
	ffmpeg, ffprobe := FindFFmpeg(), FindFFprobe()
	if ffmpeg == "" || ffprobe == "" {
		t.Fatal("FFmpeg and ffprobe are required for the capture mux regression")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	dir := t.TempDir()
	run := func(args ...string) []byte {
		t.Helper()
		output, err := exec.CommandContext(ctx, ffmpeg, args...).CombinedOutput()
		if err != nil {
			t.Fatalf("FFmpeg: %v\n%s", err, output)
		}
		return output
	}
	video := filepath.Join(dir, "video.mp4")
	run("-y", "-v", "error", "-f", "lavfi", "-i", "testsrc2=size=160x90:rate=60",
		"-frames:v", "122", "-c:v", "libx264", "-preset", "medium", "-bf", "3", "-threads", "2", video)
	videoHash := string(run("-v", "error", "-i", video, "-map", "0:v:0", "-f", "framemd5", "-"))
	for _, tc := range []struct{ name, duration string }{
		{"shorter audio", "2.026304"},
		{"longer audio", "2.200000"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			audio, output := filepath.Join(t.TempDir(), "audio.wav"), filepath.Join(t.TempDir(), "segment.mp4")
			run("-y", "-v", "error", "-f", "lavfi", "-i", "anullsrc=r=44100:cl=stereo",
				"-t", tc.duration, "-c:a", "pcm_s16le", audio)
			if err := muxPair(ctx, ffmpeg, video, audio, output); err != nil {
				t.Fatal(err)
			}
			artifact := RecordingArtifact{Path: output}
			ProbeArtifact(ctx, ffprobe, &artifact)
			if artifact.ProbeError != "" || artifact.FrameCount != 122 {
				t.Fatalf("mux must retain all 122 video frames: %+v", artifact)
			}
			if got := string(run("-v", "error", "-i", output, "-map", "0:v:0", "-f", "framemd5", "-")); got != videoHash {
				t.Fatal("mux changed decoded video frames or timestamps")
			}
		})
	}
}
