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

type stillRenderRequest struct {
	outPath              string
	textFilter           string
	chrome               []byte
	transparentBase      bool
	previewGreyBase      bool
	avatars              []avatarSlot
	platePath            string
	introPlate           bool
	outroPlate           bool
	outroPlateCropTop    int
	outroPlateCropBottom int
	layout               Layout
}

type avatarSlot struct {
	Path string
	X, Y int
	Size int
}

func introAvatarSlots(doc Document, hasPlate bool) []avatarSlot {
	l := DefaultLayout()
	if NormalizeSource(doc.Source) == SourceFACEIT {
		l.Intro = faceitIntroLayout()
	}
	geo, useGeo := IntroPlateGeo(doc.Source, hasPlate)
	var slots []avatarSlot
	add := func(cards []PlayerCard, x, y int) {
		if len(cards) > l.Intro.MaxPlayers {
			cards = cards[:l.Intro.MaxPlayers]
		}
		for i, card := range cards {
			if strings.TrimSpace(card.AvatarFile) == "" {
				continue
			}
			if _, err := os.Stat(card.AvatarFile); err != nil {
				continue
			}
			ay := y + l.Intro.HeaderH + i*l.Intro.RowHeight + l.Intro.AvatarYOff
			if useGeo && i < len(geo.RowNameCenterY) {
				ay = geo.RowNameCenterY[i] - l.Intro.AvatarSize/2
			}
			slots = append(slots, avatarSlot{
				Path: card.AvatarFile,
				X:    x + l.Intro.AvatarXOff,
				Y:    ay,
				Size: l.Intro.AvatarSize,
			})
		}
	}
	add(doc.Intro.Left, l.Intro.LeftPanelX, l.Intro.PanelTop)
	add(doc.Intro.Right, l.Intro.RightPanelX, l.Intro.PanelTop)
	return slots
}

