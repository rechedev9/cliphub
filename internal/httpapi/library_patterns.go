package httpapi

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"time"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/library"
	"github.com/rechedev9/cliphub/internal/tacticalplan"
)

// patternRepository is the saved patterns part of LibraryRepository.
type patternRepository interface {
	CreatePattern(ctx context.Context, p *library.Pattern) error
	GetPattern(ctx context.Context, id uuid.UUID) (library.Pattern, error)
	ListPatterns(ctx context.Context, filter library.PatternFilter) ([]library.Pattern, error)
	UpdatePattern(ctx context.Context, id uuid.UUID, patch library.PatternPatch) (library.Pattern, error)
	DeletePattern(ctx context.Context, id uuid.UUID) error
}

// patternStats is what a saved pattern found, as its row in a list shows it.
type patternStats struct {
	Rounds        int     `json:"rounds"`
	Matched       int     `json:"matched"`
	Won           int     `json:"won"`
	WinRate       float64 `json:"win_rate"`
	RecentMatched int     `json:"recent_matched"`
	Trend         string  `json:"trend"`
}

// patternListItem is a saved pattern with its figures when the list asked for them.
type patternListItem struct {
	library.Pattern
	Stats *patternStats `json:"stats,omitempty"`
}

func statsOf(r tacticalplan.PatternResult) *patternStats {
	return &patternStats{Rounds: r.Rounds, Matched: r.Matched, Won: r.Won, WinRate: r.WinRate, RecentMatched: r.RecentMatched, Trend: r.Trend}
}

// ListLibraryPatterns handles GET /api/library/patterns?map=&team_id=&stats=1.
// With stats, each pattern runs over the team's matches with the team view's
// scope, or over the whole library on its map without team_id.
func (h *Handlers) ListLibraryPatterns(w http.ResponseWriter, r *http.Request) {
	if !h.libraryReady(w) {
		return
	}
	q := r.URL.Query()
	filter := library.PatternFilter{Map: q.Get("map")}
	if filter.Map != "" && !mapNamePattern.MatchString(filter.Map) {
		writeError(w, http.StatusBadRequest, "map must match ^[a-z0-9_]+$")
		return
	}
	var team *library.Team
	if raw := q.Get("team_id"); raw != "" {
		id, err := uuid.Parse(raw)
		if err != nil {
			writeError(w, http.StatusBadRequest, "team_id must be a valid UUID")
			return
		}
		got, err := h.libraryRepo.GetTeam(r.Context(), id)
		if err != nil {
			writeLibraryError(w, "get team", err)
			return
		}
		team, filter.TeamID = &got, &id
	}
	stats := q.Get("stats") == "1"
	if stats && team != nil && filter.Map == "" {
		writeError(w, http.StatusBadRequest, "a team's pattern figures read one map")
		return
	}
	saved, err := h.libraryRepo.ListPatterns(r.Context(), filter)
	if err != nil {
		internalError(w, "list patterns", err)
		return
	}
	items := make([]patternListItem, len(saved))
	for i, p := range saved {
		items[i] = patternListItem{Pattern: p}
	}
	if stats {
		var err error
		if team != nil {
			err = h.teamPatternStats(r.Context(), *team, q, items)
		} else {
			err = h.libraryPatternStats(r.Context(), items)
		}
		var bad errBadSpatialQuery
		if errors.As(err, &bad) {
			writeError(w, http.StatusBadRequest, bad.Error())
			return
		}
		if err != nil {
			internalError(w, "run patterns", err)
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"patterns": items})
}

// teamPatternStats runs every pattern of one map over the team's matches in one pass per side.
func (h *Handlers) teamPatternStats(ctx context.Context, team library.Team, q url.Values, items []patternListItem) error {
	_, inputs, err := h.teamPatternInputs(ctx, team, q, "")
	if err != nil {
		return err
	}
	patterns := make([]tacticalplan.Pattern, len(items))
	for i, item := range items {
		patterns[i] = item.Pattern.Pattern
	}
	results, err := tacticalplan.RunPatterns(inputs, patterns, time.Now())
	if err != nil {
		return err
	}
	for i := range items {
		items[i].Stats = statsOf(results[i])
	}
	return nil
}

// libraryPatternStats runs each pattern over the whole library, sharing one
// read of the matches among the patterns of the same map and scope.
func (h *Handlers) libraryPatternStats(ctx context.Context, items []patternListItem) error {
	type group struct {
		mapName string
		scope   library.PatternScope
	}
	order := []group{}
	members := map[group][]int{}
	for i, item := range items {
		g := group{item.Map, item.Scope}
		if _, ok := members[g]; !ok {
			order = append(order, g)
		}
		members[g] = append(members[g], i)
	}
	for _, g := range order {
		inputs, _, err := h.libraryPatternInputs(ctx, g.mapName, g.scope)
		if err != nil {
			return err
		}
		patterns := make([]tacticalplan.Pattern, len(members[g]))
		for j, i := range members[g] {
			patterns[j] = items[i].Pattern.Pattern
		}
		results, err := tacticalplan.RunPatterns(inputs, patterns, time.Now())
		if err != nil {
			return err
		}
		for j, i := range members[g] {
			items[i].Stats = statsOf(results[j])
		}
	}
	return nil
}

