package httpapi

import (
	"cmp"
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
	"time"

	"github.com/rechedev9/cliphub/internal/library"
	"github.com/rechedev9/cliphub/internal/tacticalplan"
)

// Baseline scopes: every library match, or professional play. Pro reads the
// library's HLTV demos when they hold enough players, else the community
// server's reference when Studio has fetched one, else the bundled one.
const (
	baselineScopeLibrary = "library"
	baselineScopePro     = "pro"

	referenceSourceLibrary = "library"
	referenceSourceBundled = "bundled"
	referenceSourceServer  = "server"

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
	Scope string `json:"scope"`
	// Map is the map the numbers read; empty when the bundled reference has
	// no entry for the map asked for and falls back to every map.
	Map         string `json:"map"`
	Source      string `json:"source"`
	Description string `json:"description,omitempty"`
	// UpdatedAt is when the server reference was measured.
	UpdatedAt string `json:"updated_at,omitempty"`
	Skipped   int    `json:"skipped"`
	tacticalplan.Baseline
}

type leaderboardResponse struct {
	Scope          string `json:"scope"`
	Map            string `json:"map"`
	Source         string `json:"source"`
	Description    string `json:"description,omitempty"`
	UpdatedAt      string `json:"updated_at,omitempty"`
	Metric         string `json:"metric"`
	HigherIsBetter bool   `json:"higher_is_better"`
	MinRounds      int    `json:"min_rounds"`
	Matches        int    `json:"matches"`
	Rounds         int    `json:"rounds"`
	Skipped        int    `json:"skipped"`
	// Days, Top, TeamTop and Event echo the pro filters applied; Days is 0 for every date.
	Days    int    `json:"days,omitempty"`
	Top     int    `json:"top,omitempty"`
	TeamTop int    `json:"team_top,omitempty"`
	Event   string `json:"event,omitempty"`
	MapRole string `json:"map_role,omitempty"`
	// MapRoles are the positions players have on the side, to offer in a filter; Events the pro events in the period.
	MapRoles []tacticalplan.MapRoleOption `json:"map_roles,omitempty"`
	Events   []string                     `json:"events,omitempty"`
	// Side, Role, From, To and MatchSource echo the library filters applied.
	Side        string                        `json:"side,omitempty"`
	Role        string                        `json:"role,omitempty"`
	From        *time.Time                    `json:"from,omitempty"`
	To          *time.Time                    `json:"to,omitempty"`
	MatchSource string                        `json:"match_source,omitempty"`
	Players     []tacticalplan.LeaderboardRow `json:"players"`
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
	ref, source := h.proReference()
	b := entry.sample.Baseline(q.minRounds, ref.Spread())
	if q.scope == baselineScopePro && b.Players < tacticalplan.ProReferenceMinPlayers {
		bundled, _, mapName := ref.For(q.mapName)
		writeJSON(w, http.StatusOK, baselineResponse{Scope: q.scope, Map: mapName, Source: source, Description: ref.Description, UpdatedAt: ref.UpdatedAt, Baseline: bundled})
		return
	}
	writeJSON(w, http.StatusOK, baselineResponse{Scope: q.scope, Map: q.mapName, Source: referenceSourceLibrary, Skipped: entry.skipped, Baseline: b})
}

// GetLibraryLeaderboard handles GET /api/library/leaderboard: the players
// ranked by one metric, best first. The library scope also takes the side, a
// role, a date range and a match source; the pro scope takes none of them.
func (h *Handlers) GetLibraryLeaderboard(w http.ResponseWriter, r *http.Request) {
	if !h.libraryReady(w) {
		return
	}
	values := r.URL.Query()
	q, err := parseBaselineQuery(values)
	metric := values.Get("metric")
	if err == nil && !slices.Contains(tacticalplan.LeaderboardMetrics, metric) {
		err = errors.New("metric must be one of " + strings.Join(tacticalplan.LeaderboardMetrics, ", "))
	}
	limit := leaderboardDefaultLimit
	if raw := values.Get("limit"); err == nil && raw != "" {
		if limit, err = strconv.Atoi(raw); err != nil || limit < 1 || limit > leaderboardMaxLimit {
			err = fmt.Errorf("limit must be an integer from 1 to %d", leaderboardMaxLimit)
		}
	}
	var filters leaderboardFilters
	if err == nil {
		filters, err = parseLeaderboardFilters(values, q)
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	out := leaderboardResponse{
		Scope: q.scope, Map: q.mapName, Source: referenceSourceLibrary, Metric: metric, HigherIsBetter: !tacticalplan.LowerIsBetter(metric),
		MinRounds: q.minRounds, Side: string(filters.side), Role: cmp.Or(filters.role, filters.libraryRole), MapRole: filters.mapRole, From: filters.from, To: filters.to, MatchSource: string(filters.source),
	}
	// A pro board with the community server on always reads its per-player lines; the same
	// request without a period gets the same three months on the page and on MCP.
	if q.scope == baselineScopePro && (filters.proServer() || h.references.community.Client != nil) {
		if filters.days == 0 {
			filters.days = proDefaultDays
		}
		out.Days, out.Top, out.TeamTop, out.Event = max(filters.days, 0), filters.top, filters.teamTop, filters.event
		h.writeProLeaderboard(w, proBoard{ctx: r.Context(), out: out, filters: filters, limit: limit})
		return
	}
	if q.scope == baselineScopeLibrary {
		fold, err := h.leaderboardSample(r.Context(), leaderboardRead{query: q, filters: filters})
		if err != nil {
			internalError(w, "leaderboard", err)
			return
		}
		out.Matches, out.Rounds, out.Skipped = fold.sample.Matches, fold.sample.Rounds, fold.skipped
		out.Players = fold.sample.Rows(tacticalplan.LeaderboardOptions{
			Metric: metric, MinRounds: q.minRounds, Limit: limit, Role: filters.role, MapRole: filters.mapRole, MapRoleSide: filters.side, AnyMap: q.mapName == "",
			LibraryRoles: fold.libraryRoles, LibraryRole: filters.libraryRole,
		})
		if q.mapName != "" && filters.side != "" {
			out.MapRoles = fold.sample.MapRoleOptions(filters.side, q.minRounds)
		}
		writeJSON(w, http.StatusOK, out)
		return
	}
	entry, err := h.baselineSample(r.Context(), q)
	if err != nil {
		internalError(w, "leaderboard", err)
		return
	}
	out.Matches, out.Rounds, out.Skipped = entry.sample.Matches, entry.sample.Rounds, entry.skipped
	// The baseline's rule, so a ranking and the mean above it read the same players.
	if entry.sample.Baseline(q.minRounds, tacticalplan.ReferenceSpread{}).Players < tacticalplan.ProReferenceMinPlayers {
		ref, source := h.proReference()
		bundled, rows, mapName := ref.For(q.mapName)
		out.Map, out.Source, out.Description, out.UpdatedAt, out.Skipped = mapName, source, ref.Description, ref.UpdatedAt, 0
		out.Matches, out.Rounds = bundled.Matches, bundled.Rounds
		out.Players = tacticalplan.ProLeaderboard(rows, metric, q.minRounds, limit)
	} else {
		out.Players = entry.sample.Leaderboard(metric, q.minRounds, limit)
	}
	h.attachLibraryRounds(r.Context(), out.Players, leaderboardRead{query: baselineQuery{scope: baselineScopeLibrary, mapName: q.mapName}, filters: filters})
	writeJSON(w, http.StatusOK, out)
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
	c := &h.references.baselines
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
