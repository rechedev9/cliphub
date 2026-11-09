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
	Skipped      []scoutSkipped `json:"skipped"`
	AWPEstimated int            `json:"awp_estimated,omitempty"`
}

// grenadeQuery reads the repeatable kind parameter and rivals=1.
func grenadeQuery(q url.Values) (tacticalplan.GrenadeQuery, error) {
	out := tacticalplan.GrenadeQuery{Kinds: make([]tacticalplan.EventKind, 0, len(q["kind"]))}
	for _, raw := range q["kind"] {
		kind := tacticalplan.EventKind(raw)
		if !slices.Contains(tacticalplan.GrenadeKinds, kind) {
			return out, errors.New("kind must be smoke, flash, he, molotov or decoy")
		}
		out.Kinds = append(out.Kinds, kind)
	}
	switch q.Get("rivals") {
	case "", "0":
	case "1":
		out.Rivals = true
	default:
		return out, errors.New("rivals must be 0 or 1")
	}
	return out, nil
}

// ListLibraryTeamGrenades handles GET /api/library/teams/{id}/grenades: every
// grenade the team threw, where from, where it landed and what the team did
// after it, with every round the filter kept. It takes the scout filter (map,
// side, from, to, opponent_id, match_id, lineup), a repeatable kind and
// rivals=1 for the grenades of the team's rivals in the same rounds.
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
	read, err := grenadeQuery(q)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	team, err := h.libraryRepo.GetTeam(r.Context(), id)
	if err != nil {
		writeLibraryError(w, "get team", err)
		return
	}
	loaded, skipped, err := h.loadFramedMatches(r.Context(), framedQuery{team: team, matchIDs: matchIDs, filter: filter})
	if err != nil {
		internalError(w, "list team grenades", err)
		return
	}
	resp := teamGrenadesResponse{TeamGrenades: tacticalplan.CollectTeamGrenades(loaded, filter, read), Skipped: skipped}
	for _, m := range loaded {
		resp.AWPEstimated += awpEstimated(filter, m.ScoutMatch)
	}
	writeJSON(w, http.StatusOK, resp)
}
