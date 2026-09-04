package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/rechedev9/cliphub/internal/faceit"
)

const (
	faceitCacheTTL      = 3 * time.Minute
	maxFaceitJSONBytes  = 4 << 10
	faceitNotConfigured = "faceit_not_configured"
)

type faceitResponseCache struct {
	mu      sync.Mutex
	now     func() time.Time
	players map[string]cachedFaceitPlayer
	matches map[string]cachedFaceitMatches
	avatars map[string]string
}

type cachedFaceitPlayer struct {
	player faceit.Player
	until  time.Time
}

type cachedFaceitMatches struct {
	matches []faceit.RecentMatch
	until   time.Time
}

func (c *faceitResponseCache) clock() time.Time {
	if c.now != nil {
		return c.now()
	}
	return time.Now()
}

func (h *Handlers) LookupFaceitPlayer(w http.ResponseWriter, r *http.Request) {
	if !h.faceitReady(w) {
		return
	}
	nickname, err := faceit.ParseProfile(r.URL.Query().Get("nickname"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if player, ok := h.faceitCache.lookupPlayer(nickname); ok {
		writeJSON(w, http.StatusOK, map[string]any{"player": player})
		return
	}
	player, err := h.faceit.LookupPlayer(r.Context(), nickname)
	if err != nil {
		writeFaceitError(w, err)
		return
	}
	enrichAvatar(r.Context(), &player)
	h.faceitCache.storePlayer(nickname, player)
	writeJSON(w, http.StatusOK, map[string]any{"player": player})
}

func (h *Handlers) ListFaceitMatches(w http.ResponseWriter, r *http.Request) {
	if !h.faceitReady(w) {
		return
	}
	playerID := chi.URLParam(r, "playerID")
	if !faceit.ValidPlayerID(playerID) {
		writeError(w, http.StatusBadRequest, "FACEIT player id is invalid")
		return
	}
	limit := 0
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil {
			writeError(w, http.StatusBadRequest, "limit must be an integer")
			return
		}
		limit = parsed
	}
	if matches, ok := h.faceitCache.lookupMatches(playerID); ok {
		writeJSON(w, http.StatusOK, map[string]any{"matches": clampCachedMatches(matches, limit)})
		return
	}
	matches, err := h.faceit.RecentMatches(r.Context(), playerID, 20)
	if err != nil {
		writeFaceitError(w, err)
		return
	}
	h.faceitCache.storeMatches(playerID, matches)
	writeJSON(w, http.StatusOK, map[string]any{"matches": clampCachedMatches(matches, limit)})
}

func (h *Handlers) ListFollowedFaceitPlayers(w http.ResponseWriter, r *http.Request) {
	if h.faceitFollows == nil {
		writeCodedError(w, http.StatusServiceUnavailable, faceitNotConfigured, "FACEIT follow list is not configured")
		return
	}
	players, err := h.faceitFollows.Roster(h.faceitSeeds.Document())
	if err != nil {
		internalError(w, "list followed FACEIT players", err)
		return
	}
	if players == nil {
		players = []faceit.RosterPlayer{}
	}
	for _, player := range players {
		if player.Avatar != "" {
			h.faceitCache.storeAvatar(player.ID, player.Avatar)
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"enabled": h.faceitEnabled(),
		"players": players,
	})
}

func (h *Handlers) FollowFaceitPlayer(w http.ResponseWriter, r *http.Request) {
	if !h.faceitReady(w) {
		return
	}
	if h.faceitFollows == nil {
		writeCodedError(w, http.StatusServiceUnavailable, faceitNotConfigured, "FACEIT follow list is not configured")
		return
	}
	var req struct {
		Nickname string `json:"nickname"`
	}
	if err := decodeBoundedJSON(r.Body, maxFaceitJSONBytes, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid follow request JSON")
		return
	}
	nickname, err := faceit.ParseProfile(req.Nickname)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	player, ok := h.faceitCache.lookupPlayer(nickname)
	if !ok {
		player, err = h.faceit.LookupPlayer(r.Context(), nickname)
		if err != nil {
			writeFaceitError(w, err)
			return
		}
		enrichAvatar(r.Context(), &player)
		h.faceitCache.storePlayer(nickname, player)
	}
	followed, err := h.faceitFollows.Follow(player)
	if err != nil {
		writeFaceitError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"player": followed})
}

func (h *Handlers) UnfollowFaceitPlayer(w http.ResponseWriter, r *http.Request) {
	if h.faceitFollows == nil {
		writeCodedError(w, http.StatusServiceUnavailable, faceitNotConfigured, "FACEIT follow list is not configured")
		return
	}
	playerID := chi.URLParam(r, "playerID")
	if !faceit.ValidPlayerID(playerID) {
		writeError(w, http.StatusBadRequest, "FACEIT player id is invalid")
		return
	}
	if err := h.faceitFollows.Unfollow(playerID); err != nil {
		writeFaceitError(w, err)
		return
	}
	// A seeded row was never in followed.json, so Unfollow alone would no-op
	// and the player would reappear on the next Roster read.
	if err := h.faceitFollows.DismissSeed(playerID); err != nil {
		writeFaceitError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handlers) ProxyFaceitAvatar(w http.ResponseWriter, r *http.Request) {
	playerID := chi.URLParam(r, "playerID")
	if !faceit.ValidPlayerID(playerID) {
		http.NotFound(w, r)
		return
	}
	avatarURL, ok := h.faceitCache.lookupAvatar(playerID)
	if !ok {
		http.NotFound(w, r)
		return
	}
	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, avatarURL, nil)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil || res.StatusCode != http.StatusOK {
		if res != nil {
			_ = res.Body.Close()
		}
		http.NotFound(w, r)
		return
	}
	defer res.Body.Close()
	ct := res.Header.Get("Content-Type")
	if ct == "" {
		ct = "image/jpeg"
	}
	w.Header().Set("Content-Type", ct)
	w.Header().Set("Cache-Control", "public, max-age=3600")
	_, _ = io.Copy(w, io.LimitReader(res.Body, 2<<20))
}

