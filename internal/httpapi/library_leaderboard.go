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
	// libraryRole is a library role (awp, entry, support, rifler), read on any map.
	libraryRole string
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

// libraryRoles are the roles read over every map, from the library role rule.
var libraryRoles = []string{library.RoleAWP, library.RoleEntry, library.RoleSupport, library.RoleRifler}

// parseLeaderboardFilters reads the filters of both scopes. A role or a map role
// sits on one side of one map, so it needs a map and sets or needs the side. A
// library role and Todoterreno (map_role allround) need no map, only the library scope.
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
	if slices.Contains(libraryRoles, f.role) {
		f.libraryRole, f.role = f.role, ""
	}
	if f.role != "" && tacticalplan.RoleSide(f.role) == "" {
		return f, fmt.Errorf("role must be awp, entry, support, rifler (any map) or a_anchor, b_anchor, rotator, a_lurk, b_lurk, half_lurk, pack (one map), not %q", f.role)
	}
	anyMapAllround := base.mapName == "" && f.mapRole == tacticalplan.MapRoleAllround
	if base.scope == baselineScopePro && (f.libraryRole != "" || anyMapAllround) {
		return f, errors.New("the pro scope reads roles on one map: awp, entry, support, rifler and allround without a map need scope library")
	}
	haveMap := base.mapName != "" || anyMapAllround
	if f.side, err = tacticalplan.RoleFilterSide(tacticalplan.RoleFilter{Role: f.role, MapRole: f.mapRole, Side: f.side, HaveMap: haveMap}); err != nil {
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

// matchReadEntry is one match's read and each player's library role read on it alone.
type matchReadEntry struct {
	updatedAt time.Time
	read      tacticalplan.MatchRead
	roles     map[string]library.PlayerRole
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

// leaderboardFold is a folded leaderboard: the sample, each player's library
// role over the folded matches and how many matches could not be read.
type leaderboardFold struct {
	sample       *tacticalplan.LeaderboardSample
	libraryRoles map[string]string
	skipped      int
}

// matchReadInput is a library match to read; doc, when set, is its document already in memory.
type matchReadInput struct {
	match     library.Match
	doc       *tacticalplan.Document
	withRoles bool
}

// cachedMatchRead returns the match's read, from the cache while the match is
// unchanged. It reads without holding the cache lock, so a slow first read
// does not stall other requests; two requests may read the same match once each.
func (h *Handlers) cachedMatchRead(in matchReadInput) (matchReadEntry, error) {
	m, c := in.match, &h.references.matchReads
	if e, ok := c.get(m.ID); ok && e.updatedAt.Equal(m.UpdatedAt) && (!in.withRoles || e.read.HasRoles) {
		return e, nil
	}
	var doc tacticalplan.Document
	if in.doc != nil {
		doc = *in.doc
	} else {
		read, err := h.readLibraryDocument(m.DemoID)
		if err != nil {
			return matchReadEntry{}, err
		}
		doc = read
	}
	opts := tacticalplan.MatchReadOptions{Roles: in.withRoles}
	if in.withRoles {
		opts.Opening = h.readOpeningFrames(m.DemoID, doc)
	}
	e := matchReadEntry{
		updatedAt: m.UpdatedAt, roles: matchLibraryRoles(m.ID, doc),
		read: tacticalplan.ReadMatch(tacticalplan.MetricsMatch{MatchID: m.ID, PlayedAt: m.PlayedAt, Document: doc}, opts),
	}
	c.put(m.ID, e)
	return e, nil
}

// matchLibraryRoles reads every player's library role on one match alone, to fold across matches.
func matchLibraryRoles(id uuid.UUID, doc tacticalplan.Document) map[string]library.PlayerRole {
	docs := []library.RoleDocument{{MatchID: id, Document: doc}}
	out := map[string]library.PlayerRole{}
	for _, p := range doc.Players {
		if p.SteamID64 != "" {
			out[p.SteamID64] = library.InferPlayerRole(p.SteamID64, docs)
		}
	}
	return out
}

// leaderboardSample folds the library's matches that pass the filters, newest
// first. Roles by side are read on a map, or for Todoterreno over every map.
func (h *Handlers) leaderboardSample(ctx context.Context, in leaderboardRead) (leaderboardFold, error) {
	q, f := in.query, in.filters
	matches, err := h.allLibraryMatches(ctx, library.MatchFilter{Map: q.mapName, From: f.from, To: f.to, Source: f.source})
	if err != nil {
		return leaderboardFold{}, err
	}
	return h.foldLeaderboard(ctx, foldInput{matches: matches, side: f.side, withRoles: q.mapName != "" || f.mapRole == tacticalplan.MapRoleAllround})
}

// foldInput is the matches a board folds, the side it counts and whether it reads roles by side.
type foldInput struct {
	matches   []library.Match
	side      tacticalplan.Side
	withRoles bool
}

// foldLeaderboard folds the matches newest first, from the cached reads.
func (h *Handlers) foldLeaderboard(ctx context.Context, in foldInput) (leaderboardFold, error) {
	matches := in.matches
	sort.SliceStable(matches, func(i, j int) bool { return matches[i].PlayedAt.After(matches[j].PlayedAt) })
	withRoles := in.withRoles
	out := leaderboardFold{sample: &tacticalplan.LeaderboardSample{}, libraryRoles: map[string]string{}}
	parts := map[string][]library.PlayerRole{}
	for _, m := range matches {
		if err := ctx.Err(); err != nil {
			return leaderboardFold{}, err
		}
		e, err := h.cachedMatchRead(matchReadInput{match: m, withRoles: withRoles})
		if err != nil {
			log.Printf("httpapi: leaderboard skips match %s: %v", m.ID, err)
			out.skipped++
			continue
		}
		out.sample.Add(e.read, in.side)
		for steam, role := range e.roles {
			parts[steam] = append(parts[steam], role)
		}
	}
	for steam, p := range parts {
		if role := library.FoldPlayerRoles(steam, p).Role; role != library.RoleUnknown {
			out.libraryRoles[steam] = role
		}
	}
	return out, nil
}
