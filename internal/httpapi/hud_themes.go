package httpapi

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"unicode"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/customhud"
	"github.com/rechedev9/cliphub/internal/hudthemes"
	"github.com/rechedev9/cliphub/internal/library"
	"github.com/rechedev9/cliphub/internal/recapplan"
)

// HUDThemeRepository is the store of the user's own HUD themes.
type HUDThemeRepository interface {
	ListHUDThemes(ctx context.Context) ([]hudthemes.Theme, error)
	GetHUDTheme(ctx context.Context, id uuid.UUID) (hudthemes.Theme, error)
	CreateHUDTheme(ctx context.Context, t *hudthemes.Theme) error
	UpdateHUDTheme(ctx context.Context, id uuid.UUID, style hudthemes.Style) (hudthemes.Theme, error)
	DeleteHUDTheme(ctx context.Context, id uuid.UUID) error
}

// WithHUDThemes enables the /api/full-demo/hud-themes routes over repo.
func WithHUDThemes(repo HUDThemeRepository) Option {
	return func(h *Handlers) {
		h.hudThemes = repo
	}
}

type hudThemesBody struct {
	Themes []hudthemes.Theme `json:"themes"`
	Max    int               `json:"max"`
}

func (h *Handlers) hudThemesReady(w http.ResponseWriter) bool {
	if h.hudThemes == nil {
		writeCodedError(w, http.StatusServiceUnavailable, "not_configured", "HUD themes are not available")
		return false
	}
	return true
}

// ListHUDThemes handles GET /api/full-demo/hud-themes: the user's own themes, oldest first.
func (h *Handlers) ListHUDThemes(w http.ResponseWriter, r *http.Request) {
	if !h.hudThemesReady(w) {
		return
	}
	themes, err := h.hudThemes.ListHUDThemes(r.Context())
	if err != nil {
		internalError(w, "list hud themes", err)
		return
	}
	writeJSON(w, http.StatusOK, hudThemesBody{Themes: themes, Max: hudthemes.MaxThemes})
}

// CreateHUDTheme handles POST /api/full-demo/hud-themes: a catalog design copied with the user's style.
func (h *Handlers) CreateHUDTheme(w http.ResponseWriter, r *http.Request) {
	if !h.hudThemesReady(w) {
		return
	}
	var req struct {
		Base string `json:"base"`
		hudthemes.Style
	}
	if err := decodeSingleJSONBody(w, r, &req, true); err != nil {
		writeError(w, http.StatusBadRequest, "invalid HUD theme JSON")
		return
	}
	if _, ok := customhud.Lookup(req.Base); !ok {
		writeError(w, http.StatusBadRequest, "base must be a HUD design id from the catalog")
		return
	}
	style, err := req.Style.Clean()
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	theme := hudthemes.Theme{Base: req.Base, Style: style}
	if err := h.hudThemes.CreateHUDTheme(r.Context(), &theme); err != nil {
		writeHUDThemeError(w, "create hud theme", err)
		return
	}
	writeJSON(w, http.StatusCreated, theme)
}

// UpdateHUDTheme handles PUT /api/full-demo/hud-themes/{id}: it replaces the name, colours and radius.
func (h *Handlers) UpdateHUDTheme(w http.ResponseWriter, r *http.Request) {
	if !h.hudThemesReady(w) {
		return
	}
	id, ok := hudThemeID(w, r)
	if !ok {
		return
	}
	var req hudthemes.Style
	if err := decodeSingleJSONBody(w, r, &req, true); err != nil {
		writeError(w, http.StatusBadRequest, "invalid HUD theme JSON")
		return
	}
	style, err := req.Clean()
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	theme, err := h.hudThemes.UpdateHUDTheme(r.Context(), id, style)
	if err != nil {
		writeHUDThemeError(w, "update hud theme", err)
		return
	}
	writeJSON(w, http.StatusOK, theme)
}

