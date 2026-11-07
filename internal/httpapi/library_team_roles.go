package httpapi

import (
	"net/http"
	"sort"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/library"
	"github.com/rechedev9/cliphub/internal/tacticalplan"
)

// Suggested CT roles; T roles are InferPlayerRole's.
const (
	roleAnchor  = "anchor"
	roleRotator = "rotator"
)

// roleSuggestion is a role read from the team's matches on the map.
type roleSuggestion struct {
	SteamID64 string `json:"steamid64"`
	Player    string `json:"player"`
	Role      string `json:"role"`
	Callout   string `json:"callout,omitempty"`
	Rounds    int    `json:"rounds"`
}

type teamRolesResponse struct {
	TeamID    uuid.UUID             `json:"team_id"`
	TeamName  string                `json:"team_name"`
	Sheet     library.TeamRoleSheet `json:"sheet"`
	Suggested struct {
		T  []roleSuggestion `json:"t"`
		CT []roleSuggestion `json:"ct"`
	} `json:"suggested"`
	Matches int            `json:"matches"`
	Skipped []scoutSkipped `json:"skipped"`
}

// GetLibraryTeamRoles handles GET /api/library/teams/{id}/roles with the scout
// filter minus side: the saved sheet of the map and a suggestion for each side.
func (h *Handlers) GetLibraryTeamRoles(w http.ResponseWriter, r *http.Request) {
	q, ok := h.framedRequest(w, r)
	if !ok {
		return
	}
	if q.filter.Side != "" {
		writeError(w, http.StatusBadRequest, "the role sheet reads both sides; drop side")
		return
	}
	sheet, err := h.libraryRepo.GetTeamRoles(r.Context(), q.team.ID, q.filter.Maps[0])
	if err != nil {
		writeLibraryError(w, "get team roles", err)
		return
	}
	loaded, skipped, err := h.loadFramedMatches(r.Context(), q)
	if err != nil {
		internalError(w, "team roles", err)
		return
	}
	metrics := make([]tacticalplan.MetricsMatch, 0, len(loaded))
	for _, m := range loaded {
		metrics = append(metrics, tacticalplan.MetricsMatch{MatchID: m.MatchID, Document: m.Document, TeamKey: m.TeamKey})
	}
	docs, names := roleDocuments(metrics), playerNames(metrics)
	inferred := map[string]string{}
	resp := teamRolesResponse{TeamID: q.team.ID, TeamName: q.team.Name, Sheet: sheet, Matches: len(loaded), Skipped: skipped}
	resp.Suggested.T = []roleSuggestion{}
	for _, id := range teamSteamIDs(metrics) {
		role := library.InferPlayerRole(id, docs)
		inferred[id] = role.Role
		resp.Suggested.T = append(resp.Suggested.T, roleSuggestion{SteamID64: id, Player: names[id], Role: role.Role, Rounds: role.Rounds})
	}
	sort.SliceStable(resp.Suggested.T, func(i, j int) bool { return resp.Suggested.T[i].Rounds > resp.Suggested.T[j].Rounds })
	resp.Suggested.T = resp.Suggested.T[:min(len(resp.Suggested.T), library.MaxRoleSlots)]
	prep := tacticalplan.BuildPrep(loaded, tacticalplan.PrepOptions{Filter: q.filter, Roles: inferred})
	resp.Suggested.CT = ctRoleSuggestions(prep.CTPlayers)
	writeJSON(w, http.StatusOK, resp)
}

// ctRoleSuggestions names each CT player by where they set up: the AWP stays
// AWP, a player on one spot most rounds anchors it, anyone else rotates.
func ctRoleSuggestions(players []tacticalplan.PrepPlayer) []roleSuggestion {
	out := []roleSuggestion{}
	for _, p := range players[:min(len(players), library.MaxRoleSlots)] {
		s := roleSuggestion{SteamID64: p.SteamID64, Player: p.Name, Role: roleRotator, Rounds: p.Rounds}
		if top, ok := setupSpot(p); ok {
			s.Callout = top.Callout
			if 2*top.Share.Count >= top.Share.Total {
				s.Role = roleAnchor
			}
		}
		if p.Role == library.RoleAWP {
			s.Role = library.RoleAWP
		}
		out = append(out, s)
	}
	return out
}

// setupSpot is the player's most held spot once the round settles (20 to 40 s),
// or in the first window when that one has none.
func setupSpot(p tacticalplan.PrepPlayer) (tacticalplan.PrepSpot, bool) {
	for _, i := range []int{1, 0} {
		if i < len(p.Windows) && len(p.Windows[i].Spots) > 0 {
			return p.Windows[i].Spots[0], true
		}
	}
	return tacticalplan.PrepSpot{}, false
}

func playerNames(matches []tacticalplan.MetricsMatch) map[string]string {
	names := map[string]string{}
	for _, m := range matches {
		for _, p := range m.Document.Players {
			if p.SteamID64 != "" && names[p.SteamID64] == "" {
				names[p.SteamID64] = p.Name
			}
		}
	}
	return names
}

// PutLibraryTeamRoles handles PUT /api/library/teams/{id}/roles: the whole sheet of one map.
func (h *Handlers) PutLibraryTeamRoles(w http.ResponseWriter, r *http.Request) {
	if !h.libraryReady(w) {
		return
	}
	id, ok := libraryUUIDParam(w, r, "id")
	if !ok {
		return
	}
	var req struct {
		Map string             `json:"map"`
		T   []library.RoleSlot `json:"t"`
		CT  []library.RoleSlot `json:"ct"`
	}
	if err := decodeSingleJSONBody(w, r, &req, true); err != nil {
		writeError(w, http.StatusBadRequest, "invalid role sheet JSON")
		return
	}
	sheet, err := library.CleanTeamRoleSheet(library.TeamRoleSheet{TeamID: id, Map: req.Map, T: req.T, CT: req.CT})
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := h.libraryRepo.SetTeamRoles(r.Context(), &sheet); err != nil {
		writeLibraryError(w, "set team roles", err)
		return
	}
	writeJSON(w, http.StatusOK, sheet)
}
