package timelinerender

import (
	"strings"
	"testing"

	"github.com/rechedev9/cliphub/internal/timelineplan"
)

func TestBuildFFmpegArgs(t *testing.T) {
	t.Parallel()
	assetA := "11111111-1111-1111-1111-111111111111"
	assetB := "22222222-2222-2222-2222-222222222222"
	vol := 0.5
	end := 2.0
	doc := timelineplan.Document{
		SchemaVersion: timelineplan.SchemaVersion,
		Canvas:        timelineplan.Canvas{Width: 1080, Height: 1920, FPS: 60},
		Tracks: []timelineplan.Track{
			{
				ID:   "v1",
				Kind: timelineplan.KindVideo,
				Items: []timelineplan.Item{{
					ID: "base", AssetID: assetA,
					SourceIn: 0, SourceOut: 2,
					Filter: timelineplan.FilterGrade,
				}},
			},
			{
				ID:   "v2",
				Kind: timelineplan.KindVideo,
				Items: []timelineplan.Item{{
					ID: "pip", AssetID: assetB,
					TimelineStart: 0.5, SourceIn: 0, SourceOut: 1, Speed: 2,
					Volume:    &vol,
					Transform: &timelineplan.Transform{X: 0.6, Y: 0.05, Width: 0.35, Height: 0.25},
				}},
			},
		},
		Overlays: []timelineplan.TextOverlay{{
			ID: "title", Text: "ACE", PositionY: 0.1, StartSeconds: 0, EndSeconds: &end,
		}},
		Music: timelineplan.MusicPlan{Key: "concrete-teeth", Volume: 0.25},
	}
	in := Inputs{
		Assets: map[string]AssetInput{
			assetA: {Path: `C:\media\a.mp4`, HasAudio: true},
			assetB: {Path: `C:\media\b.mp4`, HasAudio: true},
		},
		OutputPath:       `C:\out\final.mp4`,
		MusicPath:        `C:\music\track.mp3`,
		FontPath:         `C:\fonts\Montserrat-ExtraBold.ttf`,
		TextOverlayPaths: []string{`C:\tmp\overlay-0.txt`},
	}

	args, err := BuildFFmpegArgs(in, doc)
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(args, " ")
	cases := []struct {
		name string
		want string
	}{
		{name: "two video inputs", want: "-i C:\\media\\a.mp4 -i C:\\media\\b.mp4"},
		{name: "looped music", want: "-stream_loop -1 -i C:\\music\\track.mp3"},
		{name: "black canvas", want: "color=c=black:s=1080x1920"},
		{name: "grade", want: gradeFilter},
		{name: "pip overlay", want: "overlay=x=1080*0.600000:y=1920*0.050000"},
		{name: "drawtext file", want: "expansion=none"},
		{name: "speed atempo", want: "atempo=2.000000"},
		{name: "adelay pip", want: "adelay=500:all=1"},
		{name: "amix three", want: "amix=inputs=3"},
		{name: "h264 profile", want: "-c:v libx264 -preset slow -crf 18"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if !strings.Contains(joined, tc.want) {
				t.Fatalf("args missing %q\n%s", tc.want, joined)
			}
		})
	}
}

func TestBuildFFmpegArgsRequiresAssets(t *testing.T) {
	t.Parallel()
	doc := timelineplan.DefaultDocument()
	doc.Tracks[0].Items = []timelineplan.Item{{
		ID: "x", AssetID: "11111111-1111-1111-1111-111111111111", SourceOut: 1,
	}}
	_, err := BuildFFmpegArgs(Inputs{OutputPath: "out.mp4"}, doc)
	if err == nil || !strings.Contains(err.Error(), "missing media") {
		t.Fatalf("err = %v, want missing media", err)
	}
}
