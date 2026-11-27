package editor

import (
	"fmt"
	"path/filepath"
	"slices"

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
	Path       string `json:"path"`
	CellTop    int    `json:"cell_top"`
	CellHeight int    `json:"cell_height"`
	Frames     int    `json:"frames"`
}

var subscribeBellRings = []float64{0.5, 2.6}

func subscribeBellSprite(sprite demooverlay.SubscribeSprite) *ImageSprite {
	return &ImageSprite{CellHeight: sprite.CellHeight, Frames: sprite.Frames, FPS: demooverlay.SubscribeBellFPS, PlayAt: slices.Clone(subscribeBellRings)}
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
	sprite, err := demooverlay.RenderSubscribeSprite(nil, demooverlay.SubscribeCard{LogoPath: card.LogoPath, ChannelName: card.ChannelName}, size, path)
	if err != nil {
		return nil, fmt.Errorf("subscribe banner: %w", err)
	}
	return &ShortSubscribe{Path: path, CellTop: sprite.CellTop, CellHeight: sprite.CellHeight, Frames: sprite.Frames}, nil
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
	frameW, _ := outputDimensions(short)
	sprite := demooverlay.SubscribeSprite{CellTop: subscribe.CellTop, CellHeight: subscribe.CellHeight, Frames: subscribe.Frames}
	slide := "'" + introSlideX(start, demooverlay.IntroOverlaySlideSeconds, end-demooverlay.IntroOverlaySlideOutSeconds, end, sprite.CellTop+shortSubscribeSlidePixels, sprite.CellTop) + "'"
	return []Effect{{
		Type: EffectImage, Path: subscribe.Path, Source: shortSubscribeSource,
		X: "0", Y: slide, Width: frameW, Height: sprite.Height(), Sprite: subscribeBellSprite(sprite),
		StartSeconds: start, EndSeconds: end,
		FadeInSeconds: demooverlay.IntroOverlaySlideSeconds, FadeOutSeconds: demooverlay.IntroOverlaySlideOutSeconds,
	}}
}
