package editor

import (
	"context"
	"image"
	"image/color"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rechedev9/cliphub/internal/brandkit"
	"github.com/rechedev9/cliphub/internal/recapplan"
	"github.com/rechedev9/cliphub/internal/recording"
)

func writeBrandPNG(t *testing.T, path string, w, h int, c color.NRGBA) {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.SetNRGBA(x, y, c)
		}
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
}

func TestWatermarkPlacementKeepsAspectAndShortsCaptionBand(t *testing.T) {
	for _, tc := range []struct {
		corner             string
		frameW, frameH     int
		wantW, wantH, x, y int
	}{
		{brandkit.CornerTopLeft, 1080, 1920, 172, 86, 43, 43},
		{brandkit.CornerTopRight, 1080, 1920, 172, 86, 1080 - 172 - 43, 43},
		// A 9:16 bottom corner stays above the Shorts player's caption band.
		{brandkit.CornerBottomRight, 1080, 1920, 172, 86, 1080 - 172 - 43, 1920 - 86 - 268},
		{brandkit.CornerBottomLeft, 1920, 1080, 172, 86, 43, 1080 - 86 - 43},
	} {
		w, h, x, y := watermarkPlacement(tc.corner, 40, 20, tc.frameW, tc.frameH)
		if w != tc.wantW || h != tc.wantH || x != tc.x || y != tc.y {
			t.Errorf("%s %dx%d = %dx%d at %d,%d; want %dx%d at %d,%d", tc.corner, tc.frameW, tc.frameH, w, h, x, y, tc.wantW, tc.wantH, tc.x, tc.y)
		}
	}
	if w, h := fitBox(1000, 1000, 172, 108); w != 108 || h != 108 {
		t.Fatalf("square logo = %dx%d; want it bound by the height", w, h)
	}
}

func TestPrepareBrandOverlaysBakesOpacityAndDrawsShortEffects(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "logo.media")
	writeBrandPNG(t, src, 40, 20, color.NRGBA{R: 255, A: 200})
	wm, banner, err := prepareBrandOverlays(
		&BrandWatermark{Path: src, Corner: brandkit.CornerTopLeft, Opacity: 0.5},
		&SponsorBanner{Path: src, StartSeconds: 2, EndSeconds: 30},
		filepath.Join(dir, "out"))
	if err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(wm.Path)
	if err != nil {
		t.Fatal(err)
	}
	baked, err := png.Decode(f)
	_ = f.Close()
	if err != nil {
		t.Fatal(err)
	}
	if got := color.NRGBAModel.Convert(baked.At(3, 3)).(color.NRGBA); got.A != 100 || got.R != 255 || wm.Width != 40 || wm.Height != 20 {
		t.Fatalf("baked pixel = %+v size %dx%d; want alpha 200 halved and the source size", got, wm.Width, wm.Height)
	}
	short := ShortEdit{OutputFormat: OutputFormatShort9x16, DurationSeconds: 12, Overlays: ShortOverlays{Watermark: wm, Banner: banner}}
	effects := generatedBrandEffects(short)
	if len(effects) != 2 || effects[0].Source != "brand-watermark" || effects[0].EndSeconds != 12 || effects[0].FadeInSeconds == 0 {
		t.Fatalf("effects = %+v; want a watermark over the whole Short", effects)
	}
	if b := effects[1]; b.Source != "brand-banner" || b.StartSeconds != 2 || b.EndSeconds != 12 || b.Width != 460 || b.Height != 230 {
		t.Fatalf("banner = %+v; want it clamped to the Short and fitted to 80%% x 12%%", b)
	}
	short.DurationSeconds = 1.5
	if effects := generatedBrandEffects(short); len(effects) != 1 {
		t.Fatalf("a banner starting after the Short = %+v; want only the watermark", effects)
	}
	if _, _, err := prepareBrandOverlays(&BrandWatermark{Path: src, Corner: "center", Opacity: 1}, nil, dir); err == nil {
		t.Fatal("accepted an unknown corner")
	}
}

