package store

import (
	"context"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/library"
)

// MergeTeams moves every alias, match, report, play, series slot and role sheet
// of from onto into and deletes from.
func (r *MemoryLibraryRepository) MergeTeams(ctx context.Context, from, into uuid.UUID) (library.Team, error) {
	if err := ctx.Err(); err != nil {
		return library.Team{}, err
	}
	if from == into {
		return library.Team{}, library.ErrConflict
	}
	r.mu.Lock()
	_, okFrom := r.teams[from]
	target, okInto := r.teams[into]
	if !okFrom || !okInto {
		r.mu.Unlock()
		return library.Team{}, library.ErrNotFound
	}
	for matchID, sides := range r.matchTeams {
		hasInto := false
		for _, side := range sides {
			hasInto = hasInto || (side.teamID != nil && *side.teamID == into)
		}
		for i, side := range sides {
			if side.teamID == nil || *side.teamID != from {
				continue
			}
			if hasInto {
				sides[i].teamID, sides[i].assignment = nil, library.AssignmentNone
			} else {
				sides[i].teamID = uuidPtr(into)
			}
		}
		r.matchTeams[matchID] = sides
	}
	for key, a := range r.aliases {
		if a.teamID == from {
			a.teamID = into
			r.aliases[key] = a
		}
	}
	for id, report := range r.reports {
		if report.TeamID == from {
			report.TeamID = into
			r.reports[id] = report
		}
	}
	for id, play := range r.plays {
		if play.TeamID != nil && *play.TeamID == from {
			play.TeamID = uuidPtr(into)
			r.plays[id] = play
		}
	}
	r.moveSeriesTeam(from, into)
	r.libreta.moveTeamRoles(from, into)
	r.libreta.moveTeamProfile(from, into)
	delete(r.teams, from)
	target.updatedAt = libraryNow()
	r.teams[into] = target
	r.mu.Unlock()
	return r.GetTeam(ctx, into)
}
