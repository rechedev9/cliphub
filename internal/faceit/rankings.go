package faceit

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
)

const (
	defaultRankingLimit = 10
	maxRankingLimit     = 100
	// rankingWorkers matches the detail fan-out elsewhere in this package: the
	// Data API is rate limited per key, so widening it only earns 429s.
	rankingWorkers = 4
)

// rankingRegions is the FACEIT CS2 leaderboard region allowlist. FACEIT
// publishes one leaderboard per region and nothing global, so these five are
// the whole population; a value outside the list is a caller mistake, not a
// query that happens to return nothing.
var rankingRegions = []string{"EU", "NA", "SA", "OCE", "SEA"}

type apiRankingList struct {
	Items []apiRankedPlayer `json:"items"`
	Start int               `json:"start"`
	End   int               `json:"end"`
}

type apiRankedPlayer struct {
	PlayerID  string `json:"player_id"`
	Nickname  string `json:"nickname"`
	Country   string `json:"country"`
	Position  int    `json:"position"`
	FaceitELO int    `json:"faceit_elo"`
	// FACEIT has shipped the level under both names on this endpoint; accept
	// either rather than rendering an unranked badge for the top of the world.
	GameSkillLevel int `json:"game_skill_level"`
	SkillLevel     int `json:"skill_level"`
}

func clampRankingLimit(limit int) int {
	if limit <= 0 {
		return defaultRankingLimit
	}
	if limit > maxRankingLimit {
		return maxRankingLimit
	}
	return limit
}

// rankedSkillLevel prefers the documented level field and falls back to the
// spelling the endpoint has also shipped.
func rankedSkillLevel(item apiRankedPlayer) int {
	if item.GameSkillLevel > 0 {
		return item.GameSkillLevel
	}
	if item.SkillLevel > 0 {
		return item.SkillLevel
	}
	return 0
}

// canonicalRankingRegion returns the allowlisted spelling of region, or false
// when region is not a FACEIT CS2 leaderboard region.
func canonicalRankingRegion(region string) (string, bool) {
	region = strings.ToUpper(strings.TrimSpace(region))
	for _, allowed := range rankingRegions {
		if region == allowed {
			return allowed, true
		}
	}
	return "", false
}

// Rankings returns one page of a FACEIT CS2 regional leaderboard, highest ELO
// first, optionally narrowed to a country code. Region must be one of
// rankingRegions.
//
// A row that fails validation fails the whole page: a leaderboard is seeded
// straight into the Players section, and half a page of players with unusable
// ids is worse than an error the caller can retry.
func (c *Client) Rankings(ctx context.Context, region, country string, offset, limit int) ([]RankedPlayer, error) {
	if c == nil || c.apiKey == "" {
		return nil, ErrNotConfigured
	}
	canonical, ok := canonicalRankingRegion(region)
	if !ok {
		return nil, fmt.Errorf("FACEIT ranking region %q is not one of %v", region, rankingRegions)
	}
	country = strings.TrimSpace(country)
	if country != "" && !validIdentifier(country) {
		return nil, fmt.Errorf("FACEIT ranking country is invalid")
	}
	if offset < 0 {
		offset = 0
	}
	limit = clampRankingLimit(limit)

	query := url.Values{
		"offset": {strconv.Itoa(offset)},
		"limit":  {strconv.Itoa(limit)},
	}
	if country != "" {
		query.Set("country", country)
	}
	var raw apiRankingList
	endpoint := "/rankings/games/cs2/regions/" + url.PathEscape(canonical)
	if err := c.getJSON(ctx, endpoint, query, &raw); err != nil {
		return nil, fmt.Errorf("list FACEIT %s rankings: %w", canonical, err)
	}

	players := make([]RankedPlayer, 0, len(raw.Items))
	for _, item := range raw.Items {
		player := RankedPlayer{
			PlayerID:   strings.TrimSpace(item.PlayerID),
			Nickname:   strings.TrimSpace(item.Nickname),
			Country:    strings.TrimSpace(item.Country),
			Region:     canonical,
			Position:   item.Position,
			ELO:        item.FaceitELO,
			SkillLevel: rankedSkillLevel(item),
		}
		if !ValidPlayerID(player.PlayerID) || player.Nickname == "" {
			return nil, ErrInvalidResponse
		}
		players = append(players, player)
	}
	if responseContainsCredential(players, c.apiKey) {
		return nil, ErrInvalidResponse
	}
	return players, nil
}

