package editor

import (
	"path/filepath"

	"github.com/rechedev9/cliphub/internal/demooverlay"
	"github.com/rechedev9/cliphub/internal/radaroverlay"
	"github.com/rechedev9/cliphub/internal/recapplan"
)

const fullDemoHookSource = "full-demo-hook"

func fullDemoLeadIn(item recapplan.TimelineItem) bool {
	return item.Role == "bumper" || item.IsHook()
}

func fullDemoHookItem(d recapplan.Document) (recapplan.TimelineItem, bool) {
	for _, item := range d.Timeline {
		if item.IsHook() {
			return item, true
		}
	}
	return recapplan.TimelineItem{}, false
}

func fullDemoHookOverlayEffects(manifest *Manifest, short ShortEdit) ([]Effect, error) {
	item, ok := fullDemoHookItem(short.FullDemo.Effective)
	if !ok {
		return nil, nil
	}
	path := filepath.Join(manifest.OutputDir, fullDemoHookSource+".png")
	if err := demooverlay.RenderHookPNG(demooverlay.DesignPalette(short.FullDemo.Effective.Options.Overlays), path); err != nil {
		return nil, err
	}
	return []Effect{fullDemoHookEffect(path, item)}, nil
}

func fullDemoHookEffect(path string, item recapplan.TimelineItem) Effect {
	start, last := fullDemoRoundItemSeconds(item)
	window := fullDemoStoryWindow{start, last}
	return Effect{
		Type: EffectImage, Path: path, Source: fullDemoHookSource,
		X: fullDemoStorySlide(window, -demooverlay.HookSlidePixels, 0), Y: "0",
		Width: demooverlay.FrameWidth, Height: demooverlay.FrameHeight,
		StartSeconds: window.start, EndSeconds: window.end,
		FadeInSeconds: demooverlay.IntroOverlaySlideSeconds, FadeOutSeconds: demooverlay.IntroOverlaySlideOutSeconds,
	}
}

func fullDemoRadarLevel(item recapplan.TimelineItem, track radaroverlay.Track) string {
	segment := track.Segments[0]
	if !item.IsHook() {
		return segment.POVLevel(track.Calibration)
	}
	rate := int64(track.TickRate)
	first := item.SourceStartTick + int(item.SourceOffsetFrames*rate/recapplan.OutputFPS)
	last := first + int((item.EndFrame-item.StartFrame)*rate/recapplan.OutputFPS)
	shown := segment
	shown.Frames = nil
	for _, frame := range segment.Frames {
		if frame.Tick >= first && frame.Tick <= last {
			shown.Frames = append(shown.Frames, frame)
		}
	}
	return shown.POVLevel(track.Calibration)
}
