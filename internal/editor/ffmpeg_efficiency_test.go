package editor

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestProgressPipeHandlesChunksAndBoundedPartialLines(t *testing.T) {
	input := "frame=1\nout_time_us=N/A\nout_time_us=-1\nout_time_us=1000000\r\nout_time_us=1000000\nout_time_us=500000\nout_time_us=2500000\nout_time_us=99999999\n"
	for _, chunk := range []int{1, 7, len(input)} {
		var fractions []float64
		writer := &ffmpegProgressWriter{duration: 10, onFraction: func(f float64) { fractions = append(fractions, f) }}
		for start := 0; start < len(input); start += chunk {
			end := min(start+chunk, len(input))
			if n, err := writer.Write([]byte(input[start:end])); err != nil || n != end-start {
				t.Fatal(n, err)
			}
		}
		if !reflect.DeepEqual(fractions, []float64{.1, .25, .99}) {
			t.Fatal(fractions)
		}
	}
	var fractions []float64
	writer := &ffmpegProgressWriter{duration: 10, onFraction: func(f float64) { fractions = append(fractions, f) }}
	for range 100 {
		_, _ = writer.Write([]byte(strings.Repeat("x", 1000)))
	}
	if len(writer.pending) > 4096 {
		t.Fatal("unbounded partial record")
	}
	_, _ = writer.Write([]byte("out_time_us=5000000\nout_time_us=2000000\n"))
	if !reflect.DeepEqual(fractions, []float64{.2}) {
		t.Fatal("oversized suffix was interpreted as progress", fractions)
	}
}

func TestDecodedDeliveryFramesRequiresCompletedUnmodifiedFrames(t *testing.T) {
	for _, tc := range []struct {
		output string
		want   int64
	}{
		{"frame=60\ndup_frames=0\ndrop_frames=0\nprogress=end\n", 60},
		{"frame=1\ndup_frames=0\ndrop_frames=0\nprogress=continue\nframe=120\ndup_frames=0\ndrop_frames=0\nprogress=end\n", 120},
		{"frame=60\nprogress=end\n", 0},
		{"frame=60\ndup_frames=0\ndrop_frames=0\nprogress=continue\n", 0},
		{"frame=60\ndup_frames=0\ndrop_frames=0\nprogress=continue\nprogress=end\n", 0},
		{"frame=60\ndup_frames=1\ndrop_frames=0\nprogress=end\n", 0},
		{"frame=60\ndup_frames=0\ndrop_frames=1\nprogress=end\n", 0},
		{"frame=N/A\ndup_frames=0\ndrop_frames=0\nprogress=end\n", 0},
		{"frame=-1\ndup_frames=0\ndrop_frames=0\nprogress=end\n", 0},
	} {
		got, err := decodedDeliveryFrames(tc.output)
		if tc.want == 0 {
			if err == nil {
				t.Fatalf("accepted %q", tc.output)
			}
			continue
		}
		if err != nil || got != tc.want {
			t.Fatalf("got %d, %v want %d", got, err, tc.want)
		}
	}
}

func TestFFmpegProgressPipeRealEncodeAndCancellation(t *testing.T) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg not installed")
	}
	var fractions []float64
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	command := []string{ffmpeg, "-v", "error", "-f", "lavfi", "-i", "color=s=64x64:r=60:d=1", "-f", "null", "-"}
	if err := runFFmpegOutputWithProgress(ctx, command, "pipe canary", 1, func(f float64) { fractions = append(fractions, f) }); err != nil {
		t.Fatal(err)
	}
	// FFmpeg versions report either the last frame's timestamp (59/60s)
	// or its end (1s). Completion is the successful process exit, not .99.
	if len(fractions) == 0 || fractions[len(fractions)-1] < 59.0/60-1e-6 || fractions[len(fractions)-1] > .99 {
		t.Fatal(fractions)
	}
	for i := 1; i < len(fractions); i++ {
		if fractions[i] <= fractions[i-1] {
			t.Fatal("non-monotonic progress", fractions)
		}
	}
	ctx, cancel = context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	command = []string{ffmpeg, "-v", "error", "-re", "-f", "lavfi", "-i", "color=s=64x64:r=60:d=60", "-f", "null", "-"}
	cancelledOnProgress := false
	if err := runFFmpegOutputWithProgress(ctx, command, "cancel canary", 60, func(float64) {
		cancelledOnProgress = true
		cancel()
	}); err == nil {
		t.Fatal("cancelled encode succeeded")
	}
	if !cancelledOnProgress {
		t.Fatal("no progress callback before cancellation deadline")
	}
}

func TestFullDemoDeliverySingleDecodeRealMedia(t *testing.T) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg not installed")
	}
	ffprobe, err := exec.LookPath("ffprobe")
	if err != nil {
		t.Skip("ffprobe not installed")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	file := filepath.Join(t.TempDir(), "delivery.mp4")
	command := []string{ffmpeg, "-v", "error", "-f", "lavfi", "-i", "color=s=1920x1080:r=60:d=1", "-f", "lavfi", "-i", "anullsrc=r=48000:cl=stereo", "-t", "1", "-c:v", "libx264", "-preset", "ultrafast", "-pix_fmt", "yuv420p", "-c:a", "aac", file}
	if _, err := runFFmpegOutput(ctx, command, "delivery fixture"); err != nil {
		t.Fatal(err)
	}
	var decodeProgress []float64
	evidence, err := verifyFullDemoDelivery(ctx, ffmpeg, ffprobe, file, 60, func(stage string, fraction float64) {
		if stage == "Verificando fotogramas, vídeo y audio" {
			decodeProgress = append(decodeProgress, fraction)
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	if !evidence.FullDecode || evidence.FrameCount != 60 || len(evidence.ContentSHA256) != 64 {
		t.Fatal(evidence)
	}
	if len(decodeProgress) < 2 || decodeProgress[len(decodeProgress)-1] <= decodeProgress[0] {
		t.Fatal("complete decode did not report media progress", decodeProgress)
	}
	if _, err := verifyFullDemoDelivery(ctx, ffmpeg, ffprobe, file, 59, nil); err == nil {
		t.Fatal("wrong canonical count passed")
	}
	// Fast-start makes the container readable even when the final media is cut.
	fast := filepath.Join(t.TempDir(), "truncated.mp4")
	if _, err := runFFmpegOutput(ctx, []string{ffmpeg, "-v", "error", "-i", file, "-c", "copy", "-movflags", "+faststart", fast}, "faststart fixture"); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(fast)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Truncate(fast, info.Size()-500); err != nil {
		t.Fatal(err)
	}
	if _, err := verifyFullDemoDelivery(ctx, ffmpeg, ffprobe, fast, 60, nil); err == nil {
		t.Fatal("truncated media passed complete decode")
	}
}

func BenchmarkProgressPipe(b *testing.B) {
	input := []byte("frame=100\nfps=60\nout_time_us=1000000\nprogress=continue\n")
	writer := &ffmpegProgressWriter{duration: 600, onFraction: func(float64) {}}
	b.ReportAllocs()
	for b.Loop() {
		_, _ = writer.Write(input)
	}
}

func Example_decodedDeliveryFrames() {
	count, err := decodedDeliveryFrames("frame=60\ndup_frames=0\ndrop_frames=0\nprogress=end\n")
	fmt.Println(count, err)
	// Output: 60 <nil>
}
