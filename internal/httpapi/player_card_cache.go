package httpapi

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/playercard"
)

const (
	// playerCardTTL keeps a job's player while its card panel draws; FACEIT figures can move later.
	playerCardTTL = 5 * time.Minute
	// maxPlayerCards bounds the players kept, each with its image.
	maxPlayerCards = 8
)

type playerCardKey struct {
	job     uuid.UUID
	steamID string
}

// playerCardGather is one gather of a player; done closes once facts and err are set.
type playerCardGather struct {
	done  chan struct{}
	at    time.Time
	facts playercard.Facts
	err   error
}

// playerCardCache gathers a job's player once for the card info and the previews its panel asks for
// at once. The zero value is ready.
type playerCardCache struct {
	mu      sync.Mutex
	gathers map[playerCardKey]*playerCardGather
}

func (c *playerCardCache) facts(ctx context.Context, key playerCardKey, gather func(context.Context) (playercard.Facts, error)) (playercard.Facts, error) {
	now := time.Now()
	c.mu.Lock()
	g, ok := c.gathers[key]
	if ok && now.Sub(g.at) <= playerCardTTL {
		c.mu.Unlock()
		select {
		case <-g.done:
			return g.facts, g.err
		case <-ctx.Done():
			return playercard.Facts{}, ctx.Err()
		}
	}
	if c.gathers == nil || len(c.gathers) >= maxPlayerCards {
		// A gather still running keeps its waiters through its own pointer.
		c.gathers = map[playerCardKey]*playerCardGather{}
	}
	g = &playerCardGather{done: make(chan struct{}), at: now}
	c.gathers[key] = g
	c.mu.Unlock()
	g.facts, g.err = gather(ctx)
	close(g.done)
	if g.err != nil {
		c.mu.Lock()
		if c.gathers[key] == g {
			delete(c.gathers, key)
		}
		c.mu.Unlock()
	}
	return g.facts, g.err
}

// forget drops a player's gathers, whose image just changed.
func (c *playerCardCache) forget(steamID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for key := range c.gathers {
		if key.steamID == steamID {
			delete(c.gathers, key)
		}
	}
}
