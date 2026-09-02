package demooverlay

import (
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
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
// image overlays. Intro starts ~4s after the fade-from-black and ends
// before live action. Outro covers the last beats after the win banner.
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
		chrome:          introChromePNG,
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
	// #nosec G204 -- ffmpegPath is the host FFmpeg resolved by ClipHub config.
	cmd := exec.Command(ffmpegPath, args...) //nolint:gosec
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

func introColumn(cards []PlayerCard, teamName, subtitle string, x, y int, layout IntroLayout, fontPath string, doc Document, hasPlate bool, theme CardTheme) []string {
	if len(cards) > layout.MaxPlayers {
		cards = cards[:layout.MaxPlayers]
	}
	var parts []string
	header := strings.TrimSpace(teamName)
	if header == "" {
		header = introHeader(doc.Source)
	}
	geo, useGeo := IntroPlateGeo(doc.Source, hasPlate)
	isFaceit := NormalizeSource(doc.Source) == SourceFACEIT && !useGeo
	faceit := defaultFaceitLayout.Intro
	palette := defaultFaceitLayout.Palette
	if useGeo {
		nameSize := geo.TeamNameSize
		if nameSize <= 0 {
			nameSize = introPlateTeamNameSize
		}
		parts = appendFilter(parts, drawtext(fontPath, header, x+geo.TeamNameXOff, geo.TeamNameY, nameSize, "white"))
		if sub := strings.TrimSpace(subtitle); sub != "" {
			parts = appendFilter(parts, drawtext(fontPath, sub, x+geo.TeamNameXOff, geo.SubtitleY, layout.LabelSize+2, mutedColor))
		}
	} else {
		headerName := textAnchorSpec{X: 20, Y: 18, FontSize: layout.NameSize + 2}
		headerSubtitle := textAnchorSpec{X: 20, Y: 50, FontSize: layout.LabelSize + 2}
		if isFaceit {
			headerName = faceit.Header.Name
			headerSubtitle = faceit.Header.Subtitle
		}
		parts = appendFilter(parts, drawtext(fontPath, header, x+headerName.X, y+headerName.Y, headerName.FontSize, palette.Text))
		if sub := strings.TrimSpace(subtitle); sub != "" {
			parts = appendFilter(parts, drawtext(fontPath, sub, x+headerSubtitle.X, y+headerSubtitle.Y, headerSubtitle.FontSize, palette.MutedText))
		}
	}
	textInset := IntroTextInset(layout, doc.Source, hasPlate)
	skipMonogram := hasPlate && NormalizeSource(doc.Source) == SourceFACEIT
	useChromeCards := !hasPlate && NormalizeSource(doc.Source) == SourceFACEIT
	for i, card := range cards {
		var cy, nameY int
		if useGeo && i < len(geo.RowNameCenterY) {
			cy = geo.RowNameCenterY[i]
			nameY = cy - layout.NameSize/2
		} else {
			cy = y + layout.HeaderH + i*layout.RowHeight
			nameY = cy + 10
			if isFaceit {
				nameY = cy + faceit.Name.Y
			}
		}
		if useChromeCards {
			card := faceit.Card
			rowX := x + card.X
			rowY := cy + card.Y
			rowW := card.Width
			rowH := card.Height
			if useGeo && i < len(geo.RowNameCenterY) {
				rowY = geo.RowNameCenterY[i] - layout.RowHeight/2 + layout.HeaderH/2
			}
			parts = append(parts, introPlayerCardChrome(rowX, rowY, rowW, rowH, theme)...)
		}
		nx := x + textInset
		ax := x + layout.AvatarXOff
		ay := cy + layout.AvatarYOff
		if isFaceit {
			nx = x + faceit.Name.X
			ax = x + faceit.Avatar.X
			ay = cy + faceit.Avatar.Y
		}
		if useGeo && i < len(geo.RowNameCenterY) {
			ay = geo.RowNameCenterY[i] - layout.AvatarSize/2
		}
		if !skipMonogram && strings.TrimSpace(card.AvatarFile) == "" && strings.TrimSpace(card.AvatarURL) == "" {
			parts = append(parts, monogramFilters(fontPath, ax, ay, layout.AvatarSize, card.Name)...)
		}
		nameColor := palette.Text
		if doc.IsPOV(card) {
			nameColor = palette.TargetText
		}
		parts = appendFilter(parts, drawtext(fontPath, card.Name, nx, nameY, layout.NameSize, nameColor))
		if doc.IsPOV(card) {
			tagX := introPlatePOVBadgeX(x, layout.PanelWidth)
			tagY := nameY
			tagW := introPlatePOVBadgeW
			tagH := introPlatePOVBadgeH
			textX, textY, fontSize := 5, 2, 10
			if isFaceit {
				tagX, _ = faceitIntroBadgePositions(x)
				tagY = cy + faceit.POV.Rect.Y
				tagW = faceit.POV.Rect.Width
				tagH = faceit.POV.Rect.Height
				textX, textY, fontSize = faceit.POV.TextX, faceit.POV.TextY, faceit.POV.FontSize
			}
			parts = append(parts, drawbox(tagX, tagY, tagW, tagH, palette.POVFill))
			parts = appendFilter(parts, drawtext(fontPath, "POV", tagX+textX, tagY+textY, fontSize, palette.POVText))
		}
		if card.Country != "" {
			countryY := nameY + layout.NameSize + 4
			countryX := nx
			country := badgeSpec{Rect: rectSpec{Width: 34, Height: 16}, TextX: 5, TextY: 1, FontSize: 10}
			if isFaceit {
				country = faceit.Country
				countryX = x + country.Rect.X
				countryY = cy + country.Rect.Y
			} else if !useGeo {
				countryY = cy + 10 + layout.NameSize + 4
			}
			parts = append(parts, countryBadgeFilters(fontPath, countryX, countryY, card.Country, theme, country)...)
		}
		if card.ELO != nil {
			eloX := x + layout.PanelWidth - layout.BadgeSize - 132
			eloY := nameY + 8
			eloSize := 18
			if isFaceit {
				eloX = x + faceit.ELO.X
				eloY = cy + faceit.ELO.Y
				eloSize = faceit.ELO.FontSize
			}
			parts = appendFilter(parts, drawtext(fontPath, strconv.Itoa(*card.ELO), eloX, eloY, eloSize, palette.Text))
		}
		badge := ""
		badgeFill := skillFill(10)
		if card.Ranking != nil {
			badge = "#" + strconv.Itoa(*card.Ranking)
			badgeFill = palette.RankFill
		} else if card.SkillLevel != nil {
			badge = strconv.Itoa(*card.SkillLevel)
			badgeFill = skillFill(*card.SkillLevel)
		}
		if badge != "" {
			bx := x + layout.PanelWidth - layout.BadgeSize - 18
			by := nameY + 6
			bw, bh := layout.BadgeSize, layout.BadgeSize
			textX, textY, fontSize := 2, 5, 12
			if isFaceit {
				_, bx = faceitIntroBadgePositions(x)
				by = cy + faceit.Level.Rect.Y
				bw, bh = faceit.Level.Rect.Width, faceit.Level.Rect.Height
				textX, textY, fontSize = faceit.Level.TextX, faceit.Level.TextY, faceit.Level.FontSize
			}
			parts = append(parts, drawbox(bx, by, bw, bh, badgeFill))
			parts = appendFilter(parts, drawtext(fontPath, badge, bx+textX, by+textY, fontSize, palette.Text))
		}
		statsY := cy + 112
		titleY := statsY - 14
		titleSize := 9
		statsX := nx
		statsWidth := layout.PanelWidth - textInset - 16
		if useGeo && i < len(geo.RowNameCenterY) {
			statsY = geo.RowNameCenterY[i] + layout.NameSize/2 + 12
			titleY = statsY - 14
		} else if isFaceit {
			statsX = x + faceit.Stats.X
			statsY = cy + faceit.Stats.ValueY
			titleY = cy + faceit.Stats.TitleY
			titleSize = faceit.Stats.TitleSize
			statsWidth = layout.PanelWidth - faceit.Stats.X - faceit.Stats.Right
		}
		if title := introStatsSectionTitle(card, doc.Source); title != "" {
			parts = appendFilter(parts, drawtext(fontPath, title, statsX, titleY, titleSize, palette.MutedText))
		}
		parts = append(parts, introStatGrid(card, statsX, statsY, statsWidth, fontPath, layout, doc.Source)...)
	}
	return parts
}

func faceitIntroBadgePositions(panelX int) (povX, levelX int) {
	return panelX + defaultFaceitLayout.Intro.POV.Rect.X, panelX + defaultFaceitLayout.Intro.Level.Rect.X
}

func monogramFilters(fontPath string, x, y, size int, name string) []string {
	initial := monogramInitial(name)
	if initial == "" {
		return nil
	}
	parts := []string{drawbox(x, y, size, size, defaultFaceitLayout.Palette.AvatarFill)}
	fontSize := size / 2
	if fontSize < 18 {
		fontSize = 18
	}
	textX := x + (size-fontSize)/2 - 2
	textY := y + (size-fontSize)/2 + 2
	parts = appendFilter(parts, drawtext(fontPath, initial, textX, textY, fontSize, "white"))
	return parts
}

func monogramInitial(name string) string {
	name = strings.TrimSpace(name)
	for _, r := range name {
		if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			return strings.ToUpper(string(r))
		}
	}
	return ""
}

