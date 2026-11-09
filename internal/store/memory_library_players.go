package store

import (
	"context"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/library"
)

// The players of the in-memory library: their nicks and teams come from the matches they played.

func (r *MemoryLibraryRepository) playerMatchCount(steamID string) int {
	n := 0
	for _, lines := range r.matchPlayers {
		for _, line := range lines {
			if line.SteamID64 == steamID {
				n++
			}
		}
	}
	return n
}

func (r *MemoryLibraryRepository) GetPlayer(ctx context.Context, steamID string) (library.Player, error) {
	if err := ctx.Err(); err != nil {
		return library.Player{}, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.players[steamID]
	if !ok {
		return library.Player{}, library.ErrNotFound
	}
	p.MatchCount = r.playerMatchCount(steamID)
	nicks := map[string]*library.PlayerNick{}
	teams := map[uuid.UUID]*library.PlayerTeam{}
	for _, link := range r.links() {
		if link.player.SteamID64 != steamID {
			continue
		}
		at := link.match.playedAt
		n, ok := nicks[link.player.Name]
		if !ok {
			n = &library.PlayerNick{Nick: link.player.Name, FirstSeenAt: at, LastSeenAt: at}
			nicks[link.player.Name] = n
		}
		n.FirstSeenAt, n.LastSeenAt = minTime(n.FirstSeenAt, at), maxTime(n.LastSeenAt, at)
		if link.teamID == nil {
			continue
		}
		pt, ok := teams[*link.teamID]
		if !ok {
			pt = &library.PlayerTeam{TeamID: *link.teamID, TeamName: r.teams[*link.teamID].name, FirstPlayedAt: at, LastPlayedAt: at}
			teams[*link.teamID] = pt
		}
		pt.Matches++
		pt.FirstPlayedAt, pt.LastPlayedAt = minTime(pt.FirstPlayedAt, at), maxTime(pt.LastPlayedAt, at)
	}
	p.Nicks = []library.PlayerNick{}
	for _, n := range nicks {
		p.Nicks = append(p.Nicks, *n)
	}
	sort.Slice(p.Nicks, func(i, j int) bool {
		if !p.Nicks[i].LastSeenAt.Equal(p.Nicks[j].LastSeenAt) {
			return p.Nicks[i].LastSeenAt.After(p.Nicks[j].LastSeenAt)
		}
		return p.Nicks[i].Nick < p.Nicks[j].Nick
	})
	p.Teams = []library.PlayerTeam{}
	for _, pt := range teams {
		p.Teams = append(p.Teams, *pt)
	}
	sort.Slice(p.Teams, func(i, j int) bool {
		if !p.Teams[i].LastPlayedAt.Equal(p.Teams[j].LastPlayedAt) {
			return p.Teams[i].LastPlayedAt.After(p.Teams[j].LastPlayedAt)
		}
		return p.Teams[i].TeamID.String() < p.Teams[j].TeamID.String()
	})
	return p, nil
}

func minTime(a, b time.Time) time.Time {
	if b.Before(a) {
		return b
	}
	return a
}

func maxTime(a, b time.Time) time.Time {
	if b.After(a) {
		return b
	}
	return a
}

func (r *MemoryLibraryRepository) ListPlayers(ctx context.Context, filter library.PlayerFilter) ([]library.Player, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	q := strings.ToLower(strings.TrimSpace(filter.Query))
	onTeam := map[string]bool{}
	if filter.TeamID != nil {
		for _, link := range r.links() {
			if sameUUID(link.teamID, filter.TeamID) {
				onTeam[link.player.SteamID64] = true
			}
		}
	}
	out := []library.Player{}
	for _, p := range r.players {
		if q != "" && !strings.Contains(strings.ToLower(p.Nick), q) && p.SteamID64 != q {
			continue
		}
		if filter.TeamID != nil && !onTeam[p.SteamID64] {
			continue
		}
		p.MatchCount = r.playerMatchCount(p.SteamID64)
		p.Nicks, p.Teams = nil, nil
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := strings.ToLower(out[i].Nick), strings.ToLower(out[j].Nick)
		if a != b {
			return a < b
		}
		return out[i].SteamID64 < out[j].SteamID64
	})
	return libraryPage(out, filter.Limit, filter.Offset), nil
}

func (r *MemoryLibraryRepository) SetPlayerFaceit(ctx context.Context, steamID, faceitPlayerID, faceitNickname string) (library.Player, error) {
	if err := ctx.Err(); err != nil {
		return library.Player{}, err
	}
	r.mu.Lock()
	p, ok := r.players[steamID]
	if !ok {
		r.mu.Unlock()
		return library.Player{}, library.ErrNotFound
	}
	p.FaceitPlayerID, p.FaceitNickname, p.UpdatedAt = faceitPlayerID, faceitNickname, libraryNow()
	r.players[steamID] = p
	r.mu.Unlock()
	return r.GetPlayer(ctx, steamID)
}
