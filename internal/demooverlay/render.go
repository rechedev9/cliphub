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
	header := strings.TrimSpace(teamName)
	if header == "" {
		header = introHeader(doc.Source)
	}
	if NormalizeSource(doc.Source) == SourceFACEIT && !hasPlate {
		return faceitIntroColumn(cards, header, subtitle, x, y, layout, fontPath, doc)
	}
	var parts []string
	geo, useGeo := IntroPlateGeo(doc.Source, hasPlate)
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
		parts = appendFilter(parts, drawtext(fontPath, header, x+20, y+18, layout.NameSize+2, palette.Text))
		if sub := strings.TrimSpace(subtitle); sub != "" {
			parts = appendFilter(parts, drawtext(fontPath, sub, x+20, y+50, layout.LabelSize+2, palette.MutedText))
		}
	}
	textInset := IntroTextInset(layout, doc.Source, hasPlate)
	skipMonogram := hasPlate && NormalizeSource(doc.Source) == SourceFACEIT
	for i, card := range cards {
		var cy, nameY int
		if useGeo && i < len(geo.RowNameCenterY) {
			cy = geo.RowNameCenterY[i]
			nameY = cy - layout.NameSize/2
		} else {
			cy = y + layout.HeaderH + i*layout.RowHeight
			nameY = cy + 10
		}
		nx := x + textInset
		ax := x + layout.AvatarXOff
		ay := cy + layout.AvatarYOff
		if useGeo && i < len(geo.RowNameCenterY) {
			ay = geo.RowNameCenterY[i] - layout.AvatarSize/2
		}
		if !skipMonogram && strings.TrimSpace(card.AvatarFile) == "" && strings.TrimSpace(card.AvatarURL) == "" {
			parts = append(parts, monogramFilters(fontPath, ax, ay, layout.AvatarSize, card.Name, true)...)
		}
		nameColor := palette.Text
		if doc.IsPOV(card) {
			nameColor = palette.TargetText
		}
		parts = appendFilter(parts, drawtext(fontPath, card.Name, nx, nameY, layout.NameSize, nameColor))
		if doc.IsPOV(card) {
			tag := rectSpec{X: introPlatePOVBadgeX(x, layout.PanelWidth), Y: nameY, Width: introPlatePOVBadgeW, Height: introPlatePOVBadgeH}
			parts = append(parts, drawbox(tag.X, tag.Y, tag.Width, tag.Height, "0xF59E0B@0.95"))
			parts = appendFilter(parts, drawtextInRect(fontPath, "POV", tag, 10, "white"))
		}
		if card.Country != "" {
			countryY := nameY + layout.NameSize + 4
			if !useGeo {
				countryY = cy + 10 + layout.NameSize + 4
			}
			country := badgeSpec{Rect: rectSpec{X: nx, Y: countryY, Width: 34, Height: 16}, FontSize: 10}
			parts = append(parts, countryBadgeFilters(fontPath, card.Country, theme, country)...)
		}
		if card.ELO != nil {
			eloX := x + layout.PanelWidth - layout.BadgeSize - 132
			parts = appendFilter(parts, drawtext(fontPath, strconv.Itoa(*card.ELO), eloX, nameY+8, 18, palette.Text))
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
			box := rectSpec{X: x + layout.PanelWidth - layout.BadgeSize - 18, Y: nameY + 6, Width: layout.BadgeSize, Height: layout.BadgeSize}
			parts = append(parts, drawbox(box.X, box.Y, box.Width, box.Height, badgeFill))
			parts = appendFilter(parts, drawtextInRect(fontPath, badge, box, 12, palette.Text))
		}
		statsY := cy + 112
		titleY := statsY - 14
		statsWidth := layout.PanelWidth - textInset - 16
		if useGeo && i < len(geo.RowNameCenterY) {
			statsY = geo.RowNameCenterY[i] + layout.NameSize/2 + 12
			titleY = statsY - 14
		}
		if title := introStatsSectionTitle(card, doc.Source); title != "" {
			parts = appendFilter(parts, drawtext(fontPath, title, nx, titleY, 9, palette.MutedText))
		}
		parts = append(parts, introStatGrid(card, nx, statsY, statsWidth, fontPath, layout, doc.Source)...)
	}
	return parts
}

