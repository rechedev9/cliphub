package replayvideo

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestEncodeRealFFmpeg encodes three stretches at speed 2 with a generated voice track and reads the MP4 back.
func TestEncodeRealFFmpeg(t *testing.T) {
	if testing.Short() {
		t.Skip("encodes with FFmpeg")
	}
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Fatal("FFmpeg is required for the Replay video encode:", err)
	}
	ffprobe, err := exec.LookPath("ffprobe")
	if err != nil {
		t.Fatal("ffprobe is required for the Replay video encode:", err)
	}
	dir := t.TempDir()
	voice := filepath.Join(dir, "voice.ogg")
	if out, err := exec.Command(ffmpeg, "-hide_banner", "-loglevel", "error", "-f", "lavfi", "-i", "sine=frequency=440:duration=30",
		"-ac", "1", "-c:a", "libopus", "-b:a", "32k", voice).CombinedOutput(); err != nil {
		t.Fatalf("make voice: %v: %s", err, out)
	}
	// At speed 2 each 4 s of demo is 2 s of video; the middle stretch falls before the recording,
	// and the graph goes through its file, as a long playlist's does.
	defer func(limit int) { inlineGraphLimit = limit }(inlineGraphLimit)
	inlineGraphLimit = 0
	p := Plan{FPS: 24, Width: 64, Height: 64, Speed: 2, Out: filepath.Join(dir, "out.mp4"),
		Segments: []Segment{
			{Frames: 48, Voice: &Range{FromTick: 640, ToTick: 896}},
			{Frames: 48, Voice: &Range{FromTick: 0, ToTick: 256}},
			{Frames: 48, Voice: &Range{FromTick: 1280, ToTick: 1536}},
		},
		Sync: Sync{Tickrate: 64, OffsetMS: 5000}, Tracks: []Track{{Path: voice, DurationMS: 30_000}}}
	sink, err := Encoder{FFmpeg: ffmpeg}.Start(context.Background(), p)
	if err != nil {
		t.Fatal(err)
	}
	if err := sink.Frame(bytes.NewReader(jpegOf(t, image.NewRGBA(image.Rect(0, 0, 32, 32))))); !errors.Is(err, ErrFrameSize) {
		t.Fatalf("a 32 px frame = %v, want ErrFrameSize", err)
	}
	img := image.NewRGBA(image.Rect(0, 0, 64, 64))
	for i := range 144 {
		for px := range img.Pix {
			img.Pix[px] = uint8(i * 10)
		}
		img.Set(i%64, i%64, color.White)
		var buf bytes.Buffer
		if err := jpeg.Encode(&buf, img, nil); err != nil {
			t.Fatal(err)
		}
		if err := sink.Frame(&buf); err != nil {
			t.Fatalf("frame %d: %v", i, err)
		}
	}
	if voiced, err := sink.Finish(); err != nil || !voiced {
		t.Fatalf("Finish voice=%v err=%v", voiced, err)
	}
	out, err := exec.Command(ffprobe, "-v", "error", "-show_entries", "stream=codec_name:format=duration", "-of", "csv=p=0", p.Out).Output()
	if err != nil {
		t.Fatal(err)
	}
	got := strings.Fields(strings.ReplaceAll(string(out), ",", " "))
	if len(got) < 3 || got[0] != "h264" || got[1] != "aac" || !strings.HasPrefix(got[2], "6.0") {
		t.Fatalf("ffprobe = %q, want h264, aac and about 6 s", out)
	}
	if _, err := os.Stat(p.Out + ".filter"); !os.IsNotExist(err) {
		t.Fatalf("the graph file stayed: %v", err)
	}
}

func jpegOf(t *testing.T, img image.Image) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, nil); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestFrameRefusesNonJPEG(t *testing.T) {
	r := &run{}
	if err := r.Frame(strings.NewReader("GIF89a")); err != ErrNotJPEG {
		t.Fatalf("Frame = %v, want ErrNotJPEG", err)
	}
}