type overlayStat struct {
	label string
	value string
}

func introStatsSectionTitle(card PlayerCard, source string) string {
	if len(introStats(card, source)) == 0 {
		return ""
	}
	if card.Last20 != nil && card.Last20.Matches != nil && *card.Last20.Matches > 0 {
		return fmt.Sprintf("Last %d FACEIT matches", *card.Last20.Matches)
	}
	if card.Last20 != nil {
		return "This match"
	}
	return "Recent FACEIT matches"
}

func introStatGrid(card PlayerCard, x, y, width int, fontPath string, layout IntroLayout, source string) []string {
	stats := introStats(card, source)
	if len(stats) == 0 {
		return nil
	}
	offsets := statColumnOffsets(width, len(stats))
	valueSize := layout.StatSize
	labelSize := layout.LabelSize
	labelGap := 2
	if NormalizeSource(source) == SourceFACEIT {
		valueSize = defaultFaceitLayout.Intro.Stats.ValueSize
		labelSize = defaultFaceitLayout.Intro.Stats.LabelSize
		labelGap = defaultFaceitLayout.Intro.Stats.LabelGap
	}
	palette := defaultFaceitLayout.Palette
	var parts []string
	for i, stat := range stats {
		if i >= len(offsets) {
			break
		}
		sx := x + offsets[i]
		sy := y
		parts = appendFilter(parts, drawtext(fontPath, stat.value, sx, sy, valueSize, palette.Text))
		parts = appendFilter(parts, drawtext(fontPath, stat.label, sx, sy+valueSize+labelGap, labelSize, palette.MutedText))
	}
	return parts
}