func TestFullDemoBrandOverlaysShiftTheBannerOntoTheItemClock(t *testing.T) {
	dir := t.TempDir()
	logo := filepath.Join(dir, "logo.media")
	writeBrandPNG(t, logo, 40, 20, color.NRGBA{R: 255, A: 255})
	ref := brandkit.ImageRef{ID: "33333333-3333-4333-8333-333333333333", SHA256: strings.Repeat("b", 64)}
	options := recapplan.DefaultOptions()
	options.Overlays.Brand = recapplan.BrandOverlay{
		Watermark: &brandkit.Watermark{Image: ref, Corner: brandkit.CornerTopRight, Opacity: 0.6},
		Banner:    &brandkit.Banner{Image: ref, StartSeconds: 11, EndSeconds: 14},
	}
	short := ShortEdit{FullDemo: &FullDemoRenderEvidence{Effective: recapplan.Document{Options: options}},
		fullDemo: &fullDemoRenderContext{execution: FullDemoExecution{Assets: []FullDemoLocalMedia{{Ref: recapplan.AssetRef{ID: ref.ID, SHA256: ref.SHA256}, Path: logo}}}}}
	item := recapplan.TimelineItem{Role: "round", StartFrame: 600, EndFrame: 1200}
	command, video, err := fullDemoBrandOverlays(short, item, []string{"ffmpeg", "-i", "clip"}, "[0:v]null")
	if err != nil {
		t.Fatal(err)
	}
	if len(command) != 7 || !strings.Contains(video, "colorchannelmixer=aa=0.600") || !strings.Contains(video, "x=1705:y=43") || !strings.Contains(video, "between(t\\,1.000\\,4.000)") {
		t.Fatalf("command %v\nvideo %s", command, video)
	}
	if command, _, _ := fullDemoBrandOverlays(short, recapplan.TimelineItem{Role: "round", StartFrame: 0, EndFrame: 600}, []string{"ffmpeg"}, ""); len(command) != 3 {
		t.Fatalf("an item before the banner window = %v; want only the watermark input", command)
	}
	if command, video, _ := fullDemoBrandOverlays(short, recapplan.TimelineItem{Role: "bumper", StartFrame: 600, EndFrame: 1200}, []string{"ffmpeg"}, "v"); len(command) != 1 || video != "v" {
		t.Fatal("drew the brand over the creator's own bumper clip")
	}
	short.fullDemo.execution.Assets = nil
	if _, _, err := fullDemoBrandOverlays(short, item, nil, ""); err == nil {
		t.Fatal("a missing approved brand image was silently left out")
	}
}

