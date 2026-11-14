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
		{"", "/api/maps/de_cbble/radar", http.StatusNotFound, "cs2_not_found"},
		{"", "/api/equipment/knife_future/icon", http.StatusNotFound, "cs2_not_found"},
		{empty, "/api/maps/de_cbble/radar?layer=lower", http.StatusNotFound, "radar_not_found"},
		{empty, "/api/maps/DE_MIRAGE/shot", http.StatusBadRequest, "invalid_request"},
		{empty, "/api/maps/de_cbble/shot", http.StatusNotFound, "map_picture_not_found"},
		{empty, "/api/teams/logo", http.StatusBadRequest, "invalid_request"},
		{empty, "/api/teams/logo?name=" + strings.Repeat("a", 65), http.StatusBadRequest, "invalid_request"},
		{empty, "/api/teams/logo?name=Equipo%20Inventado", http.StatusNotFound, "team_logo_not_found"},
		{empty, "/api/teams/logo?name=FURIA", http.StatusNotFound, "team_logo_not_found"},
		{empty, "/api/equipment/knife_future/icon", http.StatusNotFound, "icon_not_found"},
		{junk, "/api/maps/de_cbble/radar", http.StatusUnprocessableEntity, "game_asset_unreadable"},
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
	seed := map[string]string{"radar/de_cbble_lower.png": "png-bytes", "icon/knife_future.svg": "<svg/>", "icon/knife_past.png": "icon-png"}
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
		{"/api/maps/de_cbble/radar?layer=lower", "png-bytes", "image/png"},
		{"/api/equipment/knife_future/icon", "<svg/>", "image/svg+xml"},
		{"/api/equipment/knife_past/icon", "icon-png", "image/png"},
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
	if rw := getAsset(h, "/api/maps/de_cbble/radar"); rw.Code != http.StatusNotFound {
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

func TestBundledRadarIsServedWithoutCS2(t *testing.T) {
	h, _ := geometryHandlers("")
	rw := getAsset(h, "/api/maps/de_dust2/radar")
	if rw.Code != http.StatusOK {
		t.Fatalf("bundled radar answered %d, want 200", rw.Code)
	}
	if ct := rw.Header().Get("Content-Type"); ct != "image/png" {
		t.Errorf("content-type = %q, want image/png", ct)
	}
}

// A bundled radar revalidates by its hash, so an updated Studio never reuses the old picture from the cache.
func TestBundledRadarRevalidatesByItsHash(t *testing.T) {
	h, _ := geometryHandlers("")
	first := getAsset(h, "/api/maps/de_dust2/radar")
	tag := first.Header().Get("ETag")
	if tag == "" || first.Header().Get("Cache-Control") != "private, no-cache" {
		t.Fatalf("headers = %v, want an ETag and no-cache", first.Header())
	}
	req := httptest.NewRequest(http.MethodGet, "/api/maps/de_dust2/radar", nil)
	req.Header.Set("If-None-Match", tag)
	again := httptest.NewRecorder()
	Routes(h).ServeHTTP(again, req)
	if again.Code != http.StatusNotModified {
		t.Fatalf("revalidation answered %d, want 304", again.Code)
	}
}

func TestBundledShotIsServedWithoutCS2(t *testing.T) {
	h, _ := geometryHandlers("")
	rw := getAsset(h, "/api/maps/de_dust2/shot")
	if rw.Code != http.StatusOK {
		t.Fatalf("bundled shot answered %d, want 200", rw.Code)
	}
	if ct := rw.Header().Get("Content-Type"); ct != "image/webp" {
		t.Errorf("content-type = %q, want image/webp", ct)
	}
}

func TestBundledIconIsServedWithoutCS2(t *testing.T) {
	h, _ := geometryHandlers("")
	rw := getAsset(h, "/api/equipment/smokegrenade/icon")
	if rw.Code != http.StatusOK || !strings.Contains(rw.Body.String(), "<svg") {
		t.Fatalf("bundled icon answered %d", rw.Code)
	}
	if ct := rw.Header().Get("Content-Type"); ct != "image/svg+xml" {
		t.Errorf("content-type = %q, want image/svg+xml", ct)
	}
	if rw.Header().Get("Content-Security-Policy") != svgPolicy {
		t.Errorf("a bundled SVG must carry the inert policy, got %q", rw.Header().Get("Content-Security-Policy"))
	}
}
