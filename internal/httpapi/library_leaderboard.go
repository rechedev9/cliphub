package httpapi

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/url"
	"slices"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/community"
	"github.com/rechedev9/cliphub/internal/library"
	"github.com/rechedev9/cliphub/internal/tacticalplan"
)

// proDefaultDays is the period of a pro board that names none: three months.
const proDefaultDays = 90

// leaderboardFilters narrow the matches and the rounds a leaderboard reads.
type leaderboardFilters struct {
	side    tacticalplan.Side
	role    string
	mapRole string
	// from, to and source narrow the library's matches.
	from   *time.Time
	to     *time.Time
	source library.Source
	// days, top, teamTop and event narrow the pro maps: the period, the HLTV rank
	// of the rival and of the player's own team, and the event.
	days, top, teamTop int
	event              string
}

// libraryOnly reports a filter only the library scope reads.
func (f leaderboardFilters) libraryOnly() bool {
	return f.from != nil || f.to != nil || f.source != ""
}

// proOnly reports a filter only the pro scope reads.
func (f leaderboardFilters) proOnly() bool {
	return f.days > 0 || f.top > 0 || f.teamTop > 0 || f.event != ""
}

// proServer reports whether a pro request names something only the community
// server's per-player lines answer.
func (f leaderboardFilters) proServer() bool {
	return f.side != "" || f.role != "" || f.mapRole != "" || f.proOnly()
}

// parseLeaderboardFilters reads the filters of both scopes. A role or a map role
// sits on one side of one map, so it needs a map and sets or needs the side.
func parseLeaderboardFilters(q url.Values, base baselineQuery) (leaderboardFilters, error) {
	var f leaderboardFilters
	var err error
	if f.side, err = parseScoutSide(q.Get("side")); err != nil {
		return f, err
	}
	if f.from, err = optionalTimeQuery(q, "from"); err != nil {
		return f, err
	}
	if f.to, err = optionalTimeQuery(q, "to"); err != nil {
		return f, err
	}
	if raw := q.Get("source"); raw != "" {
		f.source = library.Source(raw)
		if !slices.Contains([]library.Source{library.SourceValve, library.SourceFaceit, library.SourceHLTV, library.SourceScrim}, f.source) {
			return f, errors.New("source must be valve, faceit, hltv or scrim")
		}
	}
	f.role, f.mapRole, f.event = q.Get("role"), q.Get("map_role"), q.Get("event")
	if f.side, err = tacticalplan.RoleFilterSide(tacticalplan.RoleFilter{Role: f.role, MapRole: f.mapRole, Side: f.side, HaveMap: base.mapName != ""}); err != nil {
		return f, err
	}
	for _, in := range []struct {
		name string
		into *int
		max  int
	}{{"days", &f.days, 365}, {"top", &f.top, 50}, {"team_top", &f.teamTop, 50}} {
		raw := q.Get(in.name)
		if raw == "" {
			continue
		}
		if in.name == "days" && raw == "all" {
			f.days = community.ProPlayersAllDates
			continue
		}
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 || n > in.max {
			return f, fmt.Errorf("%s must be an integer from 1 to %d", in.name, in.max)
		}
		*in.into = n
	}
	if base.scope == baselineScopePro && f.libraryOnly() {
		return f, errors.New("from, to and source need scope library; the pro scope takes days, top, team_top and event")
	}
	if base.scope == baselineScopeLibrary && f.proOnly() {
		return f, errors.New("days, top, team_top and event need scope pro; the library scope takes from, to and source")
	}
	return f, nil
}

// matchReadCache keeps each match read once, valid while the match is
// unchanged, so a new filter folds cached reads instead of reading documents.
type matchReadCache struct {
	mu      sync.Mutex
	entries map[uuid.UUID]matchReadEntry
}

type matchReadEntry struct {
	updatedAt time.Time
	read      tacticalplan.MatchRead
}

func (c *matchReadCache) get(id uuid.UUID) (matchReadEntry, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.entries[id]
	return e, ok
}

func (c *matchReadCache) put(id uuid.UUID, e matchReadEntry) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.entries == nil {
		c.entries = map[uuid.UUID]matchReadEntry{}
	}
	c.entries[id] = e
}

// leaderboardRead is the query and the filters a leaderboard sample folds the library by.
type leaderboardRead struct {
	query   baselineQuery
	filters leaderboardFilters
}

// leaderboardSample folds the library's matches that pass the filters, newest
// first, and returns how many it could not read. Roles are read only on a map.
// Matches are read without holding the cache lock, so a slow first read does
// not stall other requests; two requests may read the same match once each.
func (h *Handlers) leaderboardSample(ctx context.Context, in leaderboardRead) (*tacticalplan.LeaderboardSample, int, error) {
	q, f := in.query, in.filters
	matches, err := h.allLibraryMatches(ctx, library.MatchFilter{Map: q.mapName, From: f.from, To: f.to, Source: f.source})
	if err != nil {
		return nil, 0, err
	}
	sort.SliceStable(matches, func(i, j int) bool { return matches[i].PlayedAt.After(matches[j].PlayedAt) })
	withRoles := q.mapName != ""
	c := &h.references.matchReads
	sample := &tacticalplan.LeaderboardSample{}
	skipped := 0
	for _, m := range matches {
		if err := ctx.Err(); err != nil {
			return nil, 0, err
		}
		e, ok := c.get(m.ID)
		if !ok || !e.updatedAt.Equal(m.UpdatedAt) || (withRoles && !e.read.HasRoles) {
			doc, err := h.readLibraryDocument(m.DemoID)
			if err != nil {
				log.Printf("httpapi: leaderboard skips match %s: %v", m.ID, err)
				skipped++
				continue
			}
			opts := tacticalplan.MatchReadOptions{Roles: withRoles}
			if withRoles {
				opts.Opening = h.readOpeningFrames(m.DemoID, doc)
			}
			e = matchReadEntry{updatedAt: m.UpdatedAt, read: tacticalplan.ReadMatch(tacticalplan.MetricsMatch{MatchID: m.ID, PlayedAt: m.PlayedAt, Document: doc}, opts)}
			c.put(m.ID, e)
		}
		sample.Add(e.read, f.side)
	}
	return sample, skipped, nil
}
