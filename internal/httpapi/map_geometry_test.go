package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rechedev9/cliphub/internal/mapgeo"
)

// fakeCS2Install lays out game\bin\win64\cs2.exe and game\csgo\maps\<map>.vpk;
// the package bytes are junk, so only a cache hit can answer 200.
func fakeCS2Install(t *testing.T, maps ...string) string {
	t.Helper()
	root := t.TempDir()
	exe := filepath.Join(root, "game", "bin", "win64", "cs2.exe")
	files := map[string]string{exe: "exe", filepath.Join(root, "game", "csgo", "steam.inf"): "ClientVersion=123\n"}
	for _, m := range maps {
		files[filepath.Join(root, "game", "csgo", "maps", m+".vpk")] = "not a vpk"
	}
	for path, body := range files {
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return exe
}

func geometryHandlers(cs2 string) (*Handlers, *fakeStorage) {
	store := newFakeStorage()
	h := NewHandlers(newFakeRepo(), store, &fakeQueue{}, WithCapabilities(Capabilities{
		RecordTools: []CaptureTool{{Name: "ZV_CS2_PATH", Path: cs2}},
	}))
	return h, store
}

func getGeometry(h *Handlers, name string) *httptest.ResponseRecorder {
	rw := httptest.NewRecorder()
	Routes(h).ServeHTTP(rw, httptest.NewRequest(http.MethodGet, "/api/maps/"+name+"/geometry", nil))
	return rw
}

func TestGetMapGeometryErrors(t *testing.T) {
	exe := fakeCS2Install(t, "de_mirage")
	cases := []struct {
		cs2, name string
		status    int
		code      string
	}{
		{exe, "DE_MIRAGE", http.StatusBadRequest, "invalid_request"},
		{exe, "de-mirage", http.StatusBadRequest, "invalid_request"},
		{exe, "..%2F..%2Fetc", http.StatusBadRequest, "invalid_request"},
		{exe, "de_" + strings.Repeat("a", 33), http.StatusBadRequest, "invalid_request"},
		{"", "de_mirage", http.StatusNotFound, "cs2_not_found"},
		{exe, "de_cache", http.StatusNotFound, "map_not_found"},
		{exe, "de_mirage", http.StatusUnprocessableEntity, "map_geometry_unreadable"},
	}
	for _, tc := range cases {
		h, _ := geometryHandlers(tc.cs2)
		rw := getGeometry(h, tc.name)
		var body struct{ Code string }
		_ = json.Unmarshal(rw.Body.Bytes(), &body)
		if rw.Code != tc.status || body.Code != tc.code {
			t.Errorf("%s (cs2=%q): status %d code %q, want %d %q; body=%s", tc.name, tc.cs2, rw.Code, body.Code, tc.status, tc.code, rw.Body.String())
		}
	}
}

func TestGetMapGeometryServesTheCachedMesh(t *testing.T) {
	exe := fakeCS2Install(t, "de_mirage")
	h, store := geometryHandlers(exe)
	key, err := mapgeo.Source{CS2Exe: exe, Map: "de_mirage"}.CacheKey()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(key, "de_mirage-123-") {
		t.Fatalf("cache key %q does not name the map and build", key)
	}
	cached := `{"map":"de_mirage","source":"nav","build":"123","bounds":{"min":[0,0,0],"max":[1,1,0]},"vertices":[0,0,0,1,0,0,1,1,0],"triangles":[0,1,2],"areas":[{"id":1,"polygon":[0,1,2]}]}`
	if err := store.Put(key, strings.NewReader(cached)); err != nil {
		t.Fatal(err)
	}
	rw := getGeometry(h, "de_mirage")
	if rw.Code != http.StatusOK || rw.Body.String() != cached {
		t.Fatalf("status %d body %s, want the cached mesh", rw.Code, rw.Body.String())
	}
	if ct := rw.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("content type = %q", ct)
	}
}
