package editor

import (
	"math"
	"path/filepath"
	"strconv"

	"github.com/rechedev9/cliphub/internal/brandkit"
	"github.com/rechedev9/cliphub/internal/demooverlay"
	"github.com/rechedev9/cliphub/internal/recapplan"
)

const (
	fullDemoHalftimeSource  = "full-demo-halftime"
	fullDemoSubscribeSource = "full-demo-subscribe"

	fullDemoHalftimeMinSeconds    = 2.0
	fullDemoStoryGapSeconds       = 1.0
	fullDemoSubscribeApartSeconds = 20.0
	fullDemoStoryMarginPixels     = 16
)

var fullDemoSubscribeMoments = []float64{0.25, 0.75}

type fullDemoStoryWindow struct {
	start, end float64
}

func fullDemoStoryOverlayEffects(manifest *Manifest, short ShortEdit) ([]Effect, error) {
	overlays := short.FullDemo.Effective.Options.Overlays
	if !overlays.Halftime && !overlays.Subscribe {
		return nil, nil
	}
	palette := demooverlay.DesignPalette(overlays)
	busy := fullDemoStoryBusyWindows(short.Effects)
	var effects []Effect
	if overlays.Halftime && manifest.fullDemoOverlay != nil && manifest.fullDemoOverlay.Halftime != nil {
		halftime := *manifest.fullDemoOverlay.Halftime
		if window, ok := fullDemoHalftimeWindow(short, halftime.Round); ok {
			path := filepath.Join(manifest.OutputDir, fullDemoHalftimeSource+".png")
			if err := demooverlay.RenderHalftimePNG(halftime, palette, path); err != nil {
				return nil, err
			}
			effects = append(effects, fullDemoHalftimeEffect(short, path, window))
			busy = append(busy, window)
		}
	}
	if !overlays.Subscribe {
		return effects, nil
	}
	windows := fullDemoSubscribeWindows(short, busy)
	if len(windows) == 0 {
		return effects, nil
	}
	lift, err := fullDemoSubscribeLift(short)
	if err != nil {
		return nil, err
	}
	path := filepath.Join(manifest.OutputDir, fullDemoSubscribeSource+".png")
	if err := demooverlay.RenderSubscribePNG(palette, path); err != nil {
		return nil, err
	}
	for _, window := range windows {
		effects = append(effects, fullDemoSubscribeEffect(path, window, lift))
	}
	return effects, nil
}

func fullDemoStoryBusyWindows(effects []Effect) []fullDemoStoryWindow {
	var busy []fullDemoStoryWindow
	for _, effect := range effects {
		if effect.Type == EffectImage && effect.EndSeconds > effect.StartSeconds {
			busy = append(busy, fullDemoStoryWindow{effect.StartSeconds, effect.EndSeconds})
		}
	}
	return busy
}

func fullDemoRoundItemSeconds(item recapplan.TimelineItem) (float64, float64) {
	return float64(item.StartFrame) / recapplan.OutputFPS, float64(item.EndFrame-1) / recapplan.OutputFPS
}

func fullDemoHalftimeWindow(short ShortEdit, roundNumber int) (fullDemoStoryWindow, bool) {
	d := short.FullDemo.Effective
	roundID := ""
	for _, round := range d.Rounds {
		if round.Number == roundNumber {
			roundID = round.ID
		}
	}
	if roundID == "" {
		return fullDemoStoryWindow{}, false
	}
	for _, item := range d.Timeline {
		if item.Role != "round" || item.SourceRef != roundID || item.SourceOffsetFrames != 0 {
			continue
		}
		start, last := fullDemoRoundItemSeconds(item)
		for _, effect := range short.Effects {
			if effect.Source == "full-demo-intro" && effect.EndSeconds > start {
				start = effect.EndSeconds + fullDemoStoryGapSeconds
			}
			if effect.Source == "full-demo-outro" {
				last = math.Min(last, effect.StartSeconds-fullDemoStoryGapSeconds)
			}
		}
		end := math.Min(start+demooverlay.HalftimeSeconds, last)
		if end-start < fullDemoHalftimeMinSeconds {
			return fullDemoStoryWindow{}, false
		}
		return fullDemoStoryWindow{start, end}, true
	}
	return fullDemoStoryWindow{}, false
}

func fullDemoHalftimeEffect(short ShortEdit, path string, window fullDemoStoryWindow) Effect {
	rest, hidden := 0, -demooverlay.HalftimeSlidePixels
	if banner := short.FullDemo.Effective.Options.Overlays.Brand.Banner; banner != nil {
		if start, end, ok := banner.Window(math.Inf(1)); ok && start < window.end && end > window.start {
			rest = fullDemoBannerTop + fullDemoBannerMaxHeight + fullDemoStoryMarginPixels - demooverlay.HalftimeCardTop
			hidden = rest + demooverlay.HalftimeSlidePixels
		}
	}
	return Effect{
		Type: EffectImage, Path: path, Source: fullDemoHalftimeSource,
		X: "0", Y: fullDemoStorySlide(window, hidden, rest),
		Width: demooverlay.FrameWidth, Height: demooverlay.FrameHeight,
		StartSeconds: window.start, EndSeconds: window.end,
		FadeInSeconds: demooverlay.IntroOverlaySlideSeconds, FadeOutSeconds: demooverlay.IntroOverlaySlideOutSeconds,
	}
}

