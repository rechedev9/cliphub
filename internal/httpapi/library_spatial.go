package httpapi

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"time"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/artifacts"
	"github.com/rechedev9/cliphub/internal/library"
	"github.com/rechedev9/cliphub/internal/radarmap"
	"github.com/rechedev9/cliphub/internal/storage"
	"github.com/rechedev9/cliphub/internal/tacticalplan"
)

// maxSpatialSeconds bounds the heatmap window: a round, its bomb and overtime.
const maxSpatialSeconds = 300

// spatialMatchRef is one match a heatmap or an overlay read.
type spatialMatchRef struct {
	MatchID  uuid.UUID `json:"match_id"`
	Map      string    `json:"map"`
	PlayedAt time.Time `json:"played_at"`
	TeamKey  string    `json:"team_key,omitempty"`
	Opponent string    `json:"opponent,omitempty"`
}

// spatialMapCount is a map the selection holds and how many matches it has there.
type spatialMapCount struct {
	Map     string `json:"map"`
	Matches int    `json:"matches"`
}

// spatialResponse is a heatmap or an overlay with what it read. Geometry is the
// radar of the reference match, the one every position is drawn on.
type spatialResponse struct {
	TeamID           *uuid.UUID                `json:"team_id,omitempty"`
	TeamName         string                    `json:"team_name,omitempty"`
	Map              string                    `json:"map"`
	Maps             []spatialMapCount         `json:"maps"`
	Matches          []spatialMatchRef         `json:"matches"`
	Skipped          []scoutSkipped            `json:"skipped"`
	ReferenceMatchID *uuid.UUID                `json:"reference_match_id"`
	Geometry         *tacticalplan.MapGeometry `json:"geometry"`
	Heatmap          *tacticalplan.Heatmap     `json:"heatmap,omitempty"`
	Overlay          *tacticalplan.Overlay     `json:"overlay,omitempty"`
}

// spatialRequest is what to build: a heatmap of one kind, or an overlay.
type spatialRequest struct {
	overlay bool
	heatmap tacticalplan.HeatmapOptions
	limit   int
}

func secondsQuery(q url.Values, name string) (float64, error) {
	raw := q.Get(name)
	if raw == "" {
		return 0, nil
	}
	v, err := strconv.ParseFloat(raw, 64)
	if err != nil || v < 0 || v > maxSpatialSeconds {
		return 0, fmt.Errorf("%s must be a number of seconds from 0 to %d", name, maxSpatialSeconds)
	}
	return v, nil
}

func parseSpatialRequest(q url.Values, overlay bool) (spatialRequest, error) {
	req := spatialRequest{overlay: overlay}
	if overlay {
		if raw := q.Get("limit"); raw != "" {
			limit, err := strconv.Atoi(raw)
			if err != nil || limit < 1 || limit > tacticalplan.OverlayRoundLimit {
				return req, fmt.Errorf("limit must be an integer from 1 to %d", tacticalplan.OverlayRoundLimit)
			}
			req.limit = limit
		}
		return req, nil
	}
	kind := tacticalplan.HeatmapKind(q.Get("kind"))
	if kind == "" {
		kind = tacticalplan.HeatPosition
	}
	known := false
	for _, k := range tacticalplan.HeatmapKinds {
		known = known || k == kind
	}
	if !known {
		return req, errors.New("kind must be position, death, utility or timing")
	}
	var err error
	window := tacticalplan.SpatialWindow{}
	if window.From, err = secondsQuery(q, "t_from"); err != nil {
		return req, err
	}
	if window.To, err = secondsQuery(q, "t_to"); err != nil {
		return req, err
	}
	if window.To > 0 && window.To < window.From {
		return req, errors.New("t_to must not be before t_from")
	}
	var grenades []tacticalplan.EventKind
	for _, raw := range q["grenade"] {
		switch g := tacticalplan.EventKind(raw); g {
		case tacticalplan.EventSmoke, tacticalplan.EventFlash, tacticalplan.EventHE, tacticalplan.EventMolotov:
			grenades = append(grenades, g)
		default:
			return req, errors.New("grenade must be smoke, flash, he or molotov")
		}
	}
	req.heatmap = tacticalplan.HeatmapOptions{Kind: kind, Window: window, Grenades: grenades}
	return req, nil
}

