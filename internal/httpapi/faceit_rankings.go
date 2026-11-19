package httpapi

import (
	"errors"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/rechedev9/cliphub/internal/demooverlay"
	"github.com/rechedev9/cliphub/internal/faceit"
)

const defaultFaceitRankingPage = 20

// WithFaceitRankings supplies the on-disk copies of the country, region and
// zone rankings that GET /api/faceit/rankings serves.
func WithFaceitRankings(store *faceit.RankingStore) Option {
	return func(h *Handlers) {
		h.faceitRankings = store
	}
}

type faceitRankingRow struct {
	Rank       int    `json:"rank"`
	ID         string `json:"id"`
	Nickname   string `json:"nickname"`
	Country    string `json:"country,omitempty"`
	Region     string `json:"region"`
	Position   int    `json:"position"`
	ELO        int    `json:"elo"`
	SkillLevel int    `json:"skill_level,omitempty"`
	ProfileURL string `json:"profile_url"`
}

type faceitRankingPage struct {
	Kind      string             `json:"kind"`
	Code      string             `json:"code"`
	Total     int                `json:"total"`
	FetchedAt time.Time          `json:"fetched_at"`
	Stale     bool               `json:"stale"`
	StaleCode string             `json:"stale_code,omitempty"`
	Players   []faceitRankingRow `json:"players"`
}

// GetFaceitRanking serves one page of the top faceit.RankingSize of a country,
// region or zone. The whole ranking is read once and stored; pages slice it.
func (h *Handlers) GetFaceitRanking(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	entry, err := faceit.ParseRankingEntry(query.Get("kind"), query.Get("code"))
	if err == nil && entry.Kind() == faceit.RankingCountry && !slices.Contains(demooverlay.FlagCountries(), entry.Code()) {
		err = errors.New("FACEIT ranking country is not an ISO 3166-1 country")
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	offset, okOffset := rankingQueryInt(query.Get("offset"), 0, 0, faceit.RankingSize-1)
	limit, okLimit := rankingQueryInt(query.Get("limit"), defaultFaceitRankingPage, 1, faceit.RankingSize)
	if !okOffset || !okLimit {
		writeError(w, http.StatusBadRequest, "offset must be 0 to 99 and limit 1 to 100")
		return
	}
	if !h.faceitReady(w) {
		return
	}
	if h.faceitRankings == nil {
		writeCodedError(w, http.StatusServiceUnavailable, faceitNotConfigured, "FACEIT ranking store is not configured")
		return
	}
	list, err := h.faceitRankings.Top(r.Context(), h.faceit, entry)
	if err != nil {
		writeFaceitError(w, err)
		return
	}
	page := faceitRankingPage{Kind: entry.Kind(), Code: entry.Code(), Total: len(list.Players), FetchedAt: list.FetchedAt,
		Stale: list.Stale, Players: []faceitRankingRow{}}
	if list.Stale {
		page.StaleCode = faceitErrorCode(list.StaleErr)
	}
	for _, player := range list.Players[min(offset, len(list.Players)):min(offset+limit, len(list.Players))] {
		page.Players = append(page.Players, faceitRankingRow{Rank: player.Rank, ID: player.PlayerID, Nickname: player.Nickname,
			Country: player.Country, Region: player.Region, Position: player.Position, ELO: player.ELO, SkillLevel: player.SkillLevel,
			ProfileURL: player.ProfileURL()})
	}
	writeJSON(w, http.StatusOK, page)
}

func rankingQueryInt(raw string, fallback, low, high int) (int, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return fallback, true
	}
	value, err := strconv.Atoi(raw)
	return value, err == nil && value >= low && value <= high
}

// faceitErrorCode is the code writeFaceitError would answer err with, for a
// stale page that still has to say why FACEIT did not answer.
func faceitErrorCode(err error) string {
	switch {
	case errors.Is(err, faceit.ErrRateLimited):
		return "faceit_rate_limited"
	case errors.Is(err, faceit.ErrUnauthorized):
		return "faceit_unauthorized"
	case errors.Is(err, faceit.ErrInvalidResponse):
		return "faceit_invalid_response"
	default:
		return "faceit_unavailable"
	}
}

// ListFaceitCountries lists the countries the ranking selector offers: those
// with a bundled flag.
func (h *Handlers) ListFaceitCountries(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"countries": demooverlay.FlagCountries()})
}

// GetFaceitFlag serves a bundled flag, the same file the overlays draw.
func (h *Handlers) GetFaceitFlag(w http.ResponseWriter, r *http.Request) {
	svg, ok := demooverlay.FlagSVG(chi.URLParam(r, "code"))
	if !ok {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "image/svg+xml")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'")
	w.Header().Set("Cache-Control", "public, max-age=86400")
	_, _ = w.Write(svg)
}
