package httpapi

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestServeStudio(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "assets"), 0o755); err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string]string{"index.html": "<main>studio</main>", "assets/app-123.js": "export{}"} {
		if err := os.WriteFile(filepath.Join(dir, filepath.FromSlash(name)), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	h := &Handlers{uiDir: dir}
	tests := []struct {
		name      string
		method    string
		path      string
		wantCode  int
		wantBody  string
		wantCache string
	}{
		{name: "root", method: http.MethodGet, path: "/", wantCode: http.StatusOK, wantBody: "studio", wantCache: "no-store"},
		{name: "deep route", method: http.MethodGet, path: "/matches/abc", wantCode: http.StatusOK, wantBody: "studio", wantCache: "no-store"},
		{name: "immutable asset", method: http.MethodGet, path: "/assets/app-123.js", wantCode: http.StatusOK, wantBody: "export{}", wantCache: "immutable"},
		{name: "missing asset", method: http.MethodGet, path: "/missing.js", wantCode: http.StatusNotFound},
		{name: "api never falls back", method: http.MethodGet, path: "/api/missing", wantCode: http.StatusNotFound},
		{name: "mutation never falls back", method: http.MethodPost, path: "/matches", wantCode: http.StatusNotFound},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rw := httptest.NewRecorder()
			h.ServeStudio(rw, httptest.NewRequest(tc.method, tc.path, nil))
			if rw.Code != tc.wantCode {
				t.Fatalf("status = %d, want %d", rw.Code, tc.wantCode)
			}
			if tc.wantBody != "" && !strings.Contains(rw.Body.String(), tc.wantBody) {
				t.Fatalf("body = %q, want substring %q", rw.Body.String(), tc.wantBody)
			}
			if tc.wantCache != "" && !strings.Contains(rw.Header().Get("Cache-Control"), tc.wantCache) {
				t.Fatalf("cache-control = %q, want substring %q", rw.Header().Get("Cache-Control"), tc.wantCache)
			}
		})
	}
}

func TestStudioCapabilityAuthentication(t *testing.T) {
	const token = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	h := &Handlers{mutationToken: strings.Repeat("b", 64), uiCapability: token}
	tests := []struct {
		name    string
		host    string
		cookies []*http.Cookie
		want    bool
	}{
		{name: "valid", host: "127.0.0.1:8080", cookies: []*http.Cookie{{Name: studioCapabilityCookie, Value: token}}, want: true},
		{name: "wrong host", host: "example.test:8080", cookies: []*http.Cookie{{Name: studioCapabilityCookie, Value: token}}},
		{name: "duplicate", host: "127.0.0.1:8080", cookies: []*http.Cookie{{Name: studioCapabilityCookie, Value: token}, {Name: studioCapabilityCookie, Value: token}}},
		{name: "wrong value", host: "127.0.0.1:8080", cookies: []*http.Cookie{{Name: studioCapabilityCookie, Value: strings.Repeat("c", 64)}}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:8080/api/jobs", nil)
			req.Host = tc.host
			for _, cookie := range tc.cookies {
				req.AddCookie(cookie)
			}
			if got := h.tokenMatches(req); got != tc.want {
				t.Fatalf("tokenMatches() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestBootstrapStudioSession(t *testing.T) {
	const ui = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	const bootstrap = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	h := &Handlers{uiCapability: ui, uiBootstrap: bootstrap}
	req := httptest.NewRequest(http.MethodPost, "http://127.0.0.1:8080/api/session/bootstrap", strings.NewReader("capability="+bootstrap))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Origin", "http://127.0.0.1:8080")
	rw := httptest.NewRecorder()
	h.BootstrapStudioSession(rw, req)
	if rw.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d", rw.Code, http.StatusSeeOther)
	}
	cookies := rw.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Name != studioCapabilityCookie || !cookies[0].HttpOnly || cookies[0].SameSite != http.SameSiteStrictMode {
		t.Fatalf("cookies = %#v, want one strict HttpOnly Studio cookie", cookies)
	}

	second := httptest.NewRecorder()
	h.BootstrapStudioSession(second, req.Clone(req.Context()))
	if second.Code != http.StatusSeeOther || second.Header().Get("Location") != "/bootstrap?error=unavailable" {
		t.Fatalf("second bootstrap = %d %q, want unavailable redirect", second.Code, second.Header().Get("Location"))
	}
}
