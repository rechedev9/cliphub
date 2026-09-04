package faceit

import (
	"context"
	"errors"
	"fmt"
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

// OverlayPlayers looks up each SteamID on the Data API. It returns partial
// results with a joined error when any profile or recent-match lookup fails.
// The caller owns timeouts.
func (c *Client) OverlayPlayers(ctx context.Context, steamIDs []string) (map[string]OverlayPlayer, error) {
	out := map[string]OverlayPlayer{}
	if c == nil {
		return out, ErrNotConfigured
	}
	ids := uniqueNonEmpty(steamIDs)
	if len(ids) == 0 {
		return out, nil
	}
	var mu sync.Mutex
	var wg sync.WaitGroup
	failures := make(map[string]error)
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
				mu.Lock()
				failures[steamID] = ctx.Err()
				mu.Unlock()
				return
			}
			player, err := c.overlayPlayer(ctx, steamID)
			if err != nil {
				mu.Lock()
				failures[steamID] = err
				mu.Unlock()
				return
			}
			mu.Lock()
			out[steamID] = player
			mu.Unlock()
		}()
	}
	wg.Wait()
	joined := make([]error, 0, len(failures))
	for _, steamID := range ids {
		if err := failures[steamID]; err != nil {
			joined = append(joined, fmt.Errorf("FACEIT overlay player %s: %w", steamID, err))
		}
	}
	return out, errors.Join(joined...)
}

func (c *Client) overlayPlayer(ctx context.Context, steamID string) (OverlayPlayer, error) {
	player, err := c.LookupBySteamID(ctx, steamID)
	if err != nil {
		return OverlayPlayer{}, err
	}
	out := OverlayPlayer{
		Nickname:   player.Nickname,
		Country:    player.Country,
		Avatar:     player.Avatar,
		ELO:        player.ELO,
		SkillLevel: player.SkillLevel,
	}
	if player.ID == "" {
		return out, nil
	}
	// The lookup above produced everything both calls need: recent matches and
	// the regional ranking depend on player.ID (and Region), not on each other.
	// Each goroutine writes only its own pair of variables and is joined below.
	var (
		wg         sync.WaitGroup
		matches    []RecentMatch
		matchesErr error
		position   int
		rankingErr error
	)
	wg.Add(2)
	go func() {
		defer wg.Done()
		matches, matchesErr = c.RecentMatches(ctx, player.ID, maxRecentMatchLimit)
	}()
	go func() {
		defer wg.Done()
		position, rankingErr = c.RankingPosition(ctx, player.Region, player.ID)
	}()
	wg.Wait()
	if matchesErr != nil {
		return OverlayPlayer{}, fmt.Errorf("recent matches: %w", matchesErr)
	}
	out.Recent = AggregateLast20(matches)
	// Unchanged: a ranking failure is swallowed and the card ships without a
	// rank rather than failing the whole roster.
	if rankingErr == nil && position > 0 {
		out.Ranking = &position
	}
	return out, nil
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
