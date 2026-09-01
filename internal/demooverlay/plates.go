package demooverlay

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
)

const (
	plateIntroSuffix = "-intro.jpg"
	plateOutroSuffix = "-outro.jpg"
	outroPlateAlpha  = 0.95
)

// RenderOptions configures optional Full Demo overlay background plates.
type RenderOptions struct {
	// OverlayAssetsDir points at a directory containing per-source JPG plates
	// named {professional|premier|faceit}-{intro|outro}.jpg. Empty or missing
	// files fall back to the embedded chrome PNG backgrounds.
	OverlayAssetsDir string
	// PreviewGreyBase uses a mid-grey canvas instead of transparency for still
	// previews so panel-only overlays are easier to review.
	PreviewGreyBase bool
}

// IntroPlatePath returns the intro plate path when present, else "".
func IntroPlatePath(source, assetsDir string) string {
	return resolvePlatePath(source, assetsDir, plateIntroSuffix)
}

// OutroPlatePath returns the outro plate path when present, else "".
func OutroPlatePath(source, assetsDir string) string {
	return resolvePlatePath(source, assetsDir, plateOutroSuffix)
}

func resolvePlatePath(source, assetsDir, suffix string) string {
	source = NormalizeSource(source)
	if source == "" {
		return ""
	}
	dir := strings.TrimSpace(assetsDir)
	if dir == "" {
		return ""
	}
	path := filepath.Join(dir, source+suffix)
	if _, err := os.Stat(path); err != nil {
		return ""
	}
	return path
}

// OutroLayoutForSource returns the scoreboard geometry for a source without a plate.
func OutroLayoutForSource(source string) OutroLayout {
	_ = NormalizeSource(source)
	return DefaultLayout().Outro
}

// IntroTextInset returns the horizontal inset for player row text. FACEIT intro
// plates reserve avatar circles on the left; indent past the circle zone.
func IntroTextInset(layout IntroLayout, source string, hasPlate bool) int {
	inset := layout.CardInset
	if hasPlate && NormalizeSource(source) == SourceFACEIT {
		circleRight := layout.AvatarXOff + layout.AvatarSize + 8
		if circleRight > inset {
			inset = circleRight
		}
	}
	return inset
}

// plateScaleCoverCropChain scales input to cover FrameWidth x FrameHeight then center-crops.
func plateScaleCoverCropChain() string {
	return fmt.Sprintf(
		"scale=%d:%d:force_original_aspect_ratio=increase:flags=lanczos,crop=%d:%d:(iw-ow)/2:(ih-oh)/2",
		FrameWidth, FrameHeight, FrameWidth, FrameHeight,
	)
}

type introPlateOverlay struct {
	LeftCrop  string
	RightCrop string
	LeftX     int
	LeftY     int
	RightX    int
	RightY    int
}

func introPlateOverlaySpec(l Layout) introPlateOverlay {
	return introPlateOverlay{
		LeftCrop:  fmt.Sprintf("[pcov]crop=%d:%d:%d:%d[plL]", l.Intro.PanelWidth, l.Intro.PanelHeight, l.Intro.LeftPanelX, l.Intro.PanelTop),
		RightCrop: fmt.Sprintf("[pcov]crop=%d:%d:%d:%d[plR]", l.Intro.PanelWidth, l.Intro.PanelHeight, l.Intro.RightPanelX, l.Intro.PanelTop),
		LeftX:     l.Intro.LeftPanelX,
		LeftY:     l.Intro.PanelTop,
		RightX:    l.Intro.RightPanelX,
		RightY:    l.Intro.PanelTop,
	}
}

// IntroPlateFilterClauses returns ffmpeg clauses compositing intro panel crops
// onto current. plateInputIndex is the ffmpeg input index of the plate JPG.
func IntroPlateFilterClauses(current string, plateInputIndex int, l Layout) ([]string, string) {
	plateIn := fmt.Sprintf("%d:v", plateInputIndex)
	spec := introPlateOverlaySpec(l)
	clauses := []string{
		fmt.Sprintf("[%s]%s[pcov]", plateIn, plateScaleCoverCropChain()),
		"[pcov]split=2[pcovL][pcovR]",
		fmt.Sprintf("[pcovL]crop=%d:%d:%d:%d[plL]", l.Intro.PanelWidth, l.Intro.PanelHeight, l.Intro.LeftPanelX, l.Intro.PanelTop),
		fmt.Sprintf("[pcovR]crop=%d:%d:%d:%d[plR]", l.Intro.PanelWidth, l.Intro.PanelHeight, l.Intro.RightPanelX, l.Intro.PanelTop),
		fmt.Sprintf("%s[plL]overlay=x=%d:y=%d:format=auto[pl_left]", current, spec.LeftX, spec.LeftY),
	}
	next := "pl_done"
	clauses = append(clauses, fmt.Sprintf("[pl_left][plR]overlay=x=%d:y=%d:format=auto[%s]", spec.RightX, spec.RightY, next))
	return clauses, next
}

// OutroPlateBackdropClauses composites a full-frame outro plate beneath text.
func OutroPlateBackdropClauses(current string, plateInputIndex int) ([]string, string) {
	plateIn := fmt.Sprintf("%d:v", plateInputIndex)
	clauses := []string{
		fmt.Sprintf("[%s]%s[pcov]", plateIn, plateScaleCoverCropChain()),
		fmt.Sprintf("[pcov]format=rgba,colorchannelmixer=aa=%.2f[plat]", outroPlateAlpha),
		fmt.Sprintf("%s[plat]overlay=x=0:y=0:format=auto[plated]", current),
	}
	return clauses, "plated"
}

// OutroRowShadingDrawboxes returns subtle row bands aligned to the default grid.
func OutroRowShadingDrawboxes(layout OutroLayout) []string {
	var parts []string
	for team := 0; team < 2; team++ {
		x := layout.Margin + team*layout.ColGap
		rowY := layout.Row0
		for range 5 {
			parts = append(parts, drawbox(x, rowY, layout.NameWidth+4*layout.StatWidth, layout.RowHeight, "0x000000@0.15"))
			rowY += layout.RowHeight
		}
	}
	return parts
}

func logPlateFallback(kind, source, reason string) {
	log.Printf("demooverlay: %s plate for %q unavailable (%s); using chrome fallback", kind, source, reason)
}
