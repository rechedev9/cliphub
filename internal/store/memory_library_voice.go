package store

import (
	"context"
	"sort"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/library"
)

type memoryVoice struct {
	voice library.MatchVoice
	lines []library.VoiceLine
}

func (r *MemoryLibraryRepository) GetMatchVoice(ctx context.Context, matchID uuid.UUID) (library.MatchVoice, error) {
	if err := ctx.Err(); err != nil {
		return library.MatchVoice{}, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	v, ok := r.voices[matchID]
	if !ok {
		return library.MatchVoice{}, library.ErrNotFound
	}
	return voiceCopy(v.voice), nil
}

func (r *MemoryLibraryRepository) AddVoiceTracks(ctx context.Context, add library.VoiceAdd) (library.MatchVoice, error) {
	if err := ctx.Err(); err != nil {
		return library.MatchVoice{}, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.matches[add.MatchID]; !ok {
		return library.MatchVoice{}, library.ErrNotFound
	}
	now := libraryNow()
	v, ok := r.voices[add.MatchID]
	if !ok {
		v.voice = library.MatchVoice{MatchID: add.MatchID, Sync: add.Sync, CreatedAt: now,
			Transcript: library.VoiceTranscript{State: library.TranscriptNone, UpdatedAt: now}}
	}
	if len(v.voice.Tracks)+len(add.Tracks) > library.MaxVoiceTracks {
		return library.MatchVoice{}, library.ErrConflict
	}
	next := 1
	for _, t := range v.voice.Tracks {
		next = max(next, t.Track+1)
	}
	for _, t := range add.Tracks {
		t.Track = next
		next++
		t.Speech = append([]library.VoiceSpan{}, t.Speech...)
		v.voice.Tracks = append(v.voice.Tracks, t)
	}
	v.voice.UpdatedAt = now
	r.voices[add.MatchID] = v
	return voiceCopy(v.voice), nil
}

func (r *MemoryLibraryRepository) SetVoiceSync(ctx context.Context, matchID uuid.UUID, sync library.VoiceSync) (library.MatchVoice, error) {
	return r.editVoice(ctx, matchID, func(v *library.MatchVoice) error {
		v.Sync = sync
		return nil
	})
}

func (r *MemoryLibraryRepository) UpdateVoiceTrack(ctx context.Context, patch library.VoiceTrackPatch) (library.MatchVoice, error) {
	return r.editVoice(ctx, patch.MatchID, func(v *library.MatchVoice) error {
		for i, t := range v.Tracks {
			if t.Track != patch.Track {
				continue
			}
			if patch.Label != nil {
				v.Tracks[i].Label = *patch.Label
			}
			if patch.SteamID64 != nil {
				v.Tracks[i].SteamID64 = *patch.SteamID64
			}
			return nil
		}
		return library.ErrNotFound
	})
}

// editVoice applies fn to a stored voice and moves its edit time.
func (r *MemoryLibraryRepository) editVoice(ctx context.Context, matchID uuid.UUID, fn func(v *library.MatchVoice) error) (library.MatchVoice, error) {
	if err := ctx.Err(); err != nil {
		return library.MatchVoice{}, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	stored, ok := r.voices[matchID]
	if !ok {
		return library.MatchVoice{}, library.ErrNotFound
	}
	v := voiceCopy(stored.voice)
	if err := fn(&v); err != nil {
		return library.MatchVoice{}, err
	}
	v.UpdatedAt = libraryNow()
	stored.voice = v
	r.voices[matchID] = stored
	return voiceCopy(v), nil
}

func (r *MemoryLibraryRepository) SetVoiceTranscript(ctx context.Context, update library.VoiceTranscriptUpdate) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	stored, ok := r.voices[update.MatchID]
	if !ok {
		return library.ErrNotFound
	}
	stored.voice.Transcript = update.Transcript
	stored.voice.Transcript.UpdatedAt = libraryTime(update.Transcript.UpdatedAt)
	if update.Transcript.UpdatedAt.IsZero() {
		stored.voice.Transcript.UpdatedAt = libraryNow()
	}
	if update.Lines != nil {
		stored.lines = make([]library.VoiceLine, 0, len(*update.Lines))
		for _, l := range *update.Lines {
			l.MatchID = update.MatchID
			stored.lines = append(stored.lines, l)
		}
		sort.SliceStable(stored.lines, func(i, j int) bool { return voiceLineLess(stored.lines[i], stored.lines[j]) })
	}
	r.voices[update.MatchID] = stored
	return nil
}

func (r *MemoryLibraryRepository) ListVoiceLines(ctx context.Context, filter library.VoiceLineFilter) ([]library.VoiceLine, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := []library.VoiceLine{}
	for _, v := range r.voices {
		for _, l := range v.lines {
			if voiceLineMatches(l, filter) {
				out = append(out, l)
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return voiceLineLess(out[i], out[j]) })
	return out[:min(len(out), clampVoiceLineLimit(filter.Limit))], nil
}

func (r *MemoryLibraryRepository) ListVoicesByTranscript(ctx context.Context, states []string) ([]uuid.UUID, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := []uuid.UUID{}
	for id, v := range r.voices {
		for _, s := range states {
			if v.voice.Transcript.State == s {
				out = append(out, id)
				break
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].String() < out[j].String() })
	return out, nil
}

func (r *MemoryLibraryRepository) ListVoiceSummaries(ctx context.Context) ([]library.VoiceSummary, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := []library.VoiceSummary{}
	for id, v := range r.voices {
		out = append(out, library.VoiceSummary{MatchID: id, Tracks: len(v.voice.Tracks), Sync: v.voice.Sync})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].MatchID.String() < out[j].MatchID.String() })
	return out, nil
}

func (r *MemoryLibraryRepository) DeleteMatchVoice(ctx context.Context, matchID uuid.UUID) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.voices, matchID)
	return nil
}

// voiceLineLess orders lines by match id, then audio time and track, as SQLite does.
func voiceLineLess(a, b library.VoiceLine) bool {
	if a.MatchID != b.MatchID {
		return a.MatchID.String() < b.MatchID.String()
	}
	if a.StartMS != b.StartMS {
		return a.StartMS < b.StartMS
	}
	return a.Track < b.Track
}
