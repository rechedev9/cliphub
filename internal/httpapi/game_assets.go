package httpapi

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"
	"strings"
	"sync"

	"github.com/go-chi/chi/v5"

	"github.com/rechedev9/cliphub/internal/gameassets"
	"github.com/rechedev9/cliphub/internal/mapgeo"
)

// equipmentIconPattern admits the lowercase entity names CS2 gives its icons
// (ak47, m4a1_silencer, knife_karambit) and keeps them safe as path parts.
var equipmentIconPattern = regexp.MustCompile(`^[a-z0-9]+(_[a-z0-9]+){0,4}$`)

const (
	codeRadarNotFound       = "radar_not_found"
	codeIconNotFound        = "icon_not_found"
	codeGameAssetUnreadable = "game_asset_unreadable"
	maxCachedGameAsset      = 16 << 20
	maxEquipmentIconName    = 40
	// svgPolicy keeps a directly opened icon inert even if sanitizing missed something.
	svgPolicy = "default-src 'none'; style-src 'unsafe-inline'; sandbox"
)

// gameAssetsMu runs one extraction at a time; a second caller then finds the cache filled.
var gameAssetsMu sync.Mutex

// gameAssetExtract reads one asset from the install: its bytes and content type.
type gameAssetExtract func() ([]byte, string, error)

// GetMapRadar handles GET /api/maps/{map}/radar?layer=upper|lower: CS2's own
// overview image for the map, decoded from the user's install to PNG and
// cached per game build.
func (h *Handlers) GetMapRadar(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "map")
	if !geometryMapPattern.MatchString(name) {
		writeError(w, http.StatusBadRequest, "map must match "+geometryMapPattern.String())
		return
	}
	layer := r.URL.Query().Get("layer")
	if layer == "" {
		layer = gameassets.LayerUpper
	}
	if layer != gameassets.LayerUpper && layer != gameassets.LayerLower {
		writeError(w, http.StatusBadRequest, "layer must be upper or lower")
		return
	}
	install := gameassets.Install{CS2Exe: h.cs2Path()}
	key, err := install.CacheKey("radar", name+"_"+layer+".png")
	if err != nil {
		writeGameAssetError(w, name, err)
		return
	}
	h.serveGameAsset(w, name, []string{key}, func() ([]byte, string, error) {
		body, format, err := install.Radar(name, layer)
		if err == nil {
			log.Printf("httpapi: decoded the %s %s radar (%s, %d bytes of PNG)", name, layer, format, len(body))
		}
		return body, gameassets.ContentTypePNG, err
	})
}

// GetEquipmentIcon handles GET /api/equipment/{name}/icon: CS2's own icon for
// a weapon or piece of equipment, as sanitized SVG (or PNG when CS2 ships a
// texture), cached per game build.
func (h *Handlers) GetEquipmentIcon(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	if len(name) > maxEquipmentIconName || !equipmentIconPattern.MatchString(name) {
		writeError(w, http.StatusBadRequest, "name must match "+equipmentIconPattern.String())
		return
	}
	install := gameassets.Install{CS2Exe: h.cs2Path()}
	svgKey, err := install.CacheKey("icon", name+".svg")
	if err != nil {
		writeGameAssetError(w, name, err)
		return
	}
	pngKey, err := install.CacheKey("icon", name+".png")
	if err != nil {
		writeGameAssetError(w, name, err)
		return
	}
	h.serveGameAsset(w, name, []string{svgKey, pngKey}, func() ([]byte, string, error) {
		return install.Icon(name)
	})
}

// serveGameAsset answers from the first cached key, or extracts the asset and
// caches it under the key whose extension matches its content type.
func (h *Handlers) serveGameAsset(w http.ResponseWriter, name string, keys []string, extract gameAssetExtract) {
	if body, contentType, ok := h.readCachedGameAsset(keys); ok {
		writeGameAsset(w, body, contentType)
		return
	}
	gameAssetsMu.Lock()
	defer gameAssetsMu.Unlock()
	if body, contentType, ok := h.readCachedGameAsset(keys); ok {
		writeGameAsset(w, body, contentType)
		return
	}
	body, contentType, err := extract()
	if err != nil {
		writeGameAssetError(w, name, err)
		return
	}
	for _, key := range keys {
		if gameAssetContentType(key) == contentType {
			if err := h.storage.Put(key, bytes.NewReader(body)); err != nil {
				log.Printf("httpapi: cache game asset %s: %v", key, err)
			}
			break
		}
	}
	writeGameAsset(w, body, contentType)
}

func (h *Handlers) readCachedGameAsset(keys []string) ([]byte, string, bool) {
	for _, key := range keys {
		rc, err := h.storage.Open(key)
		if err != nil {
			continue
		}
		body, err := io.ReadAll(io.LimitReader(rc, maxCachedGameAsset))
		_ = rc.Close()
		if err == nil && len(body) > 0 {
			return body, gameAssetContentType(key), true
		}
	}
	return nil, "", false
}

func gameAssetContentType(key string) string {
	if strings.HasSuffix(key, ".svg") {
		return gameassets.ContentTypeSVG
	}
	return gameassets.ContentTypePNG
}

func writeGameAsset(w http.ResponseWriter, body []byte, contentType string) {
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "private, max-age=3600")
	if contentType == gameassets.ContentTypeSVG {
		w.Header().Set("Content-Security-Policy", svgPolicy)
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body)
}

func writeGameAssetError(w http.ResponseWriter, name string, err error) {
	switch {
	case errors.Is(err, mapgeo.ErrCS2NotFound):
		writeCodedError(w, http.StatusNotFound, codeCS2NotFound, "CS2 is not installed or was not found on this PC")
	case errors.Is(err, gameassets.ErrRadarNotFound):
		writeCodedError(w, http.StatusNotFound, codeRadarNotFound, fmt.Sprintf("the CS2 install has no radar for %q", name))
	case errors.Is(err, gameassets.ErrIconNotFound):
		writeCodedError(w, http.StatusNotFound, codeIconNotFound, fmt.Sprintf("the CS2 install has no icon named %q", name))
	default:
		// A CS2 update may change a format; the UI needs a stable code, the log the cause.
		log.Printf("httpapi: read game asset %s: %v", name, err)
		writeCodedError(w, http.StatusUnprocessableEntity, codeGameAssetUnreadable, fmt.Sprintf("could not read %q from the CS2 install", name))
	}
}
