package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/library"
	"github.com/rechedev9/cliphub/internal/tacticalplan"
)

// maxPatternQueryBytes bounds the pattern a query carries as JSON.
const maxPatternQueryBytes = 8 << 10

// patternRunResponse is a pattern over a team's matches or the library: what it
// found, and for the finder the matched rounds' paths and heatmap.
type patternRunResponse struct {
	spatialResponse
	Pattern tacticalplan.Pattern       `json:"pattern"`
	Result  tacticalplan.PatternResult `json:"result"`
}

// patternQuery reads the pattern parameter, a JSON pattern.
func patternQuery(q url.Values) (tacticalplan.Pattern, error) {
	var p tacticalplan.Pattern
	raw := q.Get("pattern")
	if raw == "" || len(raw) > maxPatternQueryBytes {
		return p, fmt.Errorf("pattern must be a JSON pattern of at most %d bytes", maxPatternQueryBytes)
	}
	if err := json.Unmarshal([]byte(raw), &p); err != nil {
		return p, errors.New("pattern must be a JSON pattern")
	}
	return p, p.Check()
}

// matchesQuery reads matches, how many of the most recent matches to keep; 0 keeps every one.
func matchesQuery(q url.Values) (int, error) {
	raw := q.Get("matches")
	if raw == "" {
		return 0, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 1 || n > library.MaxPatternMatches {
		return 0, fmt.Errorf("matches must be an integer from 1 to %d", library.MaxPatternMatches)
	}
	return n, nil
}

// teamPatternInputs reads a team's matches on one map through the team view's
// scope, newest first and at most matches of them.
func (h *Handlers) teamPatternInputs(ctx context.Context, team library.Team, q url.Values, side tacticalplan.Side) (spatialResponse, []tacticalplan.PatternInput, error) {
	limit, err := matchesQuery(q)
	if err != nil {
		return spatialResponse{}, nil, errBadSpatialQuery{err}
	}
	if side != "" {
		q = cloneValues(q)
		q.Set("side", string(side))
	}
	resp, matches, err := h.teamSpatial(ctx, team, q)
	if err != nil {
		return resp, nil, err
	}
	if limit > 0 && len(matches) > limit {
		matches, resp.Matches = matches[:limit], resp.Matches[:limit]
	}
	inputs := make([]tacticalplan.PatternInput, len(matches))
	for i, m := range matches {
		inputs[i] = tacticalplan.PatternInput{Match: m, PlayedAt: resp.Matches[i].PlayedAt, Team: team.Name, Opponent: resp.Matches[i].Opponent}
	}
	return resp, inputs, nil
}

func cloneValues(q url.Values) url.Values {
	out := url.Values{}
	for k, v := range q {
		out[k] = append([]string{}, v...)
	}
	return out
}

// onceBlob reads a match's blob once for both of its teams.
func onceBlob(read func() ([]byte, error)) func() ([]byte, error) {
	var once sync.Once
	var blob []byte
	var err error
	return func() ([]byte, error) {
		once.Do(func() { blob, err = read() })
		return blob, err
	}
}

// libraryPatternInputs reads every analysed match on a map from both teams'
// side, newest first: the most recent matches of them when set, with the buys.
func (h *Handlers) libraryPatternInputs(ctx context.Context, mapName string, scope library.PatternScope) ([]tacticalplan.PatternInput, int, error) {
	listed, err := h.allLibraryMatches(ctx, library.MatchFilter{Map: mapName})
	if err != nil {
		return nil, 0, err
	}
	var buys, opponentBuys []tacticalplan.BuyType
	if scope.Buy != "" {
		buys = []tacticalplan.BuyType{scope.Buy}
	}
	if scope.OpponentBuy != "" {
		opponentBuys = []tacticalplan.BuyType{scope.OpponentBuy}
	}
	inputs, read, skipped := []tacticalplan.PatternInput{}, 0, 0
	for _, m := range listed {
		if scope.Matches > 0 && read >= scope.Matches {
			break
		}
		doc, err := h.readLibraryDocument(m.DemoID)
		if err != nil {
			log.Printf("httpapi: patterns skip match %s: %v", m.ID, err)
			skipped++
			continue
		}
		read++
		blob := onceBlob(h.positionsReader(m.DemoID))
		for _, t := range m.Teams {
			filter := tacticalplan.Filter{TeamKey: t.Key, Buys: buys, OpponentBuys: opponentBuys}
			input := tacticalplan.PatternInput{Match: tacticalplan.SpatialMatch{MatchID: m.ID, Document: doc, Filter: filter, Blob: blob},
				PlayedAt: m.PlayedAt, Team: matchTeamName(t)}
			for _, other := range m.Teams {
				if other.Key != t.Key {
					input.Opponent = matchTeamName(other)
				}
			}
			inputs = append(inputs, input)
		}
	}
	return inputs, skipped, nil
}

// patternFocus is who the finder's paths and heatmap follow: the first player
// block, its window and whether it reads the rival.
func patternFocus(p tacticalplan.Pattern) (tacticalplan.PatternBlock, bool) {
	for _, b := range p.Blocks {
		if b.Kind == tacticalplan.PatternPlayer {
			return b, true
		}
	}
	return tacticalplan.PatternBlock{}, false
}

// rivalView turns matches to the rival team's perspective, for a block that reads the rival.
func rivalView(matches []tacticalplan.SpatialMatch) []tacticalplan.SpatialMatch {
	out := make([]tacticalplan.SpatialMatch, 0, len(matches))
	for _, m := range matches {
		for _, t := range m.Document.Teams {
			if t.Key != m.Filter.TeamKey {
				m.Filter = tacticalplan.Filter{TeamKey: t.Key}
				out = append(out, m)
				break
			}
		}
	}
	return out
}

// patternPicture adds the matched rounds' paths and the focused players' heatmap in the block's window.
func (h *Handlers) patternPicture(resp *patternRunResponse, inputs []tacticalplan.PatternInput) error {
	all := make([]tacticalplan.SpatialMatch, len(inputs))
	for i, in := range inputs {
		all[i] = in.Match
	}
	resp.ReferenceMatchID, resp.Geometry = referenceGeometry(all)
	matched := tacticalplan.OnlyMatched(inputs, resp.Result)
	focus, ok := patternFocus(resp.Pattern)
	if ok && focus.Rival {
		matched = rivalView(matched)
	}
	overlay, err := tacticalplan.BuildOverlay(matched, tacticalplan.OverlayRoundLimit)
	if err != nil {
		return err
	}
	resp.Overlay = &overlay
	opts := tacticalplan.HeatmapOptions{Kind: tacticalplan.HeatPosition, Window: tacticalplan.SpatialWindow{From: focus.From, To: focus.To}}
	if resp.Geometry != nil {
		opts.Radar = resp.Geometry.Calibration
	}
	for i := range matched {
		matched[i].Focus = focus.Players
	}
	heatmap, err := tacticalplan.BuildHeatmap(matched, opts)
	if err != nil {
		return err
	}
	resp.Heatmap = &heatmap
	return nil
}

// GetLibraryTeamPattern handles GET /api/library/teams/{id}/pattern: a pattern
// (the pattern parameter, JSON) over the team's matches on one map, with the
// team view's scope and matches for the most recent N. The answer lists the
// matched rounds by match, and their paths and heatmap.
func (h *Handlers) GetLibraryTeamPattern(w http.ResponseWriter, r *http.Request) {
	if !h.libraryReady(w) {
		return
	}
	id, ok := libraryUUIDParam(w, r, "id")
	if !ok {
		return
	}
	q := r.URL.Query()
	p, err := patternQuery(q)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	team, err := h.libraryRepo.GetTeam(r.Context(), id)
	if err != nil {
		writeLibraryError(w, "get team", err)
		return
	}
	resp, ok := h.runTeamPattern(w, r.Context(), patternRun{team: team, q: q, pattern: p})
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

// patternRun is one pattern over one team's matches.
type patternRun struct {
	team    library.Team
	q       url.Values
	pattern tacticalplan.Pattern
}

// runTeamPattern answers a pattern over a team, or writes the error.
func (h *Handlers) runTeamPattern(w http.ResponseWriter, ctx context.Context, run patternRun) (patternRunResponse, bool) {
	spatial, inputs, err := h.teamPatternInputs(ctx, run.team, run.q, run.pattern.Side)
	var bad errBadSpatialQuery
	if errors.As(err, &bad) {
		writeError(w, http.StatusBadRequest, bad.Error())
		return patternRunResponse{}, false
	}
	if err != nil {
		internalError(w, "read team matches", err)
		return patternRunResponse{}, false
	}
	results, err := tacticalplan.RunPatterns(inputs, []tacticalplan.Pattern{run.pattern}, time.Now())
	if err != nil {
		internalError(w, "run pattern", err)
		return patternRunResponse{}, false
	}
	resp := patternRunResponse{spatialResponse: spatial, Pattern: run.pattern, Result: results[0]}
	if err := h.patternPicture(&resp, inputs); err != nil {
		internalError(w, "draw pattern rounds", err)
		return patternRunResponse{}, false
	}
	return resp, true
}

// GetLibraryPatternRounds handles GET /api/library/patterns/{id}/rounds: a saved
// pattern over a team (team_id, else its own team) with the team view's scope,
// or over the whole library on its map when it has no team.
func (h *Handlers) GetLibraryPatternRounds(w http.ResponseWriter, r *http.Request) {
	saved, ok := h.loadLibraryPattern(w, r)
	if !ok {
		return
	}
	q := cloneValues(r.URL.Query())
	team, hasTeam, err := h.patternTeam(r.Context(), saved, q.Get("team_id"))
	if err != nil {
		writeLibraryError(w, "get team", err)
		return
	}
	if hasTeam {
		q.Set("map", saved.Map)
		applyPatternScope(q, saved.Scope)
		resp, ok := h.runTeamPattern(w, r.Context(), patternRun{team: team, q: q, pattern: saved.Pattern})
		if ok {
			writeJSON(w, http.StatusOK, resp)
		}
		return
	}
	inputs, skipped, err := h.libraryPatternInputs(r.Context(), saved.Map, saved.Scope)
	if err != nil {
		internalError(w, "read library matches", err)
		return
	}
	results, err := tacticalplan.RunPatterns(inputs, []tacticalplan.Pattern{saved.Pattern}, time.Now())
	if err != nil {
		internalError(w, "run pattern", err)
		return
	}
	resp := patternRunResponse{Pattern: saved.Pattern, Result: results[0], spatialResponse: spatialResponse{
		Map: saved.Map, Maps: []spatialMapCount{}, Matches: []spatialMatchRef{}, Skipped: []scoutSkipped{}}}
	resp.Result.MissingPositions += skipped
	writeJSON(w, http.StatusOK, resp)
}

// patternTeam is the team a saved pattern runs on: the one asked for, else its
// own. A deleted own team leaves the pattern on the whole library.
func (h *Handlers) patternTeam(ctx context.Context, p library.Pattern, asked string) (library.Team, bool, error) {
	if asked != "" {
		id, err := uuid.Parse(asked)
		if err != nil {
			return library.Team{}, false, library.ErrNotFound
		}
		team, err := h.libraryRepo.GetTeam(ctx, id)
		return team, err == nil, err
	}
	if p.TeamID == nil {
		return library.Team{}, false, nil
	}
	team, err := h.libraryRepo.GetTeam(ctx, *p.TeamID)
	if errors.Is(err, library.ErrNotFound) {
		return library.Team{}, false, nil
	}
	return team, err == nil, err
}

// applyPatternScope writes a saved pattern's own scope into a team query where the query leaves it open.
func applyPatternScope(q url.Values, s library.PatternScope) {
	if s.Matches > 0 && q.Get("matches") == "" {
		q.Set("matches", strconv.Itoa(s.Matches))
	}
	if s.Buy != "" && q.Get("buy") == "" {
		q.Set("buy", string(s.Buy))
	}
	if s.OpponentBuy != "" && q.Get("opponent_buy") == "" {
		q.Set("opponent_buy", string(s.OpponentBuy))
	}
}
