package httpapi

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rechedev9/cliphub/internal/steamgc"
	"github.com/rechedev9/cliphub/internal/steamresolve"
)

func TestResolveShareCode(t *testing.T) {
	// The decode-only default path must not depend on ambient Steam credentials.
	t.Setenv("ZV_STEAM_USERNAME", "")
	t.Setenv("ZV_STEAM_PASSWORD", "")
	t.Setenv("ZV_STEAM_GUARD", "")

	tests := []struct {
		name       string
		body       string
		wantStatus int
		wantCode   string // "code" field of an error response
		want       map[string]any
	}{
		{
			name:       "valid code without transport decodes",
			body:       `{"code":"CSGO-GADqf-jjyJ8-cSP2r-smZRo-TO2xK"}`,
			wantStatus: http.StatusOK,
			want: map[string]any{
				"status":    "decoded",
				"matchId":   "3230642215713767580",
				"outcomeId": "3230647599455273103",
				"tokenId":   float64(55788),
				"demoUrl":   "",
			},
		},
		{
			name:       "malformed code is a 400 with a stable code",
			body:       `{"code":"CSGO-nope"}`,
			wantStatus: http.StatusBadRequest,
			wantCode:   "invalid_share_code",
		},
		{
			name:       "malformed JSON body is a 400",
			body:       `{"code":`,
			wantStatus: http.StatusBadRequest,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := NewHandlers(newFakeRepo(), newFakeStorage(), &fakeQueue{})
			rw := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/api/steam/sharecode", strings.NewReader(tt.body))
			h.ResolveShareCode(rw, req)
			if rw.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d (body: %s)", rw.Code, tt.wantStatus, rw.Body.String())
			}
			if tt.wantCode != "" {
				var got struct {
					Code string `json:"code"`
				}
				if err := json.Unmarshal(rw.Body.Bytes(), &got); err != nil {
					t.Fatalf("decode error body: %v", err)
				}
				if got.Code != tt.wantCode {
					t.Errorf("error code = %q, want %q", got.Code, tt.wantCode)
				}
			}
			if tt.want != nil {
				var got map[string]any
				if err := json.Unmarshal(rw.Body.Bytes(), &got); err != nil {
					t.Fatalf("decode body: %v", err)
				}
				for key, want := range tt.want {
					if got[key] != want {
						t.Errorf("%s = %#v, want %#v", key, got[key], want)
					}
				}
			}
		})
	}
}

// TestResolveShareCodeEmitsIDsAsStrings pins the wire format of the 64-bit
// identifiers: they must round-trip as JSON strings because ~3.2e18 exceeds
// JavaScript's 2^53 integer precision.
func TestResolveShareCodeEmitsIDsAsStrings(t *testing.T) {
	t.Setenv("ZV_STEAM_USERNAME", "")
	t.Setenv("ZV_STEAM_PASSWORD", "")
	t.Setenv("ZV_STEAM_GUARD", "")

	h := NewHandlers(newFakeRepo(), newFakeStorage(), &fakeQueue{})
	rw := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/steam/sharecode",
		strings.NewReader(`{"code":"CSGO-GADqf-jjyJ8-cSP2r-smZRo-TO2xK"}`))
	h.ResolveShareCode(rw, req)
	if rw.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", rw.Code, rw.Body.String())
	}
	var got struct {
		MatchID   json.RawMessage `json:"matchId"`
		OutcomeID json.RawMessage `json:"outcomeId"`
	}
	if err := json.Unmarshal(rw.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if string(got.MatchID) != `"3230642215713767580"` {
		t.Errorf("matchId raw JSON = %s, want %q", got.MatchID, "3230642215713767580")
	}
	if string(got.OutcomeID) != `"3230647599455273103"` {
		t.Errorf("outcomeId raw JSON = %s, want %q", got.OutcomeID, "3230647599455273103")
	}
}

// TestCapabilitiesReportSteamBlock covers the new steam capability block
// without touching the existing capabilities assertions.
func TestCapabilitiesReportSteamBlock(t *testing.T) {
	tests := []struct {
		name                      string
		username, password, guard string
		want                      bool
	}{
		{name: "unconfigured", want: false},
		{name: "configured", username: "u", password: "p", guard: "g", want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("ZV_STEAM_USERNAME", tt.username)
			t.Setenv("ZV_STEAM_PASSWORD", tt.password)
			t.Setenv("ZV_STEAM_GUARD", tt.guard)
			h := NewHandlers(newFakeRepo(), newFakeStorage(), &fakeQueue{})
			rw := httptest.NewRecorder()
			h.GetCapabilities(rw, httptest.NewRequest(http.MethodGet, "/api/capabilities", nil))
			if rw.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200", rw.Code)
			}
			var got struct {
				Steam struct {
					Enabled bool `json:"enabled"`
				} `json:"steam"`
			}
			if err := json.Unmarshal(rw.Body.Bytes(), &got); err != nil {
				t.Fatalf("decode capabilities: %v", err)
			}
			if got.Steam.Enabled != tt.want {
				t.Errorf("steam.enabled = %v, want %v", got.Steam.Enabled, tt.want)
			}
		})
	}
}

type stubGC struct {
	matches []steamgc.Match
	err     error
}

func (s stubGC) RequestMatch(context.Context, steamgc.Request) ([]steamgc.Match, error) {
	return s.matches, s.err
}

