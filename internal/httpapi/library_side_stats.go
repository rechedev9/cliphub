package httpapi

import (
	"context"
	"errors"
	"log"
	"net/http"
	"slices"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/library"
	"github.com/rechedev9/cliphub/internal/tacticalplan"
)

const (
	// maxCompareTeams caps the teams a person can pick for "Comparar con".
	maxCompareTeams = 20
	// groupTeamMatches caps the matches read per group team, in both modes.
	groupTeamMatches = 20
)

// sideStatsTeam is a comparison team, by id and name.
type sideStatsTeam struct {
	TeamID uuid.UUID `json:"team_id"`
	Name   string    `json:"name"`
}

// sideStatsResponse is a team's figures per side with the group they are compared to.
// Compare is "library" (every other team in the library) or "teams" (CompareTeams).
type sideStatsResponse struct {
	TeamID         uuid.UUID       `json:"team_id"`
	TeamName       string          `json:"team_name"`
	Skipped        []scoutSkipped  `json:"skipped"`
	Compare        string          `json:"compare"`
	CompareTeams   []sideStatsTeam `json:"compare_teams"`
	CompareSkipped []uuid.UUID     `json:"compare_skipped"`
	GroupCapped    bool            `json:"group_capped"`
	// AWPEstimated counts the team's matches whose AWP rounds come from kills, not the AWP flag.
	AWPEstimated int `json:"awp_estimated"`
	tacticalplan.SideStatsReport
}

