package vodplays

import (
	"context"
	"image"
	"math"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestFindNoticesTracksWhereAndWhenTheNoticeSat(t *testing.T) {
	if testing.Short() {
		t.Skip("decodes video")
	}
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg not on PATH")
	}
	source := filepath.Join(t.TempDir(), "clip.mp4")
	// A panning picture with, from 1 s to 5 s, a highlighted notice blended
	// over it the way CS2 draws its HUD: a 60% black plate with a red border.
	notice := "color=c=black@0.6:size=402x32:rate=30:duration=6,format=rgba," +
		"drawbox=x=0:y=0:w=402:h=32:color=red:t=3:replace=1"
	cmd := exec.Command(ffmpeg, "-v", "error",
		"-f", "lavfi", "-i", "testsrc2=size=1920x1080:rate=30:duration=6",
		"-f", "lavfi", "-i", notice,
		"-filter_complex", "[0:v]scroll=h=0.013:v=0.009,format=rgb24[bg];[bg][1:v]overlay=x=1500:y=74:format=rgb:enable='between(t,1,5)',format=yuv420p",
		"-c:v", "libx264", "-preset", "ultrafast", "-crf", "12", source)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("make clip: %v: %s", err, out)
	}
	spots, err := FindNotices(context.Background(), NoticeOptions{FFmpeg: ffmpeg, Input: source, StartSeconds: 0, EndSeconds: 6})
	if err != nil {
		t.Fatal(err)
	}
	if len(spots) == 0 {
		t.Fatal("no notice found")
	}
	// Red scenery next to the notice can add a short second track on the same row.
	s := spots[0]
	for _, o := range spots[1:] {
		if o.To-o.From > s.To-s.From {
			s = o
		}
	}
	if s.From < 0.9 || s.From > 1.2 || s.To < 4.8 || s.To > 5.2 {
		t.Fatalf("on screen %.2f to %.2f, want about 1 to 5", s.From, s.To)
	}
	x, y, w, h := s.SourceArea()
	if math.Abs(x*1920-1500) > 6 || math.Abs(y*1080-74) > 6 || math.Abs(w*1920-402) > 12 || math.Abs(h*1080-32) > 12 {
		t.Fatalf("area = %.0f,%.0f %.0fx%.0f px, want about 1500,74 402x32", x*1920, y*1080, w*1920, h*1080)
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
