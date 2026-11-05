package httpapi

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"hash/fnv"
	"log"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"sync"

	"github.com/rechedev9/cliphub/internal/library"
	"github.com/rechedev9/cliphub/internal/tacticalplan"
)

// Baseline scopes: every library match, or only the HLTV demos.
const (
	baselineScopeLibrary = "library"
	baselineScopePro     = "pro"

	baselineMaxMinRounds    = 10000
	leaderboardDefaultLimit = 100
	leaderboardMaxLimit     = 500
)

// baselineCache keeps one sample per scope and map. An entry is valid while
// the set of matches it read, with their update times, is unchanged.
type baselineCache struct {
	mu      sync.Mutex
	entries map[string]baselineEntry
}

type baselineEntry struct {
	version uint64
	sample  *tacticalplan.BaselineSample
	skipped int
}

type baselineResponse struct {
	Scope   string `json:"scope"`
	Map     string `json:"map"`
	Skipped int    `json:"skipped"`
	tacticalplan.Baseline
}

type leaderboardResponse struct {
	Scope          string                        `json:"scope"`
	Map            string                        `json:"map"`
	Metric         string                        `json:"metric"`
	HigherIsBetter bool                          `json:"higher_is_better"`
	MinRounds      int                           `json:"min_rounds"`
	Matches        int                           `json:"matches"`
	Rounds         int                           `json:"rounds"`
	Skipped        int                           `json:"skipped"`
	Players        []tacticalplan.LeaderboardRow `json:"players"`
}

type baselineQuery struct {
	scope     string
	mapName   string
	minRounds int
}

func parseBaselineQuery(q url.Values) (baselineQuery, error) {
	out := baselineQuery{scope: baselineScopeLibrary, mapName: q.Get("map"), minRounds: tacticalplan.BaselineMinRounds}
	if s := q.Get("scope"); s != "" {
		if s != baselineScopeLibrary && s != baselineScopePro {
			return out, errors.New("scope must be library or pro")
		}
		out.scope = s
	}
	if out.mapName != "" && !mapNamePattern.MatchString(out.mapName) {
		return out, errors.New("map must match ^[a-z0-9_]+$")
	}
	if raw := q.Get("min_rounds"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 || n > baselineMaxMinRounds {
			return out, fmt.Errorf("min_rounds must be an integer from 1 to %d", baselineMaxMinRounds)
		}
		out.minRounds = n
	}
	return out, nil
}

// GetLibraryBaseline handles GET /api/library/baseline: the per-player average
// of each reference metric over the library or its HLTV demos.
func (h *Handlers) GetLibraryBaseline(w http.ResponseWriter, r *http.Request) {
	if !h.libraryReady(w) {
		return
	}
	q, err := parseBaselineQuery(r.URL.Query())
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	entry, err := h.baselineSample(r.Context(), q)
	if err != nil {
		internalError(w, "baseline", err)
		return
	}
	writeJSON(w, http.StatusOK, baselineResponse{Scope: q.scope, Map: q.mapName, Skipped: entry.skipped, Baseline: entry.sample.Baseline(q.minRounds)})
}

// GetLibraryLeaderboard handles GET /api/library/leaderboard: the players
// ranked by one reference metric, best first.
func (h *Handlers) GetLibraryLeaderboard(w http.ResponseWriter, r *http.Request) {
	if !h.libraryReady(w) {
		return
	}
	values := r.URL.Query()
	q, err := parseBaselineQuery(values)
	metric := values.Get("metric")
	if err == nil && !slices.Contains(tacticalplan.ReferenceMetrics, metric) {
		err = errors.New("metric must be one of " + strings.Join(tacticalplan.ReferenceMetrics, ", "))
	}
	limit := leaderboardDefaultLimit
	if raw := values.Get("limit"); err == nil && raw != "" {
		if limit, err = strconv.Atoi(raw); err != nil || limit < 1 || limit > leaderboardMaxLimit {
			err = fmt.Errorf("limit must be an integer from 1 to %d", leaderboardMaxLimit)
		}
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	entry, err := h.baselineSample(r.Context(), q)
	if err != nil {
		internalError(w, "leaderboard", err)
		return
	}
	writeJSON(w, http.StatusOK, leaderboardResponse{
		Scope: q.scope, Map: q.mapName, Metric: metric, HigherIsBetter: !tacticalplan.LowerIsBetter(metric),
		MinRounds: q.minRounds, Matches: entry.sample.Matches, Rounds: entry.sample.Rounds, Skipped: entry.skipped,
		Players: entry.sample.Leaderboard(metric, q.minRounds, limit),
	})
}

// baselineSample returns the cached sample of a scope and map, reading every
// match again only when the matches it covers changed.
func (h *Handlers) baselineSample(ctx context.Context, q baselineQuery) (baselineEntry, error) {
	filter := library.MatchFilter{Map: q.mapName}
	if q.scope == baselineScopePro {
		filter.Source = library.SourceHLTV
	}
	matches, err := h.allLibraryMatches(ctx, filter)
	if err != nil {
		return baselineEntry{}, err
	}
	version := matchesVersion(matches)
	key := q.scope + "|" + q.mapName
	c := &h.baselines
	c.mu.Lock()
	defer c.mu.Unlock()
	if e, ok := c.entries[key]; ok && e.version == version {
		return e, nil
	}
	entry := baselineEntry{version: version, sample: &tacticalplan.BaselineSample{}}
	for _, m := range matches {
		if err := ctx.Err(); err != nil {
			return baselineEntry{}, err
		}
		doc, err := h.readLibraryDocument(m.DemoID)
		if err != nil {
			log.Printf("httpapi: baseline skips match %s: %v", m.ID, err)
			entry.skipped++
			continue
		}
		// One match at a time keeps a single document in memory.
		entry.sample.Add(tacticalplan.Metrics([]tacticalplan.MetricsMatch{{MatchID: m.ID, PlayedAt: m.PlayedAt, Document: doc}}, tacticalplan.MetricsOptions{}))
	}
	if c.entries == nil {
		c.entries = map[string]baselineEntry{}
	}
	c.entries[key] = entry
	return entry, nil
}

// allLibraryMatches pages through every match the filter selects.
func (h *Handlers) allLibraryMatches(ctx context.Context, filter library.MatchFilter) ([]library.Match, error) {
	filter.Limit = library.MaxListLimit
	out := []library.Match{}
	for {
		page, err := h.libraryRepo.ListMatches(ctx, filter)
		if err != nil {
			return nil, err
		}
		out = append(out, page...)
		if len(page) < filter.Limit {
			return out, nil
		}
		filter.Offset += len(page)
	}
}

// matchesVersion fingerprints a set of matches by id and update time, so an
// import, a deletion, a new scan or a source change all invalidate the cache.
func matchesVersion(matches []library.Match) uint64 {
	h := fnv.New64a()
	var stamp [8]byte
	for _, m := range matches {
		_, _ = h.Write(m.ID[:])
		binary.LittleEndian.PutUint64(stamp[:], uint64(m.UpdatedAt.UnixNano()))
		_, _ = h.Write(stamp[:])
	}
	return h.Sum64()
}
