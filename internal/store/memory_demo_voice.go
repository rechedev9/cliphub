package store

import (
	"context"
	"slices"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/library"
)

type memoryDemoVoice struct {
	transcript library.DemoVoiceTranscript
	lines      []library.DemoVoiceLine
}

func (r *MemoryLibraryRepository) GetDemoVoice(ctx context.Context, matchID uuid.UUID) (library.DemoVoiceTranscript, error) {
	if err := ctx.Err(); err != nil {
		return library.DemoVoiceTranscript{}, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	v, ok := r.demoVoices[matchID]
	if !ok {
		return library.DemoVoiceTranscript{}, library.ErrNotFound
	}
	return v.transcript, nil
}

func (r *MemoryLibraryRepository) SetDemoVoice(ctx context.Context, update library.DemoVoiceUpdate) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.matches[update.MatchID]; !ok {
		return library.ErrNotFound
	}
	v := r.demoVoices[update.MatchID]
	v.transcript = update.Transcript
	v.transcript.MatchID, v.transcript.UpdatedAt = update.MatchID, libraryNow()
	if update.Lines != nil {
		prev := make(map[uuid.UUID]library.DemoVoiceLine, len(v.lines))
		for _, l := range v.lines {
			prev[l.ID] = l
		}
		v.lines = make([]library.DemoVoiceLine, 0, len(*update.Lines))
		for _, l := range *update.Lines {
			l.MatchID, l.Words = update.MatchID, slices.Clone(l.Words)
			if old, ok := prev[l.ID]; ok {
				l = keepDemoVoiceEdits(old, l)
			}
			v.lines = append(v.lines, l)
		}
		slices.SortStableFunc(v.lines, demoVoiceLineCompare)
	}
	r.demoVoices[update.MatchID] = v
	return nil
}

func (r *MemoryLibraryRepository) ListDemoVoiceLines(ctx context.Context, filter library.DemoVoiceLineFilter) ([]library.DemoVoiceLine, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := []library.DemoVoiceLine{}
	for _, v := range r.demoVoices {
		for _, l := range v.lines {
			if demoVoiceLineMatches(l, filter) {
				l.Words = slices.Clone(l.Words)
				out = append(out, l)
			}
		}
	}
	slices.SortFunc(out, demoVoiceLineCompare)
	if filter.All {
		return out, nil
	}
	return out[:min(len(out), clampVoiceLineLimit(filter.Limit))], nil
}

func (r *MemoryLibraryRepository) UpdateDemoVoiceLine(ctx context.Context, patch library.DemoVoiceLinePatch) (library.DemoVoiceLine, error) {
	out, err := r.patchDemoVoiceLines(ctx, patch.MatchID, []library.DemoVoiceLinePatch{patch})
	if err != nil {
		return library.DemoVoiceLine{}, err
	}
	return out[0], nil
}

func (r *MemoryLibraryRepository) CorrectDemoVoiceLines(ctx context.Context, batch library.DemoVoiceCorrections) ([]library.DemoVoiceLine, error) {
	return r.patchDemoVoiceLines(ctx, batch.MatchID, correctionPatches(batch.Lines, batch.By))
}

func (r *MemoryLibraryRepository) patchDemoVoiceLines(ctx context.Context, matchID uuid.UUID, patches []library.DemoVoiceLinePatch) ([]library.DemoVoiceLine, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	v, ok := r.demoVoices[matchID]
	if !ok {
		return nil, library.ErrNotFound
	}
	v.lines = slices.Clone(v.lines)
	out := make([]library.DemoVoiceLine, 0, len(patches))
	for _, patch := range patches {
		i := slices.IndexFunc(v.lines, func(l library.DemoVoiceLine) bool { return l.ID == patch.ID })
		if i < 0 {
			return nil, library.ErrNotFound
		}
		v.lines[i] = patchDemoVoiceLine(v.lines[i], patch)
		line := v.lines[i]
		line.Words = slices.Clone(line.Words)
		out = append(out, line)
	}
	r.demoVoices[matchID] = v
	return out, nil
}

func (r *MemoryLibraryRepository) ListDemoVoices(ctx context.Context, states []string) ([]library.DemoVoiceTranscript, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := []library.DemoVoiceTranscript{}
	for _, v := range r.demoVoices {
		if slices.Contains(states, v.transcript.State) {
			out = append(out, v.transcript)
		}
	}
	slices.SortFunc(out, func(a, b library.DemoVoiceTranscript) int { return compareUUID(a.MatchID, b.MatchID) })
	return out, nil
}

func (r *MemoryLibraryRepository) ListDemoVoiceCorrections(ctx context.Context) ([]library.VoiceCorrection, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	matches := make([]uuid.UUID, 0, len(r.demoVoices))
	for id := range r.demoVoices {
		matches = append(matches, id)
	}
	slices.SortFunc(matches, compareUUID)
	out := []library.VoiceCorrection{}
	for _, id := range matches {
		for _, l := range r.demoVoices[id].lines {
			if learnsFrom(l) {
				out = append(out, voiceCorrection(id, l))
			}
		}
	}
	return out, nil
}

func compareUUID(a, b uuid.UUID) int {
	switch as, bs := a.String(), b.String(); {
	case as < bs:
		return -1
	case as > bs:
		return 1
	}
	return 0
}
