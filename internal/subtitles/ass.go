// Package subtitles burns the team's transcribed comms into a video: the phrases a render
// reads, on the demo's tick clock, and the libass script that draws them.
package subtitles

import (
	"fmt"
	"math"
	"slices"
	"strings"

	"github.com/rechedev9/cliphub/internal/mediafont"
)

// A phrase stays on screen at least MinSeconds, so a short callout can be read, and at most MaxSeconds.
const (
	MinSeconds = 1.2
	MaxSeconds = 6.0
	maxRunes   = 160
)

// Cue is one phrase placed on the video's timeline.
type Cue struct {
	ID           string
	Speaker      string
	Text         string
	StartSeconds float64
	EndSeconds   float64
}

// Script is what a video draws: its frame, its length and its phrases; Hidden lists phrase IDs left out.
type Script struct {
	Width           int
	Height          int
	DurationSeconds float64
	Cues            []Cue
	Hidden          []string
}

// layout is the subtitle block of one format, on a 1080 px wide 9:16 or a 1920 px wide 16:9 frame.
type layout struct {
	fontSize, marginSide, marginBottom int
}

// The 9:16 block sits over the lower third, under the centered kill feed and above the radar
// corner; the 16:9 block sits above CS2's health and ammo row, between the radar and kill feed columns.
func layoutOf(width, height int) layout {
	if height > width {
		return layout{fontSize: scale(56, width, 1080), marginSide: scale(86, width, 1080), marginBottom: scale(600, height, 1920)}
	}
	return layout{fontSize: scale(46, height, 1080), marginSide: scale(230, width, 1920), marginBottom: scale(150, height, 1080)}
}

func scale(value, size, reference int) int {
	return int(math.Round(float64(value) * float64(size) / float64(reference)))
}

// ASS writes the script libass draws over the finished video. Each phrase is its speaker's name, then the text.
func ASS(s Script) (string, error) {
	if s.Width < 320 || s.Height < 320 || s.DurationSeconds <= 0 {
		return "", fmt.Errorf("invalid subtitle frame")
	}
	l := layoutOf(s.Width, s.Height)
	var b strings.Builder
	fmt.Fprintf(&b, "[Script Info]\nScriptType: v4.00+\nPlayResX: %d\nPlayResY: %d\nScaledBorderAndShadow: yes\nWrapStyle: 0\n\n", s.Width, s.Height)
	b.WriteString("[V4+ Styles]\nFormat: Name, Fontname, Fontsize, PrimaryColour, SecondaryColour, OutlineColour, BackColour, Bold, Italic, Underline, StrikeOut, ScaleX, ScaleY, Spacing, Angle, BorderStyle, Outline, Shadow, Alignment, MarginL, MarginR, MarginV, Encoding\n")
	fmt.Fprintf(&b, "Style: Subtitle,%s,%d,&H00FFFFFF,&H00FFFFFF,&H00000000,&H64000000,0,0,0,0,100,100,0,0,1,%d,1,2,%d,%d,%d,1\n\n",
		mediafont.FamilyName, l.fontSize, max(2, l.fontSize/14), l.marginSide, l.marginSide, l.marginBottom)
	b.WriteString("[Events]\nFormat: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text\n")
	for _, c := range timed(s) {
		fmt.Fprintf(&b, "Dialogue: 0,%s,%s,Subtitle,,0,0,0,,{\\c&H4FD7FF&}%s:{\\c&HFFFFFF&} %s\n", clock(c.StartSeconds), clock(c.EndSeconds), escape(c.Speaker), escape(clip(c.Text)))
	}
	return b.String(), nil
}

// timed keeps the visible phrases in time order with their final display window.
func timed(s Script) []Cue {
	cues := slices.DeleteFunc(slices.Clone(s.Cues), func(c Cue) bool {
		return slices.Contains(s.Hidden, c.ID) || strings.TrimSpace(c.Text) == "" || c.EndSeconds < c.StartSeconds
	})
	slices.SortStableFunc(cues, func(a, b Cue) int { return compare(a.StartSeconds, b.StartSeconds) })
	for i := range cues {
		c := &cues[i]
		c.StartSeconds = max(0, c.StartSeconds)
		c.EndSeconds = max(min(c.EndSeconds, c.StartSeconds+MaxSeconds), c.StartSeconds+MinSeconds)
		for _, next := range cues[i+1:] {
			if next.Speaker == c.Speaker && next.StartSeconds > c.StartSeconds {
				c.EndSeconds = min(c.EndSeconds, next.StartSeconds)
				break
			}
		}
		c.EndSeconds = min(c.EndSeconds, s.DurationSeconds)
	}
	return slices.DeleteFunc(cues, func(c Cue) bool { return c.EndSeconds-c.StartSeconds < 0.05 })
}

func compare(a, b float64) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}

func clock(seconds float64) string {
	cs := int64(math.Round(seconds * 100))
	return fmt.Sprintf("%d:%02d:%02d.%02d", cs/360000, cs/6000%60, cs/100%60, cs%100)
}

func clip(text string) string {
	text = strings.Join(strings.Fields(text), " ")
	if runes := []rune(text); len(runes) > maxRunes {
		return strings.TrimSpace(string(runes[:maxRunes-1])) + "…"
	}
	return text
}

// escape keeps a phrase literal: ASS has no escape for override braces, so they become full-width forms.
func escape(value string) string {
	return strings.NewReplacer("\\", "＼", "{", "｛", "}", "｝", "\n", " ", "\r", " ").Replace(value)
}
