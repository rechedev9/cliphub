package store

import (
	"context"
	"slices"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/library"
	"github.com/rechedev9/cliphub/internal/streamclips"
)

type memoryStreamVoice struct {
	transcript streamclips.VoiceTranscript
	stored     bool
	lines      []streamclips.VoiceLine
}

func cloneStreamVoice(t streamclips.VoiceTranscript) streamclips.VoiceTranscript {
	t.Spans, t.Pending = slices.Clone(t.Spans), slices.Clone(t.Pending)
	return t
}

func (r *MemoryStreamJobRepository) GetStreamVoice(ctx context.Context, jobID uuid.UUID) (streamclips.VoiceTranscript, error) {
	if err := ctx.Err(); err != nil {
		return streamclips.VoiceTranscript{}, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	if _, ok := r.jobs[jobID]; !ok {
		return streamclips.VoiceTranscript{}, streamclips.ErrNotFound
	}
	v := r.voices[jobID]
	if !v.stored {
		return streamclips.VoiceTranscript{JobID: jobID, State: library.TranscriptNone}, nil
	}
	return cloneStreamVoice(v.transcript), nil
}

func (r *MemoryStreamJobRepository) SetStreamVoice(ctx context.Context, update streamclips.VoiceUpdate) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.jobs[update.JobID]; !ok {
		return streamclips.ErrNotFound
	}
	v := r.voices[update.JobID]
	v.transcript, v.stored = storedStreamVoice(update), true
	if update.Lines != nil {
		v.lines = replaceStreamVoiceLines(v.lines, update)
	}
	r.voices[update.JobID] = v
	return nil
}

func (r *MemoryStreamJobRepository) ListStreamVoiceLines(ctx context.Context, jobID uuid.UUID) ([]streamclips.VoiceLine, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := []streamclips.VoiceLine{}
	for _, l := range r.voices[jobID].lines {
		l.Words = slices.Clone(l.Words)
		out = append(out, l)
	}
	return out, nil
}

func (r *MemoryStreamJobRepository) UpdateStreamVoiceLine(ctx context.Context, patch streamclips.VoiceLinePatch) (streamclips.VoiceLine, error) {
	if err := ctx.Err(); err != nil {
		return streamclips.VoiceLine{}, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	v := r.voices[patch.JobID]
	i := slices.IndexFunc(v.lines, func(l streamclips.VoiceLine) bool { return l.ID == patch.ID })
	if i < 0 {
		return streamclips.VoiceLine{}, streamclips.ErrVoiceLineNotFound
	}
	v.lines = slices.Clone(v.lines)
	v.lines[i] = patchStreamVoiceLine(v.lines[i], patch)
	r.voices[patch.JobID] = v
	out := v.lines[i]
	out.Words = slices.Clone(out.Words)
	return out, nil
}

func (r *MemoryStreamJobRepository) ListStreamVoices(ctx context.Context, states []string) ([]streamclips.VoiceTranscript, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := []streamclips.VoiceTranscript{}
	for _, v := range r.voices {
		if v.stored && slices.Contains(states, v.transcript.State) {
			out = append(out, cloneStreamVoice(v.transcript))
		}
	}
	slices.SortFunc(out, func(a, b streamclips.VoiceTranscript) int { return compareUUID(a.JobID, b.JobID) })
	return out, nil
}

func (r *MemoryStreamJobRepository) GetStreamSubtitles(ctx context.Context, jobID uuid.UUID) (streamclips.Subtitles, error) {
	if err := ctx.Err(); err != nil {
		return streamclips.Subtitles{}, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	if _, ok := r.jobs[jobID]; !ok {
		return streamclips.Subtitles{}, streamclips.ErrNotFound
	}
	s := cloneStreamSubtitles(r.subtitles[jobID])
	s.JobID = jobID
	return s, nil
}

func (r *MemoryStreamJobRepository) SetStreamSubtitles(ctx context.Context, subtitles streamclips.Subtitles) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.jobs[subtitles.JobID]; !ok {
		return streamclips.ErrNotFound
	}
	r.subtitles[subtitles.JobID] = cloneStreamSubtitles(subtitles)
	return nil
}