func TestFullDemoBrandOverlaysRender(t *testing.T) {
	mediaTest(t)
	ffmpeg := fullDemoTestFFmpeg(t)
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	dir := t.TempDir()
	source, logo, banner := filepath.Join(dir, "source.nut"), filepath.Join(dir, "logo.media"), filepath.Join(dir, "banner.media")
	if _, err := runFFmpegOutput(ctx, []string{ffmpeg, "-y", "-v", "error", "-f", "lavfi", "-i", "color=black:s=320x180:r=60:d=0.5", "-f", "lavfi", "-i", "anullsrc=r=48000:cl=stereo:d=0.5", "-c:v", "libx264", "-preset", "ultrafast", "-c:a", "pcm_f32le", source}, "brand fixture"); err != nil {
		t.Fatal(err)
	}
	writeBrandPNG(t, logo, 40, 20, color.NRGBA{R: 255, A: 255})
	writeBrandPNG(t, banner, 40, 10, color.NRGBA{G: 255, A: 255})
	logoRef := recapplan.AssetRef{ID: "44444444-4444-4444-8444-444444444444", SHA256: strings.Repeat("c", 64)}
	bannerRef := recapplan.AssetRef{ID: "55555555-5555-4555-8555-555555555555", SHA256: strings.Repeat("d", 64)}
	options := recapplan.DefaultOptions()
	options.Audio.Voice.Enabled = false
	options.Overlays.HUDTheme = ""
	options.Transitions.Enabled = false
	options.Overlays.Brand = recapplan.BrandOverlay{
		Watermark: &brandkit.Watermark{Image: brandkit.ImageRef{ID: logoRef.ID, SHA256: logoRef.SHA256}, Corner: brandkit.CornerTopLeft, Opacity: 0.5},
		Banner:    &brandkit.Banner{Image: brandkit.ImageRef{ID: bannerRef.ID, SHA256: bannerRef.SHA256}, StartSeconds: 0.25},
	}
	item := recapplan.TimelineItem{Role: "round", SourceRef: "a", EndFrame: 30, EndSample: 24000}
	d := recapplan.Document{Clock: recapplan.Clock{TickRate: 60}, Options: options, Timeline: []recapplan.TimelineItem{item}}
	short := ShortEdit{Preset: PresetGameplayPOV60, OutputFormat: OutputFormatLandscape16x9, OutputFPS: 60, VideoCRF: 18, VideoPreset: "ultrafast", Threads: 2, Parts: []ShortPart{{SegmentID: "a", Input: source}}, FullDemo: &FullDemoRenderEvidence{Effective: d}, fullDemo: &fullDemoRenderContext{ffmpeg: ffmpeg, execution: FullDemoExecution{Assets: []FullDemoLocalMedia{{Ref: logoRef, Path: logo}, {Ref: bannerRef, Path: banner}}}, recording: recording.RecordingResult{Plan: recording.RecordingPlan{Segments: []recording.RecordingSegment{{ID: "a", TickStart: 0}}}}}}
	output := filepath.Join(dir, "output.nut")
	command, err := fullDemoItemVideoCommand(short, item, output)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := runFFmpegOutput(ctx, command, "brand composition"); err != nil {
		t.Fatal(err)
	}
	pixels := func(crop string) []byte {
		data, err := exec.CommandContext(ctx, ffmpeg, "-v", "error", "-i", output, "-vf", "format=rgb24,crop="+crop, "-pix_fmt", "rgb24", "-f", "rawvideo", "pipe:1").Output()
		if err != nil || len(data) != 30*3 {
			t.Fatalf("frame count: %d %v", len(data)/3, err)
		}
		return data
	}
	for i, data := 0, pixels("1:1:100:80"); i < len(data); i += 3 {
		if data[i] < 100 || data[i] > 160 || data[i+1] > 30 {
			t.Fatalf("frame %d watermark pixel = %v; want red at half opacity", i/3, data[i:i+3])
		}
	}
	for i, data := 0, pixels("1:1:960:270"); i < len(data); i += 3 {
		green := data[i+1] > 200
		if frame := i / 3; green != (frame >= 15) {
			t.Fatalf("frame %d banner green = %v; want it from 0.25 s only", frame, green)
		}
	}
}

func TestSingleClipShortDrawsOnlyTheBrandImages(t *testing.T) {
	short := ShortEdit{Input: "clip.mp4", Output: "out.mp4", OutputFormat: OutputFormatShort9x16, DurationSeconds: 8, Effects: []Effect{
		{Type: EffectImage, Path: "sticker.png", StartSeconds: 0, EndSeconds: 8, FadeInSeconds: 0.2, Source: "effects-script"},
		{Type: EffectImage, Path: "wm.png", X: "43", Y: "43", Width: 172, Height: 86, StartSeconds: 0, EndSeconds: 8, FadeInSeconds: 0.3, Source: "brand-watermark"},
	}}
	for _, music := range []string{"", "music.m4a"} {
		short.MusicPath = music
		command := strings.Join(BuildFFmpegCommand("ffmpeg", short), " ")
		want := "-i wm.png"
		index := "[1:v]"
		if music != "" {
			index = "[2:v]"
		}
		if !strings.Contains(command, want) || strings.Contains(command, "sticker.png") || !strings.Contains(command, index+"format=rgba,scale=w=172:h=86") {
			t.Fatalf("music %q command = %s; want only the brand image as input %s", music, command, index)
		}
	}
}
