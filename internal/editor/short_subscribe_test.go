package editor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rechedev9/cliphub/internal/demooverlay"
	"github.com/rechedev9/cliphub/internal/htmlstill"
)

func TestSubscribeBannerShowsOnceAThirdIntoTheShort(t *testing.T) {
	subscribe := &ShortSubscribe{Path: "subscribe.png"}
	for _, format := range []string{OutputFormatShort9x16, OutputFormatLandscape16x9} {
		short := ShortEdit{OutputFormat: format, DurationSeconds: 30, Overlays: ShortOverlays{Subscribe: subscribe}}
		effects := generatedBrandEffects(short)
		if len(effects) != 1 {
			t.Fatalf("%s effects = %+v; want one subscribe banner", format, effects)
		}
		e := effects[0]
		w, h := outputDimensions(short)
		if e.Source != shortSubscribeSource || e.StartSeconds != 10.5 || e.EndSeconds != 15 || e.Width != w || h == 0 || !strings.Contains(e.Y, "if(") || e.FadeInSeconds == 0 {
			t.Fatalf("%s banner = %+v; want 4.5 s from 35%% of the Short, full frame, sliding in", format, e)
		}
	}
	late := ShortEdit{OutputFormat: OutputFormatShort9x16, DurationSeconds: 9, Overlays: ShortOverlays{Subscribe: subscribe}}
	if e := generatedBrandEffects(late); len(e) != 1 || e[0].EndSeconds > 9-shortSubscribeMarginSeconds || e[0].StartSeconds < shortSubscribeMarginSeconds {
		t.Fatalf("a short Short = %+v; want the banner clear of both ends", e)
	}
	tiny := ShortEdit{OutputFormat: OutputFormatShort9x16, DurationSeconds: 6, Overlays: ShortOverlays{Subscribe: subscribe}}
	if e := generatedBrandEffects(tiny); len(e) != 0 {
		t.Fatalf("a 6 s Short = %+v; want no banner", e)
	}
	if e := generatedBrandEffects(ShortEdit{OutputFormat: OutputFormatShort9x16, DurationSeconds: 30}); len(e) != 0 {
		t.Fatalf("a Short without the option = %+v", e)
	}
}

func TestSingleClipShortDrawsTheSubscribeBanner(t *testing.T) {
	short := ShortEdit{Input: "clip.mp4", Output: "out.mp4", OutputFormat: OutputFormatShort9x16, DurationSeconds: 20, Overlays: ShortOverlays{Subscribe: &ShortSubscribe{Path: "subscribe.png"}}}
	short.Effects = generatedBrandEffects(short)
	if command := strings.Join(BuildFFmpegCommand("ffmpeg", short), " "); !strings.Contains(command, "-i subscribe.png") {
		t.Fatalf("command = %s; want the subscribe banner as an image input", command)
	}
}

func TestSubscribeBannerNeedsTheRendererAndAOneLineName(t *testing.T) {
	dir := t.TempDir()
	if _, err := prepareShortSubscribe(&SubscribeCard{ChannelName: "a\nb"}, OutputFormatShort9x16, dir); err == nil {
		t.Fatal("a channel name on two lines was accepted")
	}
	t.Setenv("ZV_OVERLAY_RENDERER_PATH", "")
	if _, err := prepareShortSubscribe(&SubscribeCard{ChannelName: "ClipHub"}, OutputFormatShort9x16, dir); err == nil || !strings.Contains(err.Error(), "subscribe banner") {
		t.Fatalf("err = %v; want the render to fail without Chromium", err)
	}
	if got, err := prepareShortSubscribe(nil, OutputFormatShort9x16, dir); got != nil || err != nil {
		t.Fatalf("no card = %+v %v", got, err)
	}
	if _, err := os.Stat(filepath.Join(dir, shortSubscribeSource+".png")); !os.IsNotExist(err) {
		t.Fatalf("a failed render left a banner behind: %v", err)
	}
}

func TestLikeThenSubscribeHoldAndDingInTheCompilation(t *testing.T) {
	sprite := demooverlay.ShortSubscribeSpriteFor(htmlstill.Portrait)
	subscribe := &ShortSubscribe{Path: "subscribe.png", CellTop: sprite.CellTop, CellHeight: sprite.CellHeight, Frames: sprite.Frames}
	short := ShortEdit{Output: "out.mp4", OutputFormat: OutputFormatShort9x16, DurationSeconds: 30, Overlays: ShortOverlays{Subscribe: subscribe},
		Parts: []ShortPart{{SegmentID: "seg-001", Input: "a.mp4", DurationSeconds: 15}, {SegmentID: "seg-002", Input: "b.mp4", DurationSeconds: 15}}}
	short.Effects = generatedBrandEffects(short)
	if e := short.Effects[0]; e.Height != 150*32 || e.Sprite == nil || len(e.Sprite.Clips) != 2 || len(e.Sprite.PlayAt) != 0 {
		t.Fatalf("banner = %+v; want a 32-cell sprite with a like clip and a subscribe clip", e)
	}
	filter := CompilationFilter(short)
	cell := "crop=w=iw:h=150:x=0:y='150*if(gte(t\\,12.400)\\,min(31\\,9+trunc((t-12.400)*20))\\,if(gte(t\\,11.300)\\,min(8\\,1+trunc((t-11.300)*20))\\,0))'"
	if !strings.Contains(filter, cell) {
		t.Fatalf("filter = %s; want the like at 11.3 s and the subscribe at 12.4 s, each holding its last cell", filter)
	}
	if strings.Count(filter, "aevalsrc=") != 1 || !strings.Contains(filter, "adelay=604800S") || !strings.Contains(filter, "[catv][gameraw]") || !strings.Contains(filter, "normalize=0[gamea]") {
		t.Fatalf("filter = %s; want one ding at 12.6 s, on the bell's first swing, mixed into the game audio", filter)
	}
	short.Overlays.Subscribe, short.Effects = nil, nil
	if filter := CompilationFilter(short); strings.Contains(filter, "aevalsrc") || strings.Contains(filter, "gameraw") {
		t.Fatalf("a Short without the banner = %s", filter)
	}
}
