package faceit

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// Ranking entry kinds: what the Jugadores FACEIT selector can rank.
const (
	RankingCountry = "country"
	RankingRegion  = "region"
	RankingZone    = "zone"
)

// RankingSize is how many players one country, region or zone ranking holds.
const RankingSize = maxRankingLimit

// RankingEntry is a validated country, region or zone. Build it with
// ParseRankingEntry, which also canonicalises the code.
type RankingEntry struct {
	kind string
	code string
}

func (e RankingEntry) Kind() string { return e.kind }

func (e RankingEntry) Code() string { return e.code }

func (e RankingEntry) String() string { return e.kind + " " + e.code }

// RankingPlayer is one row of a ranking. Rank is the place in that ranking;
// Position stays what FACEIT reported, the place inside one region ladder.
type RankingPlayer struct {
	RankedPlayer
	Rank int `json:"rank"`
}

// ProfileURL is the player's FACEIT profile page.
func (p RankedPlayer) ProfileURL() string { return canonicalProfileURL(p.Nickname) }

// ParseRankingEntry validates kind and code: a country is a two-letter ISO
// 3166-1 code, a region one of rankingRegions and a zone one of Zones.
func ParseRankingEntry(kind, code string) (RankingEntry, error) {
	code = strings.TrimSpace(code)
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case RankingCountry:
		code = strings.ToLower(code)
		if !isCountryCode(code) {
			return RankingEntry{}, fmt.Errorf("FACEIT ranking country %q is not a two-letter ISO code", code)
		}
		return RankingEntry{kind: RankingCountry, code: code}, nil
	case RankingRegion:
		region, ok := canonicalRankingRegion(code)
		if !ok {
			return RankingEntry{}, fmt.Errorf("FACEIT ranking region %q is not one of %v", code, rankingRegions)
		}
		return RankingEntry{kind: RankingRegion, code: region}, nil
	case RankingZone:
		code = strings.ToLower(code)
		if _, ok := zoneCountries(code); !ok {
			return RankingEntry{}, fmt.Errorf("FACEIT ranking zone %q is not one of %v", code, Zones())
		}
		return RankingEntry{kind: RankingZone, code: code}, nil
	}
	return RankingEntry{}, fmt.Errorf("FACEIT ranking kind %q is not country, region or zone", kind)
}

func isCountryCode(code string) bool {
	return len(code) == 2 && code[0] >= 'a' && code[0] <= 'z' && code[1] >= 'a' && code[1] <= 'z'
}

// RankingTop reads the top RankingSize players of entry from FACEIT.
func (c *Client) RankingTop(ctx context.Context, entry RankingEntry) ([]RankingPlayer, error) {
	if c == nil || c.apiKey == "" {
		return nil, ErrNotConfigured
	}
	var countries []string
	switch entry.kind {
	case RankingRegion:
		return c.regionTop(ctx, entry.code)
	case RankingCountry:
		countries = []string{entry.code}
	case RankingZone:
		countries, _ = zoneCountries(entry.code)
	default:
		return nil, errors.New("FACEIT ranking entry is empty")
	}
	ladders := make([]rankingLadder, 0, len(countries)*len(rankingRegions))
	for _, country := range countries {
		for _, region := range rankingRegions {
			ladders = append(ladders, rankingLadder{region: region, country: country})
		}
	}
	players, failed := c.topAcross(ctx, ladders, RankingSize)
	if failed != nil {
		return nil, fmt.Errorf("list FACEIT %s rankings: %w", entry, failed)
	}
	out := make([]RankingPlayer, 0, len(players))
	for i, player := range players {
		out = append(out, RankingPlayer{RankedPlayer: player, Rank: i + 1})
	}
	return out, nil
}

// regionTop is one ladder, so FACEIT's own position is already the rank.
func (c *Client) regionTop(ctx context.Context, region string) ([]RankingPlayer, error) {
	players, err := c.Rankings(ctx, region, "", 0, RankingSize)
	if err != nil {
		return nil, err
	}
	out := make([]RankingPlayer, 0, len(players))
	for _, player := range players {
		out = append(out, RankingPlayer{RankedPlayer: player, Rank: player.Position})
	}
	return out, nil
}
