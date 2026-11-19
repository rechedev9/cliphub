package httpapi

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/rechedev9/cliphub/internal/faceit"
)

// faceitRosterRefreshTimeout bounds the startup refresh of the followed profiles.
const faceitRosterRefreshTimeout = 3 * time.Minute

// faceitRosterRefresh tracks the one background refresh of the followed
// players' profiles that runs when Studio starts. Without it a followed player
// kept the ELO they had on the day they were followed.
type faceitRosterRefresh struct {
	mu      sync.Mutex
	running bool
	wg      sync.WaitGroup
}

// StartFaceitRosterRefresh rereads every followed profile in the background,
// one request each. The orchestrator calls it once at startup and does not
// wait for it; a list read never reaches FACEIT, it only reports whether this
// refresh is running so the page can pick up its result.
func (h *Handlers) StartFaceitRosterRefresh() {
	if h.faceit == nil || h.faceitFollows == nil {
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
	followed, err := h.faceitFollows.List()
	if err != nil {
		log.Printf("httpapi: refresh followed FACEIT players: %v", err)
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
