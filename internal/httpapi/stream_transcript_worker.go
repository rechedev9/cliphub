package httpapi

import (
	"context"
	"errors"
	"fmt"
	"log"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/library"
	"github.com/rechedev9/cliphub/internal/store"
	"github.com/rechedev9/cliphub/internal/streamclips"
	"github.com/rechedev9/cliphub/internal/teamvoice"
	"github.com/rechedev9/cliphub/internal/vocab"
)

func (h *Handlers) requeueStreamTranscripts(ctx context.Context) {
	repo, ok := h.streamRepo.(store.StreamVoiceRepository)
	if !ok {
		return
	}
	pending, err := repo.ListStreamVoices(ctx, []string{library.TranscriptQueued, library.TranscriptRunning})
	if err != nil {
		log.Printf("httpapi: list pending stream transcripts: %v", err)
		return
	}
	for _, t := range pending {
		if !h.enqueueTranscript(voiceJob{matchID: t.JobID, stream: true}) {
			log.Printf("httpapi: no room to queue the transcript of stream %s again", t.JobID)
			h.failStreamTranscript(ctx, streamVoiceRun{repo: repo, jobID: t.JobID})
		}
	}
}

type streamVoiceRun struct {
	repo   store.StreamVoiceRepository
	jobID  uuid.UUID
	doneMS int64
}

func (h *Handlers) transcribeStream(ctx context.Context, jobID uuid.UUID) {
	repo, ok := h.streamRepo.(store.StreamVoiceRepository)
	if !ok {
		return
	}
	h.voice.setProgress(jobID, 0)
	defer h.voice.setProgress(jobID, -1)
	run := streamVoiceRun{repo: repo, jobID: jobID}
	corrector := h.streamVocabulary(ctx, repo)
	for ctx.Err() == nil {
		turn, ok, err := h.nextStreamSpan(ctx, run)
		if err != nil && ctx.Err() == nil {
			log.Printf("httpapi: read the transcript of stream %s: %v", jobID, err)
			h.failStreamTranscript(context.WithoutCancel(ctx), run)
		}
		if err != nil || !ok {
			return
		}
		span, leftMS := turn.span, turn.leftMS
		length := span.EndMS - span.StartMS
		var lines []streamclips.VoiceLine
		lines, err = h.transcribeStreamSpan(ctx, streamSpanJob{jobID: jobID, span: span, vocab: corrector, progress: func(p float64) {
			h.voice.setProgress(jobID, (float64(run.doneMS)+p*float64(length))/float64(run.doneMS+leftMS))
		}})
		if err == nil {
			err = h.storeStreamSpan(ctx, run, streamSpanResult{span: span, lines: lines})
		}
		if ctx.Err() != nil {
			return
		}
		if err != nil {
			log.Printf("httpapi: transcribe stream %s: %v", jobID, err)
			h.failStreamTranscript(context.WithoutCancel(ctx), run)
			return
		}
		run.doneMS += length
	}
}

type streamTurn struct {
	span   library.VoiceSpan
	leftMS int64
}

func (h *Handlers) nextStreamSpan(ctx context.Context, run streamVoiceRun) (streamTurn, bool, error) {
	h.voice.streamMu.Lock()
	defer h.voice.streamMu.Unlock()
	t, err := run.repo.GetStreamVoice(ctx, run.jobID)
	if err != nil || !busyTranscript(t.State) {
		return streamTurn{}, false, err
	}
	t.State = library.TranscriptReady
	if len(t.Pending) > 0 {
		t.State = library.TranscriptRunning
	}
	if err := run.repo.SetStreamVoice(ctx, streamclips.VoiceUpdate{JobID: run.jobID, Transcript: t}); err != nil || len(t.Pending) == 0 {
		return streamTurn{}, false, err
	}
	turn := streamTurn{span: t.Pending[0]}
	for _, s := range t.Pending {
		turn.leftMS += s.EndMS - s.StartMS
	}
	return turn, true, nil
}

type streamSpanJob struct {
	jobID    uuid.UUID
	span     library.VoiceSpan
	vocab    *vocab.Vocabulary
	progress func(float64)
}

func (h *Handlers) transcribeStreamSpan(ctx context.Context, in streamSpanJob) ([]streamclips.VoiceLine, error) {
	j, err := h.streamRepo.Get(ctx, in.jobID)
	if err != nil {
		return nil, err
	}
	track, availability := streamVoiceTrack(j.Probe)
	if availability == streamVoiceNoAudio {
		return nil, errNoStreamAudio
	}
	resolver, ok := h.storage.(storagePathResolver)
	if !ok {
		return nil, errors.New("stream transcripts need local storage")
	}
	source, err := resolver.ResolvePath(j.SourcePath)
	if err != nil {
		return nil, err
	}
	got, err := h.voice.deps.Transcriber.Transcribe(ctx, teamvoice.TranscribeJob{Audio: source, Track: track, Clip: &in.span, Progress: in.progress})
	if err != nil {
		return nil, err
	}
	lines := []streamclips.VoiceLine{}
	ids := demoLineIDs{match: in.jobID, seen: map[uuid.UUID]int{}}
	for _, l := range got {
		if l.StartMS < in.span.StartMS || l.StartMS >= in.span.EndMS {
			continue
		}
		words := make([]library.DemoVoiceWord, 0, len(l.Words))
		for _, w := range l.Words {
			words = append(words, library.DemoVoiceWord{Text: w.Text, StartMS: w.StartMS, EndMS: w.EndMS})
		}
		lines = append(lines, correctStreamLine(streamclips.VoiceLine{ID: ids.next(fmt.Sprintf("%d|%s", l.StartMS, l.Text)), StartMS: l.StartMS, EndMS: l.EndMS, Text: l.Text, Words: words}, in.vocab))
	}
	return lines, nil
}

type streamSpanResult struct {
	span  library.VoiceSpan
	lines []streamclips.VoiceLine
}

func (h *Handlers) storeStreamSpan(ctx context.Context, run streamVoiceRun, in streamSpanResult) error {
	h.voice.streamMu.Lock()
	defer h.voice.streamMu.Unlock()
	t, err := run.repo.GetStreamVoice(ctx, run.jobID)
	if err != nil {
		return err
	}
	done := []library.VoiceSpan{in.span}
	t.Pending = streamclips.MissingVoiceSpans(t.Pending, done)
	t.Spans = streamclips.MergeVoiceSpans(append(t.Spans, in.span))
	return run.repo.SetStreamVoice(ctx, streamclips.VoiceUpdate{JobID: run.jobID, Transcript: t, Replaced: done, Lines: &in.lines})
}

func (h *Handlers) failStreamTranscript(ctx context.Context, run streamVoiceRun) {
	h.voice.streamMu.Lock()
	defer h.voice.streamMu.Unlock()
	t, err := run.repo.GetStreamVoice(ctx, run.jobID)
	if err != nil || !busyTranscript(t.State) {
		return
	}
	t.State, t.Pending, t.Error = library.TranscriptFailed, nil, "La transcripción falló en este PC; studio.log tiene el detalle."
	if err := run.repo.SetStreamVoice(ctx, streamclips.VoiceUpdate{JobID: run.jobID, Transcript: t}); err != nil {
		log.Printf("httpapi: mark the transcript of stream %s failed: %v", run.jobID, err)
	}
}
