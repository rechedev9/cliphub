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
func RenderPNGs(ffmpegPath, fontPath string, doc Document, introPath, outroPath string) error {
	if strings.TrimSpace(ffmpegPath) == "" {
		return fmt.Errorf("render full-demo overlay: ffmpeg path is required")
	}
	if strings.TrimSpace(fontPath) == "" {
		return fmt.Errorf("render full-demo overlay: font path is required")
	}
	if err := renderStill(ffmpegPath, introPath, introFilter(doc, fontPath), introChromePNG, introAvatarSlots(doc)); err != nil {
		return fmt.Errorf("render intro overlay: %w", err)
	}
	if err := renderStill(ffmpegPath, outroPath, outroFilter(doc, fontPath), outroChromePNG, nil); err != nil {
		return fmt.Errorf("render outro overlay: %w", err)
	}
	return nil
}

type avatarSlot struct {
	Path string
	X, Y int
	Size int
}

func introAvatarSlots(doc Document) []avatarSlot {
	l := DefaultLayout()
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
			slots = append(slots, avatarSlot{
				Path: card.AvatarFile,
				X:    x + l.Intro.AvatarXOff,
				Y:    y + l.Intro.HeaderH + i*l.Intro.RowHeight + l.Intro.AvatarYOff,
				Size: l.Intro.AvatarSize,
			})
		}
	}
	add(doc.Intro.Left, l.Intro.LeftPanelX, l.Intro.PanelTop)
	add(doc.Intro.Right, l.Intro.RightPanelX, l.Intro.PanelTop)
	return slots
}

func renderStill(ffmpegPath, outPath, vf string, chrome []byte, avatars []avatarSlot) error {
	if strings.TrimSpace(outPath) == "" {
		return fmt.Errorf("output path is required")
	}
	if err := os.MkdirAll(filepath.Dir(outPath), 0o750); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}
	chromeFile, err := os.CreateTemp("", "cliphub-chrome-*.png")
	if err != nil {
		return fmt.Errorf("create overlay chrome: %w", err)
	}
	chromePath := chromeFile.Name()
	_ = chromeFile.Close()
	defer func() { _ = os.Remove(chromePath) }()
	if err := writeChrome(chromePath, chrome); err != nil {
		return err
	}
	script, err := os.CreateTemp("", "cliphub-overlay-*.fffilter")
	if err != nil {
		return fmt.Errorf("create overlay filter script: %w", err)
	}
	scriptPath := script.Name()
	defer func() { _ = os.Remove(scriptPath) }()
	args := []string{"-y", "-hide_banner", "-loglevel", "error", "-i", chromePath}
	for _, slot := range avatars {
		args = append(args, "-i", slot.Path)
	}
	if _, err := script.WriteString(stillFilterGraph(vf, avatars)); err != nil {
		_ = script.Close()
		return fmt.Errorf("write overlay filter script: %w", err)
	}
	if err := script.Close(); err != nil {
		return fmt.Errorf("close overlay filter script: %w", err)
	}
	args = append(args, "-filter_complex_script", scriptPath, "-frames:v", "1", "-pix_fmt", "rgba", outPath)
	// #nosec G204 -- ffmpegPath is the host FFmpeg resolved by ClipHub config.
	cmd := exec.Command(ffmpegPath, args...) //nolint:gosec
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func stillFilterGraph(text string, avatars []avatarSlot) string {
	current := "[0:v]"
	var clauses []string
	for i, slot := range avatars {
		scaled := fmt.Sprintf("a%d", i)
		clauses = append(clauses, fmt.Sprintf(
			"[%d:v]scale=%d:%d:flags=lanczos,format=rgba,geq=r='r(X,Y)':g='g(X,Y)':b='b(X,Y)':a='if(lte(hypot(X-W/2,Y-H/2),min(W,H)/2),255,0)'[%s]",
			i+1, slot.Size, slot.Size, scaled,
		))
		next := fmt.Sprintf("s%d", i)
		clauses = append(clauses, fmt.Sprintf("%s[%s]overlay=%d:%d:format=auto[%s]", current, scaled, slot.X, slot.Y, next))
		current = "[" + next + "]"
	}
	if text == "" || text == "null" {
		if len(clauses) == 0 {
			return "[0:v]format=rgba"
		}
		return strings.Join(clauses, ";")
	}
	if len(clauses) == 0 {
		return current + text
	}
	return strings.Join(clauses, ";") + ";" + current + text
}