func statColumnOffsets(width, n int) []int {
	weights := defaultFaceitLayout.Intro.Stats.Weights
	if n <= 0 || width <= 0 {
		return nil
	}
	if n > len(weights) {
		n = len(weights)
	}
	sum := 0.0
	for i := 0; i < n; i++ {
		sum += weights[i]
	}
	out := make([]int, n)
	cursor := 0.0
	for i := 0; i < n; i++ {
		out[i] = int(cursor)
		cursor += float64(width) * weights[i] / sum
	}
	return out
}

func introStats(card PlayerCard, source string) []overlayStat {
	switch NormalizeSource(source) {
	case SourceProfessional, SourcePremier:
		return nil
	case SourceFACEIT:
		if card.Last20 != nil {
			matches := ""
			if v := last20Int(card, func(l Last20) *int { return l.Matches }); v != nil {
				matches = formatThousands(*v)
			}
			return []overlayStat{
				{label: "Matches", value: matches},
				{label: "Wins", value: formatOptPct("", last20Float(card, func(l Last20) *float64 { return l.WinPct }))},
				{label: "K/D/A", value: overlayKDA(card)},
				{label: "K/D", value: formatOptFloat("", last20Float(card, func(l Last20) *float64 { return l.KD }))},
				{label: "K/R", value: formatOptFloat("", last20Float(card, func(l Last20) *float64 { return l.KR }))},
				{label: "ADR", value: formatOptFloat("", last20Float(card, func(l Last20) *float64 { return l.ADR }))},
			}
		}
		return nil
	}
	if card.Last20 != nil {
		matches := ""
		if v := last20Int(card, func(l Last20) *int { return l.Matches }); v != nil {
			matches = formatThousands(*v)
		}
		adrLabel, adrValue := introADRHS(card)
		return []overlayStat{
			{label: "Matches", value: matches},
			{label: "Win rate", value: formatOptPct("", last20Float(card, func(l Last20) *float64 { return l.WinPct }))},
			{label: adrLabel, value: adrValue},
			{label: "K/D / K/R", value: introKDKR(card)},
		}
	}
	stats := []overlayStat{{label: "K/D/A", value: overlayKDA(card)}}
	if card.HasADR {
		stats = append(stats, overlayStat{label: "ADR", value: overlayDecimal(card.ADR, 1)})
	}
	if card.HasRating {
		stats = append(stats, overlayStat{label: "RATING", value: overlayDecimal(card.Rating, 2)})
	}
	if NormalizeSource(source) == SourceProfessional && card.HasHSPct && (card.HSPct > 0 || card.Headshots > 0) {
		stats = append(stats, overlayStat{label: "HS%", value: fmt.Sprintf("%.0f%%", card.HSPct)})
	}
	return stats
}