func fullDemoSubscribeEffect(path string, window fullDemoStoryWindow, lift int) Effect {
	return Effect{
		Type: EffectImage, Path: path, Source: fullDemoSubscribeSource,
		X: fullDemoStorySlide(window, demooverlay.SubscribeSlidePixels, 0), Y: strconv.Itoa(-lift),
		Width: demooverlay.FrameWidth, Height: demooverlay.FrameHeight,
		StartSeconds: window.start, EndSeconds: window.end,
		FadeInSeconds: demooverlay.IntroOverlaySlideSeconds, FadeOutSeconds: demooverlay.IntroOverlaySlideOutSeconds,
	}
}

func fullDemoStorySlide(window fullDemoStoryWindow, hidden, rest int) string {
	return "'" + introSlideX(window.start, demooverlay.IntroOverlaySlideSeconds, window.end-demooverlay.IntroOverlaySlideOutSeconds, window.end, hidden, rest) + "'"
}

func fullDemoSubscribeWindows(short ShortEdit, busy []fullDemoStoryWindow) []fullDemoStoryWindow {
	spanStart, spanEnd := fullDemoGameplaySpanSeconds(short)
	var windows []fullDemoStoryWindow
	for _, moment := range fullDemoSubscribeMoments {
		target := spanStart + (spanEnd-spanStart)*moment - demooverlay.SubscribeSeconds/2
		window, ok := fullDemoSubscribeWindowNear(short.FullDemo.Effective.Timeline, busy, target)
		if !ok {
			continue
		}
		windows = append(windows, window)
		busy = append(busy, fullDemoStoryWindow{window.start - fullDemoSubscribeApartSeconds, window.end + fullDemoSubscribeApartSeconds})
	}
	return windows
}

func fullDemoSubscribeWindowNear(items []recapplan.TimelineItem, busy []fullDemoStoryWindow, target float64) (fullDemoStoryWindow, bool) {
	best, found := 0.0, false
	for _, item := range items {
		if item.Role != "round" {
			continue
		}
		first, last := fullDemoRoundItemSeconds(item)
		for _, free := range fullDemoStoryFreeWindows(fullDemoStoryWindow{first, last}, busy) {
			if free.end-free.start < demooverlay.SubscribeSeconds {
				continue
			}
			start := math.Min(math.Max(target, free.start), free.end-demooverlay.SubscribeSeconds)
			if !found || math.Abs(start-target) < math.Abs(best-target) {
				best, found = start, true
			}
		}
	}
	return fullDemoStoryWindow{best, best + demooverlay.SubscribeSeconds}, found
}

func fullDemoStoryFreeWindows(span fullDemoStoryWindow, busy []fullDemoStoryWindow) []fullDemoStoryWindow {
	free := []fullDemoStoryWindow{span}
	for _, taken := range busy {
		from, to := taken.start-fullDemoStoryGapSeconds, taken.end+fullDemoStoryGapSeconds
		var next []fullDemoStoryWindow
		for _, window := range free {
			if to <= window.start || from >= window.end {
				next = append(next, window)
				continue
			}
			if from > window.start {
				next = append(next, fullDemoStoryWindow{window.start, from})
			}
			if to < window.end {
				next = append(next, fullDemoStoryWindow{to, window.end})
			}
		}
		free = next
	}
	return free
}

func fullDemoSubscribeLift(short ShortEdit) (int, error) {
	watermark := short.FullDemo.Effective.Options.Overlays.Brand.Watermark
	if watermark == nil || watermark.Corner != brandkit.CornerBottomRight {
		return 0, nil
	}
	path, err := short.fullDemo.execution.assetPath(recapplan.AssetRef{ID: watermark.Image.ID, SHA256: watermark.Image.SHA256})
	if err != nil {
		return 0, err
	}
	width, height, err := brandImageSize(path)
	if err != nil {
		return 0, err
	}
	return subscribeLiftOverWatermark(width, height), nil
}

func subscribeLiftOverWatermark(imageWidth, imageHeight int) int {
	_, _, _, top := watermarkPlacement(brandkit.CornerBottomRight, imageWidth, imageHeight, demooverlay.FrameWidth, demooverlay.FrameHeight)
	return max(0, demooverlay.SubscribeBottomEdge-(top-fullDemoStoryMarginPixels))
}
