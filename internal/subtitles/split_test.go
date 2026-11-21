package subtitles

import (
	"math"
	"reflect"
	"strings"
	"testing"
)

var (
	portrait  = Format{LineRunes: 20}
	landscape = Format{LineRunes: 42}
)

func evenWords(text string, start, each float64) []Word {
	var words []Word
	for i, w := range strings.Fields(text) {
		words = append(words, Word{Text: w, StartSeconds: start + float64(i)*each, EndSeconds: start + float64(i+1)*each})
	}
	return words
}

func linesOf(blocks []Block) [][]string {
	var out [][]string
	for _, b := range blocks {
		out = append(out, b.Lines)
	}
	return out
}

func assertWindows(t *testing.T, blocks []Block, want [][2]float64) {
	t.Helper()
	if len(blocks) != len(want) {
		t.Fatalf("%d subtítulos, want %d: %+v", len(blocks), len(want), blocks)
	}
	for i, b := range blocks {
		if math.Abs(b.StartSeconds-want[i][0]) > 0.01 || math.Abs(b.EndSeconds-want[i][1]) > 0.01 {
			t.Errorf("subtítulo %d shows %.2f to %.2f, want %.2f to %.2f", i, b.StartSeconds, b.EndSeconds, want[i][0], want[i][1])
		}
	}
}

func TestSplitBreaksLinesWhereTheSentenceAllows(t *testing.T) {
	for _, c := range []struct {
		name, text string
		lead       int
		format     Format
		want       [][]string
	}{
		{"a frase that fits stays on one line", "dos en B, uno con AWP", 8, landscape, [][]string{{"dos en B, uno con AWP"}}},
		{"after punctuation", "dos en B, uno con AWP", 0, portrait, [][]string{{"dos en B,", "uno con AWP"}}},
		{"the speaker's name takes room on the first line", "dos en B, uno con AWP", 8, portrait, [][]string{{"dos en B,", "uno con AWP"}}},
		{"the lower line is the longer", "rush B todos juntos ahora mismo", 0, portrait, [][]string{{"rush B todos", "juntos ahora mismo"}}},
		{"before a conjunction", "voy a entrar y tiras la flash", 0, portrait, [][]string{{"voy a entrar", "y tiras la flash"}}},
		{"never between an article and its noun", "tiro la granada ya", 0, Format{LineRunes: 10}, [][]string{{"tiro", "la granada"}, {"ya"}}},
		{"a name that fills the first line stands alone", "vamos B", 18, portrait, [][]string{{"", "vamos B"}}},
		{"a short name shares the first line with 20 characters", "en la ventana y otro", 8, FormatOf(1080, 1920), [][]string{{"en la ventana y otro"}}},
		{"a short name shares the first line with 42 characters", "dos en B, uno con AWP en la ventana y otro", 14, FormatOf(1920, 1080), [][]string{{"dos en B, uno con AWP en la ventana y otro"}}},
		{"a long name takes room from the first line", "en la ventana y otro", 14, FormatOf(1080, 1920), [][]string{{"en la ventana", "y otro"}}},
		{"a name longer than the line stands alone", "vamos B", 30, FormatOf(1080, 1920), [][]string{{"", "vamos B"}}},
	} {
		t.Run(c.name, func(t *testing.T) {
			got := linesOf(Split(SplitInput{Text: c.text, StartSeconds: 1, EndSeconds: 3, LeadRunes: c.lead, Format: c.format}))
			if !reflect.DeepEqual(got, c.want) {
				t.Fatalf("lines = %q, want %q", got, c.want)
			}
		})
	}
}

func TestSplitKeepsEverySubtituloInsideTwoLinesOfItsFormat(t *testing.T) {
	text := "vale pues entonces yo creo que lo mejor es que salgamos todos juntos por el medio y luego ya veremos lo que hacemos con la bomba porque no tenemos kit"
	for _, format := range []Format{portrait, landscape} {
		var seen []string
		for _, b := range Split(SplitInput{Text: text, StartSeconds: 0, EndSeconds: 5, LeadRunes: 6, Format: format}) {
			if len(b.Lines) > 2 {
				t.Fatalf("a subtítulo has %d lines: %q", len(b.Lines), b.Lines)
			}
			for i, line := range b.Lines {
				room := format.LineRunes
				if i == 0 {
					room -= 6
				}
				if len([]rune(line)) > room {
					t.Fatalf("line %q is longer than %d", line, room)
				}
			}
			seen = append(seen, b.Lines...)
		}
		if got := strings.Join(strings.Fields(strings.Join(seen, " ")), " "); got != text {
			t.Fatalf("the subtítulos read %q, want every word of the frase in order", got)
		}
	}
}

