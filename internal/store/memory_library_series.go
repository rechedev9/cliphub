package store

import (
	"context"
	"sort"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/library"
)

func (r *MemoryLibraryRepository) seriesTeamsExist(ids []uuid.UUID) bool {
	for _, id := range ids {
		if _, ok := r.teams[id]; !ok {
			return false
		}
	}
	return true
}

func (r *MemoryLibraryRepository) CreateSeries(ctx context.Context, s *library.Series) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if s.ID == uuid.Nil {
		s.ID = uuid.New()
	}
	now := libraryNow()
	r.mu.Lock()
	if _, exists := r.series[s.ID]; exists {
		r.mu.Unlock()
		return library.ErrConflict
	}
	if !r.seriesTeamsExist(s.TeamIDs) {
		r.mu.Unlock()
		return library.ErrNotFound
	}
	stored := *s
	stored.CreatedAt, stored.UpdatedAt = now, now
	stored.TeamIDs = append([]uuid.UUID{}, s.TeamIDs...)
	stored.Vetoes = append([]library.VetoStep{}, s.Vetoes...)
	r.series[s.ID] = stored
	*s = r.seriesView(stored)
	r.mu.Unlock()
	return nil
}

// seriesView mirrors the SQLite read: slots whose team is gone are empty, and
// the matches come by date then id.
func (r *MemoryLibraryRepository) seriesView(s library.Series) library.Series {
	var matches []memoryLibraryMatch
	for _, m := range r.matches {
		if m.seriesID != nil && *m.seriesID == s.ID {
			matches = append(matches, m)
		}
	}
	sort.Slice(matches, func(i, j int) bool {
		if !matches[i].playedAt.Equal(matches[j].playedAt) {
			return matches[i].playedAt.Before(matches[j].playedAt)
		}
		return matches[i].id.String() < matches[j].id.String()
	})
	s.MatchIDs = make([]uuid.UUID, 0, len(matches))
	for _, m := range matches {
		s.MatchIDs = append(s.MatchIDs, m.id)
	}
	ids := s.TeamIDs
	s.TeamIDs, s.Teams = []uuid.UUID{}, []library.SeriesTeam{}
	for i, id := range ids {
		t, ok := r.teams[id]
		if !ok || i > 1 {
			continue
		}
		slot := library.SeriesTeamA
		if i == 1 {
			slot = library.SeriesTeamB
		}
		s.TeamIDs = append(s.TeamIDs, id)
		s.Teams = append(s.Teams, library.SeriesTeam{Slot: slot, TeamID: id, Name: t.name})
	}
	s.Vetoes = append([]library.VetoStep{}, s.Vetoes...)
	return s
}

func (r *MemoryLibraryRepository) GetSeries(ctx context.Context, id uuid.UUID) (library.Series, error) {
	if err := ctx.Err(); err != nil {
		return library.Series{}, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	s, ok := r.series[id]
	if !ok {
		return library.Series{}, library.ErrNotFound
	}
	return r.seriesView(s), nil
}

func (r *MemoryLibraryRepository) ListSeries(ctx context.Context, filter library.SeriesFilter) ([]library.Series, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]library.Series, 0, len(r.series))
	for _, s := range r.series {
		view := r.seriesView(s)
		if filter.TeamID != nil && view.Slot(*filter.TeamID) == "" {
			continue
		}
		out = append(out, view)
	}
	sort.Slice(out, func(i, j int) bool {
		if !out[i].CreatedAt.Equal(out[j].CreatedAt) {
			return out[i].CreatedAt.After(out[j].CreatedAt)
		}
		return out[i].ID.String() < out[j].ID.String()
	})
	return libraryPage(out, filter.Limit, filter.Offset), nil
}

func (r *MemoryLibraryRepository) UpdateSeries(ctx context.Context, id uuid.UUID, s library.Series) (library.Series, error) {
	if err := ctx.Err(); err != nil {
		return library.Series{}, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	stored, ok := r.series[id]
	if !ok {
		return library.Series{}, library.ErrNotFound
	}
	if !r.seriesTeamsExist(s.TeamIDs) {
		return library.Series{}, library.ErrNotFound
	}
	stored.Format, stored.Event, stored.Veto = s.Format, s.Event, s.Veto
	stored.TeamIDs = append([]uuid.UUID{}, s.TeamIDs...)
	stored.Vetoes = append([]library.VetoStep{}, s.Vetoes...)
	stored.UpdatedAt = libraryNow()
	r.series[id] = stored
	return r.seriesView(stored), nil
}

func (r *MemoryLibraryRepository) DeleteSeries(ctx context.Context, id uuid.UUID) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.series[id]; !ok {
		return library.ErrNotFound
	}
	delete(r.series, id)
	for matchID, m := range r.matches {
		if m.seriesID != nil && *m.seriesID == id {
			m.seriesID = nil
			r.matches[matchID] = m
		}
	}
	return nil
}

// moveSeriesTeam points every series slot held by from at into, as the merge
// does in SQLite. A series between the two teams keeps one slot of into.
func (r *MemoryLibraryRepository) moveSeriesTeam(from, into uuid.UUID) {
	for id, s := range r.series {
		changed := false
		s.TeamIDs = append([]uuid.UUID{}, s.TeamIDs...)
		for i, teamID := range s.TeamIDs {
			if teamID == from {
				s.TeamIDs[i] = into
				changed = true
			}
		}
		if changed && len(s.TeamIDs) == 2 && s.TeamIDs[0] == s.TeamIDs[1] {
			s.TeamIDs[1] = uuid.Nil
		}
		if changed {
			r.series[id] = s
		}
	}
}

func (r *MemoryLibraryRepository) teamHasSeries(teamID uuid.UUID) bool {
	for _, s := range r.series {
		for _, id := range s.TeamIDs {
			if id == teamID {
				return true
			}
		}
	}
	return false
}
