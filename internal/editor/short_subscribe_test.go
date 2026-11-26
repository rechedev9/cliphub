package editor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
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
		if e.Source != shortSubscribeSource || e.StartSeconds != 10.5 || e.EndSeconds != 15 || e.Width != w || e.Height != h || !strings.Contains(e.Y, "if(") || e.FadeInSeconds == 0 {
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
