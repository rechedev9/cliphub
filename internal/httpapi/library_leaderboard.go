package httpapi

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/url"
	"slices"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/library"
	"github.com/rechedev9/cliphub/internal/tacticalplan"
)

// leaderboardRoles are the role keys the leaderboard filters on.
var leaderboardRoles = []string{
	tacticalplan.PositionAnchorA, tacticalplan.PositionAnchorB, tacticalplan.PositionRotator,
	tacticalplan.PositionLurkA, tacticalplan.PositionLurkB, tacticalplan.PositionHalfLurk, tacticalplan.PositionPack,
}

// leaderboardFilters narrow the matches and the rounds a library leaderboard reads.
type leaderboardFilters struct {
	side   tacticalplan.Side
	role   string
	from   *time.Time
	to     *time.Time
	source library.Source
}

func (f leaderboardFilters) any() bool {
	return f.side != "" || f.role != "" || f.from != nil || f.to != nil || f.source != ""
}

// parseLeaderboardFilters reads side, role, from, to and source. A role sits on
// one side of one map, so it needs a map and sets the side itself.
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
	f.role = q.Get("role")
	if f.role != "" {
		if !slices.Contains(leaderboardRoles, f.role) {
			return f, fmt.Errorf("role must be one of %v", leaderboardRoles)
		}
		if base.mapName == "" {
			return f, errors.New("role needs a map: a role is read on one map")
		}
		roleSide := tacticalplan.RoleSide(f.role)
		if f.side != "" && f.side != roleSide {
			return f, fmt.Errorf("role %s is played on %s, not %s", f.role, roleSide, f.side)
		}
		f.side = roleSide
	}
	if base.scope != baselineScopeLibrary && f.any() {
		return f, errors.New("side, role, from, to and source need scope library")
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

// leaderboardSample folds the library's matches that pass the filters, newest
// first, and returns how many it could not read. Roles are read only on a map.
func (h *Handlers) leaderboardSample(ctx context.Context, q baselineQuery, f leaderboardFilters) (*tacticalplan.LeaderboardSample, int, error) {
	matches, err := h.allLibraryMatches(ctx, library.MatchFilter{Map: q.mapName, From: f.from, To: f.to, Source: f.source})
	if err != nil {
		return nil, 0, err
	}
	sort.SliceStable(matches, func(i, j int) bool { return matches[i].PlayedAt.After(matches[j].PlayedAt) })
	withRoles := q.mapName != ""
	c := &h.references.matchReads
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.entries == nil {
		c.entries = map[uuid.UUID]matchReadEntry{}
	}
	sample := &tacticalplan.LeaderboardSample{}
	skipped := 0
	for _, m := range matches {
		if err := ctx.Err(); err != nil {
			return nil, 0, err
		}
		e, ok := c.entries[m.ID]
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
			c.entries[m.ID] = e
		}
		sample.Add(e.read, f.side)
	}
	return sample, skipped, nil
}
