package httpapi

import (
	"github.com/rechedev9/cliphub/internal/library"
	"github.com/rechedev9/cliphub/internal/vocab"
)

type demoTranscriptLine struct {
	library.DemoVoiceLine
	VocabChanges []vocab.Change `json:"vocab_changes,omitempty"`
}

func demoTranscriptLines(lines []library.DemoVoiceLine) []demoTranscriptLine {
	out := make([]demoTranscriptLine, 0, len(lines))
	for _, l := range lines {
		line := demoTranscriptLine{DemoVoiceLine: l}
		if l.VocabText != "" {
			line.VocabChanges = vocab.Changes(l.HeardText(), l.VocabText)
		}
		out = append(out, line)
	}
	return out
}

func correctDemoLine(line library.DemoVoiceLine, v *vocab.Vocabulary) library.DemoVoiceLine {
	if v == nil {
		return line
	}
	words := make([]vocab.Word, len(line.Words))
	for i, w := range line.Words {
		words[i] = vocab.Word{Text: w.Text, StartMS: w.StartMS, EndMS: w.EndMS}
	}
	got := vocab.Correct(vocab.Phrase{Text: line.Text, Words: words}, *v)
	if got.Text == line.Text {
		return line
	}
	line.OriginalText, line.Text, line.VocabText = line.Text, got.Text, got.Text
	line.Words = make([]library.DemoVoiceWord, len(got.Words))
	for i, w := range got.Words {
		line.Words[i] = library.DemoVoiceWord{Text: w.Text, StartMS: w.StartMS, EndMS: w.EndMS}
	}
	if len(line.Words) == 0 {
		line.Words = library.SpreadVoiceWords(library.VoiceWordSpread{Text: line.Text, StartMS: line.StartMS, EndMS: line.EndMS})
	}
	return line
}
