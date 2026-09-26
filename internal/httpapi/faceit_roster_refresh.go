package httpapi

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/rechedev9/cliphub/internal/faceit"
)

const (
	// faceitRosterMaxAge is how stale the Players rail may get before a list
	// read refreshes it. FACEIT ELO moves every match, but a refresh reads
	// every zone ladder (about 160 requests), so it runs at most this often
	// and only while someone has the section open.
	faceitRosterMaxAge         = 15 * time.Minute
	faceitRosterRefreshTimeout = 3 * time.Minute
)

// faceitRosterRefresh single-flights the background refresh of the seeded
// zone rosters and the followed players' profiles. Before it existed the
// CIS/LATAM ELOs were whatever zones_default.json shipped with and followed
// players kept the ELO they had on the day they were followed.
type faceitRosterRefresh struct {
	mu        sync.Mutex
	running   bool
	attempted time.Time
	wg        sync.WaitGroup
}

// refreshFaceitRosterIfStale starts a background refresh when the roster
// generated at generatedAt is older than faceitRosterMaxAge, and reports
// whether one is running. A failed attempt waits the same interval before
// the next one, so a FACEIT outage is not retried on every list poll.
func (h *Handlers) refreshFaceitRosterIfStale(generatedAt time.Time) bool {
	if h.faceit == nil || h.faceitSeeds == nil || h.faceitFollows == nil {
		return false
	}
	r := &h.faceitRoster
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.running {
		return true
	}
	now := h.faceitCache.clock()
	last := generatedAt
	if r.attempted.After(last) {
		last = r.attempted
	}
	if now.Sub(last) < faceitRosterMaxAge {
		return false
	}
	r.running = true
	r.attempted = now
	r.wg.Add(1)
	go func() {
		defer r.wg.Done()
		h.refreshFaceitRoster()
		r.mu.Lock()
		r.running = false
		r.mu.Unlock()
	}()
	return true
}

func (h *Handlers) refreshFaceitRoster() {
	ctx, cancel := context.WithTimeout(context.Background(), faceitRosterRefreshTimeout)
	defer cancel()
	if _, err := h.faceitSeeds.Refresh(ctx, h.faceit, faceit.SeedZoneLimit); err != nil {
		log.Printf("httpapi: refresh FACEIT zone rosters: %v", err)
	}
	followed, err := h.faceitFollows.List()
	if err != nil {
		log.Printf("httpapi: refresh followed FACEIT players: %v", err)
		return
	}
	fresh := make([]faceit.Player, 0, len(followed))
	for _, stored := range followed {
		player, err := h.faceit.LookupPlayerByID(ctx, stored.ID)
		if err != nil {
			log.Printf("httpapi: refresh followed FACEIT player %s: %v", stored.ID, err)
			continue
		}
		// Following resolved a default FACEIT avatar to the Steam one
		// (enrichAvatar); keep it instead of one Steam request per player.
		if faceit.IsDefaultFaceitAvatar(player.Avatar) && stored.Avatar != "" {
			player.Avatar = stored.Avatar
		}
		fresh = append(fresh, player)
	}
	if err := h.faceitFollows.Refresh(fresh); err != nil {
		log.Printf("httpapi: refresh followed FACEIT players: %v", err)
	}
}
