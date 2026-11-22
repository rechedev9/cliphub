package transcriptmeasure

import (
	"math"
	"testing"
)

func TestMeasure(t *testing.T) {
	minute := int64(60_000)
	cases := []struct {
		name   string
		frases []Frase
		want   Result
	}{
		{
			name: "a frase nobody touched adds its words and no correction",
			frases: []Frase{
				{Heard: "rush b no stop", Text: "rush b no stop", EndMS: minute},
			},
			want: Result{Frases: 1, Words: 4, SpeechMS: minute},
		},
		{
			name: "one word replaced in four is a quarter",
			frases: []Frase{
				{Heard: "rush be no stop", Text: "rush b no stop", EndMS: minute},
			},
			want: Result{Frases: 1, CorrectedFrases: 1, Words: 4, CorrectedWords: 1, CorrectedShare: 0.25, SpeechMS: minute, VocabCorrectedWords: 1, VocabCorrectedShare: 0.25},
		},
		{
			name: "words the person added or removed count like replaced ones",
			frases: []Frase{
				{Heard: "stack b site", Text: "stack b they go b", EndMS: minute},
				{Heard: "eco eco eco", Text: "eco", StartMS: minute, EndMS: 2 * minute},
			},
			want: Result{Frases: 2, CorrectedFrases: 2, Words: 6, CorrectedWords: 5, CorrectedShare: 5.0 / 6.0, SpeechMS: 2 * minute, VocabCorrectedWords: 5, VocabCorrectedShare: 5.0 / 6.0},
		},
		{
			name: "capitals and punctuation are not corrections, accents are",
			frases: []Frase{
				{Heard: "vamos a, esta", Text: "Vamos A. ¡Está!", EndMS: minute},
			},
			want: Result{Frases: 1, CorrectedFrases: 1, Words: 3, CorrectedWords: 1, CorrectedShare: 1.0 / 3.0, SpeechMS: minute, VocabCorrectedWords: 1, VocabCorrectedShare: 1.0 / 3.0},
		},
		{
			name: "a discarded frase is counted per minute of the speech that stayed and adds no words",
			frases: []Frase{
				{Heard: "gracias por ver el video", Text: "gracias por ver el video", Discarded: true, EndMS: minute},
				{Heard: "thanks", Text: "nothing like it", Discarded: true, StartMS: minute, EndMS: 2 * minute},
				{Heard: "one b", Text: "one b", StartMS: 2 * minute, EndMS: 6 * minute},
			},
			want: Result{Frases: 3, DiscardedFrases: 2, Words: 2, SpeechMS: 4 * minute, DiscardedPerMinute: 0.5},
		},
		{
			name: "with no speech left there is no rate to divide",
			frases: []Frase{
				{Heard: "thanks", Text: "thanks", Discarded: true, EndMS: minute},
			},
			want: Result{Frases: 1, DiscardedFrases: 1},
		},
		{
			name: "a word the corrector writes as the person did is fixed",
			frases: []Frase{
				{Heard: "flesh mid", Vocab: "flash mid", Text: "Flash mid", EndMS: minute},
			},
			want: Result{Frases: 1, CorrectedFrases: 1, Words: 2, CorrectedWords: 1, CorrectedShare: 0.5, SpeechMS: minute, VocabFixedWords: 1},
		},
		{
			name: "a word right as heard and wrong after the corrector is broken",
			frases: []Frase{
				{Heard: "go contact", Vocab: "go contract", Text: "go contact", EndMS: minute},
			},
			want: Result{Frases: 1, Words: 2, SpeechMS: minute, VocabCorrectedWords: 1, VocabCorrectedShare: 0.5, VocabBrokenWords: 1},
		},
		{
			name: "heard words the corrector joins into the person's word fix once",
			frases: []Frase{
				{Heard: "two wers stairs", Vocab: "tower stairs", Text: "tower stairs", EndMS: minute},
			},
			want: Result{Frases: 1, CorrectedFrases: 1, Words: 3, CorrectedWords: 2, CorrectedShare: 2.0 / 3.0, SpeechMS: minute, VocabFixedWords: 1},
		},
		{
			name: "a discarded frase adds nothing the corrector fixed or broke",
			frases: []Frase{
				{Heard: "flesh", Vocab: "flash", Text: "flesh", Discarded: true, EndMS: minute},
			},
			want: Result{Frases: 1, DiscardedFrases: 1},
		},
		{
			name:   "no frases measure as zero",
			frases: nil,
			want:   Result{},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Measure(tc.frases)
			if !sameResult(got, tc.want) {
				t.Fatalf("Measure = %+v, want %+v", got, tc.want)
			}
		})
	}
}

func sameResult(a, b Result) bool {
	near := func(x, y float64) bool { return math.Abs(x-y) < 1e-9 }
	return a.Frases == b.Frases && a.CorrectedFrases == b.CorrectedFrases && a.DiscardedFrases == b.DiscardedFrases &&
		a.Words == b.Words && a.CorrectedWords == b.CorrectedWords && a.SpeechMS == b.SpeechMS &&
		near(a.CorrectedShare, b.CorrectedShare) && near(a.DiscardedPerMinute, b.DiscardedPerMinute) &&
		a.VocabCorrectedWords == b.VocabCorrectedWords && near(a.VocabCorrectedShare, b.VocabCorrectedShare) &&
		a.VocabFixedWords == b.VocabFixedWords && a.VocabBrokenWords == b.VocabBrokenWords
}