func introFilter(doc Document, fontPath string) string {
	l := DefaultLayout()
	var parts []string
	parts = append(parts, introColumn(doc.Intro.Left, l.Intro.LeftPanelX, l.Intro.PanelTop, l.Intro, fontPath)...)
	parts = append(parts, introColumn(doc.Intro.Right, l.Intro.RightPanelX, l.Intro.PanelTop, l.Intro, fontPath)...)
	if len(parts) == 0 {
		return "null"
	}
	return strings.Join(parts, ",")
}

func introColumn(cards []PlayerCard, x, y int, layout IntroLayout, fontPath string) []string {
	if len(cards) > layout.MaxPlayers {
		cards = cards[:layout.MaxPlayers]
	}
	var parts []string
	parts = appendFilter(parts, drawtext(fontPath, "PLAYERS", x+20, y+10, layout.LabelSize+2, mutedColor))
	for i, card := range cards {
		cy := y + layout.HeaderH + i*layout.RowHeight
		nx := x + layout.CardInset
		parts = appendFilter(parts, drawtext(fontPath, card.Name, nx, cy+10, layout.NameSize, "white"))
		if card.Country != "" {
			parts = appendFilter(parts, drawtext(fontPath, strings.ToUpper(card.Country), nx, cy+10+layout.NameSize+4, layout.LabelSize, mutedColor))
		}
		if card.ELO != nil {
			eloX := x + layout.PanelWidth - layout.BadgeSize - 132
			parts = appendFilter(parts, drawtext(fontPath, strconv.Itoa(*card.ELO), eloX, cy+18, 18, "white"))
		}
		badge := ""
		badgeFill := skillFill(10)
		if card.Ranking != nil {
			badge = "#" + strconv.Itoa(*card.Ranking)
			badgeFill = "0xEB1923@0.95"
		} else if card.SkillLevel != nil {
			badge = strconv.Itoa(*card.SkillLevel)
			badgeFill = skillFill(*card.SkillLevel)
		}
		if badge != "" {
			bx := x + layout.PanelWidth - layout.BadgeSize - 18
			by := cy + 16
			parts = append(parts, drawbox(bx, by, layout.BadgeSize, layout.BadgeSize, badgeFill))
			parts = appendFilter(parts, drawtext(fontPath, badge, bx+2, by+5, 12, "white"))
		}
		statsY := cy + 108
		if card.Last20 != nil {
			parts = appendFilter(parts, drawtext(fontPath, "Last 20 matches", nx, statsY-14, 9, mutedColor))
		}
		parts = append(parts, introStatGrid(card, nx, statsY, layout.PanelWidth-layout.CardInset-16, fontPath, layout)...)
	}
	return parts
}

type overlayStat struct {
	label string
	value string
}

func introStatGrid(card PlayerCard, x, y, width int, fontPath string, layout IntroLayout) []string {
	stats := introStats(card)
	if len(stats) == 0 {
		return nil
	}
	offsets := statColumnOffsets(width, len(stats))
	var parts []string
	for i, stat := range stats {
		if i >= len(offsets) {
			break
		}
		sx := x + offsets[i]
		sy := y
		parts = appendFilter(parts, drawtext(fontPath, stat.value, sx, sy, layout.StatSize, "white"))
		parts = appendFilter(parts, drawtext(fontPath, stat.label, sx, sy+layout.StatSize+2, layout.LabelSize, mutedColor))
	}
	return parts
}

