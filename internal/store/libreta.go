package store

import (
	"context"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/library"
)

// LibretaRepository stores what the Libreta adds to the library's plays:
// lineups, team role sheets and, later, the team's calls and callout names.
type LibretaRepository interface {
	// CreateLineup stores a lineup; an id that is already stored is ErrConflict.
	CreateLineup(ctx context.Context, l *library.Lineup) error
	GetLineup(ctx context.Context, id uuid.UUID) (library.Lineup, error)
	// ListLineups returns lineups by most recent edit.
	ListLineups(ctx context.Context, filter library.LineupFilter) ([]library.Lineup, error)
	UpdateLineup(ctx context.Context, id uuid.UUID, patch library.LineupPatch) (library.Lineup, error)
	// DeleteLineup removes a lineup; a missing id is ErrNotFound.
	DeleteLineup(ctx context.Context, id uuid.UUID) error
	// GetTeamRoles returns a team's role sheet on a map, empty when none is saved;
	// a missing team is ErrNotFound.
	GetTeamRoles(ctx context.Context, teamID uuid.UUID, mapName string) (library.TeamRoleSheet, error)
	// SetTeamRoles replaces a team's sheet on its map.
	SetTeamRoles(ctx context.Context, sheet *library.TeamRoleSheet) error
}

func clampLineupLimit(limit int) int {
	if limit <= 0 || limit > library.MaxLineupList {
		return library.MaxLineupList
	}
	return limit
}

// lineupCopy gives a lineup its own lists and pointers, the shape both repositories return.
func lineupCopy(l library.Lineup) library.Lineup {
	l.Tags = append([]string{}, l.Tags...)
	if l.Landing != nil {
		landing := *l.Landing
		l.Landing = &landing
	}
	if l.Source != nil {
		source := *l.Source
		l.Source = &source
	}
	if l.Status == "" {
		l.Status = library.StatusActive
	}
	return l
}
