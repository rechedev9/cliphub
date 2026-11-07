package httpapi

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strconv"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/library"
)

// libretaRepository is the Libreta part of LibraryRepository.
type libretaRepository interface {
	CreateLineup(ctx context.Context, l *library.Lineup) error
	GetLineup(ctx context.Context, id uuid.UUID) (library.Lineup, error)
	ListLineups(ctx context.Context, filter library.LineupFilter) ([]library.Lineup, error)
	UpdateLineup(ctx context.Context, id uuid.UUID, patch library.LineupPatch) (library.Lineup, error)
	DeleteLineup(ctx context.Context, id uuid.UUID) error
	GetTeamRoles(ctx context.Context, teamID uuid.UUID, mapName string) (library.TeamRoleSheet, error)
	SetTeamRoles(ctx context.Context, sheet *library.TeamRoleSheet) error
	ListTeamRoles(ctx context.Context) ([]library.TeamRoleSheet, error)
	CreateCall(ctx context.Context, c *library.Call) error
	GetCall(ctx context.Context, id uuid.UUID) (library.Call, error)
	ListCalls(ctx context.Context, filter library.CallFilter) ([]library.Call, error)
	UpdateCall(ctx context.Context, id uuid.UUID, patch library.CallPatch) (library.Call, error)
	DeleteCall(ctx context.Context, id uuid.UUID) error
	ListCalloutNames(ctx context.Context, mapName string) ([]library.CalloutName, error)
	SetCalloutName(ctx context.Context, c *library.CalloutName) error
	DeleteCalloutName(ctx context.Context, mapName, token string) error
}

// libretaListFilter is what every Libreta list reads besides its own fields.
type libretaListFilter struct {
	status, tag     string
	excludeArchived bool
	limit, offset   int
}

// libretaListQuery reads status, tag, include_archived, limit and offset. Archived
// records are left out unless asked for or filtered by status.
func libretaListQuery(q url.Values, maxList int) (libretaListFilter, error) {
	var f libretaListFilter
	if f.status = q.Get("status"); f.status != "" && !library.ValidStatus(f.status) {
		return f, errors.New("status must be active, practice, draft, review or archived")
	}
	if f.tag = library.CleanName(q.Get("tag")); len([]rune(f.tag)) > library.MaxTagRunes {
		return f, fmt.Errorf("tag must be at most %d characters", library.MaxTagRunes)
	}
	switch q.Get("include_archived") {
	case "", "false":
		f.excludeArchived = f.status == ""
	case "true":
	default:
		return f, errors.New("include_archived must be true or false")
	}
	for _, p := range []struct {
		name string
		dst  *int
		min  int
	}{{"limit", &f.limit, 1}, {"offset", &f.offset, 0}} {
		if raw := q.Get(p.name); raw != "" {
			n, err := strconv.Atoi(raw)
			if err != nil || n < p.min || (p.name == "limit" && n > maxList) {
				return f, fmt.Errorf("%s is out of range", p.name)
			}
			*p.dst = n
		}
	}
	return f, nil
}
