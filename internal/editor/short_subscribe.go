package editor

import (
	"fmt"
	"path/filepath"

	"github.com/rechedev9/cliphub/internal/brandkit"
	"github.com/rechedev9/cliphub/internal/demooverlay"
	"github.com/rechedev9/cliphub/internal/htmlstill"
)

const (
	shortSubscribeSource        = "brand-subscribe"
	shortSubscribeSeconds       = 4.5
	shortSubscribeAt            = 0.35
	shortSubscribeMarginSeconds = 1.5
	shortSubscribeSlidePixels   = 220
)

type SubscribeCard struct {
	LogoPath    string `json:"logo_path,omitempty"`
	ChannelName string `json:"channel_name,omitempty"`
}

type ShortSubscribe struct {
	Path string `json:"path"`
}

func prepareShortSubscribe(card *SubscribeCard, format, outDir string) (*ShortSubscribe, error) {
	if card == nil {
		return nil, nil
	}
	if err := brandkit.ValidateChannelName(card.ChannelName); err != nil {
		return nil, err
	}
	size := htmlstill.Portrait
	if format == OutputFormatLandscape16x9 {
		size = htmlstill.Landscape
	}
	path := filepath.Join(outDir, shortSubscribeSource+".png")
	if err := demooverlay.RenderSubscribePNG(nil, demooverlay.SubscribeCard{LogoPath: card.LogoPath, ChannelName: card.ChannelName}, size, path); err != nil {
		return nil, fmt.Errorf("subscribe banner: %w", err)
	}
	return &ShortSubscribe{Path: path}, nil
}

func shortSubscribeWindow(duration float64) (float64, float64, bool) {
	latest := duration - shortSubscribeSeconds - shortSubscribeMarginSeconds
	if latest < shortSubscribeMarginSeconds {
		return 0, 0, false
	}
	start := min(max(duration*shortSubscribeAt, shortSubscribeMarginSeconds), latest)
	return start, start + shortSubscribeSeconds, true
}

func generatedSubscribeEffects(short ShortEdit) []Effect {
	subscribe := short.Overlays.Subscribe
	if subscribe == nil || short.FullDemo != nil {
		return nil
	}
	start, end, ok := shortSubscribeWindow(short.DurationSeconds)
	if !ok {
		return nil
	}
	frameW, frameH := outputDimensions(short)
	slide := "'" + introSlideX(start, demooverlay.IntroOverlaySlideSeconds, end-demooverlay.IntroOverlaySlideOutSeconds, end, shortSubscribeSlidePixels, 0) + "'"
	return []Effect{{
		Type: EffectImage, Path: subscribe.Path, Source: shortSubscribeSource,
		X: "0", Y: slide, Width: frameW, Height: frameH,
		StartSeconds: start, EndSeconds: end,
		FadeInSeconds: demooverlay.IntroOverlaySlideSeconds, FadeOutSeconds: demooverlay.IntroOverlaySlideOutSeconds,
	}}
}
