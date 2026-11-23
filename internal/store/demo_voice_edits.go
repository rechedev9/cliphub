package store

import (
	"cmp"

	"github.com/rechedev9/cliphub/internal/library"
)

func machineText(l library.DemoVoiceLine) string {
	if l.VocabText != "" {
		return l.VocabText
	}
	return l.HeardText()
}

func setDemoVoiceText(l library.DemoVoiceLine, text string) library.DemoVoiceLine {
	heard, machine := l.HeardText(), machineText(l)
	l.Text, l.Corrected, l.OriginalText = text, text != machine, ""
	if text != heard {
		l.OriginalText = heard
	}
	if !l.Corrected {
		l.CorrectedBy = ""
	}
	return l
}

func patchDemoVoiceLine(l library.DemoVoiceLine, patch library.DemoVoiceLinePatch) library.DemoVoiceLine {
	if patch.Discarded != nil {
		l.Discarded = *patch.Discarded
	}
	if patch.Text == nil {
		return l
	}
	unchanged, author := *patch.Text == l.Text, l.CorrectedBy
	if !unchanged {
		l.Words = library.SpreadVoiceWords(library.VoiceWordSpread{Text: *patch.Text, StartMS: l.StartMS, EndMS: l.EndMS})
	}
	l = setDemoVoiceText(l, *patch.Text)
	switch {
	case !l.Corrected:
	case unchanged && patch.By == library.CorrectedByAssistant && author != "":
		l.CorrectedBy = author
	default:
		l.CorrectedBy = cmp.Or(patch.By, library.CorrectedByUser)
	}
	return l
}

func keepDemoVoiceEdits(prev, next library.DemoVoiceLine) library.DemoVoiceLine {
	next.Discarded = next.Discarded || prev.Discarded
	if !prev.Corrected || next.Corrected {
		return next
	}
	next.VocabText = prev.VocabText
	if prev.Text != next.Text {
		next.Words = library.SpreadVoiceWords(library.VoiceWordSpread{Text: prev.Text, StartMS: next.StartMS, EndMS: next.EndMS})
	}
	next = setDemoVoiceText(next, prev.Text)
	if next.Corrected {
		next.CorrectedBy = cmp.Or(prev.CorrectedBy, library.CorrectedByUser)
	}
	return next
}

func correctionPatches(lines []library.VoiceLineText, by library.CorrectionAuthor) []library.DemoVoiceLinePatch {
	out := make([]library.DemoVoiceLinePatch, len(lines))
	for i, l := range lines {
		out[i] = library.DemoVoiceLinePatch{ID: l.ID, Text: &l.Text, By: by}
	}
	return out
}

func learnsFrom(l library.DemoVoiceLine) bool {
	return l.Corrected && l.CorrectedBy != library.CorrectedByAssistant
}
