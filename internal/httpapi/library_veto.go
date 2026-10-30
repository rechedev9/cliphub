package httpapi

import (
	"context"
	"errors"
	"log"
	"net/http"
	"net/url"
	"time"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/library"
)

// libraryVetoStep is a veto step as the client sends it.
type libraryVetoStep struct {
	Team   string `json:"team"`
	Action string `json:"action"`
	Map    string `json:"map"`
}

// librarySeriesRequest is the body of POST and PATCH /api/library/series.
// Absent fields keep their value on a PATCH.
type librarySeriesRequest struct {
	Format  *string            `json:"format"`
	Event   *string            `json:"event"`
	Veto    *string            `json:"veto"`
	TeamIDs *[]string          `json:"team_ids"`
	Vetoes  *[]libraryVetoStep `json:"vetoes"`
}

func (req librarySeriesRequest) patch() (library.SeriesPatch, error) {
	p := library.SeriesPatch{Format: req.Format, Event: req.Event, Veto: req.Veto}
	if req.TeamIDs != nil {
		ids, err := uuidList(*req.TeamIDs, "team_ids", 2)
		if err != nil {
			return p, err
		}
		p.TeamIDs = &ids
	}
	if req.Vetoes != nil {
		if len(*req.Vetoes) > library.MaxVetoSteps {
			return p, errors.New("a veto has too many steps")
		}
		steps := make([]library.VetoStep, len(*req.Vetoes))
		for i, s := range *req.Vetoes {
			steps[i] = library.VetoStep{Team: s.Team, Action: s.Action, Map: s.Map}
		}
		p.Vetoes = &steps
	}
	return p, nil
}

func (h *Handlers) decodeSeriesRequest(w http.ResponseWriter, r *http.Request, base library.Series) (library.Series, bool) {
	var req librarySeriesRequest
	if err := decodeSingleJSONBody(w, r, &req, true); err != nil {
		writeError(w, http.StatusBadRequest, "invalid series JSON")
		return base, false
	}
	patch, err := req.patch()
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return base, false
	}
	s, err := library.CleanSeries(library.ApplySeriesPatch(base, patch))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return base, false
	}
	return s, true
}

// ListLibrarySeries handles GET /api/library/series?team_id=.
func (h *Handlers) ListLibrarySeries(w http.ResponseWriter, r *http.Request) {
	if !h.libraryReady(w) {
		return
	}
	q := r.URL.Query()
	limit, offset, err := libraryPaging(q)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	filter := library.SeriesFilter{Limit: limit, Offset: offset}
	if raw := q.Get("team_id"); raw != "" {
		id, err := uuid.Parse(raw)
		if err != nil {
			writeError(w, http.StatusBadRequest, "team_id must be a valid UUID")
			return
		}
		filter.TeamID = &id
	}
	series, err := h.libraryRepo.ListSeries(r.Context(), filter)
	if err != nil {
		internalError(w, "list library series", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"series": series})
}

// CreateLibrarySeries handles POST /api/library/series. Matches join it with
// PATCH /api/library/matches/{id}.
func (h *Handlers) CreateLibrarySeries(w http.ResponseWriter, r *http.Request) {
	if !h.libraryReady(w) {
		return
	}
	s, ok := h.decodeSeriesRequest(w, r, library.Series{})
	if !ok {
		return
	}
	if err := h.libraryRepo.CreateSeries(r.Context(), &s); err != nil {
		writeLibraryError(w, "create series", err)
		return
	}
	writeJSON(w, http.StatusCreated, s)
}

func (h *Handlers) loadLibrarySeries(w http.ResponseWriter, r *http.Request) (library.Series, bool) {
	if !h.libraryReady(w) {
		return library.Series{}, false
	}
	id, ok := libraryUUIDParam(w, r, "id")
	if !ok {
		return library.Series{}, false
	}
	s, err := h.libraryRepo.GetSeries(r.Context(), id)
	if err != nil {
		writeLibraryError(w, "get series", err)
		return library.Series{}, false
	}
	return s, true
}

// GetLibrarySeries handles GET /api/library/series/{id}.
func (h *Handlers) GetLibrarySeries(w http.ResponseWriter, r *http.Request) {
	if s, ok := h.loadLibrarySeries(w, r); ok {
		writeJSON(w, http.StatusOK, s)
	}
}