// positionsReader opens a match's blob only when the builder reaches it. A
// match without a blob reads as none; a broken read is logged and skipped too.
func (h *Handlers) positionsReader(demoID uuid.UUID) func() ([]byte, error) {
	return func() ([]byte, error) {
		rc, err := h.storage.Open(artifacts.LibraryTacticalPositionsKey(demoID))
		if err != nil {
			if !storage.IsNotExist(err) {
				log.Printf("httpapi: open positions of demo %s: %v", demoID, err)
			}
			return nil, nil
		}
		defer rc.Close()
		blob, err := io.ReadAll(rc)
		if err != nil {
			log.Printf("httpapi: read positions of demo %s: %v", demoID, err)
			return nil, nil
		}
		return blob, nil
	}
}

// referenceGeometry picks the radar every position is drawn on: the newest
// match whose calibration comes from the map's overview, else the newest.
func referenceGeometry(matches []tacticalplan.SpatialMatch) (*uuid.UUID, *tacticalplan.MapGeometry) {
	if len(matches) == 0 {
		return nil, nil
	}
	pick := matches[0]
	for _, m := range matches {
		if m.Document.Geometry.Calibration.Source == radarmap.SourceOverview {
			pick = m
			break
		}
	}
	id, geometry := pick.MatchID, pick.Document.Geometry
	return &id, &geometry
}

func (h *Handlers) buildSpatial(resp spatialResponse, matches []tacticalplan.SpatialMatch, req spatialRequest) (spatialResponse, error) {
	resp.ReferenceMatchID, resp.Geometry = referenceGeometry(matches)
	if req.overlay {
		overlay, err := tacticalplan.BuildOverlay(matches, req.limit)
		if err != nil {
			return resp, err
		}
		resp.Overlay = &overlay
		return resp, nil
	}
	heatmap, err := tacticalplan.BuildHeatmap(matches, req.heatmap)
	if err != nil {
		return resp, err
	}
	resp.Heatmap = &heatmap
	return resp, nil
}

