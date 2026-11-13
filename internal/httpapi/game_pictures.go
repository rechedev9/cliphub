package httpapi

import (
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"

	mapdata "github.com/rechedev9/cliphub/data/maps"
	"github.com/rechedev9/cliphub/internal/gameassets"
)

const maxTeamNameRunes = 64

const (
	mapShotRoute  = "/api/maps/{map}/shot"
	teamLogoRoute = "/api/teams/logo"
)

// optionalPictureRoutes answer 404 for many maps and teams; the UI falls back, so a miss is not an error.
var optionalPictureRoutes = map[string]bool{mapShotRoute: true, teamLogoRoute: true}

// GetMapShot handles GET /api/maps/{map}/shot: the map's loading-screen picture from the
// user's CS2 install as PNG, cached per game build.
func (h *Handlers) GetMapShot(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "map")
	if !geometryMapPattern.MatchString(name) {
		writeError(w, http.StatusBadRequest, "map must match "+geometryMapPattern.String())
		return
	}
	if body := mapdata.Shot(name); body != nil {
		writeBundledAsset(w, r, body, mapdata.ContentTypeWebP)
		return
	}
	install := gameassets.Install{CS2Exe: h.cs2Path()}
	key, err := install.CacheKey("shot", name+".png")
	if err != nil {
		writeGameAssetError(w, name, err)
		return
	}
	h.serveGameAsset(w, name, []string{key}, func() ([]byte, string, error) {
		body, err := install.MapShot(name)
		return body, gameassets.ContentTypePNG, err
	})
}

// GetTeamLogo handles GET /api/teams/logo?name=: the team's Major sticker logo from the
// user's CS2 install as sanitized SVG; a team without one answers 404.
func (h *Handlers) GetTeamLogo(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Query().Get("name")
	if name == "" || len([]rune(name)) > maxTeamNameRunes {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("name must hold 1 to %d characters", maxTeamNameRunes))
		return
	}
	code := gameassets.TeamLogoCode(name)
	if code == "" {
		writeGameAssetError(w, name, gameassets.ErrLogoNotFound)
		return
	}
	install := gameassets.Install{CS2Exe: h.cs2Path()}
	key, err := install.CacheKey("logo", code+".svg")
	if err != nil {
		writeGameAssetError(w, name, err)
		return
	}
	h.serveGameAsset(w, name, []string{key}, func() ([]byte, string, error) {
		body, err := install.TeamLogo(code)
		return body, gameassets.ContentTypeSVG, err
	})
}
