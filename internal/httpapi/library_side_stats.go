package httpapi

import (
	"context"
	"log"
	"net/http"
	"slices"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/library"
	"github.com/rechedev9/cliphub/internal/tacticalplan"
)

// maxCompareTeams caps the teams a person can pick for "Comparar con".
const maxCompareTeams = 20

// sideStatsTeam is a comparison team, by id and name.
type sideStatsTeam struct {
	TeamID uuid.UUID `json:"team_id"`
	Name   string    `json:"name"`
}

// sideStatsResponse is a team's figures per side with the group they are compared to.
// Compare is "library" (every other team in the library) or "teams" (CompareTeams).
type sideStatsResponse struct {
	TeamID       uuid.UUID       `json:"team_id"`
	TeamName     string          `json:"team_name"`
	Skipped      []scoutSkipped  `json:"skipped"`
	Compare      string          `json:"compare"`
	CompareTeams []sideStatsTeam `json:"compare_teams"`
	tacticalplan.SideStatsReport
}

// GetLibraryTeamSideStats handles GET /api/library/teams/{id}/side-stats: the
// team's figures over the scope (map, side, dates, rival, lineup and both buys),
// each against the mean of the comparison teams over the same map, side, dates and buys.
func (h *Handlers) GetLibraryTeamSideStats(w http.ResponseWriter, r *http.Request) {
	team, ok := h.loadLibraryTeam(w, r)
	if !ok {
		return
	}
	q := r.URL.Query()
	filter, matchIDs, err := scoutQuery(q)
	var rounds tacticalplan.Filter
	if err == nil {
		rounds, err = tacticalplan.FilterFromValues(q)
	}
	var compareIDs []uuid.UUID
	if err == nil {
		compareIDs, err = uuidList(q["compare_team"], "compare_team", maxCompareTeams)
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	resp := sideStatsResponse{TeamID: team.ID, TeamName: team.Name, Compare: "library", CompareTeams: []sideStatsTeam{}}
	for _, id := range compareIDs {
		other, err := h.libraryRepo.GetTeam(r.Context(), id)
		if err != nil {
			writeLibraryError(w, "get compare team", err)
			return
		}
		resp.Compare = "teams"
		resp.CompareTeams = append(resp.CompareTeams, sideStatsTeam{TeamID: other.ID, Name: other.Name})
	}
	candidates, pending, skipped, err := h.teamCandidates(r.Context(), team, matchIDs)
	if err != nil {
		internalError(w, "team side stats", err)
		return
	}
	kept, more := h.keepTeamMatches(keepQuery{teamID: team.ID, candidates: candidates, selected: filter.Select(pending), lineup: filter.Lineup})
	resp.Skipped = append(skipped, more...)
	own := make([]tacticalplan.SideStatsMatch, 0, len(kept))
	docs := map[uuid.UUID]tacticalplan.Document{}
	for _, k := range kept {
		docs[k.scout.MatchID] = k.scout.Document
		own = append(own, tacticalplan.SideStatsMatch{
			MatchID: k.scout.MatchID, PlayedAt: k.scout.PlayedAt, Document: k.scout.Document, TeamKey: k.scout.TeamKey, Opponent: k.scout.Opponent,
		})
	}
	group, err := h.sideStatsGroup(r.Context(), sideStatsGroupQuery{teamID: team.ID, filter: filter, compare: compareIDs, docs: docs})
	if err != nil {
		internalError(w, "team side stats group", err)
		return
	}
	resp.SideStatsReport = tacticalplan.TeamSideStats(own, group, tacticalplan.SideStatsOptions{
		Side: filter.Side, Buys: rounds.Buys, OpponentBuys: rounds.OpponentBuys, Lineup: filter.Lineup, Roles: sideStatsRoles(own),
	})
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

// sideStatsGroupQuery is the comparison group: the chosen teams, or every
// other team when none is chosen. docs holds the documents already read.
type sideStatsGroupQuery struct {
	teamID  uuid.UUID
	filter  tacticalplan.ScoutFilter
	compare []uuid.UUID
	docs    map[uuid.UUID]tacticalplan.Document
}

// sideStatsGroup reads the group's matches on the scope's maps and dates, per
// team. The rival and the lineup belong to the team, so they never narrow it.
func (h *Handlers) sideStatsGroup(ctx context.Context, q sideStatsGroupQuery) (map[string][]tacticalplan.SideStatsMatch, error) {
	base := library.MatchFilter{From: q.filter.From, To: q.filter.To, Limit: library.MaxListLimit}
	if len(q.filter.Maps) == 1 {
		base.Map = q.filter.Maps[0]
	}
	var matches []library.Match
	if len(q.compare) == 0 {
		listed, err := h.libraryRepo.ListMatches(ctx, base)
		if err != nil {
			return nil, err
		}
		matches = listed
	}
	seen := map[uuid.UUID]bool{}
	for _, id := range q.compare {
		f := base
		f.TeamID, f.Limit = &id, library.MaxReportMatches
		listed, err := h.libraryRepo.ListMatches(ctx, f)
		if err != nil {
			return nil, err
		}
		for _, m := range listed {
			if !seen[m.ID] {
				seen[m.ID] = true
				matches = append(matches, m)
			}
		}
	}
	group := map[string][]tacticalplan.SideStatsMatch{}
	for _, m := range matches {
		if len(q.filter.Maps) > 0 && !slices.Contains(q.filter.Maps, m.Map) {
			continue
		}
		for _, t := range m.Teams {
			if t.TeamID == nil || *t.TeamID == q.teamID || (len(q.compare) > 0 && !slices.Contains(q.compare, *t.TeamID)) {
				continue
			}
			doc, ok := h.groupDocument(q.docs, m)
			if !ok {
				break
			}
			key := t.TeamID.String()
			group[key] = append(group[key], tacticalplan.SideStatsMatch{MatchID: m.ID, PlayedAt: m.PlayedAt, Document: doc, TeamKey: t.Key})
		}
	}
	return group, nil
}

// groupDocument reads a match's document once per request; an unreadable one is left out.
func (h *Handlers) groupDocument(docs map[uuid.UUID]tacticalplan.Document, m library.Match) (tacticalplan.Document, bool) {
	if doc, ok := docs[m.ID]; ok {
		return doc, true
	}
	doc, err := h.readLibraryDocument(m.DemoID)
	if err != nil {
		log.Printf("httpapi: side stats group skips match %s: %v", m.ID, err)
		return tacticalplan.Document{}, false
	}
	docs[m.ID] = doc
	return doc, true
}
