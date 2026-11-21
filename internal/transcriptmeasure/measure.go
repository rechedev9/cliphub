package transcriptmeasure

import (
	"strings"
	"unicode"
)

type Frase struct {
	Heard     string
	Text      string
	Discarded bool
	StartMS   int64
	EndMS     int64
}

type Result struct {
	Frases             int     `json:"frases"`
	CorrectedFrases    int     `json:"corrected_frases"`
	DiscardedFrases    int     `json:"discarded_frases"`
	Words              int     `json:"words"`
	CorrectedWords     int     `json:"corrected_words"`
	CorrectedShare     float64 `json:"corrected_share"`
	SpeechMS           int64   `json:"speech_ms"`
	DiscardedPerMinute float64 `json:"discarded_per_minute"`
}

func Measure(frases []Frase) Result {
	var r Result
	for _, f := range frases {
		r.Frases++
		if f.Discarded {
			r.DiscardedFrases++
			continue
		}
		heard := words(f.Heard)
		changed := wordDistance(heard, words(f.Text))
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
	}
	if r.SpeechMS > 0 {
		r.DiscardedPerMinute = float64(r.DiscardedFrases) * 60_000 / float64(r.SpeechMS)
	}
	return r
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
