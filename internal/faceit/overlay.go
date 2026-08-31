package faceit

import (
	"context"
	"sync"
)

// OverlayPlayer is FACEIT data for a Full Demo roster card. Rating and swing
// are omitted: the Data API match stats do not provide them.
type OverlayPlayer struct {
	Nickname   string
	Country    string
	Avatar     string
	ELO        int
	SkillLevel int
	Ranking    *int
	Recent     Last20
}

// OverlayPlayers looks up each SteamID on the Data API. Missing players are
// omitted. The caller owns timeouts.
func (c *Client) OverlayPlayers(ctx context.Context, steamIDs []string) map[string]OverlayPlayer {
	out := map[string]OverlayPlayer{}
	if c == nil || len(steamIDs) == 0 {
		return out
	}
	ids := uniqueNonEmpty(steamIDs)
	var mu sync.Mutex
	var wg sync.WaitGroup
	gate := make(chan struct{}, 4)
	for _, steamID := range ids {
		steamID := steamID
		wg.Add(1)
		go func() {
			defer wg.Done()
			select {
			case gate <- struct{}{}:
				defer func() { <-gate }()
			case <-ctx.Done():
				return
			}
			player, ok := c.overlayPlayer(ctx, steamID)
			if !ok {
				return
			}
			mu.Lock()
			out[steamID] = player
			mu.Unlock()
		}()
	}
	wg.Wait()
	return out
}

func (c *Client) overlayPlayer(ctx context.Context, steamID string) (OverlayPlayer, bool) {
	player, err := c.LookupBySteamID(ctx, steamID)
	if err != nil {
		return OverlayPlayer{}, false
	}
	out := OverlayPlayer{
		Nickname:   player.Nickname,
		Country:    player.Country,
		Avatar:     player.Avatar,
		ELO:        player.ELO,
		SkillLevel: player.SkillLevel,
	}
	if player.ID == "" {
		return out, true
	}
	if matches, err := c.RecentMatches(ctx, player.ID, maxRecentMatchLimit); err == nil {
		out.Recent = AggregateLast20(matches)
	}
	if pos, err := c.RankingPosition(ctx, player.Region, player.ID); err == nil && pos > 0 {
		out.Ranking = &pos
	}
	return out, true
}

func uniqueNonEmpty(ids []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	return out
}
