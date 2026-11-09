package httpapi

import (
	"errors"
	"net/http"
	"net/url"
	"slices"

	"github.com/rechedev9/cliphub/internal/tacticalplan"
)

// teamGrenadesResponse is every grenade a team threw in its selected matches,
// with the matches it could not read.
type teamGrenadesResponse struct {
	tacticalplan.TeamGrenades
	Skipped []scoutSkipped `json:"skipped"`
}

// grenadeKindsQuery reads the repeatable kind parameter.
func grenadeKindsQuery(q url.Values) ([]tacticalplan.EventKind, error) {
	kinds := make([]tacticalplan.EventKind, 0, len(q["kind"]))
	for _, raw := range q["kind"] {
		kind := tacticalplan.EventKind(raw)
		if !slices.Contains(tacticalplan.GrenadeKinds, kind) {
			return nil, errors.New("kind must be smoke, flash, he, molotov or decoy")
		}
		kinds = append(kinds, kind)
	}
	return kinds, nil
}

// ListLibraryTeamGrenades handles GET /api/library/teams/{id}/grenades: every
// grenade the team threw, where from and where it landed. It takes the scout
// filter (map, side, from, to, opponent_id, match_id, lineup) and a repeatable kind.
func (h *Handlers) ListLibraryTeamGrenades(w http.ResponseWriter, r *http.Request) {
	if !h.libraryReady(w) {
		return
	}
	id, ok := libraryUUIDParam(w, r, "id")
	if !ok {
		return
	}
	q := r.URL.Query()
	filter, matchIDs, err := scoutQuery(q)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	kinds, err := grenadeKindsQuery(q)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	team, err := h.libraryRepo.GetTeam(r.Context(), id)
	if err != nil {
		writeLibraryError(w, "get team", err)
		return
	}
	loaded, skipped, err := h.loadTeamMatches(r.Context(), team, matchIDs, filter)
	if err != nil {
		internalError(w, "list team grenades", err)
		return
	}
	writeJSON(w, http.StatusOK, teamGrenadesResponse{
		TeamGrenades: tacticalplan.CollectTeamGrenades(loaded, filter, kinds), Skipped: skipped,
	})
}
