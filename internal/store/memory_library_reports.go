package store

import (
	"context"
	"sort"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/library"
)

// applyReportPatch is the edit both repositories apply to a stored report.
func applyReportPatch(rep library.Report, patch library.ReportPatch) library.Report {
	if patch.Title != nil {
		rep.Title = *patch.Title
	}
	if patch.MatchIDs != nil {
		rep.MatchIDs = uniqueMatchIDs(*patch.MatchIDs)
	}
	if patch.Side != nil {
		rep.Side = *patch.Side
	}
	if patch.Notes != nil {
		rep.Notes = *patch.Notes
	}
	return rep
}

func (r *MemoryLibraryRepository) teamHasReports(teamID uuid.UUID) bool {
	for _, rep := range r.reports {
		if rep.TeamID == teamID {
			return true
		}
	}
	return false
}

// reportView fills the team name and copies the match ids, so the caller owns
// what it reads.
func (r *MemoryLibraryRepository) reportView(rep library.Report) library.Report {
	rep.TeamName = r.teams[rep.TeamID].name
	rep.MatchIDs = append([]uuid.UUID{}, rep.MatchIDs...)
	return rep
}

func (r *MemoryLibraryRepository) CreateReport(ctx context.Context, rep *library.Report) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.teams[rep.TeamID]; !ok {
		return library.ErrNotFound
	}
	if rep.ID == uuid.Nil {
		rep.ID = uuid.New()
	}
	now := libraryNow()
	stored := *rep
	stored.MatchIDs = uniqueMatchIDs(rep.MatchIDs)
	stored.CreatedAt, stored.UpdatedAt = now, now
	r.reports[stored.ID] = stored
	*rep = r.reportView(stored)
	return nil
}

func (r *MemoryLibraryRepository) GetReport(ctx context.Context, id uuid.UUID) (library.Report, error) {
	if err := ctx.Err(); err != nil {
		return library.Report{}, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	rep, ok := r.reports[id]
	if !ok {
		return library.Report{}, library.ErrNotFound
	}
	return r.reportView(rep), nil
}

func (r *MemoryLibraryRepository) ListReports(ctx context.Context, filter library.ReportFilter) ([]library.Report, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := []library.Report{}
	for _, rep := range r.reports {
		if filter.TeamID != nil && rep.TeamID != *filter.TeamID {
			continue
		}
		out = append(out, r.reportView(rep))
	}
	sort.Slice(out, func(i, j int) bool {
		if !out[i].UpdatedAt.Equal(out[j].UpdatedAt) {
			return out[i].UpdatedAt.After(out[j].UpdatedAt)
		}
		return out[i].ID.String() < out[j].ID.String()
	})
	return libraryPage(out, filter.Limit, filter.Offset), nil
}

func (r *MemoryLibraryRepository) UpdateReport(ctx context.Context, id uuid.UUID, patch library.ReportPatch) (library.Report, error) {
	if err := ctx.Err(); err != nil {
		return library.Report{}, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	rep, ok := r.reports[id]
	if !ok {
		return library.Report{}, library.ErrNotFound
	}
	rep = applyReportPatch(rep, patch)
	rep.UpdatedAt = libraryNow()
	r.reports[id] = rep
	return r.reportView(rep), nil
}

func (r *MemoryLibraryRepository) DeleteReport(ctx context.Context, id uuid.UUID) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.reports[id]; !ok {
		return library.ErrNotFound
	}
	delete(r.reports, id)
	return nil
}