func introADRHS(card PlayerCard) (label, value string) {
	adr := formatOptFloat("", last20Float(card, func(l Last20) *float64 { return l.ADR }))
	hs := ""
	if card.HasHSPct && (card.HSPct > 0 || card.Headshots > 0) {
		hs = fmt.Sprintf("%.0f%%", card.HSPct)
	}
	if card.Last20 != nil {
		switch {
		case adr != "" && hs != "":
			return "ADR-HS%", adr + " / " + hs
		case adr != "":
			return "ADR", adr
		case hs != "":
			return "HS%", hs
		default:
			return "ADR", ""
		}
	}
	if adr == "" && card.HasADR {
		adr = overlayDecimal(card.ADR, 1)
	}
	switch {
	case adr != "" && hs != "":
		return "ADR-HS%", adr + " / " + hs
	case adr != "":
		return "ADR", adr
	case hs != "":
		return "HS%", hs
	default:
		return "ADR", ""
	}
}

func introKDKR(card PlayerCard) string {
	kd := formatOptFloat("", last20Float(card, func(l Last20) *float64 { return l.KD }))
	kr := formatOptFloat("", last20Float(card, func(l Last20) *float64 { return l.KR }))
	switch {
	case kd != "" && kr != "":
		return kd + " / " + kr
	case kd != "":
		return kd
	default:
		return kr
	}
}

func overlayKDA(card PlayerCard) string {
	if card.Last20 != nil && card.Last20.Matches != nil && *card.Last20.Matches > 0 {
		n := float64(*card.Last20.Matches)
		if card.Last20.Kills != nil && card.Last20.Deaths != nil && card.Last20.Assists != nil {
			return fmt.Sprintf("%d/%d/%d",
				int(math.Round(float64(*card.Last20.Kills)/n)),
				int(math.Round(float64(*card.Last20.Deaths)/n)),
				int(math.Round(float64(*card.Last20.Assists)/n)),
			)
		}
	}
	return fmt.Sprintf("%d/%d/%d", card.Kills, card.Deaths, card.Assists)
}

