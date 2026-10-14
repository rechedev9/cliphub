package demooverlay

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/rechedev9/cliphub/internal/argv"
)

const mutedColor = "0xA1A1AA"

const (
	outroScoreSize    = 34
	outroColLabelSize = 14
	outroNameSize     = 22
	outroTopNameSize  = 24
	outroStatSize     = 21
	outroPOVLabelSize = 10
)

// OverlayWindows returns the compiled-timeline windows for intro and outro
// image overlays. Intro covers the first five seconds, clamped to the video.
// Outro covers the last beats after the win banner without overlapping intro.
func OverlayWindows(durationSeconds float64) (introStart, introEnd, outroStart, outroEnd float64) {
	if durationSeconds <= 0 {
		return 0, 0, 0, 0
	}
	introStart = IntroOverlayStart()
	introEnd = IntroOverlayEnd()
	if introEnd > durationSeconds {
		introEnd = durationSeconds
	}
	if introStart >= introEnd {
		introStart, introEnd = 0, 0
	}
	outroStart = durationSeconds - OutroSeconds
	if outroStart < introEnd {
		outroStart = introEnd
	}
	if outroStart < 0 {
		outroStart = 0
	}
	return introStart, introEnd, outroStart, durationSeconds
}

// RenderPNGs writes the intro (transparent sides) and outro (full-frame)
// overlay stills. ffmpegPath and fontPath must already be resolved.
func RenderPNGs(ffmpegPath, fontPath string, doc Document, introPath, outroPath string, opts RenderOptions) error {
	if doc.Screenshots != nil {
		for _, item := range []struct {
			outro bool
			path  string
		}{{false, introPath}, {true, outroPath}} {
			html, err := ScreenshotHTML(*doc.Screenshots, item.outro, opts.PreviewGreyBase)
			if err != nil {
				return err
			}
			if err := renderHTMLStill(html, item.path); err != nil {
				return err
			}
		}
		return nil
	}
	if strings.TrimSpace(ffmpegPath) == "" {
		return fmt.Errorf("render full-demo overlay: ffmpeg path is required")
	}
	if strings.TrimSpace(fontPath) == "" {
		return fmt.Errorf("render full-demo overlay: font path is required")
	}
	introPlate := IntroPlatePath(doc.Source, opts.OverlayAssetsDir)
	if UsesProgrammaticIntroChrome(doc) {
		introPlate = ""
	}
	outroPlate := OutroPlatePath(doc.Source, opts.OverlayAssetsDir)
	if NormalizeSource(doc.Source) == SourceFACEIT {
		outroPlate = ""
	}
	if err := renderIntroStill(ffmpegPath, fontPath, doc, introPath, introPlate, opts.PreviewGreyBase); err != nil {
		return fmt.Errorf("render intro overlay: %w", err)
	}
	if err := renderOutroStill(ffmpegPath, fontPath, doc, outroPath, outroPlate, opts.PreviewGreyBase); err != nil {
		return fmt.Errorf("render outro overlay: %w", err)
	}
	return nil
}

func renderIntroStill(ffmpegPath, fontPath string, doc Document, outPath, platePath string, previewGrey bool) error {
	if NormalizeTheme(doc.Theme) == ThemeNeonViolet {
		return renderNeonIntroStill(doc, outPath, previewGrey)
	}
	l := DefaultLayout()
	if NormalizeSource(doc.Source) == SourceFACEIT {
		l.Intro = faceitIntroLayout()
	}
	hasExternalPlate := platePath != ""
	text := introFilter(doc, fontPath, hasExternalPlate)
	artworkPath := platePath
	if artworkPath == "" {
		chromeFile, err := os.CreateTemp("", "cliphub-intro-chrome-*.png")
		if err != nil {
			return fmt.Errorf("create intro chrome temp: %w", err)
		}
		chromePath := chromeFile.Name()
		_ = chromeFile.Close()
		defer func() { _ = os.Remove(chromePath) }()
		chromePNG, err := renderIntroChromePNG(doc)
		if err != nil {
			return err
		}
		if err := writeChrome(chromePath, chromePNG); err != nil {
			return err
		}
		artworkPath = chromePath
	}
	req := stillRenderRequest{
		outPath:         outPath,
		textFilter:      text,
		transparentBase: !previewGrey,
		previewGreyBase: previewGrey,
		avatars:         introAvatarSlots(doc, hasExternalPlate),
		platePath:       artworkPath,
		introPlate:      true,
		layout:          l,
	}
	return renderStill(ffmpegPath, req)
}

func renderOutroStill(ffmpegPath, fontPath string, doc Document, outPath, platePath string, previewGrey bool) error {
	if NormalizeTheme(doc.Theme) == ThemeNeonViolet {
		html, err := NeonOutroHTML(doc, previewGrey)
		if err != nil {
			return err
		}
		return renderHTMLStill(html, outPath)
	}
	l := DefaultLayout()
	hasPlate := platePath != ""
	layout, geo, usePlateGeo := OutroLayoutForSourceWithPlate(doc.Source, hasPlate)
	chrome := outroChromePNG
	if NormalizeSource(doc.Source) == SourceFACEIT {
		layout = faceitOutroLayout()
		l.Outro = layout
		generated, err := renderOutroChromePNG(doc)
		if err != nil {
			return err
		}
		chrome = generated
	}
	text := outroFilter(doc, fontPath, layout, hasPlate)
	if hasPlate && !usePlateGeo {
		shading := OutroRowShadingDrawboxes(layout)
		if len(shading) > 0 {
			if text == "" || text == "null" {
				text = strings.Join(shading, ",")
			} else {
				text = text + "," + strings.Join(shading, ",")
			}
		}
	}
	if text == "" {
		text = "null"
	}
	cropTop, cropBottom := 0, 0
	if usePlateGeo {
		cropTop = geo.PlateCropTop
		cropBottom = geo.PlateCropBottom
	}
	req := stillRenderRequest{
		outPath:              outPath,
		textFilter:           text,
		chrome:               chrome,
		transparentBase:      hasPlate && !previewGrey,
		previewGreyBase:      hasPlate && previewGrey,
		outroPlate:           hasPlate,
		outroPlateCropTop:    cropTop,
		outroPlateCropBottom: cropBottom,
		platePath:            platePath,
		layout:               l,
	}
	return renderStill(ffmpegPath, req)
}
