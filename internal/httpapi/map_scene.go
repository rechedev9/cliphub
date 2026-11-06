package httpapi

import (
	"errors"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/rechedev9/cliphub/internal/mapgeo"
)

// GetMapScene handles GET /api/maps/{map}/scene: the map's render meshes,
// materials, textures and baked lighting read from the user's CS2 install, in
// the binary scene format of mapgeo.WriteScene. The first read of a map builds
// it in a few seconds; later reads stream the cached file.
func (h *Handlers) GetMapScene(w http.ResponseWriter, r *http.Request) {
	key, ok := h.mapSceneKey(w, r)
	if !ok || h.serveCachedScene(w, r, key) {
		return
	}
	writeCodedError(w, http.StatusUnprocessableEntity, codeMapUnreadable, "could not read back the 3D scene of map "+chi.URLParam(r, "map"))
}

// GetMapSceneSummary handles GET /api/maps/{map}/scene/summary: what the
// map's 3D scene holds (triangles, textures, sun, fog, sky), building it first
// when it is not cached yet.
func (h *Handlers) GetMapSceneSummary(w http.ResponseWriter, r *http.Request) {
	key, ok := h.mapSceneKey(w, r)
	if !ok {
		return
	}
	rc, err := h.storage.Open(key)
	if err != nil {
		writeCodedError(w, http.StatusUnprocessableEntity, codeMapUnreadable, "could not read back the 3D scene of map "+chi.URLParam(r, "map"))
		return
	}
	defer rc.Close()
	summary, err := mapgeo.ReadSceneSummary(rc)
	if err != nil {
		log.Printf("httpapi: read map scene summary %s: %v", key, err)
		writeCodedError(w, http.StatusUnprocessableEntity, codeMapUnreadable, "could not read the 3D scene of map "+chi.URLParam(r, "map"))
		return
	}
	writeJSON(w, http.StatusOK, summary)
}

// mapSceneKey returns the storage key of the map's cached scene, extracting
// and caching it first when needed; it writes the error response when it fails.
func (h *Handlers) mapSceneKey(w http.ResponseWriter, r *http.Request) (string, bool) {
	name := chi.URLParam(r, "map")
	if !geometryMapPattern.MatchString(name) {
		writeError(w, http.StatusBadRequest, "map must match "+geometryMapPattern.String())
		return "", false
	}
	src := mapgeo.Source{CS2Exe: h.cs2Path(), Map: name}
	opts := mapgeo.DefaultSceneOptions
	key, err := src.SceneCacheKey(opts)
	if err != nil {
		writeMapGeometryError(w, name, err)
		return "", false
	}
	if h.sceneCached(key) {
		return key, true
	}
	mapGeometryMu.Lock()
	defer mapGeometryMu.Unlock()
	if h.sceneCached(key) {
		return key, true
	}
	scene, err := src.ExtractScene(opts)
	if err != nil {
		writeMapGeometryError(w, name, err)
		return "", false
	}
	reader, writer := io.Pipe()
	go func() {
		writer.CloseWithError(mapgeo.WriteScene(writer, scene))
	}()
	if err := h.storage.Put(key, reader); err != nil {
		_ = reader.CloseWithError(err)
		log.Printf("httpapi: cache map scene %s: %v", key, err)
		writeCodedError(w, http.StatusUnprocessableEntity, codeMapUnreadable, "could not store the 3D scene of map "+name)
		return "", false
	}
	return key, true
}

func (h *Handlers) sceneCached(key string) bool {
	ok, err := h.storage.Exists(key)
	return err == nil && ok
}

// serveCachedScene streams the cached scene file with its length, so the UI can show progress.
func (h *Handlers) serveCachedScene(w http.ResponseWriter, r *http.Request, key string) bool {
	rc, err := h.storage.Open(key)
	if err != nil {
		return false
	}
	defer rc.Close()
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Cache-Control", "no-store")
	if f, ok := rc.(*os.File); ok {
		if info, err := f.Stat(); err == nil {
			w.Header().Set("Content-Length", strconv.FormatInt(info.Size(), 10))
		}
	}
	w.WriteHeader(http.StatusOK)
	if r.Method == http.MethodHead {
		return true
	}
	if _, err := io.Copy(w, rc); err != nil && !errors.Is(err, io.ErrClosedPipe) {
		log.Printf("httpapi: send map scene %s: %v", key, err)
	}
	return true
}
