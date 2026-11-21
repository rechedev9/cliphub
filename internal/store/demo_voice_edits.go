package store

import "github.com/rechedev9/cliphub/internal/library"

func patchDemoVoiceLine(l library.DemoVoiceLine, patch library.DemoVoiceLinePatch) library.DemoVoiceLine {
	if patch.Discarded != nil {
		l.Discarded = *patch.Discarded
	}
	if patch.Text == nil {
		return l
	}
	heard := l.OriginalText
	if !l.Corrected {
		heard = l.Text
	}
	if *patch.Text != l.Text {
		l.Words = library.SpreadVoiceWords(library.VoiceWordSpread{Text: *patch.Text, StartMS: l.StartMS, EndMS: l.EndMS})
	}
	l.Text, l.Corrected, l.OriginalText = *patch.Text, *patch.Text != heard, ""
	if l.Corrected {
		l.OriginalText = heard
	}
	return l
}

func keepDemoVoiceEdits(prev, next library.DemoVoiceLine) library.DemoVoiceLine {
	next.Discarded = next.Discarded || prev.Discarded
	if prev.Corrected && !next.Corrected && prev.Text != next.Text {
		next.OriginalText, next.Text, next.Corrected = next.Text, prev.Text, true
		next.Words = library.SpreadVoiceWords(library.VoiceWordSpread{Text: next.Text, StartMS: next.StartMS, EndMS: next.EndMS})
	}
	return next
}