// GetLibraryTeamSideStats handles GET /api/library/teams/{id}/side-stats: the team's
// figures over the scope, each against the group mean over the same map, side, dates and buys.
func (h *Handlers) GetLibraryTeamSideStats(w http.ResponseWriter, r *http.Request) {
	team, ok := h.loadLibraryTeam(w, r)
	if !ok {
		return
	}
	q := r.URL.Query()
	filter, matchIDs, err := scoutQuery(q)
	var asked []uuid.UUID
	if err == nil {
		asked, err = uuidList(q["compare_team"], "compare_team", maxCompareTeams)
	}
	groupNone := q.Get("group") == "none"
	if err == nil && q.Get("group") != "" && !groupNone {
		err = errors.New("group must be none or empty")
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	resp := sideStatsResponse{TeamID: team.ID, TeamName: team.Name, Compare: "library", CompareTeams: []sideStatsTeam{}, CompareSkipped: []uuid.UUID{}}
	var compareIDs []uuid.UUID
	for _, id := range asked {
		if id == team.ID || slices.Contains(compareIDs, id) || slices.Contains(resp.CompareSkipped, id) {
			continue
		}
		other, err := h.libraryRepo.GetTeam(r.Context(), id)
		if errors.Is(err, library.ErrNotFound) {
			resp.CompareSkipped = append(resp.CompareSkipped, id)
			continue
		}
		if err != nil {
			writeLibraryError(w, "get compare team", err)
			return
		}
		compareIDs = append(compareIDs, id)
		resp.Compare = "teams"
		resp.CompareTeams = append(resp.CompareTeams, sideStatsTeam{TeamID: other.ID, Name: other.Name})
	}
	candidates, pending, skipped, err := h.teamCandidates(r.Context(), team, matchIDs)
	if err != nil {
		internalError(w, "team side stats", err)
		return
	}
	selected, unrated := selectTeamMatches(filter, pending)
	kept, more := h.keepTeamMatches(keepQuery{teamID: team.ID, candidates: candidates, selected: selected, lineup: filter.Lineup, limit: filter.MaxMatches})
	resp.Skipped = append(append(skipped, unrated...), more...)
	own := make([]tacticalplan.SideStatsMatch, 0, len(kept))
	ownDocs := map[uuid.UUID]tacticalplan.Document{}
	for _, k := range kept {
		resp.AWPEstimated += awpEstimated(filter, k.scout)
		ownDocs[k.scout.MatchID] = k.scout.Document
		own = append(own, tacticalplan.SideStatsMatch{
			MatchID: k.scout.MatchID, PlayedAt: k.scout.PlayedAt, Document: k.scout.Document, TeamKey: k.scout.TeamKey, Opponent: k.scout.Opponent,
		})
	}
	opts := tacticalplan.SideStatsOptions{
		Side: filter.Side, Rounds: filter.Rounds, Lineup: filter.Lineup, Roles: sideStatsRoles(own),
	}
	if len(filter.Maps) == 1 {
		opts.MapRoles = h.teamMapRoles(kept)
	}
	group := tacticalplan.NewSideStatsGroup(opts)
	// group=none is the players-only read: the comparison group is not folded.
	if !groupNone {
		resp.GroupCapped, err = h.foldSideStatsGroup(r.Context(), sideStatsGroupQuery{teamID: team.ID, filter: filter, compare: compareIDs, ownDocs: ownDocs, group: group})
		if err != nil {
			internalError(w, "team side stats group", err)
			return
		}
	}
	resp.SideStatsReport = tacticalplan.TeamSideStats(tacticalplan.SideStatsInput{Team: own, Options: opts, Group: group})
	writeJSON(w, http.StatusOK, resp)
}

// sideStatsRoles reads each player's role with the library's role rule.
func sideStatsRoles(matches []tacticalplan.SideStatsMatch) map[string]string {
	docs := make([]library.RoleDocument, 0, len(matches))
	var ids []string
	for _, m := range matches {
		docs = append(docs, library.RoleDocument{MatchID: m.MatchID, Document: m.Document})
		for _, p := range m.Document.Players {
			if p.TeamKey == m.TeamKey && p.SteamID64 != "" && !slices.Contains(ids, p.SteamID64) {
				ids = append(ids, p.SteamID64)
			}
		}
	}
	roles := make(map[string]string, len(ids))
	for _, id := range ids {
		roles[id] = library.InferPlayerRole(id, docs).Role
	}
	return roles
}

// sideStatsGroupQuery is the comparison group: the chosen teams, or every other
// team when none is chosen. ownDocs are the team's documents, already in memory.
type sideStatsGroupQuery struct {
	teamID  uuid.UUID
	filter  tacticalplan.ScoutFilter
	compare []uuid.UUID
	ownDocs map[uuid.UUID]tacticalplan.Document
	group   *tacticalplan.SideStatsGroup
}

// groupCandidates lists the group's matches, newest first, and whether a list hit its cap.
func (h *Handlers) groupCandidates(ctx context.Context, q sideStatsGroupQuery) ([]library.Match, bool, error) {
	base := library.MatchFilter{From: q.filter.From, To: q.filter.To, Limit: library.MaxListLimit}
	if len(q.filter.Maps) == 1 {
		base.Map = q.filter.Maps[0]
	}
	if len(q.compare) == 0 {
		listed, err := h.libraryRepo.ListMatches(ctx, base)
		return listed, len(listed) >= library.MaxListLimit, err
	}
	var matches []library.Match
	seen, capped := map[uuid.UUID]bool{}, false
	for _, id := range q.compare {
		f := base
		// One more than the cap tells a full list from a team with exactly the cap.
		f.TeamID, f.Limit = &id, groupTeamMatches+1
		listed, err := h.libraryRepo.ListMatches(ctx, f)
		if err != nil {
			return nil, false, err
		}
		capped = capped || len(listed) > groupTeamMatches
		for _, m := range listed {
			if !seen[m.ID] {
				seen[m.ID] = true
				matches = append(matches, m)
			}
		}
	}
	return matches, capped, nil
}

// foldSideStatsGroup reads each group match once, folds it into every group team
// that played it, under groupTeamMatches per team, and drops the document.
func (h *Handlers) foldSideStatsGroup(ctx context.Context, q sideStatsGroupQuery) (bool, error) {
	matches, capped, err := h.groupCandidates(ctx, q)
	if err != nil {
		return false, err
	}
	for _, m := range matches {
		if err := ctx.Err(); err != nil {
			return false, err
		}
		if len(q.filter.Maps) > 0 && !slices.Contains(q.filter.Maps, m.Map) {
			continue
		}
		var sides []library.MatchTeam
		for _, t := range m.Teams {
			if t.TeamID == nil || *t.TeamID == q.teamID || (len(q.compare) > 0 && !slices.Contains(q.compare, *t.TeamID)) {
				continue
			}
			if q.group.Added(t.TeamID.String()) >= groupTeamMatches {
				capped = true
				continue
			}
			sides = append(sides, t)
		}
		if len(sides) == 0 {
			continue
		}
		doc, ok := q.ownDocs[m.ID]
		if !ok {
			if doc, err = h.readLibraryDocument(m.DemoID); err != nil {
				log.Printf("httpapi: side stats group skips match %s: %v", m.ID, err)
				continue
			}
		}
		for _, t := range sides {
			q.group.Add(t.TeamID.String(), tacticalplan.SideStatsMatch{MatchID: m.ID, PlayedAt: m.PlayedAt, Document: doc, TeamKey: t.Key})
		}
	}
	return capped, nil
}
