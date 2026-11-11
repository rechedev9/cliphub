package httpapi

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rechedev9/cliphub/internal/gameassets"
	"github.com/rechedev9/cliphub/internal/mapgeo"
)

// emptyPak is a version 2 VPK directory with no entries.
func emptyPak() []byte {
	out := make([]byte, 29)
	binary.LittleEndian.PutUint32(out, 0x55aa1234)
	binary.LittleEndian.PutUint32(out[4:], 2)
	binary.LittleEndian.PutUint32(out[8:], 1)
	return out
}

// fakeAssetInstall lays out cs2.exe and game\csgo\pak01_dir.vpk holding pak.
func fakeAssetInstall(t *testing.T, pak []byte) string {
	t.Helper()
	exe := fakeCS2Install(t)
	if err := os.WriteFile(filepath.Join(mapgeo.CSGODir(exe), "pak01_dir.vpk"), pak, 0o600); err != nil {
		t.Fatal(err)
	}
	return exe
}

func getAsset(h *Handlers, path string) *httptest.ResponseRecorder {
	rw := httptest.NewRecorder()
	Routes(h).ServeHTTP(rw, httptest.NewRequest(http.MethodGet, path, nil))
	return rw
}

func TestGameAssetErrors(t *testing.T) {
	empty := fakeAssetInstall(t, emptyPak())
	junk := fakeAssetInstall(t, []byte("not a vpk"))
	cases := []struct {
		cs2, path string
		status    int
		code      string
	}{
		{empty, "/api/maps/DE_MIRAGE/radar", http.StatusBadRequest, "invalid_request"},
		{empty, "/api/maps/..%2Fetc/radar", http.StatusBadRequest, "invalid_request"},
		{empty, "/api/maps/de_mirage/radar?layer=middle", http.StatusBadRequest, "invalid_request"},
		{empty, "/api/equipment/AK47/icon", http.StatusBadRequest, "invalid_request"},
		{empty, "/api/equipment/..%2F..%2Fsecret/icon", http.StatusBadRequest, "invalid_request"},
		{empty, "/api/equipment/knife__x/icon", http.StatusBadRequest, "invalid_request"},
		{empty, "/api/equipment/" + strings.Repeat("a", 41) + "/icon", http.StatusBadRequest, "invalid_request"},
		{"", "/api/maps/de_mirage/radar", http.StatusNotFound, "cs2_not_found"},
		{"", "/api/equipment/ak47/icon", http.StatusNotFound, "cs2_not_found"},
		{empty, "/api/maps/de_mirage/radar?layer=lower", http.StatusNotFound, "radar_not_found"},
		{empty, "/api/maps/DE_MIRAGE/shot", http.StatusBadRequest, "invalid_request"},
		{empty, "/api/maps/de_mirage/shot", http.StatusNotFound, "map_picture_not_found"},
		{empty, "/api/teams/logo", http.StatusBadRequest, "invalid_request"},
		{empty, "/api/teams/logo?name=" + strings.Repeat("a", 65), http.StatusBadRequest, "invalid_request"},
		{empty, "/api/teams/logo?name=Equipo%20Inventado", http.StatusNotFound, "team_logo_not_found"},
		{empty, "/api/teams/logo?name=FURIA", http.StatusNotFound, "team_logo_not_found"},
		{empty, "/api/equipment/ak47/icon", http.StatusNotFound, "icon_not_found"},
		{junk, "/api/maps/de_mirage/radar", http.StatusUnprocessableEntity, "game_asset_unreadable"},
	}
	for _, tc := range cases {
		h, _ := geometryHandlers(tc.cs2)
		rw := getAsset(h, tc.path)
		var body struct{ Code string }
		_ = json.Unmarshal(rw.Body.Bytes(), &body)
		if rw.Code != tc.status || body.Code != tc.code {
			t.Errorf("%s: status %d code %q, want %d %q; body=%s", tc.path, rw.Code, body.Code, tc.status, tc.code, rw.Body.String())
		}
	}
}

func TestGameAssetsServeTheCache(t *testing.T) {
	exe := fakeAssetInstall(t, emptyPak())
	h, store := geometryHandlers(exe)
	install := gameassets.Install{CS2Exe: exe}
	seed := map[string]string{"radar/de_nuke_lower.png": "png-bytes", "icon/ak47.svg": "<svg/>", "icon/c4.png": "icon-png"}
	for file, body := range seed {
		kind, name, _ := strings.Cut(file, "/")
		key, err := install.CacheKey(kind, name)
		if err != nil {
			t.Fatal(err)
		}
		if err := store.Put(key, strings.NewReader(body)); err != nil {
			t.Fatal(err)
		}
	}
	cases := []struct{ path, body, contentType string }{
		{"/api/maps/de_nuke/radar?layer=lower", "png-bytes", "image/png"},
		{"/api/equipment/ak47/icon", "<svg/>", "image/svg+xml"},
		{"/api/equipment/c4/icon", "icon-png", "image/png"},
	}
	for _, tc := range cases {
		rw := getAsset(h, tc.path)
		if rw.Code != http.StatusOK || rw.Body.String() != tc.body || rw.Header().Get("Content-Type") != tc.contentType {
			t.Errorf("%s: %d %q %q", tc.path, rw.Code, rw.Header().Get("Content-Type"), rw.Body.String())
		}
		if rw.Header().Get("X-Content-Type-Options") != "nosniff" {
			t.Errorf("%s: missing nosniff", tc.path)
		}
		if hasPolicy := rw.Header().Get("Content-Security-Policy") != ""; hasPolicy != (tc.contentType == "image/svg+xml") {
			t.Errorf("%s: content security policy %q", tc.path, rw.Header().Get("Content-Security-Policy"))
		}
	}
	if rw := getAsset(h, "/api/maps/de_nuke/radar"); rw.Code != http.StatusNotFound {
		t.Errorf("the upper layer answered %d from the lower layer's cache", rw.Code)
	}
}

func TestServeGameAssetCachesUnderTheMatchingKey(t *testing.T) {
	h, store := geometryHandlers("")
	calls := 0
	extract := func() ([]byte, string, error) {
		calls++
		return []byte("<svg/>"), gameassets.ContentTypeSVG, nil
	}
	keys := []string{"gameassets/v1/b/icon/x.svg", "gameassets/v1/b/icon/x.png"}
	for range 2 {
		rw := httptest.NewRecorder()
		h.serveGameAsset(rw, "x", keys, extract)
		if rw.Code != http.StatusOK || rw.Header().Get("Content-Type") != gameassets.ContentTypeSVG {
			t.Fatalf("status %d type %q", rw.Code, rw.Header().Get("Content-Type"))
		}
	}
	if calls != 1 || string(store.puts[keys[0]]) != "<svg/>" || store.puts[keys[1]] != nil {
		t.Errorf("extract ran %d times; cached %v", calls, store.puts)
	}

	rw := httptest.NewRecorder()
	h.serveGameAsset(rw, "y", []string{"gameassets/v1/b/radar/y.png"}, func() ([]byte, string, error) {
		return nil, "", errors.New("format 99")
	})
	if rw.Code != http.StatusUnprocessableEntity || strings.Contains(rw.Body.String(), "format 99") {
		t.Errorf("an extraction failure answered %d %s", rw.Code, rw.Body.String())
	}
}