// faceitIntroColumn draws the text layer of one FACEIT roster panel. Every
// box, ring and band already exists in the generated chrome bitmap
// (renderIntroChromePNG); this only places text into the same geometry.
func faceitIntroColumn(cards []PlayerCard, header, subtitle string, x, y int, layout IntroLayout, fontPath string, doc Document) []string {
	spec := defaultFaceitLayout.Intro
	palette := defaultFaceitLayout.Palette
	var parts []string
	parts = appendFilter(parts, drawtext(fontPath, header, x+spec.Header.Name.X, y+spec.Header.Name.Y, spec.Header.Name.FontSize, palette.Text))
	if sub := strings.TrimSpace(subtitle); sub != "" {
		parts = appendFilter(parts, drawtext(fontPath, sub, x+spec.Header.Subtitle.X, y+spec.Header.Subtitle.Y, spec.Header.Subtitle.FontSize, palette.MutedText))
	}
	for i, card := range cards {
		cy := y + layout.HeaderH + i*layout.RowHeight
		isPOV := doc.IsPOV(card)
		geo := faceitIntroCardGeometry(x, cy, card, isPOV)
		if strings.TrimSpace(card.AvatarFile) == "" && strings.TrimSpace(card.AvatarURL) == "" {
			parts = append(parts, monogramFilters(fontPath, geo.Avatar.X, geo.Avatar.Y, geo.Avatar.Width, card.Name, false)...)
		}
		nameColor := palette.Text
		if isPOV {
			nameColor = palette.TargetText
		}
		parts = appendFilter(parts, drawtext(fontPath, card.Name, x+spec.Name.X, cy+spec.Name.Y, spec.Name.FontSize, nameColor))
		if card.Country != "" {
			parts = appendFilter(parts, drawtextInRect(fontPath, strings.ToUpper(strings.TrimSpace(card.Country)), geo.Country, spec.Country.FontSize, palette.Text))
		}
		if isPOV {
			parts = appendFilter(parts, drawtextInRect(fontPath, "POV", geo.POV, spec.POV.FontSize, palette.POVText))
		}
		if card.ELO != nil {
			parts = appendFilter(parts, drawtextRight(fontPath, strconv.Itoa(*card.ELO), geo.ELORight, cy+spec.ELO.Y, spec.ELO.FontSize, palette.Text))
			parts = appendFilter(parts, drawtextRight(fontPath, "ELO", geo.ELORight, cy+spec.ELOLabel.Y, spec.ELOLabel.FontSize, palette.MutedText))
		}
		if card.SkillLevel != nil {
			parts = appendFilter(parts, drawtextInRect(fontPath, strconv.Itoa(*card.SkillLevel), geo.Level, spec.Level.FontSize, levelTextColor(*card.SkillLevel)))
		}
		if card.Ranking != nil {
			parts = appendFilter(parts, drawtextInRect(fontPath, "#"+formatThousands(*card.Ranking), geo.Rank, spec.Rank.FontSize, palette.RankText))
		}
		statsX := x + spec.Stats.X
		statsWidth := layout.PanelWidth - spec.Stats.X - spec.Stats.Right
		if title := introStatsSectionTitle(card, doc.Source); title != "" {
			parts = appendFilter(parts, drawtext(fontPath, strings.ToUpper(title), statsX, cy+spec.Stats.TitleY, spec.Stats.TitleSize, palette.MutedText))
		}
		parts = append(parts, introStatGrid(card, statsX, cy+spec.Stats.ValueY, statsWidth, fontPath, layout, doc.Source)...)
	}
	return parts
}

