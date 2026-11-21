package subtitles

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	readRunesPerSecond = 20.0
	closedGapSeconds   = 0.5
	portraitLineRunes  = 20
	landscapeLineRunes = 42
)

const (
	breakAfterSentence = iota
	breakAfterPause
	breakBeforeLinker
	breakPlain
	breakAfterLinker
	breakAfterArticle
)

type Word struct {
	Text         string
	StartSeconds float64
	EndSeconds   float64
}

type Format struct {
	LineRunes int
}

func FormatOf(width, height int) Format {
	if height > width {
		return Format{LineRunes: portraitLineRunes}
	}
	return Format{LineRunes: landscapeLineRunes}
}

type SplitInput struct {
	Text         string
	Words        []Word
	StartSeconds float64
	EndSeconds   float64
	LeadRunes    int
	Format       Format
}

type Block struct {
	Lines        []string
	StartSeconds float64
	EndSeconds   float64
}

func Split(in SplitInput) []Block {
	words := timedWords(in)
	if len(words) == 0 {
		return nil
	}
	s := splitter{lead: max(0, in.LeadRunes), room: max(1, in.Format.LineRunes)}
	var blocks []Block
	for len(words) > 0 {
		n := s.cut(words)
		lines, _ := s.lines(words[:n])
		blocks = append(blocks, Block{Lines: lines, StartSeconds: words[0].StartSeconds, EndSeconds: words[n-1].EndSeconds})
		words = words[n:]
	}
	return s.paced(blocks)
}

func timedWords(in SplitInput) []Word {
	tokens := strings.Fields(in.Text)
	if len(in.Words) == len(tokens) {
		words := make([]Word, len(tokens))
		for i, token := range tokens {
			words[i] = Word{Text: token, StartSeconds: in.Words[i].StartSeconds, EndSeconds: max(in.Words[i].StartSeconds, in.Words[i].EndSeconds)}
		}
		return words
	}
	total := 0
	for _, token := range tokens {
		total += utf8.RuneCountInString(token)
	}
	words := make([]Word, len(tokens))
	span, done := max(0, in.EndSeconds-in.StartSeconds), 0
	for i, token := range tokens {
		start := in.StartSeconds + span*float64(done)/float64(total)
		done += utf8.RuneCountInString(token)
		words[i] = Word{Text: token, StartSeconds: start, EndSeconds: in.StartSeconds + span*float64(done)/float64(total)}
	}
	return words
}

type splitter struct {
	lead, room int
}

func (s splitter) cut(words []Word) int {
	n := 1
	for n < len(words) && words[n].EndSeconds-words[0].StartSeconds <= MaxSeconds {
		if _, ok := s.lines(words[:n+1]); !ok {
			break
		}
		n++
	}
	best, bestScore := n, s.cutScore(words, n)
	for c := n - 1; c >= 1; c-- {
		if c < (n+1)/2 && bestScore[0] < breakAfterLinker {
			break
		}
		if score := s.cutScore(words, c); less(score, bestScore) {
			best, bestScore = c, score
		}
	}
	return best
}

func (s splitter) cutScore(words []Word, c int) [3]int {
	after := breakAfterSentence
	if c < len(words) {
		after = breakRank(words[c-1].Text, words[c].Text)
	}
	worst, punctuated := max(after, s.lineRank(words[:c])), 1
	if worst < breakAfterLinker {
		worst = 0
	}
	if after <= breakAfterPause {
		punctuated = 0
	}
	return [3]int{worst, punctuated, 0}
}

func (s splitter) lineRank(words []Word) int {
	if s.lead+runesOf(words) <= s.room || len(words) == 1 && s.lead == 0 {
		return breakAfterSentence
	}
	_, rank := s.lineBreak(words)
	return rank
}

func (s splitter) lines(words []Word) ([]string, bool) {
	if s.lead+runesOf(words) <= s.room || len(words) == 1 && s.lead == 0 {
		return []string{joined(words)}, true
	}
	k, rank := s.lineBreak(words)
	if k < 0 {
		return []string{joined(words)}, false
	}
	return []string{joined(words[:k]), joined(words[k:])}, rank < breakAfterArticle
}

func (s splitter) lineBreak(words []Word) (int, int) {
	best, bestScore := -1, [3]int{}
	first := 1
	if s.lead > 0 {
		first = 0
	}
	for k := first; k < len(words); k++ {
		top, bottom := runesOf(words[:k]), runesOf(words[k:])
		if s.lead+top > s.room && (k != 1 || s.lead > 0) || bottom > s.room && k != len(words)-1 {
			continue
		}
		score := [3]int{breakPlain, 0, abs(s.lead + top - bottom)}
		if k > 0 {
			score[0] = breakRank(words[k-1].Text, words[k].Text)
		}
		if bottom < s.lead+top {
			score[1] = 1
		}
		if best < 0 || less(score, bestScore) {
			best, bestScore = k, score
		}
	}
	return best, bestScore[0]
}

func (s splitter) paced(blocks []Block) []Block {
	cursor := blocks[0].StartSeconds
	for i := range blocks {
		b := &blocks[i]
		b.StartSeconds = max(b.StartSeconds, cursor)
		runes := s.lead + len(b.Lines) - 1
		for _, line := range b.Lines {
			runes += utf8.RuneCountInString(line)
		}
		need := max(MinSeconds, float64(runes)/readRunesPerSecond)
		b.EndSeconds = min(max(b.EndSeconds, b.StartSeconds+need), b.StartSeconds+MaxSeconds)
		cursor = b.EndSeconds
	}
	for i := range blocks[:len(blocks)-1] {
		b, next := &blocks[i], blocks[i+1]
		if gap := next.StartSeconds - b.EndSeconds; gap > 0 && gap < closedGapSeconds {
			b.EndSeconds = min(next.StartSeconds, b.StartSeconds+MaxSeconds)
		}
	}
	return blocks
}

func less(a, b [3]int) bool {
	for i := range a {
		if a[i] != b[i] {
			return a[i] < b[i]
		}
	}
	return false
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func runesOf(words []Word) int {
	n := max(0, len(words)-1)
	for _, w := range words {
		n += utf8.RuneCountInString(w.Text)
	}
	return n
}

func joined(words []Word) string {
	parts := make([]string, len(words))
	for i, w := range words {
		parts[i] = w.Text
	}
	return strings.Join(parts, " ")
}

func breakRank(before, after string) int {
	switch last, _ := utf8.DecodeLastRuneInString(before); {
	case strings.ContainsRune(".?!…", last):
		return breakAfterSentence
	case strings.ContainsRune(",;:", last):
		return breakAfterPause
	}
	switch {
	case articles[bare(before)]:
		return breakAfterArticle
	case linkers[bare(before)]:
		return breakAfterLinker
	case linkers[bare(after)]:
		return breakBeforeLinker
	}
	return breakPlain
}

func bare(word string) string {
	return strings.ToLower(strings.TrimFunc(word, unicode.IsPunct))
}

var articles = wordSet("el la los las un una unos unas lo al del the an")

var linkers = wordSet("y e o u ni que pero aunque porque si cuando como mientras pues " +
	"a ante bajo con contra de desde en entre hacia hasta para por según sin sobre tras " +
	"and or but because if when while that so of in on at to for with from by into over under")

func wordSet(list string) map[string]bool {
	set := map[string]bool{}
	for _, w := range strings.Fields(list) {
		set[w] = true
	}
	return set
}
