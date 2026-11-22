package store

import (
	"context"
	"slices"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/library"
	"github.com/rechedev9/cliphub/internal/streamclips"
)

type StreamVoiceRepository interface {
	GetStreamVoice(ctx context.Context, jobID uuid.UUID) (streamclips.VoiceTranscript, error)
	SetStreamVoice(ctx context.Context, update streamclips.VoiceUpdate) error
	ListStreamVoiceLines(ctx context.Context, jobID uuid.UUID) ([]streamclips.VoiceLine, error)
	UpdateStreamVoiceLine(ctx context.Context, patch streamclips.VoiceLinePatch) (streamclips.VoiceLine, error)
	ListStreamVoices(ctx context.Context, states []string) ([]streamclips.VoiceTranscript, error)
	ListStreamVoiceCorrections(ctx context.Context) ([]library.VoiceCorrection, error)
	GetStreamSubtitles(ctx context.Context, jobID uuid.UUID) (streamclips.Subtitles, error)
	SetStreamSubtitles(ctx context.Context, subtitles streamclips.Subtitles) error
}

func streamLineAsDemo(l streamclips.VoiceLine) library.DemoVoiceLine {
	return library.DemoVoiceLine{StartMS: l.StartMS, EndMS: l.EndMS, Text: l.Text, Corrected: l.Corrected, OriginalText: l.OriginalText,
		VocabText: l.VocabText, Discarded: l.Discarded, Words: l.Words}
}

func withDemoEdits(l streamclips.VoiceLine, edited library.DemoVoiceLine) streamclips.VoiceLine {
	l.Text, l.Corrected, l.OriginalText, l.VocabText, l.Discarded, l.Words = edited.Text, edited.Corrected, edited.OriginalText, edited.VocabText, edited.Discarded, edited.Words
	return l
}

func patchStreamVoiceLine(l streamclips.VoiceLine, patch streamclips.VoiceLinePatch) streamclips.VoiceLine {
	return withDemoEdits(l, patchDemoVoiceLine(streamLineAsDemo(l), library.DemoVoiceLinePatch{Text: patch.Text, Discarded: patch.Discarded}))
}

func replaceStreamVoiceLines(existing []streamclips.VoiceLine, update streamclips.VoiceUpdate) []streamclips.VoiceLine {
	prev := map[uuid.UUID]streamclips.VoiceLine{}
	out := []streamclips.VoiceLine{}
	for _, l := range existing {
		if streamclips.InVoiceSpans(l.StartMS, update.Replaced) {
			prev[l.ID] = l
			continue
		}
		out = append(out, l)
	}
	for _, l := range *update.Lines {
		l.JobID, l.Words = update.JobID, slices.Clone(l.Words)
		if old, ok := prev[l.ID]; ok {
			l = withDemoEdits(l, keepDemoVoiceEdits(streamLineAsDemo(old), streamLineAsDemo(l)))
		}
		out = append(out, l)
	}
	slices.SortStableFunc(out, func(a, b streamclips.VoiceLine) int { return int(a.StartMS - b.StartMS) })
	return out
}

func storedStreamVoice(update streamclips.VoiceUpdate) streamclips.VoiceTranscript {
	t := update.Transcript
	t.JobID, t.UpdatedAt = update.JobID, libraryNow()
	t.Spans, t.Pending = slices.Clone(t.Spans), slices.Clone(t.Pending)
	return t
}

func cloneStreamSubtitles(s streamclips.Subtitles) streamclips.Subtitles {
	hidden := make(map[string][]uuid.UUID, len(s.Hidden))
	for video, ids := range s.Hidden {
		if len(ids) > 0 {
			hidden[video] = slices.Clone(ids)
		}
	}
	s.Hidden = hidden
	return s
}