func TestSplitFollowsTheWordTimings(t *testing.T) {
	text := "dos en B, uno con AWP en la ventana y otro rotando por medio"
	at := [][2]float64{{10, 10.2}, {10.2, 10.3}, {10.3, 10.6}, {10.8, 11}, {11, 11.2}, {11.2, 11.6}, {11.7, 11.8}, {11.8, 11.9}, {11.9, 12.4}, {12.5, 12.6}, {12.6, 12.9}, {12.9, 13.4}, {13.4, 13.6}, {13.6, 14}}
	var words []Word
	for i, w := range strings.Fields(text) {
		words = append(words, Word{Text: w, StartSeconds: at[i][0], EndSeconds: at[i][1]})
	}
	blocks := Split(SplitInput{Text: text, Words: words, StartSeconds: 10, EndSeconds: 14, Format: portrait})
	want := [][]string{{"dos en B,", "uno con AWP"}, {"en la ventana y otro", "rotando por medio"}}
	if got := linesOf(blocks); !reflect.DeepEqual(got, want) {
		t.Fatalf("lines = %q, want %q", got, want)
	}
	assertWindows(t, blocks, [][2]float64{{10, 11.7}, {11.7, 14}})
}

func TestSplitWithoutWordTimingsSharesTheTimeByCharacters(t *testing.T) {
	text := "dos en B, uno con AWP en la ventana y otro rotando por medio"
	stale := []Word{{Text: "dos", StartSeconds: 13, EndSeconds: 14}}
	for name, words := range map[string][]Word{"no words": nil, "words of another text": stale} {
		t.Run(name, func(t *testing.T) {
			blocks := Split(SplitInput{Text: text, Words: words, StartSeconds: 10, EndSeconds: 14, Format: portrait})
			assertWindows(t, blocks, [][2]float64{{10, 11.36}, {11.36, 14}})
		})
	}
}

func TestSplitGivesTimeToReadFastSpeech(t *testing.T) {
	text := "uno dos tres cuatro cinco seis siete ocho nueve diez"
	blocks := Split(SplitInput{Text: text, Words: evenWords(text, 0, 0.1), StartSeconds: 0, EndSeconds: 1, Format: portrait})
	want := [][]string{{"uno dos tres cuatro", "cinco seis siete"}, {"ocho nueve diez"}}
	if got := linesOf(blocks); !reflect.DeepEqual(got, want) {
		t.Fatalf("lines = %q, want %q", got, want)
	}
	assertWindows(t, blocks, [][2]float64{{0, 1.8}, {1.8, 3}})
}

func TestSplitCapsHowLongASubtituloStays(t *testing.T) {
	held := Split(SplitInput{Text: "vamos", StartSeconds: 0, EndSeconds: 12, Format: landscape})
	assertWindows(t, held, [][2]float64{{0, 7}})
	slow := []Word{{Text: "uno", StartSeconds: 0, EndSeconds: 1}, {Text: "dos", StartSeconds: 9, EndSeconds: 10}}
	apart := Split(SplitInput{Text: "uno dos", Words: slow, StartSeconds: 0, EndSeconds: 10, Format: landscape})
	assertWindows(t, apart, [][2]float64{{0, 1.2}, {9, 10.2}})
}

func TestSentenceCaseStartsAFraseWithACapitalAndKeepsTheRest(t *testing.T) {
	for text, want := range map[string]string{
		"dos en B, uno con AWP":    "Dos en B, uno con AWP",
		"¿están en A? creo que sí": "¿Están en A? creo que sí",
		"  ojo ":                   "Ojo",
		"Rush B":                   "Rush B",
		"3 en medio":               "3 en medio",
		"":                         "",
	} {
		if got := SentenceCase(text); got != want {
			t.Errorf("SentenceCase(%q) = %q, want %q", text, got, want)
		}
	}
}
