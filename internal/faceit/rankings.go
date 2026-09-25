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

// Zone ids name the curated rosters the Players section groups its seeded
// rows into.
const (
	ZoneCIS   = "cis"
	ZoneLATAM = "latam"
)

// rankingZones maps each zone to the FACEIT country codes it covers, in the
// order the Players section lists them. FACEIT has no zone leaderboard and its
// country field is whatever the player set on their profile, so a zone is only
// as accurate as those profiles.
var rankingZones = []struct {
	id        string
	countries []string
}{
	{id: ZoneCIS, countries: []string{"ru", "ua", "by", "kz", "uz", "kg", "tj", "tm", "am", "az", "ge", "md"}},
	{id: ZoneLATAM, countries: []string{"ar", "bo", "br", "cl", "co", "cr", "cu", "do", "ec", "gt", "hn", "mx", "ni", "pa", "pe", "pr", "py", "sv", "uy", "ve"}},
}

// Zones lists the zone ids in display order.
func Zones() []string {
	out := make([]string, 0, len(rankingZones))
	for _, zone := range rankingZones {
		out = append(out, zone.id)
	}
	return out
}

func zoneCountries(id string) ([]string, bool) {
	for _, zone := range rankingZones {
		if zone.id == id {
			return zone.countries, true
		}
	}
	return nil, false
}

// rankingLadder is one leaderboard read: a region, optionally narrowed to one
// country.
type rankingLadder struct {
	region  string
	country string
}

func (l rankingLadder) String() string {
	if l.country == "" {
		return l.region
	}
	return l.region + "/" + l.country
}

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

// ZoneTop returns the highest-ELO CS2 players whose FACEIT country belongs to
// zone. Players queue on any region regardless of country (measured
// 2026-09-25: the best Mexican played on EU, not NA), so every zone country is
// read on every region.
//
// Reading N rows per ladder is exact, not an approximation: a ladder is the
// zone ranking filtered down to one region and country, and removing other
// players can only move a player up, so anyone inside the zone top N is
// necessarily inside their own ladder's top N.
//
// A zone roster is all or nothing. A missing ladder would silently drop a
// country from it, so any failed read fails the call.
func (c *Client) ZoneTop(ctx context.Context, zone string, limit int) ([]RankedPlayer, error) {
	if c == nil || c.apiKey == "" {
		return nil, ErrNotConfigured
	}
	countries, ok := zoneCountries(zone)
	if !ok {
		return nil, fmt.Errorf("FACEIT zone %q is not one of %v", zone, Zones())
	}
	ladders := make([]rankingLadder, 0, len(countries)*len(rankingRegions))
	for _, country := range countries {
		for _, region := range rankingRegions {
			ladders = append(ladders, rankingLadder{region: region, country: country})
		}
	}
	players, failed := c.topAcross(ctx, ladders, limit)
	if failed != nil {
		return nil, fmt.Errorf("list FACEIT %s rankings: %w", zone, failed)
	}
	return players, nil
}

// topAcross reads the top `limit` of every ladder and merges them into one
// ELO ranking, with the joined per-ladder failures (nil when every ladder
// answered).
func (c *Client) topAcross(ctx context.Context, ladders []rankingLadder, limit int) ([]RankedPlayer, error) {
	limit = clampRankingLimit(limit)

	var mu sync.Mutex
	var wg sync.WaitGroup
	merged := make([]RankedPlayer, 0, limit*len(ladders))
	failures := make(map[rankingLadder]error, len(ladders))
	gate := make(chan struct{}, rankingWorkers)
	for _, ladder := range ladders {
		wg.Add(1)
		go func() {
			defer wg.Done()
			select {
			case gate <- struct{}{}:
				defer func() { <-gate }()
			case <-ctx.Done():
				mu.Lock()
				failures[ladder] = ctx.Err()
				mu.Unlock()
				return
			}
			players, err := c.Rankings(ctx, ladder.region, ladder.country, 0, limit)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				failures[ladder] = err
				return
			}
			merged = append(merged, players...)
		}()
	}
	wg.Wait()

	merged = dedupeRankedPlayers(merged)
	// Ladders are fetched concurrently, so the merge order is nondeterministic
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

	var failed []error
	for _, ladder := range ladders {
		if err := failures[ladder]; err != nil {
			failed = append(failed, fmt.Errorf("%s: %w", ladder, err))
		}
	}
	return merged, errors.Join(failed...)
}

// dedupeRankedPlayers keeps the highest-ELO row per player id. A player should
// appear on exactly one ladder, but a duplicate would otherwise take two slots
// in the Players section.
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
