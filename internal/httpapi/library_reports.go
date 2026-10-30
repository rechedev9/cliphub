package httpapi

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/artifacts"
	"github.com/rechedev9/cliphub/internal/library"
	"github.com/rechedev9/cliphub/internal/tacticalplan"
)

// Report limits. A title is a short name; notes are the coach's free text.
const (
	maxReportNotesRunes = 4000
	// scoutCandidateLimit is how many of a team's most recent matches a scout
	// without explicit match ids considers before its filter picks from them.
	scoutCandidateLimit = library.MaxListLimit
)

// Why a requested match was left out of a scout.
const (
	scoutSkipNotFound     = "not_found"
	scoutSkipTeamMissing  = "team_not_in_match"
	scoutSkipNotAnalyzed  = "analysis_missing"
	scoutSkipBeyondLimits = "over_match_limit"
)

// scoutSkipped is a match the scout could not read, and why.
type scoutSkipped struct {
	MatchID uuid.UUID `json:"match_id"`
	Reason  string    `json:"reason"`
}

// scoutResponse is a scouting report of one team with the matches it skipped.
type scoutResponse struct {
	TeamID   uuid.UUID      `json:"team_id"`
	TeamName string         `json:"team_name"`
	Skipped  []scoutSkipped `json:"skipped"`
	tacticalplan.ScoutReport
}

// scoutCandidate is a library match in which the scouted team played.
type scoutCandidate struct {
	match library.Match
	scout tacticalplan.ScoutMatch
}

// candidateFor reads which side of a match the team played and who the rival was.
func candidateFor(m library.Match, teamID uuid.UUID) (scoutCandidate, bool) {
	c := scoutCandidate{match: m, scout: tacticalplan.ScoutMatch{MatchID: m.ID, PlayedAt: m.PlayedAt}}
	c.scout.Document.Demo.Map = m.Map
	found := false
	for _, t := range m.Teams {
		if t.TeamID != nil && *t.TeamID == teamID {
			c.scout.TeamKey = t.Key
			found = true
			continue
		}
		if t.TeamID != nil {
			c.scout.OpponentID = t.TeamID.String()
		}
		c.scout.Opponent = t.TeamName
		if c.scout.Opponent == "" {
			c.scout.Opponent = t.ClanName
		}
	}
	return c, found
}

// teamCandidates reads the team's matches (the given ids, or its most recent
// ones) and the side the team played in each. Matches it cannot use are skipped.
func (h *Handlers) teamCandidates(ctx context.Context, team library.Team, matchIDs []uuid.UUID) (map[uuid.UUID]scoutCandidate, []tacticalplan.ScoutMatch, []scoutSkipped, error) {
	skipped := []scoutSkipped{}
	var matches []library.Match
	if len(matchIDs) > 0 {
		for _, id := range matchIDs {
			m, err := h.libraryRepo.GetMatch(ctx, id)
			if errors.Is(err, library.ErrNotFound) {
				skipped = append(skipped, scoutSkipped{MatchID: id, Reason: scoutSkipNotFound})
				continue
			}
			if err != nil {
				return nil, nil, nil, err
			}
			matches = append(matches, m)
		}
	} else {
		listed, err := h.libraryRepo.ListMatches(ctx, library.MatchFilter{TeamID: &team.ID, Limit: scoutCandidateLimit})
		if err != nil {
			return nil, nil, nil, err
		}
		matches = listed
	}

	candidates := map[uuid.UUID]scoutCandidate{}
	pending := make([]tacticalplan.ScoutMatch, 0, len(matches))
	for _, m := range matches {
		c, ok := candidateFor(m, team.ID)
		if !ok {
			skipped = append(skipped, scoutSkipped{MatchID: m.ID, Reason: scoutSkipTeamMissing})
			continue
		}
		candidates[m.ID] = c
		pending = append(pending, c.scout)
	}
	return candidates, pending, skipped, nil
}

