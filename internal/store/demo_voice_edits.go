package store

import "github.com/rechedev9/cliphub/internal/library"

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
	return l
}

func patchDemoVoiceLine(l library.DemoVoiceLine, patch library.DemoVoiceLinePatch) library.DemoVoiceLine {
	if patch.Discarded != nil {
		l.Discarded = *patch.Discarded
	}
	if patch.Text == nil {
		return l
	}
	if *patch.Text != l.Text {
		l.Words = library.SpreadVoiceWords(library.VoiceWordSpread{Text: *patch.Text, StartMS: l.StartMS, EndMS: l.EndMS})
	}
	return setDemoVoiceText(l, *patch.Text)
}

func keepDemoVoiceEdits(prev, next library.DemoVoiceLine) library.DemoVoiceLine {
	next.Discarded = next.Discarded || prev.Discarded
	if prev.Corrected && !next.Corrected && prev.Text != next.Text {
		next = setDemoVoiceText(next, prev.Text)
		next.Words = library.SpreadVoiceWords(library.VoiceWordSpread{Text: next.Text, StartMS: next.StartMS, EndMS: next.EndMS})
	}
	return next
}