func overlayDecimal(v float64, prec int) string {
	return strings.Replace(strconv.FormatFloat(v, 'f', prec, 64), ".", ",", 1)
}

func formatThousands(n int) string {
	sign := ""
	if n < 0 {
		sign = "-"
		n = -n
	}
	s := strconv.Itoa(n)
	var parts []string
	for len(s) > 3 {
		parts = append([]string{s[len(s)-3:]}, parts...)
		s = s[:len(s)-3]
	}
	parts = append([]string{s}, parts...)
	return sign + strings.Join(parts, ".")
}

func last20Int(card PlayerCard, fn func(Last20) *int) *int {
	if card.Last20 == nil {
		return nil
	}
	return fn(*card.Last20)
}

func last20Float(card PlayerCard, fn func(Last20) *float64) *float64 {
	if card.Last20 == nil {
		return nil
	}
	return fn(*card.Last20)
}

func outroFilter(doc Document, fontPath string, layout OutroLayout, hasPlate bool) string {
	if NormalizeSource(doc.Source) == SourceFACEIT && !hasPlate {
		return faceitOutroFilter(doc, fontPath)
	}
	teams := doc.Outro.Teams
	cols := outroGridColumns(doc.Outro.Columns)
	_, geo, useGeo := OutroLayoutForSourceWithPlate(doc.Source, hasPlate)
	var parts []string
	if len(teams) == 2 {
		parts = append(parts, outroTeam(teams[0], cols, layout.Margin, layout, fontPath, doc, geo, useGeo)...)
		parts = append(parts, outroTeam(teams[1], cols, layout.Margin+layout.ColGap, layout, fontPath, doc, geo, useGeo)...)
		if len(parts) == 0 {
			return "null"
		}
		return strings.Join(parts, ",")
	}
	for i, team := range teams {
		shifted := layout
		shifted.HeaderY = layout.HeaderY + i*(layout.HeaderH+len(team.Players)*layout.RowHeight+16)
		shifted.Row0 = shifted.HeaderY + layout.HeaderH
		parts = append(parts, outroTeam(team, cols, layout.Margin, shifted, fontPath, doc, geo, useGeo)...)
	}
	if len(parts) == 0 {
		return "null"
	}
	return strings.Join(parts, ",")
}

func faceitOutroFilter(doc Document, fontPath string) string {
	layout := faceitOutroLayout()
	spec := defaultFaceitLayout.Outro
	cols := faceitOutroGridColumns(doc.Outro.Columns)
	var parts []string
	for i, team := range doc.Outro.Teams {
		if i >= 2 {
			break
		}
		shifted := layout
		shifted.HeaderY += i * spec.TeamYGap
		shifted.Row0 += i * spec.TeamYGap
		parts = append(parts, faceitOutroTeam(team, cols, shifted, fontPath, doc)...)
	}
	if len(parts) == 0 {
		return "null"
	}
	return strings.Join(parts, ",")
}

func faceitOutroGridColumns(columns []string) []string {
	present := map[string]bool{}
	for _, col := range columns {
		present[col] = true
	}
	var out []string
	for _, column := range defaultFaceitLayout.Outro.Columns {
		if present[column.ID] {
			out = append(out, column.ID)
		}
	}
	return out
}

