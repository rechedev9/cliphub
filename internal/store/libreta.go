package store

import (
	"context"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/library"
)

// LibretaRepository stores what the Libreta adds to the library's plays:
// lineups, team role sheets, the team's calls and its own callout names.
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
	// ListTeamRoles returns every saved sheet by team and map, for the Libreta file.
	ListTeamRoles(ctx context.Context) ([]library.TeamRoleSheet, error)
	// SetTeamProfile replaces whether a team is the user's own and its HLTV rank.
	SetTeamProfile(ctx context.Context, p *library.TeamProfile) error
	// ListTeamProfiles returns every saved profile; a team without one is not own and not ranked.
	ListTeamProfiles(ctx context.Context) ([]library.TeamProfile, error)
	// CreateCall stores a call; an id that is already stored is ErrConflict.
	CreateCall(ctx context.Context, c *library.Call) error
	GetCall(ctx context.Context, id uuid.UUID) (library.Call, error)
	// ListCalls returns calls by name.
	ListCalls(ctx context.Context, filter library.CallFilter) ([]library.Call, error)
	UpdateCall(ctx context.Context, id uuid.UUID, patch library.CallPatch) (library.Call, error)
	DeleteCall(ctx context.Context, id uuid.UUID) error
	// ListCalloutNames returns the team's names on a map, or on every map for "".
	ListCalloutNames(ctx context.Context, mapName string) ([]library.CalloutName, error)
	// SetCalloutName names a place of a map, replacing a previous name.
	SetCalloutName(ctx context.Context, c *library.CalloutName) error
	// DeleteCalloutName goes back to CS2's name; an unnamed place is ErrNotFound.
	DeleteCalloutName(ctx context.Context, mapName, token string) error
}

func clampCallLimit(limit int) int {
	if limit <= 0 || limit > library.MaxCallList {
		return library.MaxCallList
	}
	return limit
}

// callCopy gives a call its own tags and a status, the shape both repositories return.
func callCopy(c library.Call) library.Call {
	c.Tags = append([]string{}, c.Tags...)
	if c.Status == "" {
		c.Status = library.StatusActive
	}
	return c
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
