package store

import (
	"context"
	"slices"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/library"
)

// memoryRecordings holds the plugin recordings and team speakers; the zero value is empty.
type memoryRecordings struct {
	byID     map[string]library.VoiceRecording
	speakers map[uuid.UUID]map[string]library.VoiceSpeaker
}

// unlinkMatch leaves a deleted match's recordings waiting again, as ON DELETE SET NULL does.
func (m *memoryRecordings) unlinkMatch(matchID uuid.UUID) {
	for id, rec := range m.byID {
		if rec.MatchID != nil && *rec.MatchID == matchID {
			rec.MatchID = nil
			m.byID[id] = rec
		}
	}
}

// moveTeamSpeakers folds a merged team's speakers into the kept team, which keeps its own on a clash.
func (m *memoryRecordings) moveTeamSpeakers(from, into uuid.UUID) {
	moved := m.speakers[from]
	if len(moved) == 0 {
		return
	}
	target := m.speakers[into]
	if target == nil {
		target = map[string]library.VoiceSpeaker{}
		m.speakers[into] = target
	}
	for key, s := range moved {
		if _, ok := target[key]; !ok {
			s.TeamID = into
			target[key] = s
		}
	}
	delete(m.speakers, from)
}

func (r *MemoryLibraryRepository) CreateVoiceRecording(ctx context.Context, rec library.VoiceRecording) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.recordings.byID == nil {
		r.recordings.byID = map[string]library.VoiceRecording{}
	}
	if _, ok := r.recordings.byID[rec.ID]; ok {
		return library.ErrConflict
	}
	now := libraryNow()
	rec = recordingCopy(rec)
	rec.MatchID, rec.CreatedAt, rec.UpdatedAt = nil, now, now
	rec.StartedAt, rec.EndedAt = fromMillis(millis(rec.StartedAt)), fromMillis(millis(rec.EndedAt))
	for i, rd := range rec.Rounds {
		rec.Rounds[i].LiveAt = fromMillis(millis(rd.LiveAt))
	}
	r.recordings.byID[rec.ID] = rec
	return nil
}

func (r *MemoryLibraryRepository) GetVoiceRecording(ctx context.Context, id string) (library.VoiceRecording, error) {
	if err := ctx.Err(); err != nil {
		return library.VoiceRecording{}, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	rec, ok := r.recordings.byID[id]
	if !ok {
		return library.VoiceRecording{}, library.ErrNotFound
	}
	return recordingCopy(rec), nil
}

func (r *MemoryLibraryRepository) ListVoiceRecordings(ctx context.Context) ([]library.VoiceRecording, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]library.VoiceRecording, 0, len(r.recordings.byID))
	for _, rec := range r.recordings.byID {
		out = append(out, recordingCopy(rec))
	}
	sortRecordings(out)
	return out, nil
}

func (r *MemoryLibraryRepository) UpdateVoiceRecording(ctx context.Context, rec library.VoiceRecording) (library.VoiceRecording, error) {
	if err := ctx.Err(); err != nil {
		return library.VoiceRecording{}, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	stored, ok := r.recordings.byID[rec.ID]
	if !ok {
		return library.VoiceRecording{}, library.ErrNotFound
	}
	if rec.MatchID != nil {
		if _, ok := r.matches[*rec.MatchID]; !ok {
			return library.VoiceRecording{}, library.ErrNotFound
		}
	}
	edit := recordingCopy(rec)
	stored.MatchID, stored.Note, stored.Tracks, stored.UpdatedAt = edit.MatchID, edit.Note, edit.Tracks, libraryNow()
	r.recordings.byID[rec.ID] = stored
	return recordingCopy(stored), nil
}

func (r *MemoryLibraryRepository) DeleteVoiceRecording(ctx context.Context, id string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.recordings.byID[id]; !ok {
		return library.ErrNotFound
	}
	delete(r.recordings.byID, id)
	return nil
}

func (r *MemoryLibraryRepository) SaveVoiceSpeakers(ctx context.Context, speakers []library.VoiceSpeaker) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, s := range speakers {
		if _, ok := r.teams[s.TeamID]; !ok {
			return library.ErrNotFound
		}
	}
	if r.recordings.speakers == nil {
		r.recordings.speakers = map[uuid.UUID]map[string]library.VoiceSpeaker{}
	}
	now := libraryNow()
	for _, s := range speakers {
		team := r.recordings.speakers[s.TeamID]
		if team == nil {
			team = map[string]library.VoiceSpeaker{}
			r.recordings.speakers[s.TeamID] = team
		}
		s.UpdatedAt = now
		team[s.SpeakerKey] = s
	}
	return nil
}

func (r *MemoryLibraryRepository) ListVoiceSpeakers(ctx context.Context, teamIDs []uuid.UUID) ([]library.VoiceSpeaker, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := []library.VoiceSpeaker{}
	for _, id := range teamIDs {
		for _, s := range r.recordings.speakers[id] {
			out = append(out, s)
		}
	}
	slices.SortFunc(out, func(a, b library.VoiceSpeaker) int {
		if c := compareStrings(a.TeamID.String(), b.TeamID.String()); c != 0 {
			return c
		}
		return compareStrings(a.SpeakerKey, b.SpeakerKey)
	})
	return out, nil
}