func faceitOutroTeam(team TeamBoard, columns []string, layout OutroLayout, fontPath string, doc Document) []string {
	spec := defaultFaceitLayout.Outro
	palette := defaultFaceitLayout.Palette
	header := fmt.Sprintf("%s  %d", team.Name, team.Score)
	parts := []string{drawtext(fontPath, header, layout.Margin+spec.Header.X, layout.HeaderY+spec.Header.Y, spec.Header.FontSize, palette.Text)}
	if team.AverageELO != nil {
		avg := fmt.Sprintf("TEAM AVG  %d ELO", *team.AverageELO)
		parts = appendFilter(parts, drawtext(fontPath, avg, FrameWidth-layout.Margin-spec.TeamAverage.Right, layout.HeaderY+spec.TeamAverage.Y, spec.TeamAverage.FontSize, palette.MutedText))
	}
	labelY := layout.HeaderY + spec.ColumnLabelsY
	for _, col := range columns {
		column, ok := faceitOutroColumn(col)
		if !ok {
			continue
		}
		cx := layout.Margin + column.X
		parts = appendFilter(parts, drawtext(fontPath, column.Label, cx, labelY, spec.ColumnLabelSize, palette.MutedText))
	}
	players := team.Players
	if len(players) > spec.MaxPlayers {
		players = players[:spec.MaxPlayers]
	}
	for row, card := range players {
		rowY := layout.Row0 + row*layout.RowHeight
		y := rowY + spec.Name.Y
		nameColor := palette.Text
		if doc.IsPOV(card) {
			nameColor = palette.TargetText
		}
		parts = appendFilter(parts, drawtext(fontPath, card.Name, layout.Margin+spec.Name.X, y, spec.Name.FontSize, nameColor))
		if doc.IsPOV(card) {
			badge := spec.POV
			tagX := layout.Margin + badge.Rect.X
			tagY := rowY + badge.Rect.Y
			parts = append(parts, drawbox(tagX, tagY, badge.Rect.Width, badge.Rect.Height, palette.POVFill))
			parts = appendFilter(parts, drawtext(fontPath, "POV", tagX+badge.TextX, tagY+badge.TextY, badge.FontSize, palette.POVText))
		}
		for _, col := range columns {
			column, ok := faceitOutroColumn(col)
			if !ok {
				continue
			}
			cx := layout.Margin + column.X
			parts = appendFilter(parts, drawtext(fontPath, outroCell(card, col), cx, y, spec.StatSize, palette.StatText))
		}
	}
	return parts
}

func faceitOutroColumn(id string) (outroColumnSpec, bool) {
	for _, column := range defaultFaceitLayout.Outro.Columns {
		if column.ID == id {
			return column, true
		}
	}
	return outroColumnSpec{}, false
}

func outroGridColumns(columns []string) []string {
	present := map[string]bool{}
	for _, col := range columns {
		present[col] = true
	}
	var out []string
	for _, col := range []string{ColRating, ColKDA, ColADR, ColHSPct, ColELO, ColLevel} {
		if present[col] {
			out = append(out, col)
		}
	}
	return out
}