func enrichAvatar(ctx context.Context, player *faceit.Player) {
	if !faceit.IsDefaultFaceitAvatar(player.Avatar) {
		return
	}
	if player.SteamID64 == "" {
		return
	}
	steamAvatar, err := faceit.ResolveSteamAvatar(ctx, http.DefaultClient, player.SteamID64)
	if err != nil || steamAvatar == "" {
		return
	}
	player.Avatar = steamAvatar
}

func (h *Handlers) faceitEnabled() bool {
	return h.faceit != nil
}

func (h *Handlers) faceitReady(w http.ResponseWriter) bool {
	if h.faceit != nil {
		return true
	}
	writeCodedError(w, http.StatusServiceUnavailable, faceitNotConfigured, "FACEIT Data API is not configured; set FACEIT_API_KEY and restart the orchestrator")
	return false
}

func writeFaceitError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, faceit.ErrNotConfigured):
		writeCodedError(w, http.StatusServiceUnavailable, faceitNotConfigured, "FACEIT Data API is not configured; set FACEIT_API_KEY and restart the orchestrator")
	case errors.Is(err, faceit.ErrPlayerNotFound):
		writeError(w, http.StatusNotFound, "FACEIT player not found")
	case errors.Is(err, faceit.ErrFollowLimit):
		writeError(w, http.StatusConflict, "followed player limit reached")
	case errors.Is(err, faceit.ErrUnauthorized):
		writeCodedError(w, http.StatusBadGateway, "faceit_unauthorized", "FACEIT Data API authorization failed")
	case errors.Is(err, faceit.ErrRateLimited):
		writeCodedError(w, http.StatusTooManyRequests, "faceit_rate_limited", "FACEIT Data API rate limited")
	case errors.Is(err, faceit.ErrUnavailable):
		writeCodedError(w, http.StatusBadGateway, "faceit_unavailable", "FACEIT Data API unavailable")
	case errors.Is(err, faceit.ErrInvalidResponse):
		writeCodedError(w, http.StatusBadGateway, "faceit_invalid_response", "FACEIT Data API response is invalid")
	default:
		internalError(w, "faceit", err)
	}
}

func decodeBoundedJSON(body io.Reader, limit int64, dst any) error {
	dec := json.NewDecoder(io.LimitReader(body, limit+1))
	if err := dec.Decode(dst); err != nil {
		return err
	}
	return nil
}

func clampCachedMatches(matches []faceit.RecentMatch, limit int) []faceit.RecentMatch {
	if limit <= 0 {
		limit = 10
	}
	if limit > 20 {
		limit = 20
	}
	if len(matches) <= limit {
		return matches
	}
	return matches[:limit]
}

func (c *faceitResponseCache) lookupPlayer(nickname string) (faceit.Player, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	entry, ok := c.players[strings.ToLower(nickname)]
	if !ok || !c.clock().Before(entry.until) {
		return faceit.Player{}, false
	}
	return entry.player, true
}

func (c *faceitResponseCache) storePlayer(nickname string, player faceit.Player) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.players == nil {
		c.players = map[string]cachedFaceitPlayer{}
	}
	c.players[strings.ToLower(nickname)] = cachedFaceitPlayer{player: player, until: c.clock().Add(faceitCacheTTL)}
	if player.Avatar != "" {
		if c.avatars == nil {
			c.avatars = map[string]string{}
		}
		c.avatars[player.ID] = player.Avatar
	}
}

func (c *faceitResponseCache) lookupAvatar(playerID string) (string, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	url, ok := c.avatars[playerID]
	return url, ok && url != ""
}

func (c *faceitResponseCache) storeAvatar(playerID, url string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.avatars == nil {
		c.avatars = map[string]string{}
	}
	c.avatars[playerID] = url
}

func (c *faceitResponseCache) lookupMatches(playerID string) ([]faceit.RecentMatch, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	entry, ok := c.matches[playerID]
	if !ok || !c.clock().Before(entry.until) {
		return nil, false
	}
	return append([]faceit.RecentMatch(nil), entry.matches...), true
}

func (c *faceitResponseCache) storeMatches(playerID string, matches []faceit.RecentMatch) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.matches == nil {
		c.matches = map[string]cachedFaceitMatches{}
	}
	c.matches[playerID] = cachedFaceitMatches{
		matches: append([]faceit.RecentMatch(nil), matches...),
		until:   c.clock().Add(faceitCacheTTL),
	}
}