// monogramFilters draws the fallback initial for a roster slot without an
// avatar. withDisc also paints the placeholder box; generated FACEIT chrome
// already carries an anti-aliased disc, so it passes false.
func monogramFilters(fontPath string, x, y, size int, name string, withDisc bool) []string {
	initial := monogramInitial(name)
	if initial == "" {
		return nil
	}
	var parts []string
	if withDisc {
		parts = append(parts, drawbox(x, y, size, size, defaultFaceitLayout.Palette.AvatarFill))
	}
	fontSize := size / 2
	if fontSize < 18 {
		fontSize = 18
	}
	parts = appendFilter(parts, drawtextInRect(fontPath, initial, rectSpec{X: x, Y: y, Width: size, Height: size}, fontSize, "white"))
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
		label := stat.label
		if NormalizeSource(source) == SourceFACEIT {
			label = strings.ToUpper(label)
		}
		parts = appendFilter(parts, drawtext(fontPath, label, sx, sy+valueSize+labelGap, labelSize, palette.MutedText))
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
	palette := defaultFaceitLayout.Palette
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
	if mapLabel := displayMapName(doc.Map); mapLabel != "" && len(doc.Outro.Teams) >= 2 {
		parts = appendFilter(parts, drawtextCentered(fontPath, strings.ToUpper(mapLabel), FrameWidth/2, spec.MapLabel.Y, spec.MapLabel.FontSize, palette.MutedText))
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

// faceitOutroTeam draws the text layer of one FACEIT scoreboard. The score
// chip, label band, side stripes and POV pill live in renderOutroChromePNG;
// text is centered into that geometry with drawtext text_w/text_h.
func faceitOutroTeam(team TeamBoard, columns []string, layout OutroLayout, fontPath string, doc Document) []string {
	spec := defaultFaceitLayout.Outro
	palette := defaultFaceitLayout.Palette
	scoreChip := rectSpec{X: layout.Margin + spec.Score.Rect.X, Y: layout.HeaderY + spec.Score.Rect.Y, Width: spec.Score.Rect.Width, Height: spec.Score.Rect.Height}
	parts := []string{drawtextInRect(fontPath, strconv.Itoa(team.Score), scoreChip, spec.Score.FontSize, palette.Text)}
	nameRow := rectSpec{X: layout.Margin + spec.Header.X, Y: scoreChip.Y + spec.Header.Y, Width: FrameWidth - 2*layout.Margin - spec.Header.X, Height: scoreChip.Height}
	parts = appendFilter(parts, drawtextLeftMiddle(fontPath, team.Name, nameRow, spec.Header.FontSize, palette.Text))
	if team.AverageELO != nil {
		right := FrameWidth - layout.Margin - spec.TeamAverage.Right
		parts = appendFilter(parts, drawtextRight(fontPath, strconv.Itoa(*team.AverageELO), right, layout.HeaderY+spec.TeamAverage.Y, spec.TeamAverage.FontSize, palette.Text))
		parts = appendFilter(parts, drawtextRight(fontPath, "AVG ELO", FrameWidth-layout.Margin-spec.TeamAverageLabel.Right, layout.HeaderY+spec.TeamAverageLabel.Y, spec.TeamAverageLabel.FontSize, palette.MutedText))
	}
	labelY := layout.HeaderY + spec.ColumnLabelsY
	parts = appendFilter(parts, drawtext(fontPath, "PLAYER", layout.Margin+spec.Name.X, labelY, spec.ColumnLabelSize, palette.MutedText))
	for _, col := range columns {
		column, ok := faceitOutroColumn(col)
		if !ok {
			continue
		}
		cx := layout.Margin + column.X + column.Width/2
		parts = appendFilter(parts, drawtextCentered(fontPath, column.Label, cx, labelY, spec.ColumnLabelSize, palette.MutedText))
	}
	players := team.Players
	if len(players) > spec.MaxPlayers {
		players = players[:spec.MaxPlayers]
	}
	for row, card := range players {
		rowY := layout.Row0 + row*layout.RowHeight
		rowH := layout.RowHeight - spec.Chrome.RowBottomGap
		isPOV := doc.IsPOV(card)
		nameColor := palette.Text
		nameX := layout.Margin + spec.Name.X
		if isPOV {
			nameColor = palette.TargetText
			nameX = layout.Margin + spec.POVNameX
			pov := rectSpec{X: layout.Margin + spec.POV.Rect.X, Y: rowY + spec.POV.Rect.Y, Width: spec.POV.Rect.Width, Height: spec.POV.Rect.Height}
			parts = appendFilter(parts, drawtextInRect(fontPath, "POV", pov, spec.POV.FontSize, palette.POVText))
		}
		nameCell := rectSpec{X: nameX, Y: rowY + spec.Name.Y, Width: layout.Margin + layout.NameWidth - nameX, Height: rowH}
		parts = appendFilter(parts, drawtextLeftMiddle(fontPath, card.Name, nameCell, spec.Name.FontSize, nameColor))
		for _, col := range columns {
			column, ok := faceitOutroColumn(col)
			if !ok {
				continue
			}
			cell := rectSpec{X: layout.Margin + column.X, Y: rowY, Width: column.Width, Height: rowH}
			parts = appendFilter(parts, drawtextInRect(fontPath, outroCell(card, col), cell, spec.StatSize, outroCellColor(card, col)))
		}
	}
	return parts
}

// outroCellColor tints the cells that carry a judgment: FACEIT level in its
// tier color, rating green above 1.15 and red below 0.85. Everything else
// stays the neutral stat color so the table does not turn into a rainbow.
func outroCellColor(card PlayerCard, col string) string {
	palette := defaultFaceitLayout.Palette
	switch col {
	case ColLevel:
		if card.SkillLevel != nil {
			return levelTextColor(*card.SkillLevel)
		}
	case ColRating:
		if card.HasRating {
			switch {
			case card.Rating >= 1.15:
				return palette.StatPositive
			case card.Rating <= 0.85:
				return palette.StatNegative
			}
		}
	}
	return palette.StatText
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

// skillFill maps a FACEIT skill level onto the official tier colors: 10 red,
// 8-9 orange, 4-7 yellow, 2-3 green, 1 grey.
func skillFill(level int) string {
	palette := defaultFaceitLayout.Palette
	switch {
	case level >= 10:
		return palette.Level10Fill
	case level >= 8:
		return palette.Level8Fill
	case level >= 4:
		return palette.Level4Fill
	case level >= 2:
		return palette.Level2Fill
	default:
		return palette.LevelDefaultFill
	}
}

// levelTextColor is the tier color without its fill alpha, for drawtext.
func levelTextColor(level int) string {
	fill := skillFill(level)
	if i := strings.Index(fill, "@"); i >= 0 {
		return fill[:i]
	}
	return fill
}

// countryBadgeFilters draws a compact country-code insignia for the plate and
// legacy chrome paths. Full flag image assets remain pending license approval.
func countryBadgeFilters(fontPath string, country string, theme CardTheme, badge badgeSpec) []string {
	code := strings.ToUpper(strings.TrimSpace(country))
	if code == "" {
		return nil
	}
	r := badge.Rect
	parts := []string{
		drawbox(r.X, r.Y, r.Width, r.Height, defaultFaceitLayout.Palette.IntroCardFill),
		drawbox(r.X, r.Y, r.Width, 1, theme.Accent+"@0.55"),
		drawbox(r.X, r.Y+r.Height-1, r.Width, 1, theme.AccentSoft+"@0.45"),
	}
	parts = appendFilter(parts, drawtextInRect(fontPath, code, r, badge.FontSize, defaultFaceitLayout.Palette.Text))
	return parts
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
	return drawtextExpr(fontPath, text, strconv.Itoa(x), strconv.Itoa(y), size, color)
}

// drawtextRight right-aligns text so its last glyph ends at right.
func drawtextRight(fontPath, text string, right, y, size int, color string) string {
	return drawtextExpr(fontPath, text, fmt.Sprintf("%d-text_w", right), strconv.Itoa(y), size, color)
}

// drawtextCentered centers text horizontally on cx.
func drawtextCentered(fontPath, text string, cx, y, size int, color string) string {
	return drawtextExpr(fontPath, text, fmt.Sprintf("%d-text_w/2", cx), strconv.Itoa(y), size, color)
}

// drawtextInRect centers text on both axes inside rect, which is how every
// badge, pill and table cell keeps its glyphs centered without guessing
// font metrics.
func drawtextInRect(fontPath, text string, rect rectSpec, size int, color string) string {
	return drawtextExpr(fontPath, text,
		fmt.Sprintf("%d+(%d-text_w)/2", rect.X, rect.Width),
		fmt.Sprintf("%d+(%d-line_h)/2", rect.Y, rect.Height),
		size, color)
}

// drawtextLeftMiddle left-aligns text at rect.X, vertically centered in rect.
func drawtextLeftMiddle(fontPath, text string, rect rectSpec, size int, color string) string {
	return drawtextExpr(fontPath, text, strconv.Itoa(rect.X), fmt.Sprintf("%d+(%d-line_h)/2", rect.Y, rect.Height), size, color)
}

// drawtextExpr emits one drawtext clause. xExpr and yExpr are ffmpeg
// expressions (text_w/text_h allowed); they must not contain commas.
func drawtextExpr(fontPath, text, xExpr, yExpr string, size int, color string) string {
	if strings.TrimSpace(text) == "" {
		return ""
	}
	// Light text gets a faint dark outline for legibility over gameplay; dark
	// text on a bright pill would only smear under the same outline.
	border := ":borderw=1:bordercolor=black@0.55"
	if isDarkOverlayColor(color) {
		border = ""
	}
	return fmt.Sprintf(
		"drawtext=fontfile='%s':text='%s':fontsize=%d:fontcolor=%s:x=%s:y=%s:expansion=none%s",
		ffmpegFilterPath(fontPath),
		ffmpegDrawtextText(text),
		size,
		color,
		xExpr,
		yExpr,
		border,
	)
}

// isDarkOverlayColor reports whether a 0xRRGGBB[@a] color reads as dark
// (relative luminance below ~30%), i.e. text meant for a bright pill.
func isDarkOverlayColor(value string) bool {
	c := parseOverlayColor(value)
	if !strings.HasPrefix(value, "0x") {
		return false
	}
	luma := 0.2126*float64(c.R) + 0.7152*float64(c.G) + 0.0722*float64(c.B)
	return luma < 76
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
