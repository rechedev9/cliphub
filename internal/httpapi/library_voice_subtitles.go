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

func (h *Handlers) Subtitles(ctx context.Context, req subtitles.Request) (subtitles.Lookup, error) {
	m, err := h.subtitleMatch(ctx, req.JobID)
	if err != nil {
		return subtitles.Lookup{}, err
	}
	t, err := h.demoVoiceState(ctx, m.ID)
	if err != nil {
		return subtitles.Lookup{}, err
	}
	switch t.State {
	case library.TranscriptReady:
		return h.subtitleTrack(ctx, m, req)
	case library.TranscriptQueued, library.TranscriptRunning:
		return subtitles.Pending(h.voice.progressOf(m.DemoID)), nil
	case library.TranscriptFailed:
		return subtitles.Lookup{}, subtitles.ErrTranscriptFailed
	}
	if _, err := h.queueDemoTranscript(ctx, m, demoTranscribeRequest{}); err != nil {
		return subtitles.Lookup{}, fmt.Errorf("the transcript cannot be queued: %w", err)
	}
	return subtitles.Pending(0), nil
}

func (h *Handlers) subtitleTrack(ctx context.Context, m library.Match, req subtitles.Request) (subtitles.Lookup, error) {
	lines, err := h.libraryRepo.ListDemoVoiceLines(ctx, library.DemoVoiceLineFilter{MatchIDs: []uuid.UUID{m.ID}, All: true})
	if err != nil {
		return subtitles.Lookup{}, err
	}
	team := teamOf(m, req.Target)
	var roster []string
	for _, p := range m.Players {
		if p.TeamKey == team {
			roster = append(roster, p.SteamID64)
		}
	}
	rate := m.Tickrate
	if rate <= 0 {
		rate = 64
	}
	track := subtitles.Track{Phrases: []subtitles.Phrase{}, Hidden: req.Hidden}
	for _, l := range lines {
		if l.TeamKey != team || l.Text == "" || l.Discarded {
			continue
		}
		tick := func(ms int64) int { return l.Tick + int(math.Round(float64(ms-l.StartMS)*rate/1000)) }
		phrase := subtitles.Phrase{ID: l.ID.String(), Speaker: l.Name, SteamID64: l.SteamID64, Slot: subtitles.Slot(roster, l.SteamID64), Text: l.Text, StartTick: l.Tick, EndTick: tick(l.EndMS)}
		for _, w := range l.Words {
			phrase.Words = append(phrase.Words, subtitles.PhraseWord{Text: w.Text, StartTick: tick(w.StartMS), EndTick: tick(w.EndMS)})
		}
		track.Phrases = append(track.Phrases, phrase)
	}
	return subtitles.Ready(track), nil
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

func (h *Handlers) queueSubtitles(ctx context.Context, jobID uuid.UUID, edit renderplan.EditRequest) {
	on := edit.Subtitles
	if edit.FullDemo != nil {
		o := edit.FullDemo.Document.Options.Subtitles
		on = o != nil && o.Enabled
	}
	if !on {
		return
	}
	m, err := h.subtitleMatch(ctx, jobID)
	if err == nil {
		_, err = h.queueDemoTranscript(ctx, m, demoTranscribeRequest{})
	}
	if err != nil {
		log.Printf("httpapi: queue the transcript for the subtitles of job %s: %v", jobID, err)
	}
}
