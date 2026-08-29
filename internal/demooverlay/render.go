package demooverlay

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// OverlayWindows returns the compiled-timeline windows for intro and outro
// image overlays. Intro is the opening freeze prefix; outro is the hold
// after the last live round.
func OverlayWindows(durationSeconds float64) (introStart, introEnd, outroStart, outroEnd float64) {
	if durationSeconds <= 0 {
		return 0, 0, 0, 0
	}
	introEnd = IntroSeconds
	if introEnd > durationSeconds {
		introEnd = durationSeconds
	}
	outroStart = durationSeconds - OutroSeconds
	if outroStart < introEnd {
		outroStart = introEnd
	}
	if outroStart < 0 {
		outroStart = 0
	}
	return 0, introEnd, outroStart, durationSeconds
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
	if err := renderStill(ffmpegPath, introPath, introFilter(doc, fontPath)); err != nil {
		return fmt.Errorf("render intro overlay: %w", err)
	}
	if err := renderStill(ffmpegPath, outroPath, outroFilter(doc, fontPath)); err != nil {
		return fmt.Errorf("render outro overlay: %w", err)
	}
	return nil
}

func renderStill(ffmpegPath, outPath, vf string) error {
	if strings.TrimSpace(outPath) == "" {
		return fmt.Errorf("output path is required")
	}
	if err := os.MkdirAll(filepath.Dir(outPath), 0o750); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}
	// #nosec G204 -- ffmpegPath is the host FFmpeg resolved by ClipHub config.
	cmd := exec.Command(ffmpegPath, //nolint:gosec
		"-y", "-hide_banner", "-loglevel", "error",
		"-f", "lavfi",
		"-i", fmt.Sprintf("color=c=black@0.0:s=%dx%d:d=1,format=rgba", FrameWidth, FrameHeight),
		"-vf", vf,
		"-frames:v", "1",
		outPath,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func introFilter(doc Document, fontPath string) string {
	l := DefaultLayout()
	var parts []string
	parts = append(parts,
		drawbox(l.Intro.LeftPanelX, l.Intro.PanelTop, l.Intro.PanelWidth, l.Intro.PanelHeight, "black@0.72"),
		drawbox(l.Intro.RightPanelX, l.Intro.PanelTop, l.Intro.PanelWidth, l.Intro.PanelHeight, "black@0.72"),
	)
	parts = append(parts, introColumn(doc.Intro.Left, doc.Intro.Columns, l.Intro.LeftPanelX, l.Intro.PanelTop, l.Intro, fontPath)...)
	parts = append(parts, introColumn(doc.Intro.Right, doc.Intro.Columns, l.Intro.RightPanelX, l.Intro.PanelTop, l.Intro, fontPath)...)
	return strings.Join(parts, ",")
}

func introColumn(cards []PlayerCard, columns []string, x, y int, layout IntroLayout, fontPath string) []string {
	if len(cards) > layout.MaxPlayers {
		cards = cards[:layout.MaxPlayers]
	}
	var parts []string
	for i, card := range cards {
		rowY := y + 12 + i*layout.RowHeight
		parts = appendFilter(parts, drawtext(fontPath, card.Name, x+16, rowY, 28, "white"))
		lineY := rowY + 36
		for _, col := range columns {
			text := introCell(card, col)
			if text == "" || col == ColName {
				continue
			}
			parts = appendFilter(parts, drawtext(fontPath, text, x+16, lineY, 18, "white"))
			lineY += 22
		}
	}
	return parts
}

func introCell(card PlayerCard, col string) string {
	switch col {
	case ColCountry:
		return strings.ToUpper(card.Country)
	case ColELO:
		return formatOptInt("ELO", card.ELO)
	case ColLevel:
		return formatOptInt("LVL", card.SkillLevel)
	case ColMatches:
		if card.Last20 != nil {
			return formatOptInt("Matches", card.Last20.Matches)
		}
	case ColWinPct:
		if card.Last20 != nil {
			return formatOptPct("Win%", card.Last20.WinPct)
		}
	case ColRating:
		if card.Last20 != nil {
			return formatOptFloat("Rating", card.Last20.Rating)
		}
	case ColSwing:
		if card.Last20 != nil {
			return formatOptSignedPct("Swing", card.Last20.Swing)
		}
	case ColKDA:
		if card.Last20 != nil && (card.Last20.Kills != nil || card.Last20.Deaths != nil) {
			return fmt.Sprintf("K/D/A %s/%s/%s", optInt(card.Last20.Kills), optInt(card.Last20.Deaths), optInt(card.Last20.Assists))
		}
		return fmt.Sprintf("K/D/A %d/%d/%d", card.Kills, card.Deaths, card.Assists)
	case ColKD:
		if card.Last20 != nil {
			return formatOptFloat("K/D", card.Last20.KD)
		}
	case ColKR:
		if card.Last20 != nil {
			return formatOptFloat("K/R", card.Last20.KR)
		}
	case ColADR:
		if card.Last20 != nil {
			return formatOptFloat("ADR", card.Last20.ADR)
		}
	}
	return ""
}

func outroFilter(doc Document, fontPath string) string {
	l := DefaultLayout()
	parts := []string{drawbox(0, 0, FrameWidth, FrameHeight, "black@0.92")}
	y := l.Outro.Margin
	for _, team := range doc.Outro.Teams {
		header := fmt.Sprintf("%s  %d", team.Name, team.Score)
		if team.AverageELO != nil {
			header += fmt.Sprintf("  %d ELO", *team.AverageELO)
		}
		parts = appendFilter(parts, drawtext(fontPath, header, l.Outro.Margin, y, 36, "white"))
		y += l.Outro.HeaderH
		for _, card := range team.Players {
			line := outroLine(card, doc.Outro.Columns)
			parts = appendFilter(parts, drawtext(fontPath, line, l.Outro.Margin+16, y, 22, "white"))
			y += l.Outro.RowHeight
		}
		y += 16
	}
	return strings.Join(parts, ",")
}

func outroLine(card PlayerCard, columns []string) string {
	var parts []string
	for _, col := range columns {
		switch col {
		case ColName:
			parts = append(parts, card.Name)
		case ColELO:
			if card.ELO != nil {
				parts = append(parts, strconv.Itoa(*card.ELO))
			}
		case ColLevel:
			if card.SkillLevel != nil {
				parts = append(parts, fmt.Sprintf("L%d", *card.SkillLevel))
			}
		case ColRating:
			if card.HasRating {
				parts = append(parts, fmt.Sprintf("%.2f", card.Rating))
			}
		case ColSwing:
			if card.Last20 != nil && card.Last20.Swing != nil {
				parts = append(parts, formatOptSignedPct("", card.Last20.Swing))
			}
		case ColKDA:
			parts = append(parts, fmt.Sprintf("%d/%d/%d", card.Kills, card.Deaths, card.Assists))
		case ColADR:
			if card.HasADR {
				parts = append(parts, fmt.Sprintf("%.1f", card.ADR))
			}
		case ColHS:
			if card.Headshots > 0 {
				parts = append(parts, fmt.Sprintf("HS %d", card.Headshots))
			}
		case ColHSPct:
			if card.HasHSPct {
				parts = append(parts, fmt.Sprintf("%.0f%%", card.HSPct))
			}
		case ColMulti:
			parts = append(parts, fmt.Sprintf("%d/%d/%d/%d", card.Rounds5K, card.Rounds4K, card.Rounds3K, card.Rounds2K))
		case ColMVP:
			if card.MVPs > 0 {
				parts = append(parts, fmt.Sprintf("MVP %d", card.MVPs))
			}
		}
	}
	return strings.Join(parts, "   ")
}

func drawbox(x, y, w, h int, color string) string {
	return fmt.Sprintf("drawbox=x=%d:y=%d:w=%d:h=%d:color=%s:t=fill", x, y, w, h, color)
}

func drawtext(fontPath, text string, x, y, size int, color string) string {
	if strings.TrimSpace(text) == "" {
		return ""
	}
	return fmt.Sprintf(
		"drawtext=fontfile='%s':text='%s':fontsize=%d:fontcolor=%s:x=%d:y=%d",
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
	return fmt.Sprintf("%s %.2f", label, *v)
}

func formatOptPct(label string, v *float64) string {
	if v == nil {
		return ""
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
		return fmt.Sprintf("%s%.2f%%", sign, *v)
	}
	return fmt.Sprintf("%s %s%.2f%%", label, sign, *v)
}

func optInt(v *int) string {
	if v == nil {
		return "0"
	}
	return strconv.Itoa(*v)
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
