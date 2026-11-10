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

const (
	// maxPatternQueryBytes bounds the pattern a query carries as JSON.
	maxPatternQueryBytes = 8 << 10
	// patternLibraryMatches caps a library-wide run at the newest matches of a map, to bound its memory.
	patternLibraryMatches = 200
	// maxPatternCacheEntries bounds the cached library-wide results; past it the cache starts over.
	maxPatternCacheEntries = 256
)

// patternRunResponse is a pattern over a team's matches or the library: what it
// found, and for the finder the matched rounds' paths and heatmap.
type patternRunResponse struct {
	spatialResponse
	Pattern tacticalplan.Pattern       `json:"pattern"`
	Result  tacticalplan.PatternResult `json:"result"`
}

// patternCache keeps library-wide results while the matches they read are unchanged.
type patternCache struct {
	mu      sync.Mutex
	entries map[string]patternCacheEntry
}

type patternCacheEntry struct {
	version uint64
	result  tacticalplan.PatternResult
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

// teamPatternRead is a team's matches as a pattern reads them; a side narrows the rounds.
type teamPatternRead struct {
	team library.Team
	q    url.Values
	side tacticalplan.Side
}

// teamPatternInputs reads a team's matches on one map through the team view's scope, newest first.
func (h *Handlers) teamPatternInputs(ctx context.Context, read teamPatternRead) (spatialResponse, []tacticalplan.PatternInput, error) {
	q := read.q
	if read.side != "" {
		q = cloneValues(q)
		q.Set("side", string(read.side))
	}
	resp, matches, err := h.teamSpatial(ctx, read.team, q)
	if err != nil {
		return resp, nil, err
	}
	inputs := make([]tacticalplan.PatternInput, len(matches))
	for i, m := range matches {
		// The run, the paths and the heatmap all read the blob: open each file once.
		if m.Blob != nil {
			m.Blob = onceBlob(m.Blob)
		}
		inputs[i] = tacticalplan.PatternInput{Match: m, PlayedAt: resp.Matches[i].PlayedAt, Team: read.team.Name, Opponent: resp.Matches[i].Opponent}
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

// onceBlob reads a match's blob once for every reader of the request.
func onceBlob(read func() ([]byte, error)) func() ([]byte, error) {
	var once sync.Once
	var blob []byte
	var err error
	return func() ([]byte, error) {
		once.Do(func() { blob, err = read() })
		return blob, err
	}
}

// libraryRun is some patterns over every analysed match of a map, from both teams' side.
type libraryRun struct {
	mapName  string
	scope    library.PatternScope
	patterns []tacticalplan.Pattern
}

func patternCacheKey(run libraryRun, p tacticalplan.Pattern) string {
	raw, _ := json.Marshal(struct {
		Map     string               `json:"map"`
		Scope   library.PatternScope `json:"scope"`
		Pattern tacticalplan.Pattern `json:"pattern"`
	}{run.mapName, run.scope, p})
	return string(raw)
}

// libraryPatternResults runs patterns over the newest matches of a map one match
// at a time, reusing a cached result while those matches are unchanged.
func (h *Handlers) libraryPatternResults(ctx context.Context, run libraryRun) ([]tacticalplan.PatternResult, error) {
	listed, err := h.allLibraryMatches(ctx, library.MatchFilter{Map: run.mapName})
	if err != nil {
		return nil, err
	}
	limit, capped := patternLibraryMatches, 0
	if run.scope.Matches > 0 {
		limit = min(limit, run.scope.Matches)
	}
	if len(listed) > limit {
		listed = listed[:limit]
		if run.scope.Matches == 0 {
			capped = limit
		}
	}
	version := matchesVersion(listed)
	keys := make([]string, len(run.patterns))
	results := make([]tacticalplan.PatternResult, len(run.patterns))
	var missing []int
	// One run at a time: a library pass holds one match in memory, never several.
	c := &h.references.patterns
	c.mu.Lock()
	defer c.mu.Unlock()
	for i, p := range run.patterns {
		keys[i] = patternCacheKey(run, p)
		if e, ok := c.entries[keys[i]]; ok && e.version == version {
			results[i] = e.result
		} else {
			missing = append(missing, i)
		}
	}
	if len(missing) == 0 {
		return results, nil
	}
	patterns := make([]tacticalplan.Pattern, len(missing))
	for j, i := range missing {
		patterns[j] = run.patterns[i]
	}
	runner := tacticalplan.NewPatternRunner(patterns, time.Now())
	for _, m := range listed {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		h.addLibraryMatch(runner, libraryMatchRead{match: m, scope: run.scope})
	}
	out := runner.Finish()
	if c.entries == nil || len(c.entries) >= maxPatternCacheEntries {
		c.entries = map[string]patternCacheEntry{}
	}
	for j, i := range missing {
		out[j].CappedAt = capped
		results[i] = out[j]
		c.entries[keys[i]] = patternCacheEntry{version: version, result: out[j]}
	}
	return results, nil
}

type libraryMatchRead struct {
	match library.Match
	scope library.PatternScope
}

// addLibraryMatch reads one match for both of its teams; its document and positions go with the call.
func (h *Handlers) addLibraryMatch(runner *tacticalplan.PatternRunner, read libraryMatchRead) {
	m := read.match
	doc, err := h.readLibraryDocument(m.DemoID)
	if err != nil {
		log.Printf("httpapi: patterns skip match %s: %v", m.ID, err)
		runner.Unreadable()
		return
	}
	var buys, opponentBuys []tacticalplan.BuyType
	if read.scope.Buy != "" {
		buys = []tacticalplan.BuyType{read.scope.Buy}
	}
	if read.scope.OpponentBuy != "" {
		opponentBuys = []tacticalplan.BuyType{read.scope.OpponentBuy}
	}
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
		runner.Add(input)
	}
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

// writePatternError answers a failed run: 400 for a bad query, else 500.
func writePatternError(w http.ResponseWriter, err error) {
	var bad errBadSpatialQuery
	if errors.As(err, &bad) {
		writeError(w, http.StatusBadRequest, bad.Error())
		return
	}
	internalError(w, "run pattern", err)
}

// GetLibraryTeamPattern handles GET /api/library/teams/{id}/pattern: the pattern
// (JSON) over the team's matches on one map, with the matched rounds' paths and heatmap.
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
	resp, err := h.runTeamPattern(r.Context(), patternRun{team: team, q: q, pattern: p})
	if err != nil {
		writePatternError(w, err)
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

func (h *Handlers) runTeamPattern(ctx context.Context, run patternRun) (patternRunResponse, error) {
	spatial, inputs, err := h.teamPatternInputs(ctx, teamPatternRead{team: run.team, q: run.q, side: run.pattern.Side})
	if err != nil {
		return patternRunResponse{}, err
	}
	results := tacticalplan.RunPatterns(inputs, []tacticalplan.Pattern{run.pattern}, time.Now())
	resp := patternRunResponse{spatialResponse: spatial, Pattern: run.pattern, Result: results[0]}
	if err := h.patternPicture(&resp, inputs); err != nil {
		return patternRunResponse{}, err
	}
	return resp, nil
}

// GetLibraryPatternRounds handles GET /api/library/patterns/{id}/rounds: a saved pattern over
// a team (team_id, else its own) with its own scope, or the whole library without one.
func (h *Handlers) GetLibraryPatternRounds(w http.ResponseWriter, r *http.Request) {
	saved, ok := h.loadLibraryPattern(w, r)
	if !ok {
		return
	}
	q := cloneValues(r.URL.Query())
	team, hasTeam, err := h.patternTeam(r.Context(), patternTeamAsk{pattern: saved, asked: q.Get("team_id")})
	if err != nil {
		writeLibraryError(w, "get team", err)
		return
	}
	if hasTeam {
		q.Set("map", saved.Map)
		applyPatternScope(q, saved.Scope)
		resp, err := h.runTeamPattern(r.Context(), patternRun{team: team, q: q, pattern: saved.Pattern})
		if err != nil {
			writePatternError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, resp)
		return
	}
	results, err := h.libraryPatternResults(r.Context(), libraryRun{mapName: saved.Map, scope: saved.Scope, patterns: []tacticalplan.Pattern{saved.Pattern}})
	if err != nil {
		internalError(w, "run pattern", err)
		return
	}
	writeJSON(w, http.StatusOK, patternRunResponse{Pattern: saved.Pattern, Result: results[0], spatialResponse: spatialResponse{
		Map: saved.Map, Maps: []spatialMapCount{}, Matches: []spatialMatchRef{}, Skipped: []scoutSkipped{}}})
}

// patternTeamAsk is a saved pattern and the team a request asked to run it on, if any.
type patternTeamAsk struct {
	pattern library.Pattern
	asked   string
}

// patternTeam is the team a saved pattern runs on: the one asked for, else its
// own. A deleted own team leaves the pattern on the whole library.
func (h *Handlers) patternTeam(ctx context.Context, ask patternTeamAsk) (library.Team, bool, error) {
	if ask.asked != "" {
		id, err := uuid.Parse(ask.asked)
		if err != nil {
			return library.Team{}, false, library.ErrNotFound
		}
		team, err := h.libraryRepo.GetTeam(ctx, id)
		return team, err == nil, err
	}
	if ask.pattern.TeamID == nil {
		return library.Team{}, false, nil
	}
	team, err := h.libraryRepo.GetTeam(ctx, *ask.pattern.TeamID)
	if errors.Is(err, library.ErrNotFound) {
		return library.Team{}, false, nil
	}
	return team, err == nil, err
}

// applyPatternScope writes a saved pattern's own scope into a team query where the query leaves it open.
func applyPatternScope(q url.Values, s library.PatternScope) {
	if s.Matches > 0 && q.Get("max_matches") == "" {
		q.Set("max_matches", strconv.Itoa(s.Matches))
	}
	for key, buy := range map[string]tacticalplan.BuyType{"buy": s.Buy, "opponent_buy": s.OpponentBuy} {
		if buy == "" || q.Has(key) {
			continue
		}
		if buy == library.ScopeBuyHalf {
			q[key] = []string{string(tacticalplan.BuySemi), string(tacticalplan.BuyForce)}
		} else {
			q.Set(key, string(buy))
		}
	}
}