// patternRequest is a saved pattern as a client sends it; nil fields are not set.
type patternRequest struct {
	Name      *string               `json:"name"`
	Map       *string               `json:"map"`
	TeamID    *uuid.UUID            `json:"team_id"`
	ClearTeam bool                  `json:"clear_team"`
	Pattern   *tacticalplan.Pattern `json:"pattern"`
	Scope     *library.PatternScope `json:"scope"`
}

func (req patternRequest) patch() library.PatternPatch {
	return library.PatternPatch{Name: req.Name, Map: req.Map, TeamID: req.TeamID, ClearTeam: req.ClearTeam, Pattern: req.Pattern, Scope: req.Scope}
}

// checkPatternTeam rejects a pattern saved for a team the library does not have.
func (h *Handlers) checkPatternTeam(w http.ResponseWriter, ctx context.Context, p library.Pattern) bool {
	if p.TeamID == nil {
		return true
	}
	if _, err := h.libraryRepo.GetTeam(ctx, *p.TeamID); err != nil {
		writeLibraryError(w, "get team", err)
		return false
	}
	return true
}

// CreateLibraryPattern handles POST /api/library/patterns.
func (h *Handlers) CreateLibraryPattern(w http.ResponseWriter, r *http.Request) {
	if !h.libraryReady(w) {
		return
	}
	var req patternRequest
	if err := decodeSingleJSONBody(w, r, &req, true); err != nil {
		writeError(w, http.StatusBadRequest, "invalid pattern JSON")
		return
	}
	p, err := library.CleanPattern(library.ApplyPatternPatch(library.Pattern{}, req.patch()))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if !h.checkPatternTeam(w, r.Context(), p) {
		return
	}
	if err := h.libraryRepo.CreatePattern(r.Context(), &p); err != nil {
		writeLibraryError(w, "create pattern", err)
		return
	}
	writeJSON(w, http.StatusCreated, p)
}

func (h *Handlers) loadLibraryPattern(w http.ResponseWriter, r *http.Request) (library.Pattern, bool) {
	if !h.libraryReady(w) {
		return library.Pattern{}, false
	}
	id, ok := libraryUUIDParam(w, r, "id")
	if !ok {
		return library.Pattern{}, false
	}
	p, err := h.libraryRepo.GetPattern(r.Context(), id)
	if err != nil {
		writeLibraryError(w, "get pattern", err)
		return library.Pattern{}, false
	}
	return p, true
}

// GetLibraryPattern handles GET /api/library/patterns/{id}.
func (h *Handlers) GetLibraryPattern(w http.ResponseWriter, r *http.Request) {
	if p, ok := h.loadLibraryPattern(w, r); ok {
		writeJSON(w, http.StatusOK, p)
	}
}

// PatchLibraryPattern handles PATCH /api/library/patterns/{id}.
func (h *Handlers) PatchLibraryPattern(w http.ResponseWriter, r *http.Request) {
	current, ok := h.loadLibraryPattern(w, r)
	if !ok {
		return
	}
	var req patternRequest
	if err := decodeSingleJSONBody(w, r, &req, true); err != nil {
		writeError(w, http.StatusBadRequest, "invalid pattern JSON")
		return
	}
	next, err := library.CleanPattern(library.ApplyPatternPatch(current, req.patch()))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if !h.checkPatternTeam(w, r.Context(), next) {
		return
	}
	updated, err := h.libraryRepo.UpdatePattern(r.Context(), current.ID, library.PatternPatch{
		Name: &next.Name, Map: &next.Map, TeamID: next.TeamID, ClearTeam: next.TeamID == nil, Pattern: &next.Pattern, Scope: &next.Scope})
	if err != nil {
		writeLibraryError(w, "update pattern", err)
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

// DeleteLibraryPattern handles DELETE /api/library/patterns/{id}.
func (h *Handlers) DeleteLibraryPattern(w http.ResponseWriter, r *http.Request) {
	if !h.libraryReady(w) {
		return
	}
	id, ok := libraryUUIDParam(w, r, "id")
	if !ok {
		return
	}
	if err := h.libraryRepo.DeletePattern(r.Context(), id); err != nil {
		writeLibraryError(w, "delete pattern", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