func TestSteamAccountRoundTrip(t *testing.T) {
	store, err := steamresolve.NewAccountStore(filepath.Join(t.TempDir(), "account.json"), time.Now)
	if err != nil {
		t.Fatal(err)
	}
	h := NewHandlers(newFakeRepo(), newFakeStorage(), &fakeQueue{}, WithSteamAccount(store, nil, nil))

	put := httptest.NewRecorder()
	h.PutSteamAccount(put, httptest.NewRequest(http.MethodPut, "/api/steam/account", strings.NewReader(
		`{"steamId":"76561198000000001","authCode":"AAAAA-BBBBB-CCCCC","apiKey":"0123456789ABCDEF","knownCode":"CSGO-GADqf-jjyJ8-cSP2r-smZRo-TO2xK"}`,
	)))
	if put.Code != http.StatusOK {
		t.Fatalf("PUT status = %d body=%s", put.Code, put.Body.String())
	}
	var afterPut struct {
		SteamID           string `json:"steamId"`
		HistoryConfigured bool   `json:"historyConfigured"`
		AuthCodeSet       bool   `json:"authCodeSet"`
		APIKeySet         bool   `json:"apiKeySet"`
		KnownCode         string `json:"knownCode"`
	}
	if err := json.Unmarshal(put.Body.Bytes(), &afterPut); err != nil {
		t.Fatal(err)
	}
	if !afterPut.HistoryConfigured || afterPut.SteamID != "76561198000000001" || !afterPut.AuthCodeSet || !afterPut.APIKeySet {
		t.Errorf("PUT payload = %+v", afterPut)
	}
	if afterPut.KnownCode == "" {
		t.Error("known code missing")
	}

	got := httptest.NewRecorder()
	h.GetSteamAccount(got, httptest.NewRequest(http.MethodGet, "/api/steam/account", nil))
	if got.Code != http.StatusOK {
		t.Fatalf("GET status = %d", got.Code)
	}

	del := httptest.NewRecorder()
	h.DeleteSteamAccount(del, httptest.NewRequest(http.MethodDelete, "/api/steam/account", nil))
	if del.Code != http.StatusOK {
		t.Fatalf("DELETE status = %d", del.Code)
	}
	var afterDel struct {
		HistoryConfigured bool `json:"historyConfigured"`
	}
	if err := json.Unmarshal(del.Body.Bytes(), &afterDel); err != nil {
		t.Fatal(err)
	}
	if afterDel.HistoryConfigured {
		t.Fatal("account still configured after delete")
	}
}

func TestImportShareCode(t *testing.T) {
	demoURL := ""
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(demoMagic)
		_, _ = io.WriteString(w, "payload")
	}))
	t.Cleanup(server.Close)

	tests := []struct {
		name       string
		body       string
		transport  steamresolve.Transport
		fetcher    *steamresolve.Fetcher
		wantStatus int
		wantCode   string
	}{
		{
			name:       "missing credentials",
			body:       `{"code":"CSGO-GADqf-jjyJ8-cSP2r-smZRo-TO2xK"}`,
			wantStatus: http.StatusConflict,
			wantCode:   steamCredentialsRequired,
		},
		{
			name: "resolved demo is queued",
			body: `{"code":"CSGO-GADqf-jjyJ8-cSP2r-smZRo-TO2xK","username":"u","password":"p","guard":"g"}`,
			transport: stubGC{matches: []steamgc.Match{{
				MatchID: 3230642215713767580,
				DemoURL: "http://replay1.valve.net/730/demo.dem",
			}}},
			wantStatus: http.StatusCreated,
		},
		{
			name:       "invalid code",
			body:       `{"code":"CSGO-nope","username":"u","password":"p","guard":"g"}`,
			transport:  stubGC{},
			wantStatus: http.StatusBadRequest,
			wantCode:   "invalid_share_code",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("ZV_STEAM_USERNAME", "")
			t.Setenv("ZV_STEAM_PASSWORD", "")
			t.Setenv("ZV_STEAM_GUARD", "")
			store, err := steamresolve.NewAccountStore(filepath.Join(t.TempDir(), "account.json"), time.Now)
			if err != nil {
				t.Fatal(err)
			}
			opts := []Option{WithSteamAccount(store, nil, steamresolve.NewFetcher(redirectingValveClient(t, server.URL)))}
			if tt.transport != nil {
				opts = append(opts, WithSteamTransport(tt.transport))
			}
			h := NewHandlers(newFakeRepo(), newFakeStorage(), &fakeQueue{}, opts...)
			rw := httptest.NewRecorder()
			h.ImportShareCode(rw, httptest.NewRequest(http.MethodPost, "/api/steam/import", strings.NewReader(tt.body)))
			if rw.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d (body %s)", rw.Code, tt.wantStatus, rw.Body.String())
			}
			if tt.wantCode != "" {
				var got struct {
					Code string `json:"code"`
				}
				if err := json.Unmarshal(rw.Body.Bytes(), &got); err != nil {
					t.Fatal(err)
				}
				if got.Code != tt.wantCode {
					t.Errorf("code = %q, want %q", got.Code, tt.wantCode)
				}
			}
			if tt.wantStatus == http.StatusCreated {
				var got struct {
					ID     string `json:"id"`
					Status string `json:"status"`
				}
				if err := json.Unmarshal(rw.Body.Bytes(), &got); err != nil {
					t.Fatal(err)
				}
				if got.ID == "" || got.Status == "" {
					t.Errorf("created payload = %+v", got)
				}
			}
		})
	}
	_ = demoURL
}

func redirectingValveClient(t *testing.T, serverURL string) *http.Client {
	t.Helper()
	base, err := url.Parse(serverURL)
	if err != nil {
		t.Fatal(err)
	}
	return &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			clone := req.Clone(req.Context())
			clone.URL.Scheme = base.Scheme
			clone.URL.Host = base.Host
			clone.Host = base.Host
			return http.DefaultTransport.RoundTrip(clone)
		}),
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
