package httpapi

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/library"
	"github.com/rechedev9/cliphub/internal/tacticalplan"
)

// teamProfiles is every saved team profile by team id.
type teamProfiles map[uuid.UUID]library.TeamProfile

func (h *Handlers) teamProfiles(ctx context.Context) (teamProfiles, error) {
	list, err := h.libraryRepo.ListTeamProfiles(ctx)
	if err != nil {
		return nil, fmt.Errorf("list team profiles: %w", err)
	}
	out := make(teamProfiles, len(list))
	for _, p := range list {
		out[p.TeamID] = p
	}
	return out, nil
}

// withProfiles fills each team's own flag and HLTV rank from its profile.
func (h *Handlers) withProfiles(ctx context.Context, teams []library.Team) error {
	profiles, err := h.teamProfiles(ctx)
	if err != nil {
		return err
	}
	for i := range teams {
		p := profiles[teams[i].ID]
		teams[i].Own, teams[i].HLTVRank = p.Own, p.HLTVRank
	}
	return nil
}

// writeTeam answers one team with its profile fields.
func (h *Handlers) writeTeam(w http.ResponseWriter, r *http.Request, team library.Team) {
	one := []library.Team{team}
	if err := h.withProfiles(r.Context(), one); err != nil {
		internalError(w, "read team profile", err)
		return
	}
	writeJSON(w, http.StatusOK, one[0])
}

// selectTeamMatches applies the scout filter; the matches the opponent level
// could not rate come back as skipped, so the view can say how many.
func selectTeamMatches(f tacticalplan.ScoutFilter, pending []tacticalplan.ScoutMatch) ([]tacticalplan.ScoutMatch, []scoutSkipped) {
	kept, unrated := f.SelectRated(pending)
	skipped := make([]scoutSkipped, 0, len(unrated))
	for _, m := range unrated {
		skipped = append(skipped, scoutSkipped{MatchID: m.MatchID, Reason: scoutSkipUnranked})
	}
	return kept, skipped
}

// skippedOnMap keeps the skips of matches played on mapName, so a one-map view
// counts only what it could have drawn.
func skippedOnMap(skipped []scoutSkipped, pending []tacticalplan.ScoutMatch, mapName string) []scoutSkipped {
	maps := make(map[uuid.UUID]string, len(pending))
	for _, m := range pending {
		maps[m.MatchID] = m.Document.Demo.Map
	}
	out := make([]scoutSkipped, 0, len(skipped))
	for _, s := range skipped {
		if maps[s.MatchID] == mapName {
			out = append(out, s)
		}
	}
	return out
}

// awpEstimated is 1 when the scope filters by AWP and the match's analysis predates the AWP flag.
func awpEstimated(f tacticalplan.ScoutFilter, m tacticalplan.ScoutMatch) int {
	if f.Rounds.ReadsAWP() && tacticalplan.AWPEstimated(m.Document) {
		return 1
	}
	return 0
}

// teamScopeQuery reads what every team view adds to the scout filter: the
// match cap, the opponent level, whose matches, and the round criteria.
func teamScopeQuery(q url.Values, f *tacticalplan.ScoutFilter) error {
	var err error
	if f.MaxMatches, err = intQuery(intParam{q: q, name: "max_matches", limit: library.MaxReportMatches}); err != nil {
		return err
	}
	top, err := intQuery(intParam{q: q, name: "opponent_top", limit: library.MaxHLTVRank})
	if err != nil {
		return err
	}
	if f.OpponentTop, err = tacticalplan.ParseOpponentTop(top); err != nil {
		return err
	}
	if f.Whose, err = tacticalplan.ParseWhose(strings.TrimSpace(q.Get("whose"))); err != nil {
		return err
	}
	rounds := url.Values{}
	for _, key := range teamRoundKeys {
		if list, ok := q[key]; ok {
			rounds[key] = list
		}
	}
	f.Rounds, err = tacticalplan.FilterFromValues(rounds)
	return err
}

// teamRoundKeys are the round filter's names a team view reads; team, side,
// slots and lineup are the scout's own.
var teamRoundKeys = []string{
	"buy", "opponent_buy", "site", "outcome", "t_pattern", "ct_pattern", "tag", "round_from", "round_to", "phase",
	"awp", "opponent_awp", "advantage",
}

// intParam names an optional query integer from 1 to limit.
type intParam struct {
	q     url.Values
	name  string
	limit int
}

func intQuery(p intParam) (int, error) {
	raw := strings.TrimSpace(p.q.Get(p.name))
	if raw == "" {
		return 0, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 1 || n > p.limit {
		return 0, fmt.Errorf("%s must be an integer from 1 to %d", p.name, p.limit)
	}
	return n, nil
}

// teamProfileResponse is a team's profile and how many teams are marked own.
type teamProfileResponse struct {
	library.TeamProfile
	OwnTeams int `json:"own_teams"`
}

func (h *Handlers) teamProfileOf(ctx context.Context, id uuid.UUID) (teamProfileResponse, error) {
	profiles, err := h.teamProfiles(ctx)
	if err != nil {
		return teamProfileResponse{}, err
	}
	out := teamProfileResponse{TeamProfile: library.TeamProfile{TeamID: id}}
	if p, ok := profiles[id]; ok {
		out.TeamProfile = p
	}
	for _, p := range profiles {
		if p.Own {
			out.OwnTeams++
		}
	}
	return out, nil
}

// GetLibraryTeamProfile handles GET /api/library/teams/{id}/profile.
func (h *Handlers) GetLibraryTeamProfile(w http.ResponseWriter, r *http.Request) {
	if !h.libraryReady(w) {
		return
	}
	id, ok := libraryUUIDParam(w, r, "id")
	if !ok {
		return
	}
	if _, err := h.libraryRepo.GetTeam(r.Context(), id); err != nil {
		writeLibraryError(w, "get team", err)
		return
	}
	out, err := h.teamProfileOf(r.Context(), id)
	if err != nil {
		internalError(w, "read team profile", err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

// PutLibraryTeamProfile handles PUT /api/library/teams/{id}/profile: whether
// the team is one of the user's own, and its HLTV rank (0 when not ranked).
func (h *Handlers) PutLibraryTeamProfile(w http.ResponseWriter, r *http.Request) {
	if !h.libraryReady(w) {
		return
	}
	id, ok := libraryUUIDParam(w, r, "id")
	if !ok {
		return
	}
	var req struct {
		Own      *bool `json:"own"`
		HLTVRank *int  `json:"hltv_rank"`
	}
	if err := decodeSingleJSONBody(w, r, &req, true); err != nil {
		writeError(w, http.StatusBadRequest, "invalid team profile JSON")
		return
	}
	current, err := h.teamProfileOf(r.Context(), id)
	if err != nil {
		internalError(w, "read team profile", err)
		return
	}
	next := current.TeamProfile
	if req.Own != nil {
		next.Own = *req.Own
	}
	if req.HLTVRank != nil {
		next.HLTVRank = *req.HLTVRank
	}
	if err := library.CheckTeamProfile(next); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := h.libraryRepo.SetTeamProfile(r.Context(), &next); err != nil {
		writeLibraryError(w, "set team profile", err)
		return
	}
	out, err := h.teamProfileOf(r.Context(), id)
	if err != nil {
		internalError(w, "read team profile", err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}
