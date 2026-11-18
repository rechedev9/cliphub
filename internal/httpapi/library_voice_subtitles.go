package httpapi

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/library"
	"github.com/rechedev9/cliphub/internal/subtitles"
)

// subtitleWait bounds how long a render waits for its demo's transcription.
const subtitleWait = 30 * time.Minute

// Subtitles gives a render of req.JobID the phrases of the target's team. It queues the
// transcription when none is stored and waits for it; an error renders the video without them.
func (h *Handlers) Subtitles(ctx context.Context, req subtitles.Request) (*subtitles.Track, error) {
	if h.voice == nil || h.libraryRepo == nil {
		return nil, errors.New("team voice is not available in this Studio")
	}
	demo, err := h.libraryRepo.DemoForJob(ctx, req.JobID)
	if err != nil || demo.MatchID == nil {
		return nil, fmt.Errorf("the job's demo is not an analysed library demo: %w", err)
	}
	m, err := h.libraryRepo.GetMatch(ctx, *demo.MatchID)
	if err != nil {
		return nil, err
	}
	if _, err := h.queueDemoTranscript(ctx, m, demoTranscribeRequest{English: req.English}); err != nil {
		return nil, err
	}
	if err := h.awaitDemoTranscript(ctx, m.ID, req.English); err != nil {
		return nil, err
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

// awaitDemoTranscript waits until the transcript, and its English when asked, is done.
func (h *Handlers) awaitDemoTranscript(ctx context.Context, matchID uuid.UUID, english bool) error {
	ctx, cancel := context.WithTimeout(ctx, subtitleWait)
	defer cancel()
	for {
		t, err := h.demoVoiceState(ctx, matchID)
		switch {
		case err != nil:
			return err
		case t.State == library.TranscriptFailed:
			return errors.New("the transcription failed")
		case english && t.English == library.TranscriptFailed:
			return errors.New("the translation failed")
		case t.State == library.TranscriptReady && (!english || t.English == library.TranscriptReady):
			return nil
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("the transcription did not finish: %w", ctx.Err())
		case <-time.After(time.Second):
		}
	}
}