// serveMatchSpatial answers the heatmap and overlay of one match. The query is
// the tactical filter of the aggregate, so the rounds are the ones the
// workspace lists.
func (h *Handlers) serveMatchSpatial(w http.ResponseWriter, r *http.Request, overlay bool) {
	m, ok := h.loadLibraryMatch(w, r)
	if !ok {
		return
	}
	q := r.URL.Query()
	filter, err := tacticalplan.FilterFromValues(q)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	req, err := parseSpatialRequest(q, overlay)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	doc, err := h.readLibraryDocument(m.DemoID)
	if err != nil {
		if storage.IsNotExist(err) {
			writeError(w, http.StatusConflict, "tactical analysis is not ready")
			return
		}
		internalError(w, "read tactical document", err)
		return
	}
	resp := spatialResponse{
		Map: m.Map, Maps: []spatialMapCount{{Map: m.Map, Matches: 1}}, Skipped: []scoutSkipped{},
		Matches: []spatialMatchRef{{MatchID: m.ID, Map: m.Map, PlayedAt: m.PlayedAt, TeamKey: filter.TeamKey}},
	}
	matches := []tacticalplan.SpatialMatch{{MatchID: m.ID, Document: doc, Filter: filter, Blob: h.positionsReader(m.DemoID)}}
	resp, err = h.buildSpatial(resp, matches, req)
	if err != nil {
		internalError(w, "build match spatial view", err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

// GetLibraryMatchHeatmap handles GET /api/library/matches/{id}/tactical/heatmap.
func (h *Handlers) GetLibraryMatchHeatmap(w http.ResponseWriter, r *http.Request) {
	h.serveMatchSpatial(w, r, false)
}

// GetLibraryMatchOverlay handles GET /api/library/matches/{id}/tactical/overlay.
func (h *Handlers) GetLibraryMatchOverlay(w http.ResponseWriter, r *http.Request) {
	h.serveMatchSpatial(w, r, true)
}

// chooseSpatialMap counts the maps of the selection and picks the one asked
// for, or the most played one (ties by name).
func chooseSpatialMap(selected []tacticalplan.ScoutMatch, asked []string) (string, []spatialMapCount, error) {
	if len(asked) > 1 {
		return "", nil, errors.New("a heatmap or an overlay reads one map")
	}
	counts := map[string]int{}
	for _, m := range selected {
		counts[m.Document.Demo.Map]++
	}
	maps := make([]spatialMapCount, 0, len(counts))
	for name, n := range counts {
		maps = append(maps, spatialMapCount{Map: name, Matches: n})
	}
	sort.Slice(maps, func(i, j int) bool {
		if maps[i].Matches != maps[j].Matches {
			return maps[i].Matches > maps[j].Matches
		}
		return maps[i].Map < maps[j].Map
	})
	if len(asked) == 1 {
		return asked[0], maps, nil
	}
	if len(maps) == 0 {
		return "", maps, nil
	}
	return maps[0].Map, maps, nil
}

// teamSpatial reads a team's matches on one map with the scout filter, and
// follows the team through each with the round filter of the query.
func (h *Handlers) teamSpatial(ctx context.Context, team library.Team, q url.Values) (spatialResponse, []tacticalplan.SpatialMatch, error) {
	scout, matchIDs, err := scoutQuery(q)
	if err != nil {
		return spatialResponse{}, nil, errBadSpatialQuery{err}
	}
	rounds, err := tacticalplan.FilterFromValues(q)
	if err != nil {
		return spatialResponse{}, nil, errBadSpatialQuery{err}
	}
	// Slots differ between demos, so a team view cannot filter by them.
	rounds.Slots = nil
	candidates, pending, skipped, err := h.teamCandidates(ctx, team, matchIDs)
	if err != nil {
		return spatialResponse{}, nil, err
	}
	asked := scout.Maps
	scout.Maps = nil
	selected := scout.Select(pending)
	mapName, maps, err := chooseSpatialMap(selected, asked)
	if err != nil {
		return spatialResponse{}, nil, errBadSpatialQuery{err}
	}
	teamID := team.ID
	resp := spatialResponse{
		TeamID: &teamID, TeamName: team.Name, Map: mapName, Maps: maps,
		Matches: []spatialMatchRef{}, Skipped: skipped,
	}
	matches := []tacticalplan.SpatialMatch{}
	for _, sm := range selected {
		if sm.Document.Demo.Map != mapName {
			continue
		}
		if len(matches) >= library.MaxReportMatches {
			resp.Skipped = append(resp.Skipped, scoutSkipped{MatchID: sm.MatchID, Reason: scoutSkipBeyondLimits})
			continue
		}
		c := candidates[sm.MatchID]
		doc, err := h.readLibraryDocument(c.match.DemoID)
		if err != nil {
			log.Printf("httpapi: spatial view of %s skips match %s: %v", team.ID, sm.MatchID, err)
			resp.Skipped = append(resp.Skipped, scoutSkipped{MatchID: sm.MatchID, Reason: scoutSkipNotAnalyzed})
			continue
		}
		filter := rounds
		filter.TeamKey, filter.Side = sm.TeamKey, scout.Side
		matches = append(matches, tacticalplan.SpatialMatch{
			MatchID: sm.MatchID, Document: doc, Filter: filter, Blob: h.positionsReader(c.match.DemoID),
		})
		resp.Matches = append(resp.Matches, spatialMatchRef{
			MatchID: sm.MatchID, Map: mapName, PlayedAt: sm.PlayedAt, TeamKey: sm.TeamKey, Opponent: sm.Opponent,
		})
	}
	return resp, matches, nil
}

// errBadSpatialQuery marks a query error, answered as 400.
type errBadSpatialQuery struct{ err error }

func (e errBadSpatialQuery) Error() string { return e.err.Error() }

func (h *Handlers) serveTeamSpatial(w http.ResponseWriter, r *http.Request, overlay bool) {
	if !h.libraryReady(w) {
		return
	}
	id, ok := libraryUUIDParam(w, r, "id")
	if !ok {
		return
	}
	q := r.URL.Query()
	req, err := parseSpatialRequest(q, overlay)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	team, err := h.libraryRepo.GetTeam(r.Context(), id)
	if err != nil {
		writeLibraryError(w, "get team", err)
		return
	}
	resp, matches, err := h.teamSpatial(r.Context(), team, q)
	var bad errBadSpatialQuery
	if errors.As(err, &bad) {
		writeError(w, http.StatusBadRequest, bad.Error())
		return
	}
	if err != nil {
		internalError(w, "read team matches", err)
		return
	}
	resp, err = h.buildSpatial(resp, matches, req)
	if err != nil {
		internalError(w, "build team spatial view", err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

// GetLibraryTeamHeatmap handles GET /api/library/teams/{id}/heatmap.
func (h *Handlers) GetLibraryTeamHeatmap(w http.ResponseWriter, r *http.Request) {
	h.serveTeamSpatial(w, r, false)
}

// GetLibraryTeamOverlay handles GET /api/library/teams/{id}/overlay.
func (h *Handlers) GetLibraryTeamOverlay(w http.ResponseWriter, r *http.Request) {
	h.serveTeamSpatial(w, r, true)
}

type libraryAnnotationRequest struct {
	MatchID *uuid.UUID        `json:"match_id"`
	Round   *int              `json:"round"`
	Tick    *int              `json:"tick"`
	Text    *string           `json:"text"`
	Strokes *[]library.Stroke `json:"strokes"`
}

// ListLibraryAnnotations handles GET /api/library/annotations?match_id=&round=.
func (h *Handlers) ListLibraryAnnotations(w http.ResponseWriter, r *http.Request) {
	if !h.libraryReady(w) {
		return
	}
	q := r.URL.Query()
	ids, err := uuidList(q["match_id"], "match_id", library.MaxReportMatches)
	if err == nil && len(ids) == 0 {
		err = errors.New("match_id is required")
	}
	filter := library.AnnotationFilter{MatchIDs: ids}
	if err == nil && q.Get("round") != "" {
		if filter.Round, err = strconv.Atoi(q.Get("round")); err != nil || filter.Round < 1 {
			err = errors.New("round must be a positive integer")
		}
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	notes, err := h.libraryRepo.ListAnnotations(r.Context(), filter)
	if err != nil {
		internalError(w, "list annotations", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"annotations": notes})
}

// checkAnnotationRound rejects a note on a round the match does not have.
func checkAnnotationRound(m library.Match, round int) error {
	if m.RoundCount > 0 && round > m.RoundCount {
		return fmt.Errorf("round %d is not in the match (it has %d)", round, m.RoundCount)
	}
	return nil
}

// CreateLibraryAnnotation handles POST /api/library/annotations.
func (h *Handlers) CreateLibraryAnnotation(w http.ResponseWriter, r *http.Request) {
	if !h.libraryReady(w) {
		return
	}
	var req libraryAnnotationRequest
	if err := decodeSingleJSONBody(w, r, &req, true); err != nil {
		writeError(w, http.StatusBadRequest, "invalid annotation JSON")
		return
	}
	if req.MatchID == nil || req.Round == nil || req.Tick == nil {
		writeError(w, http.StatusBadRequest, "match_id, round and tick are required")
		return
	}
	note := library.Annotation{MatchID: *req.MatchID, Round: *req.Round, Tick: *req.Tick}
	if req.Text != nil {
		note.Text = *req.Text
	}
	if req.Strokes != nil {
		note.Strokes = *req.Strokes
	}
	note, err := library.CleanAnnotation(note)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	m, err := h.libraryRepo.GetMatch(r.Context(), note.MatchID)
	if err != nil {
		writeLibraryError(w, "get match", err)
		return
	}
	if err := checkAnnotationRound(m, note.Round); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := h.libraryRepo.CreateAnnotation(r.Context(), &note); err != nil {
		writeLibraryError(w, "create annotation", err)
		return
	}
	writeJSON(w, http.StatusCreated, note)
}

// PatchLibraryAnnotation handles PATCH /api/library/annotations/{id}. A note
// stays on its match and round; only its tick, text and drawing change.
func (h *Handlers) PatchLibraryAnnotation(w http.ResponseWriter, r *http.Request) {
	if !h.libraryReady(w) {
		return
	}
	id, ok := libraryUUIDParam(w, r, "id")
	if !ok {
		return
	}
	var req libraryAnnotationRequest
	if err := decodeSingleJSONBody(w, r, &req, true); err != nil {
		writeError(w, http.StatusBadRequest, "invalid annotation JSON")
		return
	}
	if req.MatchID != nil || req.Round != nil {
		writeError(w, http.StatusBadRequest, "match_id and round cannot change")
		return
	}
	current, err := h.libraryRepo.GetAnnotation(r.Context(), id)
	if err != nil {
		writeLibraryError(w, "get annotation", err)
		return
	}
	next := current
	if req.Tick != nil {
		next.Tick = *req.Tick
	}
	if req.Text != nil {
		next.Text = *req.Text
	}
	if req.Strokes != nil {
		next.Strokes = *req.Strokes
	}
	if next, err = library.CleanAnnotation(next); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	updated, err := h.libraryRepo.UpdateAnnotation(r.Context(), id, library.AnnotationPatch{
		Tick: &next.Tick, Text: &next.Text, Strokes: &next.Strokes,
	})
	if err != nil {
		writeLibraryError(w, "update annotation", err)
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

// DeleteLibraryAnnotation handles DELETE /api/library/annotations/{id}.
func (h *Handlers) DeleteLibraryAnnotation(w http.ResponseWriter, r *http.Request) {
	if !h.libraryReady(w) {
		return
	}
	id, ok := libraryUUIDParam(w, r, "id")
	if !ok {
		return
	}
	if err := h.libraryRepo.DeleteAnnotation(r.Context(), id); err != nil {
		writeLibraryError(w, "delete annotation", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