// introStatWeights stretch ADR/HS and K/D / K/R in the four-column Last 20 grid.
var introStatWeights = []float64{1.0, 1.0, 1.25, 1.25}

func statColumnOffsets(width, n int) []int {
	if n <= 0 || width <= 0 {
		return nil
	}
	if n > len(introStatWeights) {
		n = len(introStatWeights)
	}
	sum := 0.0
	for i := 0; i < n; i++ {
		sum += introStatWeights[i]
	}
	out := make([]int, n)
	cursor := 0.0
	for i := 0; i < n; i++ {
		out[i] = int(cursor)
		cursor += float64(width) * introStatWeights[i] / sum
	}
	return out
}

func introStats(card PlayerCard) []overlayStat {
	if card.Last20 == nil {
		return []overlayStat{{label: "K/D/A", value: overlayKDA(card)}}
	}
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

func introADRHS(card PlayerCard) (label, value string) {
	return "ADR", formatOptFloat("", last20Float(card, func(l Last20) *float64 { return l.ADR }))
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

func outroFilter(doc Document, fontPath string) string {
	l := DefaultLayout()
	teams := doc.Outro.Teams
	cols := outroGridColumns(doc.Outro.Columns)
	var parts []string
	if len(teams) == 2 {
		parts = append(parts, outroTeam(teams[0], cols, l.Outro.Margin, l.Outro, fontPath)...)
		parts = append(parts, outroTeam(teams[1], cols, l.Outro.Margin+l.Outro.ColGap, l.Outro, fontPath)...)
		if len(parts) == 0 {
			return "null"
		}
		return strings.Join(parts, ",")
	}
	layout := l.Outro
	for i, team := range teams {
		shifted := layout
		shifted.HeaderY = layout.HeaderY + i*(layout.HeaderH+len(team.Players)*layout.RowHeight+16)
		shifted.Row0 = shifted.HeaderY + layout.HeaderH
		parts = append(parts, outroTeam(team, cols, layout.Margin, shifted, fontPath)...)
	}
	if len(parts) == 0 {
		return "null"
	}
	return strings.Join(parts, ",")
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

func outroTeam(team TeamBoard, columns []string, x int, layout OutroLayout, fontPath string) []string {
	header := fmt.Sprintf("%s  %d", team.Name, team.Score)
	if team.AverageELO != nil {
		header += fmt.Sprintf("  %d ELO", *team.AverageELO)
	}
	parts := []string{drawtext(fontPath, header, x, layout.HeaderY, 26, "white")}
	labelY := layout.HeaderY + 32
	for i, col := range columns {
		cx := x + layout.NameWidth + i*layout.StatWidth
		parts = appendFilter(parts, drawtext(fontPath, outroColLabel(col), cx, labelY, 11, mutedColor))
	}
	rowY := layout.Row0
	for _, card := range team.Players {
		parts = appendFilter(parts, drawtext(fontPath, card.Name, x, rowY+8, 20, "white"))
		for i, col := range columns {
			cx := x + layout.NameWidth + i*layout.StatWidth
			parts = appendFilter(parts, drawtext(fontPath, outroCell(card, col), cx, rowY+36, 16, mutedColor))
		}
		rowY += layout.RowHeight
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
	}
	return ""
}

func skillFill(level int) string {
	switch {
	case level >= 10:
		return "0xEB1923@0.95"
	case level >= 8:
		return "0xF59E0B@0.95"
	case level >= 4:
		return "0x7C3AED@0.95"
	default:
		return "0x22C55E@0.95"
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

func ffmpegDrawtextText(text string) string {
	text = strings.ReplaceAll(text, `\`, `\\`)
	text = strings.ReplaceAll(text, `'`, `\'`)
	text = strings.ReplaceAll(text, `:`, `\:`)
	text = strings.ReplaceAll(text, `%`, `%%`)
	return text
}