func outroTeam(team TeamBoard, columns []string, x int, layout OutroLayout, fontPath string, doc Document, geo OutroPlateGeometry, useGeo bool) []string {
	header := fmt.Sprintf("%s  %d", team.Name, team.Score)
	if team.AverageELO != nil {
		header += fmt.Sprintf("  %d ELO", *team.AverageELO)
	}
	parts := []string{drawtext(fontPath, header, x, layout.HeaderY, outroScoreSize, "white")}
	labelY := layout.HeaderY + 36
	if useGeo {
		labelY = geo.mappedColLabelY()
	}
	for i, col := range columns {
		cx := x + layout.NameWidth + i*layout.StatWidth
		parts = appendFilter(parts, drawtext(fontPath, outroColLabel(col), cx, labelY, outroColLabelSize, mutedColor))
	}
	players := team.Players
	if len(players) > outroMaxPlayersPerTeam {
		players = players[:outroMaxPlayersPerTeam]
	}
	topKills := 0
	for _, card := range players {
		if card.Kills > topKills {
			topKills = card.Kills
		}
	}
	rowY := layout.Row0
	maxRows := len(players)
	if useGeo {
		maxRows = min(maxRows, geo.rowCount())
	}
	for pi, card := range players {
		if pi >= maxRows {
			break
		}
		nameColor := "white"
		nameSize := outroNameSize
		if doc.IsPOV(card) {
			nameColor = "0xFCD34D"
			nameSize = outroTopNameSize
		} else if topKills > 0 && card.Kills == topKills {
			nameColor = "0xFCD34D"
		}
		nameY := rowY + 8
		statY := rowY + 38
		if useGeo {
			nameY = geo.rowNameY(pi)
			statY = geo.rowStatY(pi)
		}
		parts = appendFilter(parts, drawtext(fontPath, card.Name, x, nameY, nameSize, nameColor))
		if doc.IsPOV(card) {
			tagX := outroPlatePOVBadgeX(x, layout.NameWidth)
			parts = append(parts, drawbox(tagX, nameY+2, outroPlatePOVBadgeW, outroPlatePOVBadgeH, "0xF59E0B@0.95"))
			parts = appendFilter(parts, drawtext(fontPath, "POV", tagX+5, nameY+3, outroPOVLabelSize, "white"))
		}
		for i, col := range columns {
			cx := x + layout.NameWidth + i*layout.StatWidth
			parts = appendFilter(parts, drawtext(fontPath, outroCell(card, col), cx, statY, outroStatSize, mutedColor))
		}
		if !useGeo {
			rowY += layout.RowHeight
		}
	}
	return parts
}

func outroColLabel(col string) string {
	switch col {
	case ColRating:
		return "RATING"
	case ColKDA:
		return "K/D/A"
	case ColADR:
		return "ADR"
	case ColHSPct:
		return "HS%"
	case ColKR:
		return "K/R"
	case Col2K:
		return "2K"
	case Col3K:
		return "3K"
	case Col4K:
		return "4K"
	case Col5K:
		return "5K"
	case ColMVP:
		return "MVP"
	case ColELO:
		return "ELO"
	case ColLevel:
		return "LVL"
	default:
		return strings.ToUpper(col)
	}
}

func outroCell(card PlayerCard, col string) string {
	switch col {
	case ColELO:
		if card.ELO != nil {
			return strconv.Itoa(*card.ELO)
		}
	case ColLevel:
		if card.SkillLevel != nil {
			return strconv.Itoa(*card.SkillLevel)
		}
	case ColRating:
		if card.HasRating {
			return overlayDecimal(card.Rating, 2)
		}
	case ColKDA:
		return fmt.Sprintf("%d/%d/%d", card.Kills, card.Deaths, card.Assists)
	case ColADR:
		if card.HasADR {
			return overlayDecimal(card.ADR, 1)
		}
	case ColHSPct:
		if card.HasHSPct && (card.HSPct > 0 || card.Headshots > 0) {
			return fmt.Sprintf("%.0f%%", card.HSPct)
		}
	case ColKR:
		if card.Rounds > 0 {
			return overlayDecimal(float64(card.Kills)/float64(card.Rounds), 2)
		}
	case Col2K:
		return strconv.Itoa(card.Rounds2K)
	case Col3K:
		return strconv.Itoa(card.Rounds3K)
	case Col4K:
		return strconv.Itoa(card.Rounds4K)
	case Col5K:
		return strconv.Itoa(card.Rounds5K)
	case ColMVP:
		return strconv.Itoa(card.MVPs)
	}
	return "-"
}

func skillFill(level int) string {
	palette := defaultFaceitLayout.Palette
	switch {
	case level >= 10:
		return palette.Level10Fill
	case level >= 8:
		return palette.Level8Fill
	case level >= 4:
		return palette.Level4Fill
	default:
		return palette.LevelDefaultFill
	}
}

