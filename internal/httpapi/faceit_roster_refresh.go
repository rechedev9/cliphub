package httpapi

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/rechedev9/cliphub/internal/faceit"
)

// faceitRosterRefreshTimeout bounds each refresh phase (zone ladders, then
// followed profiles).
const faceitRosterRefreshTimeout = 3 * time.Minute

// faceitZoneRosterMaxAge is how long a stored zone roster is reused. Its
// refresh costs about 160 FACEIT requests, too many for every Studio start.
const faceitZoneRosterMaxAge = 6 * time.Hour

// faceitRosterRefresh tracks the one background refresh of the seeded zone
// rosters and the followed players' profiles that runs when Studio starts.
// Before it existed the CIS/LATAM ELOs were whatever zones_default.json
// shipped with and followed players kept the ELO they had on the day they
// were followed.
type faceitRosterRefresh struct {
	mu      sync.Mutex
	running bool
	wg      sync.WaitGroup
}

// StartFaceitRosterRefresh refreshes the Players ELOs in the background: the
// followed profiles, and the zone rosters, whose ladders are rebuilt at most
// every faceitZoneRosterMaxAge but whose ELOs are reread on every start. The
// orchestrator calls it once at startup, which is the only refresh: a list
// read never reaches FACEIT, it only reports whether this one is running so
// the page can pick up its result.
func (h *Handlers) StartFaceitRosterRefresh() {
	if h.faceit == nil || h.faceitSeeds == nil || h.faceitFollows == nil {
		return
	}
	r := &h.faceitRoster
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.running {
		return
	}
	r.running = true
	r.wg.Add(1)
	go func() {
		defer r.wg.Done()
		h.refreshFaceitRoster()
		r.mu.Lock()
		r.running = false
		r.mu.Unlock()
	}()
}

func (h *Handlers) faceitRosterRefreshing() bool {
	h.faceitRoster.mu.Lock()
	defer h.faceitRoster.mu.Unlock()
	return h.faceitRoster.running
}

func (h *Handlers) refreshFaceitRoster() {
	followed, listErr := h.faceitFollows.List()
	// Each phase gets its own deadline: a zone refresh slowed down by 429
	// backoff must not leave the followed profiles with an expired context.
	seedCtx, cancelSeed := context.WithTimeout(context.Background(), faceitRosterRefreshTimeout)
	_, rebuilt, err := h.faceitSeeds.RefreshIfStale(seedCtx, h.faceit, faceit.SeedZoneLimit, faceitZoneRosterMaxAge)
	if err != nil {
		log.Printf("httpapi: refresh FACEIT zone rosters: %v", err)
	} else if !rebuilt {
		// Reused ladders still get this session's ELOs. A followed player shows
		// their own row instead, refreshed below.
		skip := make(map[string]bool, len(followed))
		for _, stored := range followed {
			skip[stored.ID] = true
		}
		if err := h.faceitSeeds.RefreshProfiles(seedCtx, h.faceit, skip); err != nil {
			log.Printf("httpapi: refresh FACEIT zone roster ELOs: %v", err)
		}
	}
	cancelSeed()
	if listErr != nil {
		log.Printf("httpapi: refresh followed FACEIT players: %v", listErr)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), faceitRosterRefreshTimeout)
	defer cancel()
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
