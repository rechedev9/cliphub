package httpapi

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/library"
	"github.com/rechedev9/cliphub/internal/renderplan"
	"github.com/rechedev9/cliphub/internal/subtitles"
)

// Subtitles gives a render of req.JobID the phrases of the target's team. It never holds the render
// for a transcription: without a finished one it queues it and the video renders without subtitles.
func (h *Handlers) Subtitles(ctx context.Context, req subtitles.Request) (*subtitles.Track, error) {
	m, err := h.subtitleMatch(ctx, req.JobID)
	if err != nil {
		return nil, err
	}
	t, err := h.demoVoiceState(ctx, m.ID)
	if err != nil {
		return nil, err
	}
	if t.State != library.TranscriptReady || (req.English && t.English != library.TranscriptReady) {
		if _, err := h.queueDemoTranscript(ctx, m, demoTranscribeRequest{English: req.English}); err != nil {
			return nil, fmt.Errorf("the transcript is not ready and cannot be queued: %w", err)
		}
		return nil, errors.New("the transcript is not ready yet")
	}
	lines, err := h.libraryRepo.ListDemoVoiceLines(ctx, library.DemoVoiceLineFilter{MatchIDs: []uuid.UUID{m.ID}, All: true})
	if err != nil {
		return nil, err
	}
	team := teamOf(m, req.Target)
	rate := m.Tickrate
	if rate <= 0 {
		rate = 64
	}
	track := &subtitles.Track{Phrases: []subtitles.Phrase{}, Hidden: req.Hidden}
	for _, l := range lines {
		text := l.Text
		if req.English {
			text = l.English
		}
		if l.TeamKey != team || text == "" {
			continue
		}
		end := l.Tick + int(math.Round(float64(l.EndMS-l.StartMS)*rate/1000))
		track.Phrases = append(track.Phrases, subtitles.Phrase{ID: l.ID.String(), Speaker: l.Name, Text: text, StartTick: l.Tick, EndTick: end})
	}
	return track, nil
}

// subtitleMatch is the analysed library match of the job's demo, whose transcript the subtitles read.
func (h *Handlers) subtitleMatch(ctx context.Context, jobID uuid.UUID) (library.Match, error) {
	if h.voice == nil || h.libraryRepo == nil {
		return library.Match{}, errors.New("team voice is not available in this Studio")
	}
	demo, err := h.libraryRepo.DemoForJob(ctx, jobID)
	if err != nil {
		return library.Match{}, fmt.Errorf("the job's demo is not an analysed library demo: %w", err)
	}
	if demo.MatchID == nil {
		return library.Match{}, errors.New("the job's demo is not an analysed library demo")
	}
	return h.libraryRepo.GetMatch(ctx, *demo.MatchID)
}

// queueSubtitles starts the transcription a new video's subtitles read, so it runs while CS2 records.
// It is best effort: a render whose transcript is not ready goes without subtitles.
func (h *Handlers) queueSubtitles(ctx context.Context, jobID uuid.UUID, edit renderplan.EditRequest) {
	on, english := edit.Subtitles, edit.SubtitlesEnglish
	if edit.FullDemo != nil {
		o := edit.FullDemo.Document.Options.Subtitles
		on, english = o != nil && o.Enabled, o != nil && o.English
	}
	if !on {
		return
	}
	m, err := h.subtitleMatch(ctx, jobID)
	if err == nil {
		_, err = h.queueDemoTranscript(ctx, m, demoTranscribeRequest{English: english})
	}
	if err != nil {
		log.Printf("httpapi: queue the transcript for the subtitles of job %s: %v", jobID, err)
	}
}