// GlobalTop returns the highest-ELO CS2 players FACEIT knows about, across
// every allowlisted region.
//
// FACEIT publishes no global leaderboard, so this queries each region's own top
// `limit` and merges them. That is exact, not an approximation: a regional
// leaderboard is the global one filtered down to that region, and removing
// other players can only move a player up, so anyone inside the true global
// top N is necessarily inside their own region's top N. Reading N rows per
// region therefore cannot miss a global top-N player. (Measured 2026-09-04:
// EU's 10th at 4107 elo outranks every other region's 1st, so the current
// global top 10 is entirely EU. The merge still has to run, because which
// region leads is data, not a constant.)
func (c *Client) GlobalTop(ctx context.Context, limit int) ([]RankedPlayer, error) {
	players, _, err := c.globalTop(ctx, limit)
	return players, err
}

// globalTop also reports which regions actually answered. A partial outage
// still produces a usable roster, and the covered list is how that gets
// recorded instead of being silently dropped.
func (c *Client) globalTop(ctx context.Context, limit int) ([]RankedPlayer, []string, error) {
	if c == nil || c.apiKey == "" {
		return nil, nil, ErrNotConfigured
	}
	limit = clampRankingLimit(limit)

	var mu sync.Mutex
	var wg sync.WaitGroup
	merged := make([]RankedPlayer, 0, limit*len(rankingRegions))
	covered := make(map[string]bool, len(rankingRegions))
	failures := make(map[string]error, len(rankingRegions))
	gate := make(chan struct{}, rankingWorkers)
	for _, region := range rankingRegions {
		wg.Add(1)
		go func() {
			defer wg.Done()
			select {
			case gate <- struct{}{}:
				defer func() { <-gate }()
			case <-ctx.Done():
				mu.Lock()
				failures[region] = ctx.Err()
				mu.Unlock()
				return
			}
			players, err := c.Rankings(ctx, region, "", 0, limit)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				failures[region] = err
				return
			}
			covered[region] = true
			merged = append(merged, players...)
		}()
	}
	wg.Wait()

	if len(failures) == len(rankingRegions) {
		joined := make([]error, 0, len(failures))
		for _, region := range rankingRegions {
			if err := failures[region]; err != nil {
				joined = append(joined, fmt.Errorf("region %s: %w", region, err))
			}
		}
		return nil, nil, fmt.Errorf("list FACEIT global rankings: %w", errors.Join(joined...))
	}

	merged = dedupeRankedPlayers(merged)
	// Regions are fetched concurrently, so the merge order is nondeterministic
	// until it is sorted. ELO is the ranking; the player id breaks ties so two
	// players on the same ELO keep a stable order across refreshes.
	sort.Slice(merged, func(i, j int) bool {
		if merged[i].ELO != merged[j].ELO {
			return merged[i].ELO > merged[j].ELO
		}
		return merged[i].PlayerID < merged[j].PlayerID
	})
	if len(merged) > limit {
		merged = merged[:limit]
	}

	regions := make([]string, 0, len(covered))
	for _, region := range rankingRegions {
		if covered[region] {
			regions = append(regions, region)
		}
	}
	return merged, regions, nil
}

// dedupeRankedPlayers keeps the highest-ELO row per player id. A player should
// appear on exactly one regional leaderboard, but a duplicate would otherwise
// take two slots in the Players section.
func dedupeRankedPlayers(players []RankedPlayer) []RankedPlayer {
	best := make(map[string]int, len(players))
	out := make([]RankedPlayer, 0, len(players))
	for _, player := range players {
		index, seen := best[player.PlayerID]
		if !seen {
			best[player.PlayerID] = len(out)
			out = append(out, player)
			continue
		}
		if player.ELO > out[index].ELO {
			out[index] = player
		}
	}
	return out
}

// responseContainsCredential reports whether decoded upstream data would carry
// the API key into ClipHub state. Leaderboard rows are user-controlled text,
// and index.go applies the same guard to players and recent matches.
func responseContainsCredential(value any, credential string) bool {
	if credential == "" {
		return false
	}
	body, err := json.Marshal(value)
	return err != nil || bytes.Contains(body, []byte(credential))
}
