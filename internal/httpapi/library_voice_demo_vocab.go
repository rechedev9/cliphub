package httpapi

import (
	"context"
	"log"

	"github.com/rechedev9/cliphub/internal/library"
	"github.com/rechedev9/cliphub/internal/placename"
	"github.com/rechedev9/cliphub/internal/tacticalplan"
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

func (h *Handlers) demoVocabulary(ctx context.Context, doc tacticalplan.Document) vocab.Vocabulary {
	v := vocab.Vocabulary{}
	for _, c := range doc.Geometry.Callouts {
		if c.Name != "" {
			v.Terms = append(v.Terms, vocab.Term{Text: placename.Label(c.Name)})
		}
	}
	if mapName := doc.Demo.Map; mapName != "" {
		names, err := h.libraryRepo.ListCalloutNames(ctx, mapName)
		if err != nil {
			log.Printf("httpapi: read the callout names of %s for the corrector: %v", mapName, err)
		}
		for _, n := range names {
			v.Terms = append(v.Terms, vocab.Term{Text: n.Name})
		}
	}
	for _, p := range doc.Players {
		v.Terms = append(v.Terms, vocab.Term{Text: p.Name, KeepCase: true})
	}
	v.Learned = h.learnedCorrections(ctx)
	return v
}

func (h *Handlers) learnedCorrections(ctx context.Context) vocab.Learned {
	lines, err := h.libraryRepo.ListDemoVoiceLines(ctx, library.DemoVoiceLineFilter{All: true})
	if err != nil {
		log.Printf("httpapi: read the library frases for the learned corrections: %v", err)
		return vocab.Learned{}
	}
	edits := []vocab.Edit{}
	for _, l := range lines {
		if l.Corrected {
			edits = append(edits, vocab.Edit{Heard: l.HeardText(), Machine: l.VocabText, Written: l.Text})
		}
	}
	return vocab.Learn(edits)
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