// PatchLibrarySeries handles PATCH /api/library/series/{id}: format, event,
// notes, the two teams and the known veto order.
func (h *Handlers) PatchLibrarySeries(w http.ResponseWriter, r *http.Request) {
	current, ok := h.loadLibrarySeries(w, r)
	if !ok {
		return
	}
	s, ok := h.decodeSeriesRequest(w, r, current)
	if !ok {
		return
	}
	updated, err := h.libraryRepo.UpdateSeries(r.Context(), current.ID, s)
	if err != nil {
		writeLibraryError(w, "update series", err)
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

// DeleteLibrarySeries handles DELETE /api/library/series/{id}. Its matches
// stay in the library without a series.
func (h *Handlers) DeleteLibrarySeries(w http.ResponseWriter, r *http.Request) {
	if !h.libraryReady(w) {
		return
	}
	id, ok := libraryUUIDParam(w, r, "id")
	if !ok {
		return
	}
	if err := h.libraryRepo.DeleteSeries(r.Context(), id); err != nil {
		writeLibraryError(w, "delete series", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// teamMapsResponse is a team's record on every map, with the matches left
// out of the side counts.
type teamMapsResponse struct {
	TeamID   uuid.UUID           `json:"team_id"`
	TeamName string              `json:"team_name"`
	Matches  int                 `json:"matches"`
	Series   int                 `json:"series"`
	Skipped  []scoutSkipped      `json:"skipped"`
	Maps     []library.MapRecord `json:"maps"`
}

// teamMapHistory reads the team's most recent matches in the date range and
// its series. withDocuments also reads each tactical document for the side
// counts; without it only the scores and vetoes are counted.
func (h *Handlers) teamMapHistory(ctx context.Context, team library.Team, from, to *time.Time, withDocuments bool) (teamMapsResponse, error) {
	resp := teamMapsResponse{TeamID: team.ID, TeamName: team.Name, Skipped: []scoutSkipped{}}
	matches, err := h.libraryRepo.ListMatches(ctx, library.MatchFilter{TeamID: &team.ID, From: from, To: to, Limit: scoutCandidateLimit})
	if err != nil {
		return resp, err
	}
	series, err := h.libraryRepo.ListSeries(ctx, library.SeriesFilter{TeamID: &team.ID, Limit: library.MaxListLimit})
	if err != nil {
		return resp, err
	}
	history := make([]library.MapHistoryMatch, 0, len(matches))
	for _, m := range matches {
		c, ok := candidateFor(m, team.ID)
		if !ok {
			continue
		}
		hm := library.MapHistoryMatch{Match: m, TeamKey: c.scout.TeamKey}
		if withDocuments {
			doc, err := h.readLibraryDocument(m.DemoID)
			if err != nil {
				log.Printf("httpapi: map history of team %s skips the sides of match %s: %v", team.ID, m.ID, err)
				resp.Skipped = append(resp.Skipped, scoutSkipped{MatchID: m.ID, Reason: scoutSkipNotAnalyzed})
			} else {
				hm.Document = &doc
			}
		}
		history = append(history, hm)
	}
	resp.Matches, resp.Series = len(history), len(series)
	resp.Maps = library.TeamMapHistory(team.ID, history, series)
	return resp, nil
}

func (h *Handlers) loadLibraryTeam(w http.ResponseWriter, r *http.Request) (library.Team, bool) {
	if !h.libraryReady(w) {
		return library.Team{}, false
	}
	id, ok := libraryUUIDParam(w, r, "id")
	if !ok {
		return library.Team{}, false
	}
	team, err := h.libraryRepo.GetTeam(r.Context(), id)
	if err != nil {
		writeLibraryError(w, "get team", err)
		return library.Team{}, false
	}
	return team, true
}

// GetLibraryTeamMaps handles GET /api/library/teams/{id}/maps?from=&to=: the
// team's results, side rounds and vetoes on every map.
func (h *Handlers) GetLibraryTeamMaps(w http.ResponseWriter, r *http.Request) {
	team, ok := h.loadLibraryTeam(w, r)
	if !ok {
		return
	}
	q := r.URL.Query()
	from, err := optionalTimeQuery(q, "from")
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	to, err := optionalTimeQuery(q, "to")
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	resp, err := h.teamMapHistory(r.Context(), team, from, to, true)
	if err != nil {
		internalError(w, "team map history", err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

// vetoResponse is a simulated veto between a team and an opponent.
type vetoResponse struct {
	TeamID       uuid.UUID        `json:"team_id"`
	TeamName     string           `json:"team_name"`
	OpponentID   uuid.UUID        `json:"opponent_id"`
	OpponentName string           `json:"opponent_name"`
	Plan         library.VetoPlan `json:"plan"`
}

func vetoQuery(q url.Values) (library.VetoRequest, uuid.UUID, error) {
	req := library.VetoRequest{Format: q.Get("format"), First: q.Get("first"), Pool: q["pool"], Done: q["done"]}
	if req.Format == "" {
		req.Format = library.FormatBO3
	}
	if req.First == "" {
		req.First = library.SeriesTeamA
	}
	if len(req.Pool) == 0 {
		req.Pool = library.DefaultMapPool
	}
	if len(req.Pool) > library.MaxVetoSteps || len(req.Done) > library.MaxVetoSteps {
		return req, uuid.Nil, errors.New("a veto has at most 9 maps")
	}
	opponent, err := uuid.Parse(q.Get("opponent_id"))
	if err != nil {
		return req, uuid.Nil, errors.New("opponent_id must be a valid UUID")
	}
	return req, opponent, nil
}

// GetLibraryTeamVeto handles GET /api/library/teams/{id}/veto: a BO1, BO3 or
// BO5 veto against opponent_id over pool, with the steps already done kept
// and the rest recommended from both teams' map records.
func (h *Handlers) GetLibraryTeamVeto(w http.ResponseWriter, r *http.Request) {
	team, ok := h.loadLibraryTeam(w, r)
	if !ok {
		return
	}
	req, opponentID, err := vetoQuery(r.URL.Query())
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if opponentID == team.ID {
		writeError(w, http.StatusBadRequest, "the opponent must be another team")
		return
	}
	opponent, err := h.libraryRepo.GetTeam(r.Context(), opponentID)
	if err != nil {
		writeLibraryError(w, "get opponent", err)
		return
	}
	ours, err := h.teamMapHistory(r.Context(), team, nil, nil, false)
	if err != nil {
		internalError(w, "team map history", err)
		return
	}
	theirs, err := h.teamMapHistory(r.Context(), opponent, nil, nil, false)
	if err != nil {
		internalError(w, "opponent map history", err)
		return
	}
	req.Ours, req.Theirs = ours.Maps, theirs.Maps
	plan, err := library.SimulateVeto(req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, vetoResponse{TeamID: team.ID, TeamName: team.Name, OpponentID: opponent.ID, OpponentName: opponent.Name, Plan: plan})
}
