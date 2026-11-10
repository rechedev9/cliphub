package httpapi

import (
	"context"
	"errors"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/rechedev9/cliphub/internal/library"
	"github.com/rechedev9/cliphub/internal/tacticalplan"
)

// referenceCaches hold the library-wide references: player baselines, team
// norms, and the community server's pro reference and rankings.
type referenceCaches struct {
	baselines  baselineCache
	matchReads matchReadCache
	norms      normsCache
	patterns   patternCache
	community  CommunityDeps
	pro        proMatchState
}

// normsCache keeps one pass per map and date range, valid while the matches
// it read, with their update times, are unchanged.
type normsCache struct {
	mu      sync.Mutex
	entries map[string]normsEntry
}

type normsEntry struct {
	version uint64
	norms   tacticalplan.Norms
	skipped int
}

type normsResponse struct {
	Map     string     `json:"map"`
	From    *time.Time `json:"from,omitempty"`
	To      *time.Time `json:"to,omitempty"`
	Skipped int        `json:"skipped"`
	tacticalplan.Norms
}

type normsQuery struct {
	mapName  string
	from, to *time.Time
}

func (q normsQuery) key() string {
	stamp := func(t *time.Time) string {
		if t == nil {
			return ""
		}
		return t.UTC().Format(time.RFC3339Nano)
	}
	return q.mapName + "|" + stamp(q.from) + "|" + stamp(q.to)
}

// GetLibraryNorms handles GET /api/library/norms: the library's team rates
// (bombsites, buy matchups and, with a map, playbook categories) pooled over
// every team of every match in the map and dates.
func (h *Handlers) GetLibraryNorms(w http.ResponseWriter, r *http.Request) {
	if !h.libraryReady(w) {
		return
	}
	values := r.URL.Query()
	q := normsQuery{mapName: values.Get("map")}
	var err error
	if q.mapName != "" && !mapNamePattern.MatchString(q.mapName) {
		err = errors.New("map must match ^[a-z0-9_]+$")
	}
	if err == nil {
		q.from, err = optionalTimeQuery(values, "from")
	}
	if err == nil {
		q.to, err = optionalTimeQuery(values, "to")
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	entry, err := h.libraryNorms(r.Context(), q)
	if err != nil {
		internalError(w, "norms", err)
		return
	}
	writeJSON(w, http.StatusOK, normsResponse{Map: q.mapName, From: q.from, To: q.to, Skipped: entry.skipped, Norms: entry.norms})
}

// libraryNorms returns the cached pass, reading every match again only when
// the matches it covers changed. Playbook categories need a map's positions.
func (h *Handlers) libraryNorms(ctx context.Context, q normsQuery) (normsEntry, error) {
	matches, err := h.allLibraryMatches(ctx, library.MatchFilter{Map: q.mapName, From: q.from, To: q.to})
	if err != nil {
		return normsEntry{}, err
	}
	version := matchesVersion(matches)
	c := &h.references.norms
	c.mu.Lock()
	defer c.mu.Unlock()
	if e, ok := c.entries[q.key()]; ok && e.version == version {
		return e, nil
	}
	b := tacticalplan.NewNormsBuilder(q.mapName != "")
	entry := normsEntry{version: version}
	for _, m := range matches {
		if err := ctx.Err(); err != nil {
			return normsEntry{}, err
		}
		doc, err := h.readLibraryDocument(m.DemoID)
		if err != nil {
			log.Printf("httpapi: norms skip match %s: %v", m.ID, err)
			entry.skipped++
			continue
		}
		b.Add(tacticalplan.FramedMatch{
			ScoutMatch: tacticalplan.ScoutMatch{MatchID: m.ID, PlayedAt: m.PlayedAt, Document: doc},
			Blob:       h.positionsReader(m.DemoID),
		})
	}
	entry.norms = b.Norms()
	if c.entries == nil {
		c.entries = map[string]normsEntry{}
	}
	c.entries[q.key()] = entry
	return entry, nil
}
