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
	out, err := r.patchStreamVoiceLines(ctx, patch.JobID, []streamclips.VoiceLinePatch{patch})
	if err != nil {
		return streamclips.VoiceLine{}, err
	}
	return out[0], nil
}

func (r *MemoryStreamJobRepository) CorrectStreamVoiceLines(ctx context.Context, batch streamclips.VoiceCorrections) ([]streamclips.VoiceLine, error) {
	return r.patchStreamVoiceLines(ctx, batch.JobID, streamCorrectionPatches(batch))
}

func (r *MemoryStreamJobRepository) patchStreamVoiceLines(ctx context.Context, jobID uuid.UUID, patches []streamclips.VoiceLinePatch) ([]streamclips.VoiceLine, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	v := r.voices[jobID]
	v.lines = slices.Clone(v.lines)
	out := make([]streamclips.VoiceLine, 0, len(patches))
	for _, patch := range patches {
		i := slices.IndexFunc(v.lines, func(l streamclips.VoiceLine) bool { return l.ID == patch.ID })
		if i < 0 {
			return nil, streamclips.ErrVoiceLineNotFound
		}
		v.lines[i] = patchStreamVoiceLine(v.lines[i], patch)
		line := v.lines[i]
		line.Words = slices.Clone(line.Words)
		out = append(out, line)
	}
	r.voices[jobID] = v
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

func (r *MemoryStreamJobRepository) ListStreamVoiceCorrections(ctx context.Context) ([]library.VoiceCorrection, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	jobs := make([]uuid.UUID, 0, len(r.voices))
	for id := range r.voices {
		jobs = append(jobs, id)
	}
	slices.SortFunc(jobs, compareUUID)
	out := []library.VoiceCorrection{}
	for _, id := range jobs {
		for _, l := range r.voices[id].lines {
			if learnsFrom(streamLineAsDemo(l)) {
				out = append(out, voiceCorrection(id, streamLineAsDemo(l)))
			}
		}
	}
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
