package httpapi

import (
	"context"

	"github.com/rechedev9/cliphub/internal/demovocab"
	"github.com/rechedev9/cliphub/internal/library"
	"github.com/rechedev9/cliphub/internal/store"
	"github.com/rechedev9/cliphub/internal/streamclips"
	"github.com/rechedev9/cliphub/internal/vocab"
)

type streamTranscriptLine struct {
	streamclips.VoiceLine
	VocabChanges []vocab.Change `json:"vocab_changes,omitempty"`
}

func streamTranscriptLines(lines []streamclips.VoiceLine) []streamTranscriptLine {
	out := make([]streamTranscriptLine, 0, len(lines))
	for _, l := range lines {
		line := streamTranscriptLine{VoiceLine: l}
		if l.VocabText != "" {
			line.VocabChanges = vocab.Changes(l.HeardText(), l.VocabText)
		}
		out = append(out, line)
	}
	return out
}

func (h *Handlers) streamVocabulary(ctx context.Context, repo store.StreamVoiceRepository) *vocab.Vocabulary {
	if !h.voice.deps.VocabCorrector {
		return nil
	}
	in := demovocab.StreamSources{Streams: repo}
	if h.libraryRepo != nil {
		in.Demos = h.libraryRepo
	}
	v := demovocab.BuildStream(ctx, in)
	return &v
}

func correctStreamLine(line streamclips.VoiceLine, v *vocab.Vocabulary) streamclips.VoiceLine {
	got := correctDemoLine(library.DemoVoiceLine{StartMS: line.StartMS, EndMS: line.EndMS, Text: line.Text, Words: line.Words}, v)
	line.Text, line.OriginalText, line.VocabText, line.Words = got.Text, got.OriginalText, got.VocabText, got.Words
	return line
}
