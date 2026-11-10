package replayvideo

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/jpeg"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestEncodeRealFFmpeg encodes two seconds of frames with a generated voice track and reads the MP4 back.
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
	p := Plan{FPS: 24, Width: 64, Height: 64, Speed: 1, Out: filepath.Join(dir, "out.mp4"),
		// The second second falls before the recording, so it is a silent segment in the concat.
		Segments: []Segment{{Frames: 24, Voice: &Range{FromTick: 640, ToTick: 704}}, {Frames: 24, Voice: &Range{FromTick: 0, ToTick: 64}}},
		Sync:     Sync{Tickrate: 64, OffsetMS: 5000}, Tracks: []Track{{Path: voice, DurationMS: 30_000}}}
	sink, err := Encoder{FFmpeg: ffmpeg}.Start(context.Background(), p)
	if err != nil {
		t.Fatal(err)
	}
	img := image.NewRGBA(image.Rect(0, 0, 64, 64))
	for i := range 48 {
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
	if len(got) < 3 || got[0] != "h264" || got[1] != "aac" || !strings.HasPrefix(got[2], "2.0") {
		t.Fatalf("ffprobe = %q, want h264, aac and about 2 s", out)
	}
}

func TestFrameRefusesNonJPEG(t *testing.T) {
	r := &run{}
	if err := r.Frame(strings.NewReader("GIF89a")); err != ErrNotJPEG {
		t.Fatalf("Frame = %v, want ErrNotJPEG", err)
	}
}