// countryBadgeFilters draws a compact country-code insignia. Full flag image
// assets remain pending license approval.
func countryBadgeFilters(fontPath string, x, y int, country string, theme CardTheme, badge badgeSpec) []string {
	code := strings.ToUpper(strings.TrimSpace(country))
	if code == "" {
		return nil
	}
	w, h := badge.Rect.Width, badge.Rect.Height
	parts := []string{
		drawbox(x, y, w, h, defaultFaceitLayout.Palette.IntroCardFill),
		drawbox(x, y, w, 1, theme.Accent+"@0.55"),
		drawbox(x, y+h-1, w, 1, theme.AccentSoft+"@0.45"),
	}
	parts = appendFilter(parts, drawtext(fontPath, code, x+badge.TextX, y+badge.TextY, badge.FontSize, defaultFaceitLayout.Palette.Text))
	return parts
}

func introPlayerCardChrome(x, y, w, h int, theme CardTheme) []string {
	if w < 8 || h < 8 {
		return nil
	}
	chrome := defaultFaceitLayout.Intro.Chrome
	palette := defaultFaceitLayout.Palette
	return []string{
		drawbox(x, y, w, h, palette.IntroCardFill),
		drawbox(x, y, chrome.CardAccentWidth, h, theme.Accent+"@0.72"),
		drawbox(x, y+h-1, w, 1, palette.IntroCardDivider),
	}
}

func drawbox(x, y, w, h int, color string) string {
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	return fmt.Sprintf("drawbox=x=%d:y=%d:w=%d:h=%d:color=%s:t=fill", x, y, w, h, color)
}

func drawtext(fontPath, text string, x, y, size int, color string) string {
	if strings.TrimSpace(text) == "" {
		return ""
	}
	return fmt.Sprintf(
		"drawtext=fontfile='%s':text='%s':fontsize=%d:fontcolor=%s:x=%d:y=%d:expansion=none:borderw=1:bordercolor=black@0.55",
		ffmpegFilterPath(fontPath),
		ffmpegDrawtextText(text),
		size,
		color,
		x,
		y,
	)
}

func appendFilter(parts []string, part string) []string {
	if part == "" {
		return parts
	}
	return append(parts, part)
}

func formatOptInt(label string, v *int) string {
	if v == nil {
		return ""
	}
	if label == "" {
		return strconv.Itoa(*v)
	}
	return fmt.Sprintf("%s %d", label, *v)
}

func formatOptFloat(label string, v *float64) string {
	if v == nil {
		return ""
	}
	if label == "" {
		return overlayDecimal(*v, 2)
	}
	return fmt.Sprintf("%s %s", label, overlayDecimal(*v, 2))
}

func formatOptPct(label string, v *float64) string {
	if v == nil {
		return ""
	}
	if label == "" {
		return fmt.Sprintf("%.0f%%", *v)
	}
	return fmt.Sprintf("%s %.0f%%", label, *v)
}

func formatOptSignedPct(label string, v *float64) string {
	if v == nil {
		return ""
	}
	sign := "+"
	if *v < 0 {
		sign = ""
	}
	if label == "" {
		return sign + overlayDecimal(*v, 2) + "%"
	}
	return fmt.Sprintf("%s %s%s%%", label, sign, overlayDecimal(*v, 2))
}

func ffmpegFilterPath(path string) string {
	path = strings.ReplaceAll(path, `\`, `/`)
	path = strings.ReplaceAll(path, `'`, `\'`)
	path = strings.ReplaceAll(path, `:`, `\:`)
	path = strings.ReplaceAll(path, `[`, `\[`)
	path = strings.ReplaceAll(path, `]`, `\]`)
	return path
}

// ffmpegDrawtextText escapes a value for drawtext's text option. Every
// drawtext clause here sets expansion=none, so % is literal and must not be
// doubled: escaping it as %% painted "79%%" on intro/outro stat cells.
func ffmpegDrawtextText(text string) string {
	text = strings.ReplaceAll(text, `\`, `\\`)
	text = strings.ReplaceAll(text, `'`, `\'`)
	text = strings.ReplaceAll(text, `:`, `\:`)
	return text
}
