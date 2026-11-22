package transcriptmeasure

import (
	"strings"
	"unicode"
)

type Frase struct {
	Heard     string
	Vocab     string
	Text      string
	Discarded bool
	StartMS   int64
	EndMS     int64
}

type Result struct {
	Frases              int     `json:"frases"`
	CorrectedFrases     int     `json:"corrected_frases"`
	DiscardedFrases     int     `json:"discarded_frases"`
	Words               int     `json:"words"`
	CorrectedWords      int     `json:"corrected_words"`
	CorrectedShare      float64 `json:"corrected_share"`
	SpeechMS            int64   `json:"speech_ms"`
	DiscardedPerMinute  float64 `json:"discarded_per_minute"`
	VocabCorrectedWords int     `json:"vocab_corrected_words"`
	VocabCorrectedShare float64 `json:"vocab_corrected_share"`
	VocabFixedWords     int     `json:"vocab_fixed_words"`
	VocabBrokenWords    int     `json:"vocab_broken_words"`
}

func Measure(frases []Frase) Result {
	var r Result
	for _, f := range frases {
		r.Frases++
		if f.Discarded {
			r.DiscardedFrases++
			continue
		}
		heard, text := words(f.Heard), words(f.Text)
		changed := wordDistance(heard, text)
		r.measureCorrector(correctorFrase{heard: heard, vocab: f.Vocab, text: text})
		r.Words += len(heard)
		r.CorrectedWords += changed
		if changed > 0 {
			r.CorrectedFrases++
		}
		if f.EndMS > f.StartMS {
			r.SpeechMS += f.EndMS - f.StartMS
		}
	}
	if r.Words > 0 {
		r.CorrectedShare = float64(r.CorrectedWords) / float64(r.Words)
		r.VocabCorrectedShare = float64(r.VocabCorrectedWords) / float64(r.Words)
	}
	if r.SpeechMS > 0 {
		r.DiscardedPerMinute = float64(r.DiscardedFrases) * 60_000 / float64(r.SpeechMS)
	}
	return r
}

type correctorFrase struct {
	heard []string
	vocab string
	text  []string
}

type wordCheck struct {
	words []string
	other []string
	text  []string
}

func (r *Result) measureCorrector(f correctorFrase) {
	written := f.heard
	if f.vocab != "" {
		written = words(f.vocab)
	}
	r.VocabCorrectedWords += wordDistance(written, f.text)
	r.VocabFixedWords += count(changedAndKept(wordCheck{words: written, other: f.heard, text: f.text}))
	r.VocabBrokenWords += count(changedAndKept(wordCheck{words: f.heard, other: written, text: f.text}))
}

func changedAndKept(c wordCheck) []bool {
	unchanged, kept := common(c.words, c.other), common(c.words, c.text)
	out := make([]bool, len(c.words))
	for i := range c.words {
		out[i] = !unchanged[i] && kept[i]
	}
	return out
}

func common(a, b []string) []bool {
	lcs := make([][]int, len(a)+1)
	for i := range lcs {
		lcs[i] = make([]int, len(b)+1)
	}
	for i := len(a) - 1; i >= 0; i-- {
		for j := len(b) - 1; j >= 0; j-- {
			if a[i] == b[j] {
				lcs[i][j] = lcs[i+1][j+1] + 1
			} else {
				lcs[i][j] = max(lcs[i+1][j], lcs[i][j+1])
			}
		}
	}
	out := make([]bool, len(a))
	for i, j := 0, 0; i < len(a) && j < len(b); {
		switch {
		case a[i] == b[j]:
			out[i] = true
			i, j = i+1, j+1
		case lcs[i+1][j] >= lcs[i][j+1]:
			i++
		default:
			j++
		}
	}
	return out
}

func count(flags []bool) int {
	n := 0
	for _, f := range flags {
		if f {
			n++
		}
	}
	return n
}

func words(text string) []string {
	return strings.FieldsFunc(strings.ToLower(text), func(c rune) bool {
		return !unicode.IsLetter(c) && !unicode.IsDigit(c)
	})
}

func wordDistance(from, to []string) int {
	row := make([]int, len(to)+1)
	for j := range row {
		row[j] = j
	}
	for i, word := range from {
		diagonal := row[0]
		row[0] = i + 1
		for j, other := range to {
			cost := diagonal
			if word != other {
				cost = min(diagonal, row[j], row[j+1]) + 1
			}
			diagonal, row[j+1] = row[j+1], cost
		}
	}
	return row[len(to)]
}