// DeleteHUDTheme handles DELETE /api/full-demo/hud-themes/{id}. Plans already made keep their copy.
func (h *Handlers) DeleteHUDTheme(w http.ResponseWriter, r *http.Request) {
	if !h.hudThemesReady(w) {
		return
	}
	id, ok := hudThemeID(w, r)
	if !ok {
		return
	}
	if err := h.hudThemes.DeleteHUDTheme(r.Context(), id); err != nil {
		writeHUDThemeError(w, "delete hud theme", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// GetHUDThemePreview handles GET /api/full-demo/hud-themes/preview: the example
// scene drawn as SVG with a style, the composition switches and team names.
// With vertical=1 it draws the band of the 9:16 Short HUD in that design instead.
func (h *Handlers) GetHUDThemePreview(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	o := recapplan.OverlayOptions{HUDTheme: q.Get("base")}
	if _, ok := customhud.Lookup(o.HUDTheme); !ok {
		writeError(w, http.StatusBadRequest, "base must be a HUD design id from the catalog")
		return
	}
	style, err := hudPreviewStyle(q)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	o.HUDCustom = style
	o.HUDOptions = customhud.Options{CompactHeader: q.Get("compact_header") == "1", HideLoadout: q.Get("hide_loadout") == "1", RoundWinBanner: q.Get("round_win_banner") == "1", WinProbability: q.Get("win_probability") == "1"}
	o.TeamNames = customhud.Teams{CTStart: strings.TrimSpace(q.Get("ct_name")), TStart: strings.TrimSpace(q.Get("t_name"))}
	renderer, err := o.HUDRenderer()
	if err != nil {
		internalError(w, "hud preview", err)
		return
	}
	if q.Get("vertical") == "1" {
		writeHUDPreview(w, renderer.VerticalSVG(customhud.VerticalExample()))
		return
	}
	state := customhud.Example()
	state.SidesKnown = true
	if o.HUDOptions.RoundWinBanner {
		// Illustrative: the example's T side takes its third round.
		state.Phase, state.TScore, state.Winner = "ended", state.TScore+1, "T"
	} else if o.HUDOptions.WinProbability {
		// Illustrative: a 4v2 after the plant, with a second T alive.
		state.Phase, state.TimeRemaining, state.WinCT = "planted", 28, 62
		state.Players[7].Alive, state.Players[7].Health = true, 100
	}
	writeHUDPreview(w, renderer.SVG(state, customhud.ExampleTarget))
}

func writeHUDPreview(w http.ResponseWriter, svg string) {
	w.Header().Set("Content-Type", "image/svg+xml")
	w.Header().Set("Cache-Control", "private, max-age=3600")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; font-src data:")
	_, _ = w.Write([]byte(svg))
}

func hudThemeID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	raw := chi.URLParam(r, "id")
	id, err := uuid.Parse(raw)
	if err != nil || id.String() != raw {
		writeError(w, http.StatusBadRequest, "invalid HUD theme id")
		return uuid.Nil, false
	}
	return id, true
}

func writeHUDThemeError(w http.ResponseWriter, op string, err error) {
	switch {
	case errors.Is(err, hudthemes.ErrNotFound):
		writeError(w, http.StatusNotFound, "HUD theme not found")
	case errors.Is(err, hudthemes.ErrLimit):
		writeCodedError(w, http.StatusConflict, "hud_theme_limit", err.Error())
	default:
		internalError(w, op, err)
	}
}

// fullDemoTeamNames names the teams by starting side from the library match
// of the job's demo: the assigned team first, then the clan name.
func (h *Handlers) fullDemoTeamNames(ctx context.Context, jobID uuid.UUID) customhud.Teams {
	if h.libraryRepo == nil {
		return customhud.Teams{}
	}
	demo, err := h.libraryRepo.DemoForJob(ctx, jobID)
	if err != nil || demo.MatchID == nil {
		return customhud.Teams{}
	}
	match, err := h.libraryRepo.GetMatch(ctx, *demo.MatchID)
	if err != nil {
		return customhud.Teams{}
	}
	return teamNamesFromMatch(match)
}

func teamNamesFromMatch(match library.Match) customhud.Teams {
	var teams customhud.Teams
	for _, team := range match.Teams {
		name := cleanTeamName(team.TeamName)
		if name == "" {
			name = cleanTeamName(team.ClanName)
		}
		if runes := []rune(name); len(runes) > recapplan.MaxTeamNameRunes {
			name = strings.TrimSpace(string(runes[:recapplan.MaxTeamNameRunes]))
		}
		switch strings.ToUpper(team.StartSide) {
		case "CT":
			teams.CTStart = name
		case "T":
			teams.TStart = name
		}
	}
	return teams
}

func cleanTeamName(name string) string {
	return strings.TrimSpace(strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, name))
}
