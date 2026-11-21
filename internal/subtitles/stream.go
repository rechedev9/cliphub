package subtitles

import (
	"fmt"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/rechedev9/cliphub/internal/mediafont"
)

const (
	streamFontSize      = 88
	streamOutline       = 7
	streamShadow        = 2
	streamLineRunes     = 14
	streamChunkWords    = 4
	streamHoldSeconds   = 0.35
	streamPauseSeconds  = 0.7
	streamSpokenColour  = "00E6FF"
	streamWaitingColour = "FFFFFF"
	streamDropped       = ".,;:"
)

type StreamPart struct {
	StartSeconds float64
	EndSeconds   float64
	Speed        float64
}

type StreamLine struct {
	ID           string
	Text         string
	StartSeconds float64
	EndSeconds   float64
	Words        []Word
}

type StreamVideo struct {
	Width  int
	Height int
	Parts  []StreamPart
	Lines  []StreamLine
}

func StreamASS(v StreamVideo) ([]string, error) {
	scripts := make([]string, len(v.Parts))
	for i, part := range v.Parts {
		speed := part.Speed
		if speed <= 0 {
			speed = 1
		}
		length := (part.EndSeconds - part.StartSeconds) / speed
		at := func(source float64) float64 { return (source - part.StartSeconds) / speed }
		var lines []StreamLine
		for _, l := range v.Lines {
			if l.EndSeconds <= part.StartSeconds || l.StartSeconds >= part.EndSeconds {
				continue
			}
			on := StreamLine{ID: l.ID, Text: l.Text, StartSeconds: at(l.StartSeconds), EndSeconds: at(l.EndSeconds)}
			for _, w := range l.Words {
				on.Words = append(on.Words, Word{Text: w.Text, StartSeconds: at(w.StartSeconds), EndSeconds: at(w.EndSeconds)})
			}
			lines = append(lines, on)
		}
		if len(lines) == 0 || length <= 0 {
			continue
		}
		script, err := streamScript(v, lines, length)
		if err != nil {
			return nil, err
		}
		scripts[i] = script
	}
	return scripts, nil
}

func streamScript(v StreamVideo, lines []StreamLine, length float64) (string, error) {
	if v.Height > v.Width {
		return spokenWordsASS(v, lines, length)
	}
	format := FormatOf(v.Width, v.Height)
	var cues []Cue
	for _, l := range lines {
		for _, b := range Split(SplitInput{Text: SentenceCase(l.Text), Words: l.Words, StartSeconds: l.StartSeconds, EndSeconds: l.EndSeconds, Format: format}) {
			start, end := max(b.StartSeconds, 0), min(b.EndSeconds, length)
			if end-start >= 0.05 {
				cues = append(cues, Cue{ID: l.ID, POV: true, Lines: b.Lines, StartSeconds: start, EndSeconds: end})
			}
		}
	}
	if len(cues) == 0 {
		return "", nil
	}
	return ASS(Script{Width: v.Width, Height: v.Height, DurationSeconds: length, Cues: cues})
}

type wordChunk struct {
	words []Word
	start float64
	end   float64
}

func spokenWordsASS(v StreamVideo, lines []StreamLine, length float64) (string, error) {
	if v.Width < 320 || v.Height < 320 {
		return "", fmt.Errorf("invalid subtitle frame")
	}
	var chunks []wordChunk
	for _, l := range lines {
		words := timedWords(SplitInput{Text: l.Text, Words: l.Words, StartSeconds: l.StartSeconds, EndSeconds: l.EndSeconds})
		for len(words) > 0 {
			n := chunkLength(words)
			chunks = append(chunks, wordChunk{words: words[:n], start: words[0].StartSeconds, end: words[n-1].EndSeconds + streamHoldSeconds})
			words = words[n:]
		}
	}
	slices.SortStableFunc(chunks, func(a, b wordChunk) int { return compare(a.start, b.start) })
	var events strings.Builder
	for i, c := range chunks {
		if i+1 < len(chunks) {
			c.end = min(c.end, chunks[i+1].start)
		}
		c.start, c.end = max(c.start, 0), min(c.end, length)
		writeChunk(&events, c)
	}
	if events.Len() == 0 {
		return "", nil
	}
	placed := PlacementOf(v.Width, v.Height)
	var b strings.Builder
	fmt.Fprintf(&b, "[Script Info]\nScriptType: v4.00+\nPlayResX: %d\nPlayResY: %d\nScaledBorderAndShadow: yes\nWrapStyle: 0\n\n", v.Width, v.Height)
	b.WriteString("[V4+ Styles]\nFormat: Name, Fontname, Fontsize, PrimaryColour, SecondaryColour, OutlineColour, BackColour, Bold, Italic, Underline, StrikeOut, ScaleX, ScaleY, Spacing, Angle, BorderStyle, Outline, Shadow, Alignment, MarginL, MarginR, MarginV, Encoding\n")
	fmt.Fprintf(&b, "Style: Stream,%s,%d,&H00FFFFFF,&H00FFFFFF,&H00000000,&H96000000,0,0,0,0,100,100,0,0,1,%d,%d,2,%d,%d,%d,1\n\n",
		mediafont.FamilyName, scale(streamFontSize, v.Width, zoneFrameWidth), scale(streamOutline, v.Width, zoneFrameWidth), scale(streamShadow, v.Width, zoneFrameWidth),
		placed.Box.Left, v.Width-placed.Box.Right, v.Height-placed.Box.Bottom)
	b.WriteString("[Events]\nFormat: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text\n")
	b.WriteString(events.String())
	return b.String(), nil
}