// loadTeamMatches reads the team's matches (the given ids, or its most recent
// ones), keeps those the filter selects, and loads their tactical documents
// and setup frames. Matches it cannot read are returned as skipped.
func (h *Handlers) loadTeamMatches(ctx context.Context, team library.Team, matchIDs []uuid.UUID, filter tacticalplan.ScoutFilter) ([]tacticalplan.ScoutMatch, []scoutSkipped, error) {
	candidates, pending, skipped, err := h.teamCandidates(ctx, team, matchIDs)
	if err != nil {
		return nil, nil, err
	}
	selected := filter.Select(pending)
	loaded := make([]tacticalplan.ScoutMatch, 0, len(selected))
	for i, sm := range selected {
		if i >= library.MaxReportMatches {
			skipped = append(skipped, scoutSkipped{MatchID: sm.MatchID, Reason: scoutSkipBeyondLimits})
			continue
		}
		c := candidates[sm.MatchID]
		doc, err := h.readLibraryDocument(c.match.DemoID)
		if err != nil {
			log.Printf("httpapi: team %s skips match %s: %v", team.ID, sm.MatchID, err)
			skipped = append(skipped, scoutSkipped{MatchID: sm.MatchID, Reason: scoutSkipNotAnalyzed})
			continue
		}
		sm.Document = doc
		sm.Setup = h.readSetupFrames(c.match.DemoID, doc)
		loaded = append(loaded, sm)
	}
	return loaded, skipped, nil
}

// buildScout aggregates the tactical documents of the team's selected matches.
func (h *Handlers) buildScout(ctx context.Context, team library.Team, matchIDs []uuid.UUID, filter tacticalplan.ScoutFilter) (scoutResponse, error) {
	loaded, skipped, err := h.loadTeamMatches(ctx, team, matchIDs, filter)
	if err != nil {
		return scoutResponse{}, err
	}
	return scoutResponse{
		TeamID: team.ID, TeamName: team.Name, Skipped: skipped,
		ScoutReport: tacticalplan.Scout(loaded, filter),
	}, nil
}

// readSetupFrames reads the CT setup frames from a match's position blob. A
// missing or broken blob leaves the setups without positions, never the report.
func (h *Handlers) readSetupFrames(demoID uuid.UUID, doc tacticalplan.Document) map[int]tacticalplan.Frame {
	rc, err := h.storage.Open(artifacts.LibraryTacticalPositionsKey(demoID))
	if err != nil {
		return nil
	}
	defer rc.Close()
	blob, err := io.ReadAll(rc)
	if err != nil {
		log.Printf("httpapi: read positions of demo %s: %v", demoID, err)
		return nil
	}
	frames, err := tacticalplan.SetupFrames(blob, doc)
	if err != nil {
		log.Printf("httpapi: setup frames of demo %s: %v", demoID, err)
		return nil
	}
	return frames
}

func parseScoutSide(raw string) (tacticalplan.Side, error) {
	switch strings.ToUpper(strings.TrimSpace(raw)) {
	case "":
		return "", nil
	case "CT":
		return tacticalplan.SideCT, nil
	case "T":
		return tacticalplan.SideT, nil
	default:
		return "", errors.New("side must be CT or T")
	}
}

func uuidList(raw []string, name string, limit int) ([]uuid.UUID, error) {
	if len(raw) > limit {
		return nil, fmt.Errorf("at most %d %s values", limit, name)
	}
	out := make([]uuid.UUID, 0, len(raw))
	for _, item := range raw {
		id, err := uuid.Parse(item)
		if err != nil {
			return nil, fmt.Errorf("%s must be a valid UUID", name)
		}
		out = append(out, id)
	}
	return out, nil
}

// scoutQuery reads the scout filter and the explicit match ids of a request.
func scoutQuery(q url.Values) (tacticalplan.ScoutFilter, []uuid.UUID, error) {
	var f tacticalplan.ScoutFilter
	var err error
	for _, m := range q["map"] {
		if !mapNamePattern.MatchString(m) {
			return f, nil, errors.New("map must match ^[a-z0-9_]+$")
		}
		f.Maps = append(f.Maps, m)
	}
	if f.Side, err = parseScoutSide(q.Get("side")); err != nil {
		return f, nil, err
	}
	if f.From, err = optionalTimeQuery(q, "from"); err != nil {
		return f, nil, err
	}
	if f.To, err = optionalTimeQuery(q, "to"); err != nil {
		return f, nil, err
	}
	opponents, err := uuidList(q["opponent_id"], "opponent_id", library.MaxListLimit)
	if err != nil {
		return f, nil, err
	}
	for _, id := range opponents {
		f.Opponents = append(f.Opponents, id.String())
	}
	matchIDs, err := uuidList(q["match_id"], "match_id", library.MaxReportMatches)
	if err != nil {
		return f, nil, err
	}
	return f, matchIDs, nil
}

