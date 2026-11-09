package store

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/library"
)

// ProMatchRepository stores the pro maps imported from the community catalog
// and the HLTV teams Studio follows.
type ProMatchRepository interface {
	// SaveProMap records an imported pro map, replacing an earlier record of it.
	SaveProMap(ctx context.Context, m *library.ProMap) error
	// GetProMap returns one record; an unknown map is ErrNotFound.
	GetProMap(ctx context.Context, matchID int, entry string) (library.ProMap, error)
	// ProMapForDemo returns the record of a library demo, or ErrNotFound.
	ProMapForDemo(ctx context.Context, demoID uuid.UUID) (library.ProMap, error)
	// ListProMaps returns the records of one HLTV match, or of every match for 0, newest first.
	ListProMaps(ctx context.Context, matchID int) ([]library.ProMap, error)
	// FollowProTeam starts following a team, or renames one already followed.
	FollowProTeam(ctx context.Context, f *library.ProFollow) error
	// UnfollowProTeam stops following; a team not followed is ErrNotFound.
	UnfollowProTeam(ctx context.Context, teamID int) error
	// ListProFollows returns the followed teams by name.
	ListProFollows(ctx context.Context) ([]library.ProFollow, error)
	// MarkProFollowSynced records when the follow sync last imported a team's matches.
	MarkProFollowSynced(ctx context.Context, teamID int, at time.Time) error
}

func proMapCopy(m library.ProMap) library.ProMap {
	m.Teams = append([]library.ProTeam{}, m.Teams...)
	if m.DemoID != nil {
		id := *m.DemoID
		m.DemoID = &id
	}
	return m
}