func chunkLength(words []Word) int {
	n := min(streamChunkWords, len(words))
	for n > 1 && !fitsTwoLines(words[:n]) {
		n--
	}
	for k := 2; k < n; k++ {
		last, _ := utf8.DecodeLastRuneInString(words[k-1].Text)
		if strings.ContainsRune(".?!…,;:", last) || words[k].StartSeconds-words[k-1].EndSeconds > streamPauseSeconds {
			n = k
			break
		}
	}
	if len(words)-n == 1 {
		switch {
		case n > 2:
			n--
		case fitsTwoLines(words[:n+1]):
			n++
		}
	}
	return n
}

func shownRunes(words []Word) int {
	n := max(0, len(words)-1)
	for _, w := range words {
		n += utf8.RuneCountInString(shownWord(w.Text))
	}
	return n
}

func shownWord(text string) string {
	if trimmed := strings.TrimRight(text, streamDropped); trimmed != "" {
		text = trimmed
	}
	return strings.ToUpper(text)
}

func fitsLine(words []Word) bool {
	return len(words) == 1 || shownRunes(words) <= streamLineRunes
}

func fitsTwoLines(words []Word) bool {
	return fitsLine(words) || lineBreakOf(words) > 0
}

func lineBreakOf(words []Word) int {
	best, bestGap := 0, 0
	for k := 1; k < len(words); k++ {
		if !fitsLine(words[:k]) || !fitsLine(words[k:]) {
			continue
		}
		top, bottom := min(shownRunes(words[:k]), streamLineRunes), min(shownRunes(words[k:]), streamLineRunes)
		if gap := abs(top - bottom); best == 0 || gap < bestGap {
			best, bestGap = k, gap
		}
	}
	return best
}

func writeChunk(b *strings.Builder, c wordChunk) {
	breakAt := 0
	if !fitsLine(c.words) {
		breakAt = lineBreakOf(c.words)
	}
	for i := range c.words {
		from, to := max(c.words[i].StartSeconds, c.start), c.end
		if i == 0 {
			from = c.start
		}
		if i+1 < len(c.words) {
			to = min(max(c.words[i+1].StartSeconds, c.start), c.end)
		}
		from = min(from, to)
		if clock(from) == clock(to) {
			continue
		}
		var text strings.Builder
		for j, w := range c.words {
			switch {
			case j == 0:
			case j == breakAt:
				text.WriteString(`\N`)
			default:
				text.WriteString(" ")
			}
			shown := escape(shownWord(w.Text))
			if runes := utf8.RuneCountInString(shown); runes > streamLineRunes {
				size := 100 * streamLineRunes / runes
				shown = fmt.Sprintf(`{\fscx%d\fscy%d}%s{\fscx100\fscy100}`, size, size, shown)
			}
			if j == i {
				shown = fmt.Sprintf(`{\c&H%s&}%s{\c&H%s&}`, streamSpokenColour, shown, streamWaitingColour)
			}
			text.WriteString(shown)
		}
		fmt.Fprintf(b, "Dialogue: 0,%s,%s,Stream,,0,0,0,,%s\n", clock(from), clock(to), text.String())
	}
}
