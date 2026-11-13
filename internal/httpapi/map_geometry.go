package httpapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"
	"sync"

	"github.com/go-chi/chi/v5"

	"github.com/rechedev9/cliphub/internal/mapgeo"
)

// geometryMapPattern admits the map prefixes CS2 ships (defuse, hostage, arms
// race) and keeps the name safe to join into a file path and a storage key.
var geometryMapPattern = regexp.MustCompile(`^(de|cs|ar)_[a-z0-9_]{1,32}$`)

const (
	codeMapNotFound   = "map_not_found"
	codeMapHasNoNav   = "map_has_no_nav"
	codeMapUnreadable = "map_geometry_unreadable"
	maxCachedMeshSize = 32 << 20
)

// mapGeometryMu runs one extraction at a time: a physics read of a big map
// holds a few hundred MB for a second or two, and a second caller then finds
// the cache filled.
var mapGeometryMu sync.Mutex

// GetMapGeometry handles GET /api/maps/{map}/geometry?source=physics|nav: the
// map's collision (default) or walkable floor, read from the user's CS2
// install and cached per map, source and game build. When the physics read
// fails the nav mesh answers instead, and the body's source says so.
func (h *Handlers) GetMapGeometry(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "map")
	if !geometryMapPattern.MatchString(name) {
		writeError(w, http.StatusBadRequest, "map must match "+geometryMapPattern.String())
		return
	}
	kind := r.URL.Query().Get("source")
	if kind == "" {
		kind = mapgeo.SourcePhysics
	}
	if kind != mapgeo.SourcePhysics && kind != mapgeo.SourceNav {
		writeError(w, http.StatusBadRequest, "source must be physics or nav")
		return
	}
	src := mapgeo.Source{CS2Exe: h.cs2Path(), Map: name}
	key, err := src.CacheKey(kind)
	if err != nil {
		writeMapGeometryError(w, name, err)
		return
	}
	if cached, ok := h.readCachedMesh(key); ok {
		writeRawJSON(w, cached)
		return
	}
	mapGeometryMu.Lock()
	defer mapGeometryMu.Unlock()
	if cached, ok := h.readCachedMesh(key); ok {
		writeRawJSON(w, cached)
		return
	}
	mesh, err := src.Extract(kind)
	if err != nil {
		writeMapGeometryError(w, name, err)
		return
	}
	if mesh.Fallback != "" {
		log.Printf("httpapi: physics geometry of %s failed, serving the nav mesh: %s", name, mesh.Fallback)
	}
	body, err := json.Marshal(mesh)
	if err != nil {
		internalError(w, "encode map geometry", err)
		return
	}
	if err := h.gameCache().Put(key, bytes.NewReader(body)); err != nil {
		log.Printf("httpapi: cache map geometry %s: %v", key, err)
	}
	writeRawJSON(w, body)
}

// cs2Path is the CS2 executable the orchestrator detected at startup.
func (h *Handlers) cs2Path() string {
	if h.references.game.cs2Exe != "" {
		return h.references.game.cs2Exe
	}
	for _, tool := range h.capabilities.RecordTools {
		if tool.Name == "ZV_CS2_PATH" {
			return tool.Path
		}
	}
	return ""
}

func (h *Handlers) readCachedMesh(key string) ([]byte, bool) {
	rc, err := h.gameCache().Open(key)
	if err != nil {
		return nil, false
	}
	defer rc.Close()
	body, err := io.ReadAll(io.LimitReader(rc, maxCachedMeshSize))
	if err != nil || !json.Valid(body) {
		return nil, false
	}
	return body, true
}

func writeRawJSON(w http.ResponseWriter, body []byte) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body)
}

func writeMapGeometryError(w http.ResponseWriter, name string, err error) {
	switch {
	case errors.Is(err, mapgeo.ErrCS2NotFound):
		writeCodedError(w, http.StatusNotFound, codeCS2NotFound, "CS2 is not installed or was not found on this PC")
	case errors.Is(err, mapgeo.ErrMapNotFound):
		writeCodedError(w, http.StatusNotFound, codeMapNotFound, fmt.Sprintf("map %q is not in the CS2 install", name))
	case errors.Is(err, mapgeo.ErrNoNavMesh):
		writeCodedError(w, http.StatusNotFound, codeMapHasNoNav, fmt.Sprintf("map %q has no navigation mesh", name))
	default:
		// A CS2 update may change the nav format; the UI needs a stable code, the log the cause.
		log.Printf("httpapi: read map geometry %s: %v", name, err)
		writeCodedError(w, http.StatusUnprocessableEntity, codeMapUnreadable, fmt.Sprintf("could not read the geometry of map %q from the CS2 install", name))
	}
}