func renderStill(ffmpegPath string, req stillRenderRequest) error {
	if strings.TrimSpace(req.outPath) == "" {
		return fmt.Errorf("output path is required")
	}
	if err := os.MkdirAll(filepath.Dir(req.outPath), 0o750); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}
	var args []string
	var chromeCleanup func()
	inputIndex := 0
	if req.previewGreyBase {
		args = append(args, "-f", "lavfi", "-i", fmt.Sprintf("color=c=0x808080:s=%dx%d", FrameWidth, FrameHeight))
	} else if req.transparentBase {
		args = append(args, "-f", "lavfi", "-i", fmt.Sprintf("color=c=black@0:s=%dx%d,format=rgba", FrameWidth, FrameHeight))
	} else {
		chromeFile, err := os.CreateTemp("", "cliphub-chrome-*.png")
		if err != nil {
			return fmt.Errorf("create overlay chrome: %w", err)
		}
		chromePath := chromeFile.Name()
		_ = chromeFile.Close()
		chromeCleanup = func() { _ = os.Remove(chromePath) }
		defer chromeCleanup()
		if err := writeChrome(chromePath, req.chrome); err != nil {
			return err
		}
		args = append(args, "-i", chromePath)
	}
	inputIndex++
	plateInput := -1
	if req.platePath != "" {
		args = append(args, "-i", req.platePath)
		plateInput = inputIndex
		inputIndex++
	}
	for _, slot := range req.avatars {
		args = append(args, "-i", slot.Path)
	}
	script, err := os.CreateTemp("", "cliphub-overlay-*.fffilter")
	if err != nil {
		return fmt.Errorf("create overlay filter script: %w", err)
	}
	scriptPath := script.Name()
	defer func() { _ = os.Remove(scriptPath) }()
	graph := stillFilterGraph(stillFilterGraphOptions{
		text:                 req.textFilter,
		avatars:              req.avatars,
		plateInput:           plateInput,
		introPlate:           req.introPlate,
		outroPlate:           req.outroPlate,
		outroPlateCropTop:    req.outroPlateCropTop,
		outroPlateCropBottom: req.outroPlateCropBottom,
		layout:               req.layout,
	})
	if _, err := script.WriteString(graph); err != nil {
		_ = script.Close()
		return fmt.Errorf("write overlay filter script: %w", err)
	}
	if err := script.Close(); err != nil {
		return fmt.Errorf("close overlay filter script: %w", err)
	}
	args = append([]string{"-y", "-hide_banner", "-loglevel", "error"}, args...)
	args = append(args, "-filter_complex_script", scriptPath, "-frames:v", "1", "-pix_fmt", "rgba", req.outPath)
	cmd := argv.Command(ffmpegPath, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

type stillFilterGraphOptions struct {
	text                 string
	avatars              []avatarSlot
	plateInput           int
	introPlate           bool
	outroPlate           bool
	outroPlateCropTop    int
	outroPlateCropBottom int
	layout               Layout
}

func stillFilterGraph(opts stillFilterGraphOptions) string {
	current := "[0:v]"
	var clauses []string
	if opts.plateInput >= 0 {
		if opts.introPlate {
			plateClauses, next := IntroPlateFilterClauses(current, opts.plateInput, opts.layout)
			clauses = append(clauses, plateClauses...)
			current = "[" + next + "]"
		} else if opts.outroPlate {
			plateClauses, next := OutroPlateBackdropClauses(current, opts.plateInput, opts.outroPlateCropTop, opts.outroPlateCropBottom)
			clauses = append(clauses, plateClauses...)
			current = "[" + next + "]"
		}
	}
	avatarStart := 1
	if opts.plateInput >= 0 {
		avatarStart = opts.plateInput + 1
	}
	for i, slot := range opts.avatars {
		scaled := fmt.Sprintf("a%d", i)
		clauses = append(clauses, fmt.Sprintf(
			"[%d:v]scale=%d:%d:flags=lanczos,format=rgba,geq=r='r(X,Y)':g='g(X,Y)':b='b(X,Y)':a='if(lte(hypot(X-W/2,Y-H/2),min(W,H)/2),255,0)'[%s]",
			avatarStart+i, slot.Size, slot.Size, scaled,
		))
		next := fmt.Sprintf("s%d", i)
		clauses = append(clauses, fmt.Sprintf("%s[%s]overlay=%d:%d:format=auto[%s]", current, scaled, slot.X, slot.Y, next))
		current = "[" + next + "]"
	}
	text := opts.text
	if text == "" || text == "null" {
		if len(clauses) == 0 {
			return current + "format=rgba"
		}
		return strings.Join(clauses, ";")
	}
	if len(clauses) == 0 {
		return current + text
	}
	return strings.Join(clauses, ";") + ";" + current + text
}

func introFilter(doc Document, fontPath string, hasPlate bool) string {
	l := DefaultLayout()
	if NormalizeSource(doc.Source) == SourceFACEIT {
		l.Intro = faceitIntroLayout()
	}
	theme := ResolveTheme(doc)
	var parts []string
	parts = append(parts, introColumn(doc.Intro.Left, doc.Intro.LeftTeamName, doc.Intro.LeftSubtitle, l.Intro.LeftPanelX, l.Intro.PanelTop, l.Intro, fontPath, doc, hasPlate, theme)...)
	parts = append(parts, introColumn(doc.Intro.Right, doc.Intro.RightTeamName, doc.Intro.RightSubtitle, l.Intro.RightPanelX, l.Intro.PanelTop, l.Intro, fontPath, doc, hasPlate, theme)...)
	if len(parts) == 0 {
		return "null"
	}
	return strings.Join(parts, ",")
}

func introHeader(source string) string {
	switch NormalizeSource(source) {
	case SourcePremier:
		return "PREMIER"
	case SourceProfessional:
		return "LINEUP"
	default:
		return "PLAYERS"
	}
}

func ffmpegDrawtextText(text string) string {
	text = strings.ReplaceAll(text, `\\`, `\\\\`)
	text = strings.ReplaceAll(text, `'`, `\\'`)
	text = strings.ReplaceAll(text, `:`, `\\:`)
	return text
}
