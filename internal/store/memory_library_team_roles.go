package store

import (
	"context"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/library"
)

type teamRolesKey struct {
	teamID  uuid.UUID
	mapName string
}

func (r *MemoryLibraryRepository) GetTeamRoles(ctx context.Context, teamID uuid.UUID, mapName string) (library.TeamRoleSheet, error) {
	if err := ctx.Err(); err != nil {
		return library.TeamRoleSheet{}, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.teams[teamID]; !ok {
		return library.TeamRoleSheet{}, library.ErrNotFound
	}
	sheet, ok := r.libreta.teamRoles[teamRolesKey{teamID, mapName}]
	if !ok {
		sheet = library.TeamRoleSheet{TeamID: teamID, Map: mapName}
	}
	return teamRolesCopy(sheet), nil
}

func (r *MemoryLibraryRepository) SetTeamRoles(ctx context.Context, sheet *library.TeamRoleSheet) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.teams[sheet.TeamID]; !ok {
		return library.ErrNotFound
	}
	if r.libreta.teamRoles == nil {
		r.libreta.teamRoles = map[teamRolesKey]library.TeamRoleSheet{}
	}
	sheet.UpdatedAt = libraryNow()
	stored := teamRolesCopy(*sheet)
	r.libreta.teamRoles[teamRolesKey{sheet.TeamID, sheet.Map}] = stored
	*sheet = teamRolesCopy(stored)
	return nil
}

// moveTeamRoles hands from's sheets to into, which keeps its own for a map both have.
func (l *memoryLibreta) moveTeamRoles(from, into uuid.UUID) {
	for key, sheet := range l.teamRoles {
		if key.teamID != from {
			continue
		}
		delete(l.teamRoles, key)
		next := teamRolesKey{into, key.mapName}
		if _, taken := l.teamRoles[next]; !taken {
			sheet.TeamID = into
			l.teamRoles[next] = sheet
		}
	}
}

// teamRolesCopy gives a sheet its own slices, never nil, the shape both repositories return.
func teamRolesCopy(s library.TeamRoleSheet) library.TeamRoleSheet {
	s.T = append([]library.RoleSlot{}, s.T...)
	s.CT = append([]library.RoleSlot{}, s.CT...)
	return s
}
