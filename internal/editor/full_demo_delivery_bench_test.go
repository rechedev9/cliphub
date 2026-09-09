package editor

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// Same benchmark can be copied unchanged into a baseline checkout. The fixture
// is synthetic, generated outside the timed section; this is verification cost,
// not capture time or a complete Full Demo export benchmark.
func BenchmarkFullDemoDelivery(b *testing.B) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		b.Skip("ffmpeg not installed")
	}
	ffprobe, err := exec.LookPath("ffprobe")
	if err != nil {
		b.Skip("ffprobe not installed")
	}
	file := os.Getenv("FULL_DEMO_DELIVERY_BENCH_MP4")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	if file == "" {
		file = filepath.Join(b.TempDir(), "delivery.mp4")
		command := []string{ffmpeg, "-v", "error", "-f", "lavfi", "-i", "testsrc2=s=1920x1080:r=60:d=5", "-f", "lavfi", "-i", "sine=f=440:r=48000:d=5", "-map", "0:v:0", "-map", "1:a:0", "-c:v", "libx264", "-preset", "ultrafast", "-pix_fmt", "yuv420p", "-c:a", "aac", "-ac", "2", file}
		if _, err := runFFmpegOutput(ctx, command, "delivery benchmark fixture"); err != nil {
			b.Fatal(err)
		}
	}
	b.ReportAllocs()
	for b.Loop() {
		evidence, err := verifyFullDemoDelivery(ctx, ffmpeg, ffprobe, file, 300)
		if err != nil || !evidence.FullDecode || evidence.FrameCount != 300 {
			b.Fatalf("delivery: %+v %v", evidence, err)
		}
	}
}
