package vodplays

import (
	"context"
	"image"
	"image/png"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestExtractNoticesRecoversThePlateOpacityAndTheOpaqueText(t *testing.T) {
	if testing.Short() {
		t.Skip("decodes video")
	}
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg not on PATH")
	}
	dir := t.TempDir()
	source := filepath.Join(dir, "clip.mp4")
	// A panning picture with, from 1 s to 5 s, a notice blended over it in RGB
	// the way CS2 draws its HUD: a 60% black plate, a red border and an opaque
	// white mark where a name would be.
	notice := "color=c=black@0.6:size=402x32:rate=30:duration=6,format=rgba," +
		"drawbox=x=0:y=0:w=402:h=32:color=red:t=3:replace=1,drawbox=x=40:y=10:w=40:h=12:color=white:t=fill:replace=1"
	cmd := exec.Command(ffmpeg, "-v", "error",
		"-f", "lavfi", "-i", "testsrc2=size=1920x1080:rate=30:duration=6",
		"-f", "lavfi", "-i", notice,
		"-filter_complex", "[0:v]scroll=h=0.013:v=0.009,format=rgb24[bg];[bg][1:v]overlay=x=1500:y=74:format=rgb:enable='between(t,1,5)',format=yuv420p",
		"-c:v", "libx264", "-preset", "ultrafast", "-crf", "12", source)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("make clip: %v: %s", err, out)
	}
	sprites, err := ExtractNotices(context.Background(), NoticeOptions{FFmpeg: ffmpeg, Input: source, StartSeconds: 0, EndSeconds: 6, WorkDir: dir})
	if err != nil {
		t.Fatal(err)
	}
	if len(sprites) != 1 {
		t.Fatalf("sprites = %+v, want one notice", sprites)
	}
	s := sprites[0]
	if s.AppearSeconds < 0.9 || s.AppearSeconds > 1.2 || s.DisappearSeconds < 4.8 || s.DisappearSeconds > 5.2 {
		t.Fatalf("on screen %.2f to %.2f, want about 1 to 5", s.AppearSeconds, s.DisappearSeconds)
	}
	f, err := os.Open(s.Path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		t.Fatal(err)
	}
	// Sprite pixels are the notice bounds (x from 1500 in the frame) plus the pad.
	text := img.At(1560-1500+noticePad, 90-74+noticePad)
	plate := img.At(1700-1500+noticePad, 90-74+noticePad)
	_, _, _, textA := text.RGBA()
	tr, _, _, _ := text.RGBA()
	_, _, _, plateA := plate.RGBA()
	if textA>>8 < 230 || tr>>8 < 200 {
		t.Fatalf("text pixel = %v, want opaque white", text)
	}
	if a := float64(plateA>>8) / 255; a < 0.54 || a > 0.66 {
		t.Fatalf("plate opacity = %.2f, want 0.6", a)
	}
}

func TestNoticeSourceAreaMapsTheFeedCornerToSourceFractions(t *testing.T) {
	x, y, w, h := NoticeSpot{Area: image.Rect(0, 0, FeedWidth, FeedHeight)}.SourceArea()
	if x != 0.6 || y != 0 || math.Abs(w-0.4) > 1e-9 || math.Abs(h-0.3) > 1e-9 {
		t.Fatalf("whole corner = %v %v %v %v, want 0.6 0 0.4 0.3", x, y, w, h)
	}
}

func TestNoticeTrackStartsANewSpotWhenTheNoticeChangesRow(t *testing.T) {
	tr := &noticeTrack{}
	tr.addSpot(image.Rect(500, 40, 760, 75), 1)
	tr.addSpot(image.Rect(500, 41, 760, 76), 2)
	tr.addSpot(image.Rect(500, 5, 760, 40), 3)
	if len(tr.spots) != 2 || tr.spots[0].From != 1 || tr.spots[1].From != 3 || tr.spots[0].To >= 3 {
		t.Fatalf("spots = %+v, want one per row with its own times", tr.spots)
	}
}