// GetLibraryTeamScout handles GET /api/library/teams/{id}/scout: the scouting
// report of a team over its matches, filtered by map, side, date and rival.
func (h *Handlers) GetLibraryTeamScout(w http.ResponseWriter, r *http.Request) {
	if !h.libraryReady(w) {
		return
	}
	id, ok := libraryUUIDParam(w, r, "id")
	if !ok {
		return
	}
	filter, matchIDs, err := scoutQuery(r.URL.Query())
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	team, err := h.libraryRepo.GetTeam(r.Context(), id)
	if err != nil {
		writeLibraryError(w, "get team", err)
		return
	}
	resp, err := h.buildScout(r.Context(), team, matchIDs, filter)
	if err != nil {
		internalError(w, "scout team", err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

// ListLibraryReports handles GET /api/library/reports.
func (h *Handlers) ListLibraryReports(w http.ResponseWriter, r *http.Request) {
	if !h.libraryReady(w) {
		return
	}
	q := r.URL.Query()
	var filter library.ReportFilter
	var err error
	if filter.Limit, filter.Offset, err = libraryPaging(q); err == nil {
		filter.TeamID, err = optionalUUIDQuery(q, "team_id")
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	reports, err := h.libraryRepo.ListReports(r.Context(), filter)
	if err != nil {
		internalError(w, "list reports", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"reports": reports})
}

type libraryReportRequest struct {
	Title    *string      `json:"title"`
	TeamID   *uuid.UUID   `json:"team_id"`
	MatchIDs *[]uuid.UUID `json:"match_ids"`
	Side     *string      `json:"side"`
	Notes    *string      `json:"notes"`
}

// cleanReportRequest validates the fields a request carries. Every match id
// must exist and have teamID on one of its sides.
func (h *Handlers) cleanReportRequest(ctx context.Context, req libraryReportRequest, teamID uuid.UUID) (libraryReportRequest, string, error) {
	if req.Title != nil {
		title := cleanLibraryText(*req.Title)
		if title == "" {
			return req, "", errors.New("title must not be empty")
		}
		req.Title = &title
	}
	if req.Side != nil {
		side, err := parseScoutSide(*req.Side)
		if err != nil {
			return req, "", err
		}
		value := string(side)
		req.Side = &value
	}
	if req.Notes != nil {
		notes := strings.TrimSpace(*req.Notes)
		if len([]rune(notes)) > maxReportNotesRunes {
			return req, "", fmt.Errorf("notes must be at most %d characters", maxReportNotesRunes)
		}
		req.Notes = &notes
	}
	if req.MatchIDs != nil {
		if len(*req.MatchIDs) > library.MaxReportMatches {
			return req, "", fmt.Errorf("a report reads at most %d matches", library.MaxReportMatches)
		}
		for _, id := range *req.MatchIDs {
			m, err := h.libraryRepo.GetMatch(ctx, id)
			if errors.Is(err, library.ErrNotFound) {
				return req, "unknown_match", fmt.Errorf("match %s does not exist", id)
			}
			if err != nil {
				return req, "", err
			}
			if _, ok := candidateFor(m, teamID); !ok {
				return req, scoutSkipTeamMissing, fmt.Errorf("the team did not play match %s", id)
			}
		}
	}
	return req, "", nil
}

func writeReportRequestError(w http.ResponseWriter, code string, err error) {
	if code != "" {
		writeCodedError(w, http.StatusBadRequest, code, err.Error())
		return
	}
	writeError(w, http.StatusBadRequest, err.Error())
}

// CreateLibraryReport handles POST /api/library/reports.
func (h *Handlers) CreateLibraryReport(w http.ResponseWriter, r *http.Request) {
	if !h.libraryReady(w) {
		return
	}
	var req libraryReportRequest
	if err := decodeSingleJSONBody(w, r, &req, true); err != nil {
		writeError(w, http.StatusBadRequest, "invalid report JSON")
		return
	}
	if req.Title == nil || req.TeamID == nil {
		writeError(w, http.StatusBadRequest, "title and team_id are required")
		return
	}
	if _, err := h.libraryRepo.GetTeam(r.Context(), *req.TeamID); err != nil {
		writeLibraryError(w, "get team", err)
		return
	}
	req, code, err := h.cleanReportRequest(r.Context(), req, *req.TeamID)
	if err != nil {
		writeReportRequestError(w, code, err)
		return
	}
	report := library.Report{Title: *req.Title, TeamID: *req.TeamID, MatchIDs: []uuid.UUID{}}
	if req.MatchIDs != nil {
		report.MatchIDs = *req.MatchIDs
	}
	if req.Side != nil {
		report.Side = *req.Side
	}
	if req.Notes != nil {
		report.Notes = *req.Notes
	}
	if err := h.libraryRepo.CreateReport(r.Context(), &report); err != nil {
		writeLibraryError(w, "create report", err)
		return
	}
	writeJSON(w, http.StatusCreated, report)
}

func (h *Handlers) loadLibraryReport(w http.ResponseWriter, r *http.Request) (library.Report, bool) {
	if !h.libraryReady(w) {
		return library.Report{}, false
	}
	id, ok := libraryUUIDParam(w, r, "id")
	if !ok {
		return library.Report{}, false
	}
	report, err := h.libraryRepo.GetReport(r.Context(), id)
	if err != nil {
		writeLibraryError(w, "get report", err)
		return library.Report{}, false
	}
	return report, true
}

// GetLibraryReport handles GET /api/library/reports/{id}.
func (h *Handlers) GetLibraryReport(w http.ResponseWriter, r *http.Request) {
	if report, ok := h.loadLibraryReport(w, r); ok {
		writeJSON(w, http.StatusOK, report)
	}
}

// PatchLibraryReport handles PATCH /api/library/reports/{id}. The team of a
// report is fixed: a report on another rival is a new report.
func (h *Handlers) PatchLibraryReport(w http.ResponseWriter, r *http.Request) {
	report, ok := h.loadLibraryReport(w, r)
	if !ok {
		return
	}
	var req libraryReportRequest
	if err := decodeSingleJSONBody(w, r, &req, true); err != nil {
		writeError(w, http.StatusBadRequest, "invalid report JSON")
		return
	}
	if req.TeamID != nil && *req.TeamID != report.TeamID {
		writeError(w, http.StatusBadRequest, "team_id cannot change")
		return
	}
	req, code, err := h.cleanReportRequest(r.Context(), req, report.TeamID)
	if err != nil {
		writeReportRequestError(w, code, err)
		return
	}
	updated, err := h.libraryRepo.UpdateReport(r.Context(), report.ID, library.ReportPatch{
		Title: req.Title, MatchIDs: req.MatchIDs, Side: req.Side, Notes: req.Notes,
	})
	if err != nil {
		writeLibraryError(w, "update report", err)
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

// DeleteLibraryReport handles DELETE /api/library/reports/{id}.
func (h *Handlers) DeleteLibraryReport(w http.ResponseWriter, r *http.Request) {
	if !h.libraryReady(w) {
		return
	}
	id, ok := libraryUUIDParam(w, r, "id")
	if !ok {
		return
	}
	if err := h.libraryRepo.DeleteReport(r.Context(), id); err != nil {
		writeLibraryError(w, "delete report", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// GetLibraryReportScout handles GET /api/library/reports/{id}/scout: the
// report's sections, computed from its matches' tactical documents.
func (h *Handlers) GetLibraryReportScout(w http.ResponseWriter, r *http.Request) {
	report, ok := h.loadLibraryReport(w, r)
	if !ok {
		return
	}
	team, err := h.libraryRepo.GetTeam(r.Context(), report.TeamID)
	if err != nil {
		writeLibraryError(w, "get team", err)
		return
	}
	filter := tacticalplan.ScoutFilter{Side: tacticalplan.Side(report.Side)}
	if len(report.MatchIDs) == 0 {
		writeJSON(w, http.StatusOK, scoutResponse{
			TeamID: team.ID, TeamName: team.Name, Skipped: []scoutSkipped{},
			ScoutReport: tacticalplan.Scout(nil, filter),
		})
		return
	}
	resp, err := h.buildScout(r.Context(), team, report.MatchIDs, filter)
	if err != nil {
		internalError(w, "scout report", err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}
